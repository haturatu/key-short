package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestCaptureImportsControlPageUp(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	target := widget.NewSelectEntry(nil)
	target.SetText("original")
	s.recordKeys(target)
	var capture *captureControl
	var accept *widget.Button
	seen := map[fyne.CanvasObject]bool{}
	var inspect func(fyne.CanvasObject)
	inspect = func(o fyne.CanvasObject) {
		if seen[o] {
			return
		}
		seen[o] = true
		switch w := o.(type) {
		case *captureControl:
			capture = w
		case *widget.Button:
			if w.Text == "割り当てに取り込む" {
				accept = w
			}
		}
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
	inspect(s.w.Canvas().Overlays().Top())
	if capture == nil || accept == nil {
		t.Fatal("capture controls missing")
	}
	capture.KeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
	capture.KeyUp(&fyne.KeyEvent{Name: fyne.KeyA})
	if capture.rec.Value() != "a" {
		t.Fatal(capture.rec.Value())
	}
	capture.KeyDown(&fyne.KeyEvent{Name: "LeftControl"})
	capture.KeyDown(&fyne.KeyEvent{Name: fyne.KeyPageUp})
	capture.KeyUp(&fyne.KeyEvent{Name: fyne.KeyPageUp})
	capture.KeyUp(&fyne.KeyEvent{Name: "LeftControl"})
	if capture.rec.Value() != "ctrl+pageup" {
		t.Fatal(capture.rec.Value())
	}
	screenshot(t, s.w, "key-recording.png")
	test.Tap(accept)
	if target.Text != "ctrl+pageup" {
		t.Fatal("not imported", target.Text)
	}
}
