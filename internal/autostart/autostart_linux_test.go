package autostart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"key-short/internal/daemon"
	"key-short/internal/filelock"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

func fixture(t *testing.T) (*Manager, string) {
	t.Helper()
	root := t.TempDir()
	m := &Manager{Home: root, ConfigHome: filepath.Join(root, "config"), DataHome: filepath.Join(root, "data"), StateDir: filepath.Join(root, "state"), Binary: filepath.Join(root, "bin", "key-short")}
	m.Systemctl = func(context.Context, ...string) error { return errors.New("no systemd") }
	if err := writeFile(m.Binary, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	p := profile.New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "true"}})
	path := filepath.Join(root, "profile with spaces.json")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	return m, path
}
func TestAutoFallbackXDGAndUninstallPreservesUserData(t *testing.T) {
	m, path := fixture(t)
	ctx := context.Background()
	if err := m.Enable(ctx, path, "auto"); err != nil {
		t.Fatal(err)
	}
	status, err := m.Status(ctx)
	if err != nil || !status.Enabled || status.Backend != "xdg" || status.Profile != path {
		t.Fatal(status, err)
	}
	data, err := os.ReadFile(m.desktopPath())
	if err != nil || !strings.Contains(string(data), "-daemon-logs") {
		t.Fatal("XDG entry missing", err)
	}
	// User state/profiles remain while every app-owned startup file is removed.
	if err := os.MkdirAll(m.StateDir, 0700); err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(m.StateDir, "history.json")
	if err := os.WriteFile(history, []byte("history"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{m.Binary, m.desktopPath(), m.configPath()} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatal("not removed", file, err)
		}
	}
	for _, file := range []string{path, history} {
		if _, err := os.Stat(file); err != nil {
			t.Fatal("user data deleted", file)
		}
	}
	if err := m.Uninstall(ctx); err != nil {
		t.Fatal("uninstall not idempotent", err)
	}
}
func TestSystemdAutoChoiceAndSafeBackendSwitch(t *testing.T) {
	m, path := fixture(t)
	ctx := context.Background()
	var commands []string
	m.Systemctl = func(_ context.Context, args ...string) error {
		commands = append(commands, strings.Join(args, " "))
		return nil
	}
	if err := m.Enable(ctx, path, "auto"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(m.unitPath())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"PartOf=graphical-session.target", "WantedBy=graphical-session.target", "KillMode=process", " -daemon ", " -daemon-state "} {
		if !strings.Contains(text, want) {
			t.Fatal("missing unit directive", want)
		}
	}
	if !strings.Contains(strings.Join(commands, "\n"), "enable key-short.service") {
		t.Fatal("unit not enabled", commands)
	}
	if strings.Contains(strings.Join(commands, "\n"), "start key-short.service") {
		t.Fatal("registration started monitoring unexpectedly")
	}
	if err := m.Enable(ctx, path, "xdg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.unitPath()); !os.IsNotExist(err) {
		t.Fatal("old unit retained")
	}
	if _, err := os.Stat(m.desktopPath()); err != nil {
		t.Fatal("new backend missing")
	}
	if !strings.Contains(strings.Join(commands, "\n"), "stop key-short.service") {
		t.Fatal("old service not stopped")
	}
}
func TestInvalidProfileDoesNotReplaceWorkingRegistration(t *testing.T) {
	m, path := fixture(t)
	ctx := context.Background()
	if err := m.Enable(ctx, path, "xdg"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(m.desktopPath())
	configBefore, _ := os.ReadFile(m.configPath())
	bad := filepath.Join(m.Home, "bad.json")
	if err := os.WriteFile(bad, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if m.Enable(ctx, bad, "xdg") == nil {
		t.Fatal("invalid profile accepted")
	}
	after, _ := os.ReadFile(m.desktopPath())
	configAfter, _ := os.ReadFile(m.configPath())
	if string(before) != string(after) || string(configBefore) != string(configAfter) {
		t.Fatal("invalid input replaced registration")
	}
	if m.Enable(ctx, path, "systemd") == nil {
		t.Fatal("missing systemd accepted")
	}
}
func TestUninstallStopsOnlyManagedListener(t *testing.T) {
	m, path := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := m.Enable(ctx, path, "xdg"); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	s, err := daemon.Serve(m.StateDir, func() { close(stopped) })
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { <-stopped; s.Close(); close(done) }()
	if err := m.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	<-done
	if running, err := daemon.Running(ctx, m.StateDir); err != nil || running {
		t.Fatal("listener survived uninstall", err)
	}
}
func TestStartRefusesAnExistingGUIListener(t *testing.T) {
	m, path := fixture(t)
	ctx := context.Background()
	if err := m.Enable(ctx, path, "xdg"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(m.StateDir, 0700); err != nil {
		t.Fatal(err)
	}
	lock, ok, err := filelock.Acquire(filepath.Join(m.StateDir, "listener.lock"))
	if err != nil || !ok {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := m.Start(ctx); err == nil || !strings.Contains(err.Error(), "GUI") {
		t.Fatal("GUI listener not protected", err)
	}
}
func TestFailedSystemdEnableIsRolledBack(t *testing.T) {
	m, path := fixture(t)
	ctx := context.Background()
	m.Systemctl = func(_ context.Context, args ...string) error {
		if len(args) > 0 && args[0] == "enable" {
			return errors.New("enable failed")
		}
		return nil
	}
	if m.Enable(ctx, path, "systemd") == nil {
		t.Fatal("failed enable returned success")
	}
	if _, err := os.Stat(m.unitPath()); !os.IsNotExist(err) {
		t.Fatal("failed unit not cleaned up")
	}
	status, err := m.Status(ctx)
	if err != nil || status.Enabled {
		t.Fatal("failed registration marked enabled", err)
	}
}
func TestEscapingForSystemdAndDesktopEntries(t *testing.T) {
	value := "/tmp/profile $HOME %f `x` \\\".json"
	unit := unitArg(value)
	if !strings.Contains(unit, "$$HOME") || !strings.Contains(unit, "%%f") {
		t.Fatal("unit expansions not escaped", unit)
	}
	desktop := desktopArg(value)
	if !strings.Contains(desktop, "%%f") || !strings.Contains(desktop, `\\$HOME`) || !strings.Contains(desktop, `\\\\`) {
		t.Fatal("desktop expansions not escaped", desktop)
	}
}

func TestRealXDGStartRestartAndDisable(t *testing.T) {
	m, path := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nexec " + quote(self) + " -test.run=^TestAutostartDaemonHelper$ -- \"$@\"\n"
	if err := os.WriteFile(m.Binary, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KEY_SHORT_AUTOSTART_HELPER", "1")
	if err := m.Enable(ctx, path, "xdg"); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(context.Background())
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := m.Status(ctx)
	if err != nil || !status.Running {
		t.Fatal("child did not start", err)
	}
	if err := m.Enable(ctx, path, "xdg"); err != nil {
		t.Fatal("active reconfiguration", err)
	}
	if st, err := m.Status(ctx); err != nil || !st.Running {
		t.Fatal("reconfiguration stopped running listener", err)
	}
	if err := m.Restart(ctx); err != nil {
		t.Fatal("restart", err)
	}
	if err := m.Disable(ctx); err != nil {
		t.Fatal("disable", err)
	}
	status, err = m.Status(ctx)
	if err != nil || status.Running || status.Enabled {
		t.Fatal("child survived disable", status, err)
	}
	for _, name := range []string{"daemon.stdout.log", "daemon.stderr.log"} {
		if _, err := os.Stat(filepath.Join(m.StateDir, name)); err != nil {
			t.Fatal("missing split log", err)
		}
	}
}
func TestAutostartDaemonHelper(t *testing.T) {
	if os.Getenv("KEY_SHORT_AUTOSTART_HELPER") != "1" {
		return
	}
	dir := ""
	for i, arg := range os.Args {
		if arg == "-daemon-state" && i+1 < len(os.Args) {
			dir = os.Args[i+1]
		}
	}
	if dir == "" {
		os.Exit(2)
	}
	fmt.Fprintln(os.Stdout, "INFO helper ready")
	fmt.Fprintln(os.Stderr, "WARN helper diagnostic")
	ctx, cancel := context.WithCancel(context.Background())
	s, err := daemon.Serve(dir, cancel)
	if err != nil {
		os.Exit(3)
	}
	<-ctx.Done()
	s.Close()
	os.Exit(0)
}
func TestGeneratedStartupFilesWithNativeValidators(t *testing.T) {
	m, path := fixture(t)
	m.Binary = filepath.Join(m.Home, `bin $HOME %f \ " quoted`, "key-short")
	if err := writeFile(m.Binary, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	c := Config{Enabled: true, Backend: "systemd", Profile: path, StateDir: m.StateDir}
	if err := writeFile(m.unitPath(), m.unit(c), 0600); err != nil {
		t.Fatal(err)
	}
	if tool, err := exec.LookPath("systemd-analyze"); err == nil {
		cmd := exec.Command(tool, "--user", "verify", "--man=no", m.unitPath())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("invalid systemd unit: %v\n%s", err, out)
		}
	} else {
		t.Log("systemd-analyze unavailable; CI validates the unit")
	}
	if err := writeFile(m.desktopPath(), m.desktop(c), 0600); err != nil {
		t.Fatal(err)
	}
	if tool, err := exec.LookPath("desktop-file-validate"); err == nil {
		if out, err := exec.Command(tool, m.desktopPath()).CombinedOutput(); err != nil {
			t.Fatalf("invalid desktop entry: %v\n%s", err, out)
		}
	} else {
		t.Log("desktop-file-validate unavailable; CI validates the entry")
	}
}

func TestAutoUsesXDGWithoutAnActiveGraphicalTarget(t *testing.T) {
	m, path := fixture(t)
	m.Systemctl = func(_ context.Context, args ...string) error {
		if args[0] == "is-active" {
			return errors.New("inactive")
		}
		return nil
	}
	if err := m.Enable(context.Background(), path, "auto"); err != nil {
		t.Fatal(err)
	}
	st, err := m.Status(context.Background())
	if err != nil || st.Backend != "xdg" {
		t.Fatal(st, err)
	}
}
func TestFailedStopDoesNotUninstallTheApplication(t *testing.T) {
	m, path := fixture(t)
	ctx := context.Background()
	m.Systemctl = func(context.Context, ...string) error { return nil }
	if err := m.Enable(ctx, path, "systemd"); err != nil {
		t.Fatal(err)
	}
	m.Systemctl = func(_ context.Context, args ...string) error {
		if args[0] == "stop" {
			return errors.New("stop failed")
		}
		return nil
	}
	if m.Uninstall(ctx) == nil {
		t.Fatal("failed stop reported success")
	}
	if _, err := os.Stat(m.Binary); err != nil {
		t.Fatal("removed binary while listener may run")
	}
}
func TestProfileAliasCannotOverwriteStartupMetadata(t *testing.T) {
	m, path := fixture(t)
	p, err := profile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Save(m.configPath()); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(m.Home, "alias.json")
	if err = os.Symlink(m.configPath(), alias); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(m.configPath())
	if m.Enable(context.Background(), alias, "xdg") == nil {
		t.Fatal("metadata alias accepted")
	}
	after, _ := os.ReadFile(m.configPath())
	if string(before) != string(after) {
		t.Fatal("overwrote user profile through alias")
	}
}

func TestFixedExecShimPassesArgumentsWithoutExpansion(t *testing.T) {
	m, _ := fixture(t)
	executable := filepath.Join(m.Home, `binary $HOME %f \ " = literal`)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\"\n"
	if err := writeFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	value := `path $HOME $(touch should-not-exist) %f \ " = literal`
	out, err := exec.Command("/bin/sh", "-c", `exec "$@"`, "key-short", executable, "-daemon", value).Output()
	if err != nil || string(out) != "-daemon\n"+value+"\n" {
		t.Fatal("shim expanded or lost arguments", string(out), err)
	}
}
