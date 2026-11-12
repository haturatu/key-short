package window

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in desktop integration: only the test's own child window is activated.
func TestDesktopBackends(t *testing.T) {
	if os.Getenv("KEY_SHORT_TEST_DESKTOP") != "1" {
		t.Skip("requires a running desktop session")
	}
	dir := t.TempDir()
	source := `package main
import("fyne.io/fyne/v2";"fyne.io/fyne/v2/app";"fyne.io/fyne/v2/widget")
func main(){a:=app.NewWithID("io.keyshort.verification");w:=a.NewWindow("Key Short launcher verification");w.SetContent(widget.NewLabel("Launcher verification window"));w.Resize(fyne.NewSize(400,120));w.ShowAndRun()}`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(dir, "window-helper")
	build := exec.Command("go", "build", "-o", helper, filepath.Join(dir, "main.go"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("helper build: %s %v", output, err)
	}
	cmd := exec.Command(helper)
	logPath := filepath.Join(dir, "helper.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	factories := map[string]func() (Backend, error){"kwin": func() (Backend, error) { return newKWin() }, "x11": func() (Backend, error) { return newX11() }}
	for name, factory := range factories {
		t.Run(name, func(t *testing.T) {
			b, err := factory()
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var found Window
			for ctx.Err() == nil {
				windows, err := b.List(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for _, w := range windows {
					if w.PID == cmd.Process.Pid {
						found = w
						break
					}
				}
				if found.ID != "" {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if found.ID == "" {
				output, _ := os.ReadFile(logPath)
				t.Fatalf("helper window not discovered: %s", output)
			}
			if err = b.Activate(ctx, found.ID); err != nil {
				t.Fatal(err)
			}
			for ctx.Err() == nil {
				windows, err := b.List(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for _, w := range windows {
					if w.ID == found.ID && w.Active {
						t.Log(fmt.Sprintf("activated child window through %s", name))
						return
					}
				}
				time.Sleep(50 * time.Millisecond)
			}
			t.Fatal("window did not become active")
		})
	}
}
