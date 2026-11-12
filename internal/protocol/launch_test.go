package protocol

import (
	"bytes"
	"reflect"
	"runtime"
	"testing"
)

func TestCommandLinePreservesArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command line rules")
	}
	args, err := CommandArgs(`chromium --remote-allow-origins=* --remote-debugging-port=9222 --user-data-dir="/tmp/debug profile"`)
	want := []string{"chromium", "--remote-allow-origins=*", "--remote-debugging-port=9222", "--user-data-dir=/tmp/debug profile"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("%#v %v", args, err)
	}
	args, err = CommandArgs(`printf '%s' '$HOME' '*.txt'`)
	if err != nil || args[2] != "$HOME" || args[3] != "*.txt" {
		t.Fatalf("expanded command: %#v %v", args, err)
	}
	for _, s := range []string{"", `chromium "broken`, `echo hello; another`, `echo a | another`} {
		if _, err := CommandArgs(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestLauncherOnlyProgramsTrigger(t *testing.T) {
	spec := &LaunchSpec{Mode: "command", Command: "chromium --remote-debugging-port=9222", ReuseWindow: true}
	b := Binding{Layer: 2, Slot: 18, Kind: "launcher", Launch: spec}
	frames, err := b.Frames()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := (Binding{Layer: 2, Slot: 18, Kind: "keyboard", Value: Trigger(2, 18)}).Frames()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(frames, expected) {
		t.Fatal("launcher wire format is not a keyboard chord")
	}
	for _, f := range frames {
		if bytes.Contains(f[:], []byte("chromium")) {
			t.Fatal("command leaked into device reports")
		}
	}
	for _, invalid := range []LaunchSpec{{Mode: "other"}, {Mode: "application", DesktopFile: "relative.desktop"}, {Mode: "command", Command: "x", WorkingDir: "relative"}, {Mode: "command", Command: "x", DesktopFile: "/tmp/x.desktop"}} {
		if invalid.Validate() == nil {
			t.Errorf("accepted %+v", invalid)
		}
	}
}
