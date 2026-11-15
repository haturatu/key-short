package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"key-short/internal/protocol"
)

const bindingClipboardPrefix = "key-short-bindings/v1\n"

func (s *studio) selectedInputs() []int {
	slots := []int{}
	for slot, selected := range s.selectedSlots {
		if selected && s.p.EffectiveLayout().Contains(slot) {
			slots = append(slots, slot)
		}
	}
	if len(slots) == 0 {
		return []int{s.selectedSlot}
	}
	sort.Ints(slots)
	return slots
}
func (s *studio) copyAssignments() {
	if s.busy || s.hasDraft() {
		return
	}
	bindings := []protocol.Binding{}
	for _, slot := range s.selectedInputs() {
		b, ok := s.p.Get(s.layer, slot)
		if !ok {
			s.status.SetText("コピーするキーには割り当てが必要です。")
			return
		}
		bindings = append(bindings, b)
	}
	data, _ := json.Marshal(bindings)
	s.app.Clipboard().SetContent(bindingClipboardPrefix + string(data))
	s.status.SetText(fmt.Sprintf("%d 件をコピーしました。貼り付け先を選んでください。", len(bindings)))
}
func (s *studio) pasteAssignments() {
	if s.busy || s.hasDraft() {
		return
	}
	text := s.app.Clipboard().Content()
	if !strings.HasPrefix(text, bindingClipboardPrefix) {
		s.status.SetText("キー割り当てをコピーしてから貼り付けてください。")
		return
	}
	var bindings []protocol.Binding
	if len(text) > 1024*1024 || json.Unmarshal([]byte(strings.TrimPrefix(text, bindingClipboardPrefix)), &bindings) != nil || len(bindings) == 0 || len(bindings) > 18 {
		return
	}
	slots := s.selectedInputs()
	if len(bindings) != 1 && len(bindings) != len(slots) {
		s.status.SetText("コピーした数と同じ数の貼り付け先を複数選択してください。")
		return
	}
	q := s.p.Clone()
	for i, slot := range slots {
		b := bindings[0]
		if len(bindings) > 1 {
			b = bindings[i]
		}
		b.Layer, b.Slot = s.layer, slot
		q.Set(b)
	}
	if err := q.Validate(); err != nil {
		s.fail(err)
		return
	}
	s.p = q.Clone()
	s.activateSelection(s.layer, s.selectedSlot)
	s.changed()
	s.status.SetText(fmt.Sprintf("%d 件に貼り付けました。Undo で戻せます。", len(slots)))
}
func (s *studio) duplicateAssignment() {
	if s.busy || s.hasDraft() {
		return
	}
	inputs := s.selectedInputs()
	if len(inputs) != 1 {
		s.status.SetText("複製するキーを1つ選択してください。")
		return
	}
	l := s.p.EffectiveLayout()
	slots := append([]int(nil), l.Slots...)
	for _, k := range l.Knobs {
		slots = append(slots, k.Left, k.Press, k.Right)
	}
	for _, slot := range slots {
		if _, ok := s.p.Get(s.layer, slot); ok {
			continue
		}
		b, ok := s.p.Get(s.layer, inputs[0])
		if !ok {
			return
		}
		b.Slot = slot
		q, err := s.p.WithBinding(b)
		if err != nil {
			s.fail(err)
			return
		}
		s.p = q
		s.selectedSlots = map[int]bool{}
		if s.multiSelect.Checked {
			s.selectedSlots[slot] = true
		}
		s.activateSelection(s.layer, slot)
		s.changed()
		return
	}
	s.status.SetText("複製先の未設定キーがありません。コピー・貼り付けで対象を選べます。")
}
func (s *studio) installShortcuts() {
	item := func(label string, shortcut fyne.Shortcut, action func()) *fyne.MenuItem {
		guarded := func() {
			if !s.busy && s.w.Canvas().Overlays().Top() == nil {
				action()
			}
		}
		m := fyne.NewMenuItem(label, guarded)
		m.Shortcut = shortcut
		s.w.Canvas().AddShortcut(shortcut, func(fyne.Shortcut) { guarded() })
		return m
	}
	// Menus receive shortcuts before the focused widget. Text copy/paste stays local
	// to an entry; Undo/Redo restores a complete profile operation, even while typing.
	textOrAssignment := func(copy bool) func() {
		return func() {
			if target, ok := s.w.Canvas().Focused().(interface {
				SelectedText() string
				TypedShortcut(fyne.Shortcut)
			}); ok {
				if copy {
					target.TypedShortcut(&fyne.ShortcutCopy{Clipboard: s.app.Clipboard()})
				} else {
					target.TypedShortcut(&fyne.ShortcutPaste{Clipboard: s.app.Clipboard()})
				}
				return
			}
			if copy {
				s.copyAssignments()
			} else {
				s.pasteAssignments()
			}
		}
	}
	save := item("JSON を保存", &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierShortcutDefault}, s.save)
	undo := item("元に戻す", &fyne.ShortcutUndo{}, s.undo)
	redo := item("やり直す", &desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}, s.redo)
	redoAlt := item("やり直す (Ctrl+Y)", &fyne.ShortcutRedo{}, s.redo)
	copy := item("コピー", &fyne.ShortcutCopy{}, textOrAssignment(true))
	paste := item("貼り付け", &fyne.ShortcutPaste{}, textOrAssignment(false))
	duplicate := item("未設定キーへ複製", &desktop.CustomShortcut{KeyName: fyne.KeyD, Modifier: fyne.KeyModifierShortcutDefault}, s.duplicateAssignment)
	s.w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("ファイル", save), fyne.NewMenu("編集", undo, redo, redoAlt, fyne.NewMenuItemSeparator(), copy, paste, duplicate)))
}
