package launcher

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"key-short/internal/protocol"
)

// Launches only a disposable helper, never the user's installed applications.
func TestNativeLaunchAndPersistentFocus(t *testing.T) {
	if os.Getenv("KEY_SHORT_TEST_DESKTOP") != "1" {
		t.Skip("requires a running desktop session")
	}
	dir := t.TempDir()
	source := `package main
import("fyne.io/fyne/v2";"fyne.io/fyne/v2/app";"fyne.io/fyne/v2/widget")
func main(){a:=app.New();w:=a.NewWindow("Key Short process tracking verification");w.SetContent(widget.NewLabel("Process tracking verification"));w.Resize(fyne.NewSize(400,120));w.ShowAndRun()}`
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(dir, "window helper")
	build := exec.Command("go", "build", "-o", helper, path)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("helper build: %s %v", out, err)
	}
	for _, backend := range []string{"kwin", "x11"} {
		t.Run(backend, func(t *testing.T) {
			t.Setenv("KEY_SHORT_WINDOW_BACKEND", backend)
			stateDir := t.TempDir()
			m, err := New(stateDir)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			spec := protocol.LaunchSpec{Mode: "command", Command: strconv.Quote(helper), ReuseWindow: true}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			result, err := m.Run(ctx, spec)
			if err != nil || !result.Started {
				t.Fatalf("start: %+v %v", result, err)
			}
			var r record
			data, err := os.ReadFile(filepath.Join(stateDir, signature(spec)+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if p, err := os.FindProcess(r.Root.PID); err == nil {
					p.Kill()
				}
			}()
			if len(r.Windows) != 1 {
				t.Fatalf("no tracked window: %+v", r)
			}
			second, err := New(stateDir)
			if err != nil {
				t.Fatal(err)
			}
			defer second.Close()
			result, err = second.Run(ctx, spec)
			if err != nil || !result.Focused || result.Started {
				t.Fatalf("persistent focus: %+v %v", result, err)
			}
			if !alive(r.Root) {
				t.Fatal("tracked process unexpectedly exited")
			}
		})
	}
}
