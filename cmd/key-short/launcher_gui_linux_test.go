package main

import (
	"os"
	"path/filepath"

	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/profile"
)

func TestInstalledApplicationPicker(t *testing.T) {
	root := t.TempDir()
	apps := filepath.Join(root, "applications")
	if err := os.MkdirAll(apps, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(apps, "key-short-picker-fixture.desktop")
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nType=Application\nName=KeyShortPickerFixture\nExec=/bin/true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("XDG_DATA_DIRS", root)
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	s.edit(1)
	kind, add := s.editor.kind, s.editor.apply
	var choose *widget.Button
	visitWidgets(s.editor.box, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "アプリを選択…" {
			choose = b
		}
	})
	if choose == nil {
		t.Fatal("picker controls missing")
	}
	kind.SetSelected("ランチャー")
	test.Tap(choose)
	var search *widget.Entry
	var list *widget.List
	visitWidgets(s.w.Canvas().Overlays().Top(), func(o fyne.CanvasObject) {
		switch w := o.(type) {
		case *widget.Entry:
			if w.PlaceHolder == "アプリ名で検索" {
				search = w
			}
		case *widget.List:
			list = w
		}
	})
	if search == nil || list == nil {
		t.Fatal("application search missing")
	}
	search.SetText("KeyShortPickerFixture")
	if list.Length() != 1 {
		t.Fatalf("search: %d results", list.Length())
	}
	list.Select(0)
	test.Tap(add)
	b, ok := s.p.Get(1, 1)
	if !ok || b.Launch == nil || b.Launch.Mode != "application" || b.Launch.DesktopFile != path || b.Launch.Command != "" {
		t.Fatalf("lost application selection: %+v", b)
	}
}

func TestLauncherEditorSavesQuotedCommandAndReuseOption(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	test.Tap(s.buttons[1])
	kind, mode := s.editor.kind, s.editor.launch.mode
	command, reuse, add := s.editor.launch.command, s.editor.launch.reuse, s.editor.apply
	kind.SetSelected("ランチャー")
	mode.SetSelected("コマンドラインで実行")
	line := `chromium --remote-debugging-port=9222 --user-data-dir="/tmp/debug profile"`
	command.SetText(line)
	reuse.SetChecked(true)
	screenshot(t, s.w, "launcher-editor.png")
	test.Tap(add)
	b, ok := s.p.Get(1, 2)
	if !ok || b.Kind != "launcher" || b.Launch.Command != line || !b.Launch.ReuseWindow {
		t.Fatalf("lost launcher configuration: %+v", b)
	}
	path := filepath.Join(t.TempDir(), "launcher.json")
	if err := s.p.Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := profile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = p.Get(1, 2)
	if b.Launch == nil || b.Launch.Command != line || !b.Launch.ReuseWindow {
		t.Fatal("lost launcher metadata on disk")
	}
}
