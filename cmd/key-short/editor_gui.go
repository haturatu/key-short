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
	s                          *studio
	layer, slot                int
	box                        *fyne.Container
	kind                       *widget.Select
	value                      *widget.SelectEntry
	launch                     *launchEditor
	remove, reset, record      *widget.Button
	clear, search              *widget.Button
	validation, details        *widget.Label
	original                   protocol.Binding
	dirty, valid, initializing bool
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
		dialog.ShowCustomConfirm("入力エラーがある変更", "破棄して切り替える", "編集を続ける", widget.NewLabel("無効な入力を破棄しますか？ 最後の有効な設定は保持されます。"), func(ok bool) {
			if ok {
				selectNow()
			}
		}, s.w)
	} else {
		selectNow()
	}
}
func (s *studio) activateSelection(layer, slot int) {
	if s.layer != layer {
		s.selectedSlots = map[int]bool{}
	}
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
	e.details = widget.NewLabel("")
	e.details.Wrapping = fyne.TextWrapWord
	e.validation.Wrapping = fyne.TextWrapWord
	existing, ok := s.p.Get(layer, slot)
	label := ""
	if existing.Kind == "launcher" {
		label = existing.Value
	}
	e.launch = s.newLaunchEditor(existing.Launch, label)
	e.record = widget.NewButton("キー入力から記録…", func() { s.recordKeys(e.value) })
	e.search = widget.NewButton("操作を検索…", e.searchOperations)
	tools := container.NewGridWithColumns(2, e.search, e.record)
	valueForm := widget.NewForm(widget.NewFormItem("割り当て", e.value))

	e.kind = widget.NewSelect([]string{"キーボード", "メディア", "マウス", "ランチャー"}, func(v string) {
		tools.Show()
		e.launch.box.Hide()
		valueForm.Show()
		e.record.Hide()
		switch v {
		case "キーボード":
			e.record.Show()
			e.value.SetOptions([]string{"ctrl+c", "ctrl+v", "ctrl+pageup", "ctrl+shift+s", "super+e", "f13", "ctrl+c,ctrl+v"})
		case "メディア":
			e.value.SetOptions(operationOptions("メディア"))
		case "マウス":
			e.value.SetOptions(operationOptions("マウス"))
		case "ランチャー":
			tools.Hide()
			e.launch.box.Show()
			valueForm.Hide()
		}
		e.validate()
	})
	e.clear = widget.NewButton("無効化を設定（USB 反映が必要）", func() {
		if s.busy {
			return
		}
		q, err := s.p.WithBinding(protocol.Binding{Layer: layer, Slot: slot, Kind: "keyboard", Value: "none"})
		if err != nil {
			s.fail(err)
			return
		}
		s.p = q
		s.activateSelection(layer, slot)
		s.changed()
	})
	e.remove = widget.NewButton("送信対象から除外（実機は保持）", func() {
		if s.busy {
			return
		}
		q := s.p.Clone()
		q.Remove(layer, slot)
		s.p = q
		s.activateSelection(layer, slot)
		s.changed()
	})
	e.reset = widget.NewButton("入力を戻す", func() {
		if !s.busy {
			s.activateSelection(layer, slot)
		}
	})
	management := widget.NewButton("割り当ての管理…", func() {
		d := dialog.NewCustom("割り当ての管理", "閉じる", container.NewVBox(e.clear, e.remove, wrappedLabel("除外は実機の設定を保持します。無効化は USB へ無操作のデータを送ります。転送後は実機で動作を確認してください。")), s.w)
		d.Resize(fyne.NewSize(520, 250))
		d.Show()
	})
	details := widget.NewButton("内部データ…", func() { dialog.ShowCustom("割り当ての内部データ", "閉じる", e.details, s.w) })
	e.box = container.NewVBox(widget.NewLabelWithStyle(fmt.Sprintf("%s の編集 · レイヤー %d", s.inputName(slot), layer), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewForm(widget.NewFormItem("操作の種類", e.kind)), valueForm, e.validation, tools, e.launch.box, container.NewHBox(management, details, e.reset))

	if ok {
		e.kind.SetSelected(map[string]string{"keyboard": "キーボード", "media": "メディア", "mouse": "マウス", "launcher": "ランチャー"}[existing.Kind])
		value := existing.Value
		if existing.Kind == "media" || existing.Kind == "mouse" {
			if label, ok := operationLabels[value]; ok {
				value = label
			}
		}
		e.value.SetText(value)
	} else {
		e.kind.SetSelectedIndex(0)
	}
	e.original = e.binding()
	e.initializing = false
	e.value.OnChanged = func(string) { e.validate() }
	e.value.OnSubmitted = func(string) { e.validate() }
	e.launch.onChanged = e.validate
	return e
}
func (e *bindingEditor) binding() protocol.Binding {
	if e.kind.Selected == "ランチャー" {
		spec := e.launch.spec()
		return protocol.Binding{Layer: e.layer, Slot: e.slot, Kind: "launcher", Value: e.launch.label.Text, Launch: &spec}
	}
	return protocol.Binding{Layer: e.layer, Slot: e.slot, Kind: map[string]string{"キーボード": "keyboard", "メディア": "media", "マウス": "mouse"}[e.kind.Selected], Value: operationValue(strings.TrimSpace(e.value.Text))}
}
func (e *bindingEditor) validate() {
	if e.initializing || e.s.busy {
		return
	}
	binding := e.binding()
	e.details.SetText(fmt.Sprintf("スロット: %d (%s)\n種類: %s / 値: %s", binding.Slot, protocol.SlotName(binding.Slot), binding.Kind, binding.Value))
	e.dirty = !reflect.DeepEqual(e.original, binding)
	q, err := e.s.p.WithBinding(binding)
	e.valid = err == nil
	text := "有効な入力は自動反映します。"
	if e.dirty && err == nil {
		e.original = binding
		e.dirty = false
		e.s.p = q
		e.s.changed()
		text = "作業中の設定に反映済み。保存・USB 反映は下部から。"
	} else if e.dirty {
		text = "未反映: " + err.Error()
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
	for _, c := range []fyne.Disableable{e.kind, e.value, e.record, e.reset, e.remove, e.clear, e.search, e.launch.mode, e.launch.command, e.launch.label, e.launch.class, e.launch.dir, e.launch.reuse, e.launch.choose} {
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
	if !e.dirty {
		e.reset.Hide()
	} else {
		e.reset.Show()
	}
	if _, ok := e.s.p.Get(e.layer, e.slot); !ok {
		e.remove.Disable()
	}
}
