package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/protocol"
)

var operationLabels = map[string]string{
	"play_pause": "再生・一時停止", "previous": "前の曲", "next": "次の曲", "stop": "再生を停止", "mute": "ミュート", "volume_up": "音量を上げる", "volume_down": "音量を下げる",
	"left_click": "左クリック", "right_click": "右クリック", "middle_click": "中央クリック", "wheel_up": "上へスクロール", "wheel_down": "下へスクロール", "none": "無効化",
}

func operationValue(text string) string {
	for code, label := range operationLabels {
		if text == label {
			return code
		}
	}
	return text
}
func operationOptions(kind string) []string {
	names := sortedNames(protocol.Media)
	if kind == "マウス" {
		names = sortedNames(protocol.Mouse)
	}
	for i, name := range names {
		names[i] = operationLabels[name]
	}
	return names
}
func bindingLabel(b protocol.Binding) string {
	if b.Kind == "launcher" {
		return b.DisplayValue()
	}
	var steps []string
	for _, step := range strings.Split(b.Value, ",") {
		parts := strings.Split(step, "+")
		for i, part := range parts {
			part = strings.TrimSpace(part)
			if label, ok := operationLabels[part]; ok {
				parts[i] = label
			} else {
				switch part {
				case "ctrl":
					parts[i] = "Ctrl"
				case "shift":
					parts[i] = "Shift"
				case "alt":
					parts[i] = "Alt"
				case "super":
					parts[i] = "Super"
				default:
					parts[i] = strings.ToUpper(part)
				}
			}
		}
		steps = append(steps, strings.Join(parts, " + "))
	}
	return strings.Join(steps, " → ")
}
func bindingIcon(b protocol.Binding, assigned bool) fyne.Resource {
	if !assigned {
		return theme.ContentAddIcon()
	}
	if emptyAssignment(b) {
		return theme.CancelIcon()
	}
	switch b.Kind {
	case "media":
		return theme.MediaMusicIcon()
	case "mouse":
		return theme.ComputerIcon()
	case "launcher":
		return theme.ComputerIcon()
	default:
		return theme.DocumentIcon()
	}
}
func (e *bindingEditor) searchOperations() {
	if e.s.busy {
		return
	}
	type operation struct{ category, value, label string }
	all := []operation{}
	for _, category := range []string{"キーボード", "メディア", "マウス"} {
		var names []string
		switch category {
		case "キーボード":
			names = append([]string{"ctrl+c", "ctrl+v", "ctrl+x", "ctrl+z", "ctrl+shift+s", "super+e"}, sortedNames(protocol.Keys)...)
		case "メディア":
			names = sortedNames(protocol.Media)
		case "マウス":
			names = sortedNames(protocol.Mouse)
		}
		for _, value := range names {
			label := bindingLabel(protocol.Binding{Value: value})
			all = append(all, operation{category, value, label})
		}
	}
	filtered := append([]operation(nil), all...)
	search := widget.NewEntry()
	search.SetPlaceHolder("操作名・キー名で検索…")
	var d dialog.Dialog
	list := widget.NewList(func() int { return len(filtered) }, func() fyne.CanvasObject { return widget.NewLabel("") }, func(i widget.ListItemID, o fyne.CanvasObject) {
		op := filtered[i]
		o.(*widget.Label).SetText(fmt.Sprintf("%s · %s", op.category, op.label))
	})
	list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(filtered) || e.s.busy || e.s.editor != e {
			return
		}
		op := filtered[i]
		e.initializing = true
		e.kind.SetSelected(op.category)
		value := op.value
		if label, ok := operationLabels[value]; ok && op.category != "キーボード" {
			value = label
		}
		e.value.SetText(value)
		e.initializing = false
		e.validate()
		d.Hide()
	}
	search.OnChanged = func(query string) {
		list.UnselectAll()
		filtered = nil
		for _, op := range all {
			if strings.Contains(strings.ToLower(op.category+" "+op.label+" "+op.value), strings.ToLower(query)) {
				filtered = append(filtered, op)
			}
		}
		list.Refresh()
	}
	d = dialog.NewCustom("操作を検索", "閉じる", container.NewBorder(search, nil, nil, nil, list), e.s.w)
	d.Resize(fyne.NewSize(560, 440))
	d.Show()
	e.s.w.Canvas().Focus(search)
}

func wrappedLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

func (s *studio) inputName(slot int) string {
	if slot <= 12 {
		return fmt.Sprintf("K%d", slot)
	}
	for i, k := range s.p.EffectiveLayout().Knobs {
		for j, mapped := range []int{k.Left, k.Press, k.Right} {
			if mapped == slot {
				return fmt.Sprintf("ノブ%d %s", i+1, []string{"左", "押す", "右"}[j])
			}
		}
	}
	return protocol.SlotName(slot)
}
