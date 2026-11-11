package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func walk(object fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(object)
	if c, ok := object.(*fyne.Container); ok {
		for _, child := range c.Objects {
			walk(child, visit)
		}
	}
}

func TestEditorRejectsInvalidAndSavesKnob2(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	s.w.Show()
	screenshot(t, s.w, "main.png")
	if len(s.buttons) != 18 {
		t.Fatalf("%d inputs", len(s.buttons))
	}
	test.Tap(s.buttons[17])
	var value *widget.SelectEntry
	var add *widget.Button
	overlay := s.w.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("editor did not open")
	}
	// Dialog content is exposed through the widget renderer.
	var inspect func(fyne.CanvasObject)
	visited := map[fyne.CanvasObject]bool{}
	inspect = func(o fyne.CanvasObject) {
		if visited[o] {
			return
		}
		visited[o] = true
		switch w := o.(type) {
		case *widget.SelectEntry:
			value = w
		case *widget.Button:
			if w.Text == "設定に追加" {
				add = w
			}
		}
		if w, ok := o.(fyne.Widget); ok {
			for _, child := range test.WidgetRenderer(w).Objects() {
				walk(child, inspect)
			}
		}
	}
	walk(overlay, inspect)
	if value == nil || add == nil {
		t.Fatal("editor controls missing")
	}
	value.SetText("invalid-key")
	test.Tap(add)
	if len(s.p.Bindings) != 0 {
		t.Fatal("invalid binding was stored")
	}
	value.SetText("ctrl+shift+s")
	screenshot(t, s.w, "editor.png")
	test.Tap(add)
	b, ok := s.p.Get(1, 18)
	if !ok || b.Value != "ctrl+shift+s" {
		t.Fatalf("knob 2 not stored: %+v", s.p)
	}
	if !s.dirty {
		t.Fatal("local edit must be marked unsaved")
	}
	if s.w.Canvas().Overlays().Top() != nil {
		t.Fatal("editor remained open")
	}
}

func screenshot(t *testing.T, w fyne.Window, name string) {
	t.Helper()
	dir := os.Getenv("KEY_SHORT_SCREENSHOTS")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
