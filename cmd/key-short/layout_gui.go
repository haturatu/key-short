package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

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
	var arrangement fyne.CanvasObject = container.NewVBox(grid, knobs)
	switch l.KnobPosition {
	case "top":
		arrangement = container.NewVBox(knobs, grid)
	case "left":
		arrangement = container.NewGridWithColumns(2, knobs, grid)
	case "right":
		arrangement = container.NewGridWithColumns(2, grid, knobs)
	}
	s.keyHost.Objects = []fyne.CanvasObject{arrangement}
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
		dialog.ShowCustomConfirm("入力エラーがある変更", "破棄して配置を変更", "編集を続ける", widget.NewLabel("無効な入力を破棄して配置を変えますか？ 最後の有効な設定は保持されます。"), func(ok bool) {
			if ok {
				s.activateSelection(s.layer, s.selectedSlot)
				s.confirmLayout(l)
			}
		}, s.w)
		return
	}
	s.confirmLayout(l)
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
	s.resetHistory()
	s.dirty = profileSignature(s.p) != s.savedSignature
	s.rebuildLayout()
	s.activateSelection(s.layer, l.Slots[0])
	if s.layoutNote != nil {
		s.layoutNote.SetText("この接続位置の保存済み配置を復元しました。機種の自動判定ではありません。")
	}
}
func (s *studio) installProfile(p profile.Profile, path string) {
	s.pauseMonitoring("JSON 読み込みのため一時停止中")
	path, _ = filepath.Abs(path)
	s.p = p.Clone()
	s.selectedSlots = map[int]bool{}
	s.profilePath = path
	s.savedSignature = profileSignature(s.p)
	s.resetHistory()
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
