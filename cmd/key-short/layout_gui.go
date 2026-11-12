package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	devlayout "key-short/internal/layout"
	"key-short/internal/profile"
)

func (s *studio) rebuildLayout() {
	l := s.p.EffectiveLayout()
	grid := container.NewGridWithColumns(l.Columns)
	for _, slot := range l.Slots {
		grid.Add(s.buttons[slot-1])
	}
	knobs := container.NewVBox()
	for i, k := range l.Knobs {
		knobs.Add(container.NewVBox(widget.NewLabelWithStyle(fmt.Sprintf("ノブ %d · 左 / 押す / 右", i+1), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewGridWithColumns(3, s.buttons[k.Left-1], s.buttons[k.Press-1], s.buttons[k.Right-1])))
	}
	s.keyHost.Objects = []fyne.CanvasObject{grid, knobs}
	s.keyHost.Refresh()
	s.syncLayoutSelect()
}
func (s *studio) syncLayoutSelect() {
	l := s.p.EffectiveLayout()
	name := "カスタム配置…"
	// A preset name is used only for its exact mapping. A modified mapping is custom.
	for _, p := range devlayout.Presets() {
		a, _ := json.Marshal(l)
		b, _ := json.Marshal(p.Layout)
		if string(a) == string(b) {
			name = p.Name
		}
	}
	s.loading = true
	s.layouts.SetSelected(name)
	s.loading = false
}
func (s *studio) applyLayout(l devlayout.Layout) {
	copy := l.Clone()
	s.p.Layout = &copy
	if !l.Contains(s.selectedSlot) {
		s.selectedSlot = l.Slots[0]
	}
	s.rebuildLayout()
	s.activateSelection(s.layer, s.selectedSlot)
	s.changed()
	s.rememberLayout()
	s.status.SetText("表示配置を変更しました。既存の割り当てを保持しています。USB には送信していません。")
	if _, err := s.p.Jobs(); err != nil {
		s.status.SetText(err.Error())
	}
}
func (s *studio) requestLayout(l devlayout.Layout) {
	s.syncLayoutSelect()
	if s.busy {
		return
	}
	if err := l.Validate(); err != nil {
		s.fail(err)
		return
	}
	if s.hasDraft() {
		dialog.ShowCustomConfirm("キーに未適用の変更", "破棄して配置を変更", "編集を続ける", widget.NewLabel("キーの編集中の変更を破棄して、表示配置を変えますか？"), func(ok bool) {
			if ok {
				s.applyLayout(l)
			}
		}, s.w)
		return
	}
	s.applyLayout(l)
}
func (s *studio) layoutDialog() {
	s.syncLayoutSelect()
	if s.busy {
		return
	}
	l := s.p.EffectiveLayout()
	cols := widget.NewEntry()
	cols.SetText(strconv.Itoa(l.Columns))
	slots := widget.NewEntry()
	parts := []string{}
	for _, n := range l.Slots {
		parts = append(parts, strconv.Itoa(n))
	}
	slots.SetText(strings.Join(parts, ","))
	slots.SetPlaceHolder("例: 3,6,2,5,1,4（左上から行順）")
	knobs := widget.NewEntry()
	parts = nil
	for _, k := range l.Knobs {
		parts = append(parts, fmt.Sprintf("%d,%d,%d", k.Left, k.Press, k.Right))
	}
	knobs.SetText(strings.Join(parts, ";"))
	knobs.SetPlaceHolder("13,14,15;16,17,18（各ノブの左,押す,右）")
	preview := widget.NewLabel("")
	preview.Wrapping = fyne.TextWrapWord
	validation := widget.NewLabel("")
	validation.Wrapping = fyne.TextWrapWord
	parse := func() (devlayout.Layout, error) {
		n, err := strconv.Atoi(strings.TrimSpace(cols.Text))
		if err != nil {
			return devlayout.Layout{}, fmt.Errorf("列数を数値で指定してください")
		}
		out := devlayout.Layout{Preset: "custom", Columns: n, Slots: []int{}, Knobs: []devlayout.Knob{}}
		numbers := func(text string) ([]int, error) {
			ns := []int{}
			for _, word := range strings.Split(text, ",") {
				v, e := strconv.Atoi(strings.TrimSpace(word))
				if e != nil {
					return nil, fmt.Errorf("スロット番号をカンマで区切って指定してください")
				}
				ns = append(ns, v)
			}
			return ns, nil
		}
		out.Slots, err = numbers(slots.Text)
		if err != nil {
			return out, err
		}
		if strings.TrimSpace(knobs.Text) != "" {
			for _, group := range strings.Split(knobs.Text, ";") {
				ns, e := numbers(group)
				if e != nil || len(ns) != 3 {
					return out, fmt.Errorf("ノブは左,押す,右の3番号で指定し、ノブ同士は ; で区切ってください")
				}
				out.Knobs = append(out.Knobs, devlayout.Knob{Left: ns[0], Press: ns[1], Right: ns[2]})
			}
		}
		return out, out.Validate()
	}
	var d dialog.Dialog
	apply := widget.NewButton("この配置を使う", func() {
		out, err := parse()
		if err != nil {
			validation.SetText(err.Error())
			return
		}
		d.Hide()
		s.requestLayout(out)
	})
	update := func(string) {
		out, err := parse()
		if err != nil {
			validation.SetText(err.Error())
			preview.SetText("")
			apply.Disable()
			return
		}
		validation.SetText("")
		apply.Enable()
		var rows []string
		for i := 0; i < len(out.Slots); i += out.Columns {
			end := i + out.Columns
			if end > len(out.Slots) {
				end = len(out.Slots)
			}
			row := []string{}
			for _, n := range out.Slots[i:end] {
				row = append(row, fmt.Sprintf("K%d", n))
			}
			rows = append(rows, strings.Join(row, "  "))
		}
		preview.SetText(strings.Join(rows, "\n"))
	}
	cols.OnChanged = update
	slots.OnChanged = update
	knobs.OnChanged = update
	note := widget.NewLabel("表示順と物理スロット番号を調整します。既存の割り当ては移動・削除されません。識別用のキー割り当てを実機へ書き込む操作は行いません。")
	note.Wrapping = fyne.TextWrapWord
	d = dialog.NewCustom("実機との配置を確認・調整", "キャンセル", container.NewVBox(note, widget.NewForm(widget.NewFormItem("列数", cols), widget.NewFormItem("キーの順序", slots), widget.NewFormItem("ノブ", knobs)), preview, validation, apply), s.w)
	d.Resize(fyne.NewSize(760, 500))
	update("")
	d.Show()
}
func (s *studio) selectedDeviceKey() string {
	for _, info := range s.deviceInfos {
		if info.ID() == s.devices.Selected {
			return info.LayoutKey()
		}
	}
	return ""
}
func (s *studio) rememberLayout() {
	key := s.selectedDeviceKey()
	if key == "" {
		return
	}
	data, _ := json.Marshal(s.p.EffectiveLayout())
	s.app.Preferences().SetString("layout-"+key, string(data))
}
func (s *studio) deviceSelected() {
	if s.layouts == nil {
		return
	}
	key := s.selectedDeviceKey()
	if s.layoutNote != nil {
		s.layoutNote.SetText("機種未確定 · 1189:8890 だけではキー数を判定できません。配置を選択してください。")
	}
	if key == "" {
		return
	}
	if s.profilePath != "" {
		if s.layoutNote != nil {
			s.layoutNote.SetText("JSON に保存された配置を表示中（layout がない場合は従来配置）。")
		}
		return
	}
	data := s.app.Preferences().String("layout-" + key)
	if data == "" {
		return
	}
	var l devlayout.Layout
	if json.Unmarshal([]byte(data), &l) != nil || l.Validate() != nil {
		return
	}
	if s.hasDraft() || len(s.p.Bindings) > 0 || s.dirty {
		if s.layoutNote != nil {
			s.layoutNote.SetText("保存済み配置あり · 編集内容を保護するため自動切替を保留しました。")
		}
		return
	}
	copy := l.Clone()
	s.p.Layout = &copy
	s.dirty = profileSignature(s.p) != s.savedSignature
	s.rebuildLayout()
	s.activateSelection(s.layer, l.Slots[0])
	if s.layoutNote != nil {
		s.layoutNote.SetText("この接続位置の保存済み配置を復元しました。機種の自動判定ではありません。")
	}
}
func (s *studio) installProfile(p profile.Profile, path string) {
	if s.monitorStop != nil {
		s.stopMonitoring()
	}
	path, _ = filepath.Abs(path)
	s.p = p.Clone()
	s.profilePath = path
	s.savedSignature = profileSignature(s.p)
	s.dirty = false
	s.loading = true
	if p.LED == nil {
		s.led.SetSelectedIndex(0)
	} else {
		s.led.SetSelectedIndex(*p.LED + 1)
	}
	s.loading = false
	l := p.EffectiveLayout()
	s.rebuildLayout()
	s.activateSelection(s.layer, l.Slots[0])
	s.refresh()
	s.app.Preferences().SetString("profile-path", path)
	s.deviceSelected()
}
