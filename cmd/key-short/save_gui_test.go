package main

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	devlayout "key-short/internal/layout"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

func dialogButton(t *testing.T, s *studio, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	root := s.w.Canvas().Overlays().Top()
	if root == nil {
		t.Fatal("missing dialog", text)
	}
	visitWidgets(root, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			found = b
		}
	})
	if found == nil {
		t.Fatal("missing dialog button", text)
	}
	return found
}
func TestSaveAsNeverTruncatesBeforeConfirmedValidReplacement(t *testing.T) {
	for _, action := range []string{"cancel", "replace", "stale"} {
		t.Run(action, func(t *testing.T) {
			a := test.NewApp()
			defer a.Quit()
			s := newStudio(a)
			defer s.shutdown()
			s.w.Show()
			path := filepath.Join(t.TempDir(), "existing.json")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			s.profilePath = path
			s.p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "keyboard", Value: "a"})
			s.changed()
			assertOriginal := func() {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "original" {
					t.Fatal("original file changed before replacement", err)
				}
			}
			s.saveAs()
			assertOriginal()
			screenshot(t, s.w, "save.png")
			test.Tap(dialogButton(t, s, "この名前で保存"))
			assertOriginal()
			switch action {
			case "cancel":
				test.Tap(dialogButton(t, s, "戻る"))
				assertOriginal()
				if !s.dirty {
					t.Fatal("cancel marked saved")
				}
			case "replace":
				test.Tap(dialogButton(t, s, "置き換えて保存"))
				p, err := profile.Load(path)
				if err != nil || len(p.Bindings) != 1 || s.dirty {
					t.Fatal("replacement failed", err)
				}
			case "stale":
				s.p.Set(protocol.Binding{Layer: 1, Slot: 2, Kind: "keyboard", Value: "b"})
				test.Tap(dialogButton(t, s, "置き換えて保存"))
				assertOriginal()
				if !s.dirty {
					t.Fatal("stale save marked saved")
				}
			}
		})
	}
}
func TestDraftSelectionRequiresExplicitDiscard(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.editor.value.SetText("ctrl+pageup")
	test.Tap(s.buttons[1])
	if s.selectedSlot != 1 || s.editor.value.Text != "ctrl+pageup" {
		t.Fatal("selection silently lost draft")
	}
	test.Tap(dialogButton(t, s, "編集を続ける"))
	if !s.hasDraft() {
		t.Fatal("cancel lost draft")
	}
	test.Tap(s.buttons[1])
	test.Tap(dialogButton(t, s, "破棄して切り替える"))
	if s.selectedSlot != 2 || s.hasDraft() || len(s.p.Bindings) != 0 {
		t.Fatal("discard applied draft or failed to switch")
	}
}
func TestCustomLayoutEditorChangesOnlyDisplayMapping(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	s.applyLayout(devlayout.Presets()[1].Layout)
	s.layoutDialog()
	var slots *widget.Entry
	visitWidgets(s.w.Canvas().Overlays().Top(), func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.PlaceHolder == "例: 3,6,2,5,1,4（左上から行順）" {
			slots = e
		}
	})
	if slots == nil {
		t.Fatal("mapping entry missing")
	}
	apply := dialogButton(t, s, "この配置を使う")
	slots.SetText("3,3")
	if !apply.Disabled() {
		t.Fatal("duplicate slot can be applied")
	}
	slots.SetText("3,6,2,5,1,4")
	if apply.Disabled() {
		t.Fatal("valid slot order rejected")
	}
	screenshot(t, s.w, "layout.png")
	test.Tap(apply)
	if s.p.EffectiveLayout().Slots[0] != 3 || s.p.EffectiveLayout().Columns != 2 || len(s.p.Bindings) != 0 || s.usbSignature != "" {
		t.Fatal("layout changed bindings or USB state")
	}
}
