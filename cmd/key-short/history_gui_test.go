package main

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"key-short/internal/protocol"
)

func TestAutomaticEditsSurviveSelectionAndHistoryTracksSavedState(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.editor.value.SetText("ctrl+c")
	test.Tap(s.buttons[1])
	if s.w.Canvas().Overlays().Top() != nil {
		t.Fatal("valid edit requires confirmation")
	}
	s.editor.value.SetText("ctrl+v")
	if len(s.p.Bindings) != 2 || s.hasDraft() {
		t.Fatal("consecutive edits were lost")
	}
	if err := s.saveSnapshot(filepath.Join(t.TempDir(), "profile.json"), s.p.Clone()); err != nil {
		t.Fatal(err)
	}
	s.undo()
	if _, ok := s.p.Get(1, 2); ok || !s.dirty {
		t.Fatal("undo did not restore prior unsaved state")
	}
	s.redo()
	if b, ok := s.p.Get(1, 2); !ok || b.Value != "ctrl+v" || s.dirty {
		t.Fatal("redo did not restore saved state")
	}
	s.undo()
	s.editor.value.SetText("ctrl+x")
	if len(s.redoStack) != 0 {
		t.Fatal("new edit retained obsolete redo history")
	}
}

func TestInvalidInputPreservesLastValidBindingAndUndoMove(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.editor.value.SetText("ctrl+c")
	s.editor.value.SetText("invalid-key")
	b, _ := s.p.Get(1, 1)
	if b.Value != "ctrl+c" || !s.hasDraft() {
		t.Fatal("invalid input replaced valid binding")
	}
	s.undo() // First undo discards only the invalid input.
	if s.hasDraft() || len(s.p.Bindings) != 1 {
		t.Fatal("invalid input recovery removed valid edit")
	}
	s.moveAssignment(1, 1, 2, profileSignature(s.p))
	s.undo()
	b, ok := s.p.Get(1, 1)
	if !ok || b.Value != "ctrl+c" {
		t.Fatal("move undo lost source")
	}
	if _, ok := s.p.Get(1, 2); ok {
		t.Fatal("move undo left destination")
	}
	s.redo()
	b, _ = s.p.Get(1, 1)
	if b != (protocol.Binding{Layer: 1, Slot: 1, Kind: "keyboard", Value: "none"}) {
		t.Fatal("redo lost explicit hardware clear")
	}
}
