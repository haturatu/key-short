package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	devlayout "key-short/internal/layout"
	"key-short/internal/protocol"
)

func assignmentCenter(s *studio, slot int) fyne.Position {
	b := s.buttons[slot-1]
	return s.app.Driver().AbsolutePositionForObject(b).Add(fyne.NewPos(b.Size().Width/2, b.Size().Height/2))
}
func dragAssignment(s *studio, from int, end fyne.Position) {
	start := assignmentCenter(s, from)
	s.buttons[from-1].Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: end}, Dragged: fyne.NewDelta(end.X-start.X, end.Y-start.Y)})
	s.buttons[from-1].DragEnd()
}
func launcherBinding(slot int) protocol.Binding {
	return protocol.Binding{Layer: 1, Slot: slot, Kind: "launcher", Value: "Chromium debug", Launch: &protocol.LaunchSpec{Mode: "command", Command: "chromium --remote-debugging-port=9222", ReuseWindow: true}}
}
func TestDragMovesLauncherAndInvalidatesAppliedStates(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	s.p.Set(launcherBinding(1))
	s.activateSelection(1, 1)
	s.savedSignature = profileSignature(s.p)
	s.dirty = false
	s.usbSignature = wireSignature(s.p)
	s.usbTarget = s.devices.Selected
	stopped := false
	s.monitorStop = func() { stopped = true }
	end := assignmentCenter(s, 2)
	if s.dropTarget(end) != 2 {
		t.Fatal("visible destination not hit")
	}
	s.buttons[0].Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: end}})
	if s.dragTarget != 2 || stopped {
		t.Fatal("drag did not highlight or prematurely stopped monitoring")
	}
	screenshot(t, s.w, "drag-assignment.png")
	s.buttons[0].DragEnd()
	moved, ok := s.p.Get(1, 2)
	source, _ := s.p.Get(1, 1)
	if !ok || moved.Launch == nil || !moved.Launch.ReuseWindow || moved.Launch.Command != "chromium --remote-debugging-port=9222" || source.Value != "none" {
		t.Fatal("move lost launch configuration or failed to clear source")
	}
	if !stopped || !s.dirty || s.selectedSlot != 2 || s.hasDraft() || s.dragTarget != 0 {
		t.Fatal("move did not update studio state")
	}
	if !strings.Contains(s.usbState.Text, "未反映") {
		t.Fatal("USB was marked current after local move", s.usbState.Text)
	}
}
func TestDragOccupiedDestinationRequiresConfirmedSwap(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	s.p.Set(launcherBinding(1))
	s.p.Set(protocol.Binding{Layer: 1, Slot: 2, Kind: "keyboard", Value: "ctrl+pageup"})
	s.activateSelection(1, 1)
	before := profileSignature(s.p)
	dragAssignment(s, 1, assignmentCenter(s, 2))
	if profileSignature(s.p) != before {
		t.Fatal("swap occurred before confirmation")
	}
	test.Tap(dialogButton(t, s, "キャンセル"))
	if profileSignature(s.p) != before {
		t.Fatal("cancel changed assignments")
	}
	dragAssignment(s, 1, assignmentCenter(s, 2))
	test.Tap(dialogButton(t, s, "入れ替える"))
	source, _ := s.p.Get(1, 1)
	target, _ := s.p.Get(1, 2)
	if source.Value != "ctrl+pageup" || target.Kind != "launcher" || len(s.p.Bindings) != 2 {
		t.Fatal("swap destroyed an assignment")
	}
}
func TestDragCancelBusyDraftAndStaleConfirmation(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	s.p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "keyboard", Value: "a"})
	s.activateSelection(1, 1)
	before := profileSignature(s.p)
	dragAssignment(s, 1, fyne.NewPos(-20, -20))
	dragAssignment(s, 1, assignmentCenter(s, 1))
	if profileSignature(s.p) != before || s.dragTarget != 0 {
		t.Fatal("canceled drag changed profile")
	}
	s.busy = true
	dragAssignment(s, 1, assignmentCenter(s, 2))
	s.busy = false
	if profileSignature(s.p) != before {
		t.Fatal("drag during USB write changed profile")
	}
	s.editor.value.SetText("b")
	dragAssignment(s, 1, assignmentCenter(s, 2))
	if profileSignature(s.p) != before || !s.hasDraft() {
		t.Fatal("drag discarded pending editor changes")
	}
	s.activateSelection(1, 1)
	s.p.Set(protocol.Binding{Layer: 1, Slot: 2, Kind: "keyboard", Value: "b"})
	s.refresh()
	dragAssignment(s, 1, assignmentCenter(s, 2))
	s.p.Set(protocol.Binding{Layer: 2, Slot: 1, Kind: "keyboard", Value: "c"})
	changed := profileSignature(s.p)
	test.Tap(dialogButton(t, s, "入れ替える"))
	if profileSignature(s.p) != changed || !strings.Contains(s.status.Text, "ドラッグし直") {
		t.Fatal("stale swap overwrote a newer change")
	}
}
func TestDropTargetRespectsViewportAndLayout(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	p := assignmentCenter(s, 1)
	viewport := s.app.Driver().AbsolutePositionForObject(s.keyScroll)
	if s.dropTarget(fyne.NewPos(p.X, viewport.Y-1)) != 0 {
		t.Fatal("accepted drop outside viewport")
	}
	// Enlarging the window makes the knob rows visible and verifies that their
	// explicit slot mapping is usable as a drop destination too.
	s.w.Resize(fyne.NewSize(1180, 1200))
	s.refresh()
	if s.dropTarget(assignmentCenter(s, 13)) != 13 {
		t.Fatal("knob slot not recognized")
	}
	s.applyLayout(devlayout.Presets()[0].Layout)
	if s.dropTarget(assignmentCenter(s, 18)) == 18 {
		t.Fatal("hidden slot accepted as drop destination")
	}
}
