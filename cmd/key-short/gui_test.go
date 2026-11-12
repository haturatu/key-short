package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
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
	defer s.shutdown()
	s.w.Show()
	screenshot(t, s.w, "main.png")
	if len(s.buttons) != 18 {
		t.Fatalf("%d inputs", len(s.buttons))
	}
	test.Tap(s.buttons[17])
	if s.w.Canvas().Overlays().Top() != nil {
		t.Fatal("key selection opened a modal")
	}
	value, add := s.editor.value, s.editor.apply
	value.SetText("invalid-key")
	if !add.Disabled() {
		t.Fatal("invalid binding can be applied")
	}
	test.Tap(add)
	if len(s.p.Bindings) != 0 {
		t.Fatal("invalid binding was stored")
	}
	value.SetText("ctrl+shift+s")
	if !s.hasDraft() || len(s.p.Bindings) != 0 || !s.saveButton.Disabled() {
		t.Fatal("draft mutated applied state or can be saved")
	}
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
