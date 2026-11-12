package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"key-short/internal/autostart"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

func TestMakeInstallCLIAndUninstallInIsolatedHome(t *testing.T) {
	if testing.Short() {
		t.Skip("make install integration")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	tools := filepath.Join(home, "tools")
	if err := os.Mkdir(tools, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "systemctl"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	// Preserve the build cache/module cache while isolating every installed file.
	env := append([]string(nil), os.Environ()...)
	for _, key := range []string{"GOCACHE", "GOPATH"} {
		out, err := exec.Command("go", "env", key).Output()
		if err != nil {
			t.Fatal(err)
		}
		env = append(env, key+"="+strings.TrimSpace(string(out)))
	}
	config := filepath.Join(home, "config")
	data := filepath.Join(home, "data")
	state := filepath.Join(home, "state")
	env = append(env, "HOME="+home, "XDG_CONFIG_HOME="+config, "XDG_DATA_HOME="+data, "XDG_STATE_HOME="+state, "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "LC_ALL=en_US.UTF-8")
	run := func(extra []string, name string, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = root
		cmd.Env = append(append([]string(nil), env...), extra...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return out
	}
	p := profile.New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "true"}})
	path := filepath.Join(home, "my launcher.json")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	run(nil, "make", "install-xdg", "PROFILE="+path)
	binary := filepath.Join(home, ".local", "bin", "key-short")
	entry := filepath.Join(config, "autostart", "key-short.desktop")
	if _, err := os.Stat(entry); err != nil {
		t.Fatal("make did not register startup", err)
	}
	// Use Output separately so Fyne locale diagnostics on stderr cannot corrupt JSON.
	cmd := exec.Command(binary, "-autostart", "status")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var status autostart.Status
	if err = json.Unmarshal(out, &status); err != nil || !status.Enabled || status.Backend != "xdg" {
		t.Fatal("invalid CLI status", string(out), err)
	}
	run(nil, binary, "-autostart", "disable")
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Fatal("CLI disable retained startup entry")
	}
	run(nil, binary, "-autostart", "enable", "-autostart-backend", "xdg", "-profile", path)
	history := filepath.Join(state, "key-short", "history.json")
	if err := os.MkdirAll(filepath.Dir(history), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(history, []byte("user history"), 0600); err != nil {
		t.Fatal(err)
	}
	// Uninstall builds only the pure-Go helper, including when CGO is disabled.
	run([]string{"CGO_ENABLED=0"}, "make", "uninstall")
	for _, file := range []string{binary, entry, filepath.Join(config, "key-short", "autostart.json"), filepath.Join(data, "applications", "key-short.desktop"), filepath.Join(data, "icons", "hicolor", "scalable", "apps", "key-short.svg")} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatal("uninstall retained file", file, err)
		}
	}
	for _, file := range []string{path, history} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal("uninstall deleted user data", file, err)
		}
	}
}
