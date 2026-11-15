package main

import (
	"fyne.io/fyne/v2/widget"
)

const historyLimit = 200

func (s *studio) resetHistory() {
	s.historyCurrent = s.p.Clone()
	s.undoStack, s.redoStack = nil, nil
}
func (s *studio) recordHistory() {
	if profileSignature(s.p) == profileSignature(s.historyCurrent) {
		return
	}
	s.undoStack = append(s.undoStack, s.historyCurrent.Clone())
	if len(s.undoStack) > historyLimit {
		s.undoStack = s.undoStack[1:]
	}
	s.redoStack = nil
	s.historyCurrent = s.p.Clone()
}
func (s *studio) undo() { s.restoreHistory(false) }
func (s *studio) redo() { s.restoreHistory(true) }
func (s *studio) restoreHistory(redo bool) {
	if s.busy || s.closing || s.w.Canvas().Overlays().Top() != nil {
		return
	}
	if s.hasDraft() {
		if !redo {
			s.activateSelection(s.layer, s.selectedSlot)
		} else {
			s.status.SetText("入力エラーを修正するか入力を戻してから、やり直してください。")
		}
		return
	}
	source, dest := &s.undoStack, &s.redoStack
	if redo {
		source, dest = dest, source
	}
	if len(*source) == 0 {
		return
	}
	*dest = append(*dest, s.p.Clone())
	s.p = (*source)[len(*source)-1].Clone()
	*source = (*source)[:len(*source)-1]
	s.historyCurrent = s.p.Clone()
	s.loading = true
	if s.p.LED == nil {
		s.led.SetSelectedIndex(0)
	} else {
		s.led.SetSelectedIndex(*s.p.LED + 1)
	}
	s.loading = false
	l := s.p.EffectiveLayout()
	if !l.Contains(s.selectedSlot) {
		s.selectedSlot = l.Slots[0]
	}
	s.rebuildLayout()
	s.activateSelection(s.layer, s.selectedSlot)
	s.changed()
	s.rememberLayout()
	s.status.SetText("作業中の設定を戻しました。JSON 保存と USB 反映は別に実行してください。")
}
func (s *studio) updateHistoryControls() {
	for _, item := range []struct {
		button    *widget.Button
		available bool
	}{{s.undoButton, len(s.undoStack) > 0 || s.hasDraft()}, {s.redoButton, len(s.redoStack) > 0 && !s.hasDraft()}} {
		if item.button == nil {
			continue
		}
		if s.busy || !item.available {
			item.button.Disable()
		} else {
			item.button.Enable()
		}
	}
}
