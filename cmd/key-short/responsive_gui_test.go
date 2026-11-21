package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestWindowSwitchesPanesWithoutLosingInvalidInput(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	s.editor.value.SetText("ctrl+c")
	s.editor.value.SetText("invalid-key")
	editor := s.editor
	s.w.Resize(fyne.NewSize(640, 740))
	s.refresh()
	if !s.workspace.narrow || !s.workspace.tabs.Visible() || s.workspace.split.Visible() {
		t.Fatal("small window did not use tabs", s.w.Canvas().Size())
	}
	s.workspace.tabs.SelectIndex(1)
	if s.editor != editor || s.editor.value.Text != "invalid-key" || !s.hasDraft() {
		t.Fatal("resize lost draft")
	}
	screenshot(t, s.w, "compact-editor.png")
	s.workspace.tabs.SelectIndex(0)
	screenshot(t, s.w, "compact-layout.png")
	s.w.Resize(fyne.NewSize(1280, 900))
	s.refresh()
	if s.workspace.narrow || s.workspace.tabs.Visible() || !s.workspace.split.Visible() || s.editor != editor {
		t.Fatal("wide window did not restore split")
	}
}
