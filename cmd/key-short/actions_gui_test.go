package main

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"key-short/internal/protocol"
)

func TestBatchPasteIsAtomicAndRegeneratesLauncherTriggers(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.p.Set(launcherBinding(1))
	s.changed()
	s.activateSelection(1, 1)
	s.copyAssignments()
	s.multiSelect.SetChecked(true)
	test.Tap(s.buttons[1])
	test.Tap(s.buttons[2])
	before := profileSignature(s.p)
	s.pasteAssignments()
	for _, slot := range []int{2, 3} {
		b, ok := s.p.Get(1, slot)
		if !ok || b.Launch == nil || b.Launch.Command != "chromium --remote-debugging-port=9222" {
			t.Fatal("batch paste lost launcher", slot)
		}
	}
	if _, err := s.p.Jobs(); err != nil {
		t.Fatal("pasted launcher triggers collide", err)
	}
	s.undo()
	if profileSignature(s.p) != before {
		t.Fatal("batch paste requires more than one undo")
	}
	s.redo()
	s.p.Bindings[0].Launch.Command = "changed"
	b, _ := s.p.Get(1, 2)
	if b.Launch.Command == "changed" {
		t.Fatal("pasted launchers share mutable pointers")
	}
	// Reserved trigger conflicts reject the entire batch, without partial writes.
	s.multiSelect.SetChecked(false)
	s.activateSelection(1, 4)
	s.editor.value.SetText(protocol.Trigger(1, 7))
	s.copyAssignments()
	s.multiSelect.SetChecked(true)
	test.Tap(s.buttons[4])
	test.Tap(s.buttons[5])
	s.p.Set(launcherBinding(7))
	before = profileSignature(s.p)
	s.pasteAssignments()
	if profileSignature(s.p) != before {
		t.Fatal("failed batch paste partially changed profile")
	}
}
func TestShortcutsSaveUndoRedoAndPreserveEntryCopy(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.editor.value.SetText("ctrl+c")
	s.profilePath = filepath.Join(t.TempDir(), "keys.json")
	canvas := s.w.Canvas().(interface{ TypedShortcut(fyne.Shortcut) })
	canvas.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierShortcutDefault})
	if s.dirty {
		t.Fatal("save shortcut did not save")
	}
	canvas.TypedShortcut(&fyne.ShortcutUndo{})
	if len(s.p.Bindings) != 0 {
		t.Fatal("undo shortcut failed")
	}
	canvas.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift})
	if b, ok := s.p.Get(1, 1); !ok || b.Value != "ctrl+c" {
		t.Fatal("Ctrl+Shift+Z failed")
	}
	s.w.Canvas().Focus(s.editor.value)
	s.editor.value.TypedShortcut(&fyne.ShortcutSelectAll{})
	canvas.TypedShortcut(&fyne.ShortcutCopy{})
	if s.app.Clipboard().Content() != "ctrl+c" {
		t.Fatal("assignment copy intercepted entry text")
	}
	s.w.Canvas().Unfocus()
	canvas.TypedShortcut(&fyne.ShortcutCopy{})
	s.activateSelection(1, 2)
	canvas.TypedShortcut(&fyne.ShortcutPaste{})
	if b, ok := s.p.Get(1, 2); !ok || b.Value != "ctrl+c" {
		t.Fatal("assignment shortcuts failed")
	}
}
