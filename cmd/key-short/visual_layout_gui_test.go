package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	devlayout "key-short/internal/layout"
)

func TestVisualDragAndKnobOrientationRemainLocalUntilApplied(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	l := devlayout.Presets()[1].Layout
	e := &visualLayoutEditor{s: s, draft: l.Clone(), selected: -1, grid: container.NewGridWithColumns(l.Columns), knobs: container.NewVBox(), hint: widget.NewLabel("")}
	e.render()
	s.w.SetContent(container.NewVBox(e.grid, e.knobs))
	s.w.Resize(fyne.NewSize(640, 650))
	before := profileSignature(s.p)
	source, target := e.tiles[0], e.tiles[1]
	end := a.Driver().AbsolutePositionForObject(target).Add(fyne.NewPos(target.Size().Width/2, target.Size().Height/2))
	source.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: end}})
	source.DragEnd()
	if e.draft.Slots[0] != 2 || e.draft.Slots[1] != 1 || profileSignature(s.p) != before {
		t.Fatal("drag changed profile or did not swap preview")
	}
	var reverse *widget.Button
	visitWidgets(e.knobs, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "回転方向を反転" {
			reverse = b
		}
	})
	test.Tap(reverse)
	if e.draft.Knobs[0].Left != 15 || e.draft.Knobs[0].Right != 13 {
		t.Fatal("knob reversal failed")
	}
	e.draft.KnobPosition = "left"
	if err := e.draft.Validate(); err != nil {
		t.Fatal(err)
	}
	s.applyLayout(e.draft)
	if s.p.EffectiveLayout().KnobPosition != "left" || s.usbSignature != "" {
		t.Fatal("placement did not remain local")
	}
	s.undo()
	if s.p.EffectiveLayout().KnobPosition != "" {
		t.Fatal("layout undo lost original position")
	}
}
