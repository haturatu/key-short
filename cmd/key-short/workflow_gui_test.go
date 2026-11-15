package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	devlayout "key-short/internal/layout"
	"key-short/internal/protocol"
)

func TestLayoutWarningListsAllLayersAndOffersRecovery(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.p.Set(protocol.Binding{Layer: 2, Slot: 12, Kind: "keyboard", Value: "ctrl+c"})
	s.changed()
	before := profileSignature(s.p)
	s.requestLayout(devlayout.Presets()[0].Layout)
	if profileSignature(s.p) != before {
		t.Fatal("layout changed before warning")
	}
	test.Tap(dialogButton(t, s, "キャンセル"))
	if profileSignature(s.p) != before {
		t.Fatal("canceled warning changed profile")
	}
	s.requestLayout(devlayout.Presets()[0].Layout)
	test.Tap(dialogButton(t, s, "保持して配置を変更"))
	if len(s.hiddenBindings(s.p.EffectiveLayout())) != 1 || !s.writeButton.Disabled() {
		t.Fatal("hidden binding was lost or send is enabled")
	}
	s.hiddenAssignmentsDialog()
	test.Tap(dialogButton(t, s, "全キーを表示する"))
	if len(s.hiddenBindings(s.p.EffectiveLayout())) != 0 {
		t.Fatal("recovery did not restore hidden slot")
	}
	s.requestLayout(devlayout.Presets()[0].Layout)
	test.Tap(dialogButton(t, s, "送信対象から除外して変更（実機は保持）"))
	if len(s.p.Bindings) != 0 {
		t.Fatal("exclude did not remove binding")
	}
	s.undo()
	if b, ok := s.p.Get(2, 12); !ok || b.Value != "ctrl+c" {
		t.Fatal("undo did not restore excluded binding")
	}
}
func TestMonitoringPauseReasonSurvivesRefresh(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	stopped := false
	s.monitorStop = func() { stopped = true }
	s.editor.value.SetText("ctrl+c")
	s.devices.ClearSelected()
	s.refreshMonitorState()
	s.refresh()
	if !stopped || !strings.Contains(s.monitorStatus.Text, "設定変更") || !strings.Contains(s.monitorButton.Text, "再開") {
		t.Fatal("pause reason or recovery was lost", s.monitorStatus.Text)
	}
	if !strings.Contains(s.nextAction.Text, "JSON") || !strings.Contains(s.connectionState.Text, "オフライン") {
		t.Fatal("next action or offline edit not visible", s.nextAction.Text, s.connectionState.Text)
	}
}

func TestOmittingAndDisablingAssignmentsHaveDifferentTransferData(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.editor.value.SetText("ctrl+c")
	test.Tap(s.editor.remove)
	if jobs, err := s.p.Jobs(); err != nil || len(jobs) != 0 {
		t.Fatal("exclude generated a hardware clear", jobs, err)
	}
	s.undo()
	test.Tap(s.editor.clear)
	b, ok := s.p.Get(1, 1)
	if !ok || b.Value != "none" {
		t.Fatal("disable omitted the slot instead of sending none")
	}
	if jobs, err := s.p.Jobs(); err != nil || len(jobs) != 1 {
		t.Fatal("disable did not generate transfer", err)
	}
}

func TestEmptyOfflineProfileGuidesEditingBeforeSaving(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.devices.Options = nil
	s.devices.ClearSelected()
	s.refresh()
	if s.nextAction.Text != "割り当てを設定" {
		t.Fatal("empty profile suggests saving before editing", s.nextAction.Text)
	}
	test.Tap(s.nextAction)
	if s.w.Canvas().Focused() != s.editor.value {
		t.Fatal("next action did not focus editor")
	}
	s.editor.value.SetText("ctrl+c")
	if !strings.Contains(s.nextAction.Text, "JSON") {
		t.Fatal("valid offline edit did not guide saving")
	}
}
