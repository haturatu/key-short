package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
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
	if !s.editor.apply.Disabled() || !strings.Contains(s.editor.validation.Text, "予約") {
		t.Fatal("whole-profile conflict not shown", s.editor.validation.Text)
	}
	if _, ok := s.p.Get(1, 2); ok {
		t.Fatal("draft stored before apply")
	}
	s.editor.value.SetText("ctrl+pageup")
	if s.editor.apply.Disabled() {
		t.Fatal("valid chord rejected")
	}
	test.Tap(s.editor.apply)
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
	s.editor.value.SetText("b")
	if s.saveSnapshot(path, s.p.Clone()) == nil {
		t.Fatal("unapplied draft saved")
	}
	data, _ = os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("draft save truncated original")
	}
	test.Tap(s.editor.apply)
	if err := s.saveSnapshot(path, s.p.Clone()); err != nil {
		t.Fatal(err)
	}
	if s.dirty || s.jsonState.Text != "JSON 保存済み" || s.usbSignature != "" {
		t.Fatal("save altered USB state or remains dirty")
	}
	s.usbSignature = wireSignature(s.p)
	s.usbTarget = s.devices.Selected
	s.refresh()
	if !strings.Contains(s.usbState.Text, "実機未検証") {
		t.Fatal("transfer overstated hardware verification")
	}
	s.editor.value.SetText("c")
	test.Tap(s.editor.apply)
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
