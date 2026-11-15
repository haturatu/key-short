package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	devlayout "key-short/internal/layout"
	"key-short/internal/protocol"
)

func (s *studio) pauseMonitoring(reason string) {
	if s.monitorStop != nil || s.daemonRunning || s.daemonStopPending {
		s.monitorPause = reason
	}
	if s.monitorStop != nil {
		s.stopMonitoring()
	}
	if s.daemonRunning {
		s.daemonStopPending = true
		if !s.autostartBusy {
			s.manageAutostart("stop")
		}
	}
}
func (s *studio) refreshMonitorState() {
	if s.monitorStop != nil {
		return
	}
	text, action := "ランチャー停止中", "ランチャー監視を開始"
	if s.monitorPause != "" {
		text, action = s.monitorPause, "ランチャー監視を再開"
	}
	if s.daemonRunning {
		text, action = "ランチャー監視デーモン実行中", "監視デーモンを停止"
	}
	if s.daemonStopPending {
		text = "設定変更により監視デーモンの停止を要求中"
		if s.daemonStopError != "" {
			text = "監視デーモン停止に失敗: " + s.daemonStopError
		}
	}
	s.monitorStatus.SetText(text)
	s.monitorButton.SetText(action)
}
func (s *studio) hasLauncher() bool {
	for _, b := range s.p.Bindings {
		if b.Kind == "launcher" {
			return true
		}
	}
	return false
}
func (s *studio) refreshWorkflow() {
	if s.nextAction == nil {
		return
	}
	s.editState.SetText("キーの編集: 作業中の設定に反映済み")
	if s.hasDraft() {
		s.editState.SetText("キーの編集: 入力エラーあり・最後の有効な設定を保持")
	}
	s.connectionState.SetText("USB 未接続 · オフラインで設定を編集できます")
	if len(s.devices.Options) > 0 && s.devices.Selected == "" {
		s.connectionState.SetText("USB デバイスを選択してください。選択前もオフライン編集できます。")
	}
	if s.devices.Selected != "" {
		s.connectionState.SetText("USB 接続先: " + s.devices.Selected)
	}
	hidden := s.hiddenBindings(s.p.EffectiveLayout())
	s.hiddenButton.Hide()
	if len(hidden) > 0 {
		s.hiddenButton.SetText(fmt.Sprintf("非表示の割り当て %d 件を確認…", len(hidden)))
		s.hiddenButton.Show()
	}
	focusEditor := func() {
		if s.workspace != nil && s.workspace.narrow {
			s.workspace.tabs.SelectIndex(1)
		}
		s.w.Canvas().Focus(s.editor.value)
	}
	label, action := "USB に反映…", s.write
	switch {
	case s.busy:
		label, action = "USB 転送中…", nil
	case s.hasDraft():
		label, action = "入力エラーを修正する", focusEditor
	case len(hidden) > 0:
		label, action = "非表示の割り当てを確認…", s.hiddenAssignmentsDialog
	case len(s.p.Bindings) == 0 && s.p.LED == nil && !s.dirty && s.profilePath == "":
		label, action = "キーを選んで割り当てを設定", focusEditor
	case s.dirty || s.profilePath == "":
		label, action = "JSON に保存…", s.save
	case s.devices.Selected == "":
		label, action = "USB を接続して再検出", s.scan
		if len(s.devices.Options) > 0 {
			label, action = "USB の接続先を選択する", func() { s.w.Canvas().Focus(s.devices) }
		}
	case s.usbFailed || s.usbTarget != s.devices.Selected || s.usbSignature != wireSignature(s.p):
	case s.hasLauncher() && s.monitorStop == nil && !s.daemonRunning:
		label, action = "ランチャー監視を再開", s.startMonitoring
	default:
		label, action = "実機のキー・ノブで動作を確認してください", nil
	}
	if jobs, err := s.p.Jobs(); err == nil && len(jobs) == 0 && label == "USB に反映…" {
		label, action = "キーを選んで割り当てを設定", nil
	}
	s.nextAction.SetText(label)
	s.nextAction.OnTapped = action
	if action == nil || s.autostartBusy {
		s.nextAction.Disable()
	} else {
		s.nextAction.Enable()
	}
}
func (s *studio) hiddenBindings(l devlayout.Layout) []protocol.Binding {
	var out []protocol.Binding
	for _, b := range s.p.Bindings {
		if !l.Contains(b.Slot) {
			out = append(out, b)
		}
	}
	return out
}
func hiddenDescription(bindings []protocol.Binding) string {
	var lines []string
	for _, b := range bindings {
		lines = append(lines, fmt.Sprintf("レイヤー %d / %s: %s", b.Layer, protocol.SlotName(b.Slot), bindingLabel(b)))
	}
	return strings.Join(lines, "\n")
}
func (s *studio) confirmLayout(l devlayout.Layout) {
	hidden := s.hiddenBindings(l)
	if len(hidden) == 0 {
		s.applyLayout(l)
		return
	}
	signature := profileSignature(s.p)
	var d dialog.Dialog
	apply := func(remove bool) {
		if s.busy || s.hasDraft() || profileSignature(s.p) != signature {
			s.status.SetText("設定が変わりました。配置を選び直してください。")
			return
		}
		if remove {
			for _, b := range hidden {
				s.p.Remove(b.Layer, b.Slot)
			}
		}
		d.Hide()
		s.applyLayout(l)
	}
	note := widget.NewLabel("次の割り当てが配置の外に隠れます。保持すると USB 送信を停止します。除外しても実機の割り当ては消えません。Undo で戻せます。")
	note.Wrapping = fyne.TextWrapWord
	list := widget.NewLabel(hiddenDescription(hidden))
	list.Wrapping = fyne.TextWrapWord
	d = dialog.NewCustom("非表示になる割り当て", "キャンセル", container.NewBorder(note, container.NewVBox(widget.NewButton("保持して配置を変更", func() { apply(false) }), widget.NewButton("送信対象から除外して変更（実機は保持）", func() { apply(true) })), nil, nil, container.NewVScroll(list)), s.w)
	d.Resize(fyne.NewSize(620, 420))
	d.Show()
}
func (s *studio) hiddenAssignmentsDialog() {
	hidden := s.hiddenBindings(s.p.EffectiveLayout())
	if s.busy || len(hidden) == 0 {
		return
	}
	signature := profileSignature(s.p)
	var d dialog.Dialog
	list := widget.NewLabel(hiddenDescription(hidden))
	list.Wrapping = fyne.TextWrapWord
	note := widget.NewLabel("非表示の割り当てがあるため USB 送信を停止しています。全キーを表示するか、送信対象から除外してください。除外しても実機の設定は保持されます。")
	note.Wrapping = fyne.TextWrapWord
	remove := widget.NewButton("一覧を送信対象から除外（実機は保持）", func() {
		if s.busy || s.hasDraft() || profileSignature(s.p) != signature {
			return
		}
		for _, b := range hidden {
			s.p.Remove(b.Layer, b.Slot)
		}
		d.Hide()
		s.activateSelection(s.layer, s.selectedSlot)
		s.changed()
	})
	restore := widget.NewButton("全キーを表示する", func() {
		if !s.busy && !s.hasDraft() && profileSignature(s.p) == signature {
			d.Hide()
			s.applyLayout(devlayout.Default())
		}
	})
	d = dialog.NewCustom("非表示の割り当て", "閉じる", container.NewBorder(note, container.NewVBox(restore, remove), nil, nil, container.NewVScroll(list)), s.w)
	d.Resize(fyne.NewSize(620, 420))
	d.Show()
}
