package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/device"
	devlayout "key-short/internal/layout"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

func TestEditorValidatesWholeProfile(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "chromium"}})
	s.activateSelection(1, 2)
	s.editor.value.SetText(protocol.Trigger(1, 1))
	if s.editor.valid || !strings.Contains(s.editor.validation.Text, "予約") {
		t.Fatal("whole-profile conflict not shown", s.editor.validation.Text)
	}
	if _, ok := s.p.Get(1, 2); ok {
		t.Fatal("draft stored before apply")
	}
	s.editor.value.SetText("ctrl+pageup")
	if !s.editor.valid {
		t.Fatal("valid chord rejected")
	}
	if s.hasDraft() {
		t.Fatal("applied draft still dirty")
	}
}
func TestSafeSaveRevalidatesSnapshotAndKeepsUSBStateIndependent(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := s.p.Clone()
	s.p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "keyboard", Value: "bad"})
	if s.saveSnapshot(path, snapshot) == nil {
		t.Fatal("invalid profile saved")
	}
	s.p = profile.New()
	s.p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "keyboard", Value: "a"})
	if s.saveSnapshot(path, snapshot) == nil {
		t.Fatal("stale snapshot saved")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("failed save truncated original")
	}
	s.activateSelection(1, 1)
	s.editor.value.SetText("invalid-key")
	if s.saveSnapshot(path, s.p.Clone()) == nil {
		t.Fatal("unapplied draft saved")
	}
	data, _ = os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("draft save truncated original")
	}
	s.editor.value.SetText("b")
	if err := s.saveSnapshot(path, s.p.Clone()); err != nil {
		t.Fatal(err)
	}
	if s.dirty || s.jsonState.Text != "JSON 保存済み" || s.usbSignature != "" {
		t.Fatal("save altered USB state or remains dirty")
	}
	s.usbSignature = wireSignature(s.p)
	s.usbTarget = s.devices.Selected
	s.refresh()
	if !strings.Contains(s.usbState.Text, "実機動作未確認") {
		t.Fatal("transfer overstated hardware verification")
	}
	s.editor.value.SetText("c")
	if !s.dirty || !strings.Contains(s.usbState.Text, "未反映") {
		t.Fatal("applied change did not update independent states")
	}
}
func TestLayoutRestoreAndHiddenBindingProtection(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	info := device.Info{Bus: 2, Address: 3, Location: "test-port", Serial: "shared"}
	s.deviceInfos = []device.Info{info}
	s.devices.Options = []string{info.ID()}
	l := devlayout.Presets()[1].Layout
	l.Preset = "custom"
	l.Slots = []int{3, 6, 2, 5, 1, 4}
	data, _ := json.Marshal(l)
	a.Preferences().SetString("layout-"+info.LayoutKey(), string(data))
	s.devices.SetSelected(info.ID())
	if s.p.Layout == nil || s.p.Layout.Slots[0] != 3 || s.p.Layout.Knobs[0].Left != 13 {
		t.Fatal("remembered layout not restored")
	}
	s.p.Set(protocol.Binding{Layer: 1, Slot: 12, Kind: "keyboard", Value: "a"})
	s.applyLayout(devlayout.Presets()[0].Layout)
	if _, ok := s.p.Get(1, 12); !ok {
		t.Fatal("layout switch destroyed binding")
	}
	if !s.writeButton.Disabled() {
		t.Fatal("hidden binding can be sent")
	}
	path := filepath.Join(t.TempDir(), "hidden.json")
	if err := s.saveSnapshot(path, s.p.Clone()); err != nil {
		t.Fatal("retained binding cannot be backed up", err)
	}
	if err := s.loadProfile(path); err != nil {
		t.Fatal(err)
	}
	if len(s.p.EffectiveLayout().Slots) != 3 {
		t.Fatal("JSON layout not restored")
	}
	s.applyLayout(devlayout.Default())
	if _, err := s.p.Jobs(); err != nil {
		t.Fatal("restored layout still invalid", err)
	}
}
func TestWireSignatureIgnoresLauncherMetadataAndDisplayOrder(t *testing.T) {
	p := profile.New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "chromium"}})
	q := p.Clone()
	q.Bindings[0].Launch.Command = "chromium --remote-debugging-port=9222"
	l := devlayout.Default()
	l.Slots[0], l.Slots[1] = l.Slots[1], l.Slots[0]
	q.Layout = &l
	if wireSignature(p) != wireSignature(q) || profileSignature(p) == profileSignature(q) {
		t.Fatal("file and USB signatures conflated")
	}
}

func TestJapaneseOperationLabelsKeepProtocolValues(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.editor.kind.SetSelected("メディア")
	s.editor.value.SetText("音量を上げる")
	b, ok := s.p.Get(1, 1)
	if !ok || b.Kind != "media" || b.Value != "volume_up" {
		t.Fatal("Japanese label leaked into protocol", b)
	}
	s.activateSelection(1, 1)
	if s.editor.value.Text != "音量を上げる" {
		t.Fatal("editor displayed internal operation name")
	}
	test.Tap(s.buttons[1])
	s.editor.kind.SetSelected("マウス")
	s.editor.value.SetText("左クリック")
	b, ok = s.p.Get(1, 2)
	if !ok || b.Kind != "mouse" || b.Value != "left_click" {
		t.Fatal("mouse label did not map to protocol", b)
	}
}

func TestOperationSearchUsesJapaneseAndAppliesChosenCategory(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	s := newStudio(a)
	defer s.shutdown()
	s.w.Show()
	s.editor.searchOperations()
	var search *widget.Entry
	var list *widget.List
	visitWidgets(s.w.Canvas().Overlays().Top(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Entry); ok && v.PlaceHolder == "操作名・キー名で検索…" {
			search = v
		}
		if v, ok := o.(*widget.List); ok {
			list = v
		}
	})
	if search == nil || list == nil {
		t.Fatal("search controls missing")
	}
	search.SetText("音量を上げる")
	if list.Length() != 1 {
		t.Fatal("Japanese search failed", list.Length())
	}
	list.Select(0)
	b, ok := s.p.Get(1, 1)
	if !ok || b.Kind != "media" || b.Value != "volume_up" || s.hasDraft() {
		t.Fatal("search selection failed", b)
	}
}
