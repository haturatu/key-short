package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/profile"
)

func visitWidgets(root fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	seen := map[fyne.CanvasObject]bool{}
	var inspect func(fyne.CanvasObject)
	inspect = func(o fyne.CanvasObject) {
		if seen[o] {
			return
		}
		seen[o] = true
		visit(o)
		if c, ok := o.(*fyne.Container); ok {
			for _, child := range c.Objects {
				inspect(child)
			}
		}
		if w, ok := o.(fyne.Widget); ok {
			for _, child := range test.WidgetRenderer(w).Objects() {
				inspect(child)
			}
		}
	}
	inspect(root)
}

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
	editor := s.w.Canvas().Overlays().Top()
	var kind *widget.Select
	var choose, add *widget.Button
	visitWidgets(editor, func(o fyne.CanvasObject) {
		switch w := o.(type) {
		case *widget.Select:
			if len(w.Options) == 4 && w.Options[3] == "ランチャー" {
				kind = w
			}
		case *widget.Button:
			if w.Text == "アプリを選択…" {
				choose = w
			}
			if w.Text == "設定に追加" {
				add = w
			}
		}
	})
	if kind == nil || choose == nil || add == nil {
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
	var kind, mode *widget.Select
	var command *widget.Entry
	var reuse *widget.Check
	var add *widget.Button
	visitWidgets(s.w.Canvas().Overlays().Top(), func(o fyne.CanvasObject) {
		switch w := o.(type) {
		case *widget.Select:
			if len(w.Options) == 4 && w.Options[3] == "ランチャー" {
				kind = w
			}
			if len(w.Options) == 2 && w.Options[1] == "コマンドラインで実行" {
				mode = w
			}
		case *widget.Entry:
			if strings.HasPrefix(w.PlaceHolder, "chromium ") {
				command = w
			}
		case *widget.Check:
			if strings.HasPrefix(w.Text, "起動済みなら") {
				reuse = w
			}
		case *widget.Button:
			if w.Text == "設定に追加" {
				add = w
			}
		}
	})
	if kind == nil || mode == nil || command == nil || reuse == nil || add == nil {
		t.Fatal("launcher controls missing")
	}
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
