package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/protocol"
)

// assignmentButton retains Button's click/keyboard behavior while letting Fyne
// dispatch a drag to the source widget through the end of the pointer gesture.
type assignmentButton struct {
	widget.Button
	studio       *studio
	slot         int
	dragging     bool
	dragLayer    int
	signature    string
	lastPosition fyne.Position
}

var _ fyne.Draggable = (*assignmentButton)(nil)

func newAssignmentButton(s *studio, slot int) *assignmentButton {
	b := &assignmentButton{studio: s, slot: slot}
	b.OnTapped = func() { s.edit(slot) }
	b.ExtendBaseWidget(b)
	return b
}
func emptyAssignment(b protocol.Binding) bool {
	return b.Kind == "keyboard" && strings.EqualFold(strings.TrimSpace(b.Value), "none")
}
func (b *assignmentButton) Dragged(e *fyne.DragEvent) {
	s := b.studio
	if b.Disabled() || s.busy || s.closing {
		return
	}
	if s.hasDraft() {
		s.status.SetText("キーの編集中の変更を適用するか戻してから、ドラッグしてください。")
		return
	}
	if !b.dragging {
		source, ok := s.p.Get(s.layer, b.slot)
		if !ok || emptyAssignment(source) {
			s.status.SetText("割り当てのあるキーかノブをドラッグしてください。")
			return
		}
		b.dragging = true
		b.dragLayer = s.layer
		b.signature = profileSignature(s.p)
	}
	b.lastPosition = e.AbsolutePosition
	target := s.dropTarget(e.AbsolutePosition)
	if target == b.slot {
		target = 0
	}
	if target != s.dragTarget {
		s.dragTarget = target
		s.highlightInputs()
	}
	if target == 0 {
		s.status.SetText("移動先のキーかノブの上で離してください。配置の外で離すとキャンセルします。")
		return
	}
	action := "移動"
	if existing, ok := s.p.Get(s.layer, target); ok && !emptyAssignment(existing) {
		action = "入れ替え"
	}
	s.status.SetText(fmt.Sprintf("%s → %s · 離すと%sします。", protocol.SlotName(b.slot), protocol.SlotName(target), action))
}
func (b *assignmentButton) DragEnd() {
	if !b.dragging {
		return
	}
	s := b.studio
	b.dragging = false
	target := s.dropTarget(b.lastPosition)
	s.dragTarget = 0
	s.highlightInputs()
	if target == 0 || target == b.slot {
		s.status.SetText("割り当ての移動をキャンセルしました。")
		return
	}
	s.moveAssignment(b.dragLayer, b.slot, target, b.signature)
}
func (s *studio) highlightInputs() {
	for i, b := range s.buttons {
		importance := widget.MediumImportance
		if i+1 == s.selectedSlot {
			importance = widget.HighImportance
		}
		if i+1 == s.dragTarget {
			importance = widget.SuccessImportance
		}
		if b.Importance != importance {
			b.Importance = importance
			b.Refresh()
		}
	}
}
func (s *studio) pointInside(o fyne.CanvasObject, p fyne.Position) bool {
	origin := s.app.Driver().AbsolutePositionForObject(o)
	size := o.Size()
	return p.X >= origin.X && p.Y >= origin.Y && p.X < origin.X+size.Width && p.Y < origin.Y+size.Height
}
func (s *studio) dropTarget(p fyne.Position) int {
	if s.w.Canvas().Overlays().Top() != nil || !s.pointInside(s.keyScroll, p) {
		return 0
	}
	l := s.p.EffectiveLayout()
	for i, b := range s.buttons {
		if l.Contains(i+1) && b.Visible() && !b.Disabled() && s.pointInside(b, p) {
			return i + 1
		}
	}
	return 0
}
func (s *studio) moveAssignment(layer, from, to int, signature string) {
	apply := func() {
		if s.busy || s.closing || s.hasDraft() || s.layer != layer || profileSignature(s.p) != signature {
			s.status.SetText("設定が変わりました。ドラッグし直してください。")
			return
		}
		q, err := s.p.RelocateBinding(layer, from, to)
		if err != nil {
			s.fail(err)
			return
		}
		s.p = q
		s.activateSelection(layer, to)
		s.changed()
		s.status.SetText("割り当てを移動しました。JSON の保存と USB への書き込み後に、実機で確認してください。")
	}
	if s.busy || s.closing || s.hasDraft() || s.layer != layer || profileSignature(s.p) != signature {
		return
	}
	source, ok := s.p.Get(layer, from)
	if !ok || emptyAssignment(source) {
		return
	}
	target, occupied := s.p.Get(layer, to)
	if occupied && !emptyAssignment(target) {
		note := widget.NewLabel(fmt.Sprintf("%s: %s\n%s: %s\n\nこの2つの割り当てを入れ替えます。", protocol.SlotName(from), source.DisplayValue(), protocol.SlotName(to), target.DisplayValue()))
		note.Wrapping = fyne.TextWrapWord
		dialog.ShowCustomConfirm("割り当てを入れ替える", "入れ替える", "キャンセル", note, func(ok bool) {
			if ok {
				apply()
			} else {
				s.status.SetText("割り当ての入れ替えをキャンセルしました。")
			}
		}, s.w)
	} else {
		apply()
	}
}
