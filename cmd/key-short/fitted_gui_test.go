package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	devlayout "key-short/internal/layout"
)

func TestAllPresetInputsFitWithoutScrollingOnLaptop(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	for _, size := range []fyne.Size{fyne.NewSize(1180, 680), fyne.NewSize(1024, 640), fyne.NewSize(640, 640)} {
		for _, preset := range devlayout.Presets() {
			s.w.Resize(size)
			s.applyLayout(preset.Layout)
			if s.workspace.narrow {
				s.workspace.tabs.SelectIndex(0)
			}
			s.refresh()
			if s.keyScroll.Direction != container.ScrollNone || !s.keyScroll.Offset.IsZero() {
				t.Fatal("keyboard still requires scrolling", size, preset.ID)
			}
			viewport := a.Driver().AbsolutePositionForObject(s.keyScroll)
			limit := viewport.Add(fyne.NewPos(s.keyScroll.Size().Width, s.keyScroll.Size().Height))
			for _, b := range s.buttons {
				if !preset.Layout.Contains(b.slot) {
					continue
				}
				top := a.Driver().AbsolutePositionForObject(b)
				bottom := top.Add(fyne.NewPos(b.Size().Width, b.Size().Height))
				if top.X < viewport.X || top.Y < viewport.Y || bottom.X > limit.X+.5 || bottom.Y > limit.Y+.5 {
					t.Fatalf("%s at %v: slot %d lies outside visible pane (%v–%v, viewport %v–%v)", preset.ID, size, b.slot, top, bottom, viewport, limit)
				}
				if b.Size().Height < 44 {
					t.Fatalf("%s at %v: slot %d is too short: %v", preset.ID, size, b.slot, b.Size())
				}
			}
			for _, slot := range []int{preset.Layout.Slots[0], preset.Layout.Knobs[len(preset.Layout.Knobs)-1].Right} {
				if s.dropTarget(assignmentCenter(s, slot)) != slot {
					t.Fatal("visible input cannot be used for drag", preset.ID, size, slot)
				}
			}
		}
	}
}
func TestNormalEditorsFitWithoutScrolling(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	for _, size := range []fyne.Size{fyne.NewSize(1180, 680), fyne.NewSize(1024, 640), fyne.NewSize(640, 640)} {
		s.w.Resize(size)
		if s.workspace.narrow {
			s.workspace.tabs.SelectIndex(1)
		}
		for _, kind := range []string{"キーボード", "メディア", "マウス", "ランチャー"} {
			s.activateSelection(1, 1)
			s.editor.kind.SetSelected(kind)
			switch kind {
			case "キーボード":
				s.editor.value.SetText("ctrl+c")
			case "メディア":
				s.editor.value.SetText("音量を上げる")
			case "マウス":
				s.editor.value.SetText("左クリック")
			case "ランチャー":
				s.editor.launch.mode.SetSelected("コマンドラインで実行")
				s.editor.launch.command.SetText("chromium --remote-debugging-port=9222")
			}
			s.refresh()
			if s.editor.box.MinSize().Height > s.editorHost.Size().Height+.5 {
				t.Fatalf("%s editor at %v needs scrolling: min=%v host=%v", kind, size, s.editor.box.MinSize(), s.editorHost.Size())
			}
		}
	}
	s.activateSelection(1, 2)
	s.editor.value.SetText("invalid-key")
	s.refresh()
	if s.editor.box.MinSize().Height > s.editorHost.Size().Height+.5 {
		t.Fatal("validation moved editor controls below the visible area")
	}
	screenshot(t, s.w, "compact-editor.png")
}
