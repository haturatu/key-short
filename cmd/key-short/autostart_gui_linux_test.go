package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/autostart"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

func TestGUIAndCLIShareAutostartConfiguration(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	fakebin := filepath.Join(root, "tools")
	if err := os.MkdirAll(fakebin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakebin, "systemctl"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	m, err := autostart.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(m.Binary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.Binary, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	p := profile.New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "true"}})
	path := filepath.Join(root, "launcher.json")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.installProfile(p, path)
	// Inspect the actual shared settings panel without scrolling the key grid.
	s.w.SetContent(container.NewPadded(s.autostartControls()))
	s.w.Resize(fyne.NewSize(760, 650))
	s.w.Show()
	s.autostartBackend.SetSelectedIndex(2)
	var enable, disable *widget.Button
	visitWidgets(s.w.Content(), func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok {
			switch b.Text {
			case "この JSON で自動起動を有効にする":
				enable = b
			case "自動起動を無効にする":
				disable = b
			}
		}
	})
	if enable == nil || disable == nil {
		t.Fatal("autostart controls missing")
	}
	test.Tap(enable)
	s.launchWG.Wait()
	status, err := m.Status(context.Background())
	if err != nil || !status.Enabled || status.Backend != "xdg" || status.Profile != path {
		t.Fatal("GUI setup not readable from CLI manager", status, err)
	}
	screenshot(t, s.w, "autostart-settings.png")
	test.Tap(disable)
	s.launchWG.Wait()
	status, err = m.Status(context.Background())
	if err != nil || status.Enabled {
		t.Fatal("GUI disable failed", err)
	}
	// Simulate a CLI change and verify that the GUI's status refresh sees it.
	if err = m.Apply(context.Background(), "enable", path, "xdg"); err != nil {
		t.Fatal(err)
	}
	s.manageAutostart("status")
	s.launchWG.Wait()
	if !strings.Contains(s.autostartStatus.Text, "自動起動: 有効") {
		t.Fatal("GUI did not see CLI setup", s.autostartStatus.Text)
	}
}
func TestGUIRequiresSavedProfileBeforeEnablingAutostart(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.editor.value.SetText("ctrl+pageup")
	s.manageAutostart("enable")
	if s.autostartBusy || !s.hasDraft() || s.w.Canvas().Overlays().Top() == nil {
		t.Fatal("unsaved profile registered or draft lost")
	}
}
