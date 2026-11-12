package main

import (
	"fmt"
	"reflect"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/protocol"
)

type bindingEditor struct {
	s                            *studio
	layer, slot                  int
	box                          *fyne.Container
	kind                         *widget.Select
	value                        *widget.SelectEntry
	launch                       *launchEditor
	apply, remove, reset, record *widget.Button
	validation                   *widget.Label
	original                     protocol.Binding
	dirty, valid, initializing   bool
}

func (s *studio) edit(slot int) { s.selectInput(s.layer, slot) }
func (s *studio) selectInput(layer, slot int) {
	if s.busy {
		return
	}
	if layer == s.layer && slot == s.selectedSlot {
		return
	}
	selectNow := func() { s.activateSelection(layer, slot) }
	if s.hasDraft() {
		s.loading = true
		s.layers.SetSelectedIndex(s.layer - 1)
		s.loading = false
		dialog.ShowCustomConfirm("キーに未適用の変更", "破棄して切り替える", "編集を続ける", widget.NewLabel("編集中のキーの変更を破棄しますか？"), func(ok bool) {
			if ok {
				selectNow()
			}
		}, s.w)
	} else {
		selectNow()
	}
}
func (s *studio) activateSelection(layer, slot int) {
	s.layer = layer
	s.selectedSlot = slot
	s.loading = true
	s.layers.SetSelectedIndex(layer - 1)
	s.loading = false
	e := s.newBindingEditor(layer, slot)
	s.editor = e
	s.editorHost.Objects = []fyne.CanvasObject{e.box}
	s.editorHost.Refresh()
	e.validate()
	s.refresh()
}
func (s *studio) newBindingEditor(layer, slot int) *bindingEditor {
	e := &bindingEditor{s: s, layer: layer, slot: slot, initializing: true, value: widget.NewSelectEntry(nil), validation: widget.NewLabel("")}
	e.validation.Wrapping = fyne.TextWrapWord
	existing, ok := s.p.Get(layer, slot)
	label := ""
	if existing.Kind == "launcher" {
		label = existing.Value
	}
	e.launch = s.newLaunchEditor(existing.Launch, label)
	e.record = widget.NewButton("キー入力から記録…", func() { s.recordKeys(e.value) })
	valueForm := widget.NewForm(widget.NewFormItem("割り当て", e.value))
	note := widget.NewLabel("A や Ctrl + PageUp はキー入力から記録できます。連続操作は , で区切ります（最大5操作）。")
	note.Wrapping = fyne.TextWrapWord
	e.kind = widget.NewSelect([]string{"キーボード", "メディア", "マウス", "ランチャー"}, func(v string) {
		e.launch.box.Hide()
		valueForm.Show()
		e.record.Hide()
		note.Show()
		switch v {
		case "キーボード":
			e.record.Show()
			e.value.SetOptions([]string{"ctrl+c", "ctrl+v", "ctrl+pageup", "ctrl+shift+s", "super+e", "f13", "ctrl+c,ctrl+v"})
		case "メディア":
			e.value.SetOptions(sortedNames(protocol.Media))
		case "マウス":
			e.value.SetOptions(sortedNames(protocol.Mouse))
		case "ランチャー":
			e.launch.box.Show()
			valueForm.Hide()
			note.Hide()
		}
		e.validate()
	})
	e.apply = widget.NewButton("変更を適用", func() {
		if s.busy {
			return
		}
		q, err := s.p.WithBinding(e.binding())
		if err != nil {
			e.validation.SetText(err.Error())
			return
		}
		s.p = q
		s.activateSelection(layer, slot)
		s.changed()
		s.status.SetText("キーの変更を適用しました。保存と USB 書き込みは別に実行してください。")
	})
	e.apply.Importance = widget.HighImportance
	e.remove = widget.NewButton("書き込み対象から外す", func() {
		if s.busy {
			return
		}
		q := s.p.Clone()
		q.Remove(layer, slot)
		s.p = q
		s.activateSelection(layer, slot)
		s.changed()
	})
	e.reset = widget.NewButton("編集を戻す", func() {
		if !s.busy {
			s.activateSelection(layer, slot)
		}
	})
	e.box = container.NewVBox(widget.NewLabelWithStyle(fmt.Sprintf("レイヤー %d / %s", layer, protocol.SlotName(slot)), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewForm(widget.NewFormItem("操作の種類", e.kind)), valueForm, e.record, note, e.launch.box, e.validation, container.NewGridWithColumns(2, e.reset, e.apply), e.remove)
	if ok {
		e.kind.SetSelected(map[string]string{"keyboard": "キーボード", "media": "メディア", "mouse": "マウス", "launcher": "ランチャー"}[existing.Kind])
		e.value.SetText(existing.Value)
	} else {
		e.kind.SetSelectedIndex(0)
	}
	e.original = e.binding()
	e.initializing = false
	e.value.OnChanged = func(string) { e.validate() }
	e.value.OnSubmitted = func(string) {
		if !e.apply.Disabled() {
			e.apply.OnTapped()
		}
	}
	e.launch.onChanged = e.validate
	return e
}
func (e *bindingEditor) binding() protocol.Binding {
	if e.kind.Selected == "ランチャー" {
		spec := e.launch.spec()
		return protocol.Binding{Layer: e.layer, Slot: e.slot, Kind: "launcher", Value: e.launch.label.Text, Launch: &spec}
	}
	return protocol.Binding{Layer: e.layer, Slot: e.slot, Kind: map[string]string{"キーボード": "keyboard", "メディア": "media", "マウス": "mouse"}[e.kind.Selected], Value: strings.TrimSpace(e.value.Text)}
}
func (e *bindingEditor) validate() {
	if e.initializing {
		return
	}
	e.dirty = !reflect.DeepEqual(e.original, e.binding())
	_, err := e.s.p.WithBinding(e.binding())
	e.valid = err == nil
	text := "変更はありません。"
	if e.dirty {
		if err != nil {
			text = err.Error()
		} else {
			text = "適用できます。JSON 保存と USB 書き込みは、適用後に実行できます。"
		}
	}
	e.validation.SetText(text)
	e.validation.Importance = widget.MediumImportance
	if e.dirty && !e.valid {
		e.validation.Importance = widget.DangerImportance
	}
	e.validation.Refresh()
	e.s.refresh()
}
func (e *bindingEditor) updateControls() {
	for _, c := range []fyne.Disableable{e.kind, e.value, e.record, e.reset, e.remove, e.launch.mode, e.launch.command, e.launch.label, e.launch.class, e.launch.dir, e.launch.reuse, e.launch.choose} {
		if e.s.busy {
			c.Disable()
		} else {
			c.Enable()
		}
	}
	if e.s.busy || e.launch.testing {
		e.launch.testButton.Disable()
	} else {
		e.launch.testButton.Enable()
	}
	if !e.dirty || !e.valid || e.s.busy {
		e.apply.Disable()
	} else {
		e.apply.Enable()
	}
	if _, ok := e.s.p.Get(e.layer, e.slot); !ok {
		e.remove.Disable()
	}
}
