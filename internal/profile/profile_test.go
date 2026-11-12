package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"key-short/internal/layout"
	"key-short/internal/protocol"
)

func TestSparseRoundTrip(t *testing.T) {
	p := New()
	p.Set(protocol.Binding{Layer: 1, Slot: 18, Kind: "media", Value: "mute"})
	jobs, err := p.Jobs()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs: %v %v", jobs, err)
	}
	path := filepath.Join(t.TempDir(), "nested", "profile.json")
	if err = p.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := q.Get(1, 18)
	if !ok || b.Value != "mute" || q.LED != nil {
		t.Fatalf("round trip: %+v", q)
	}
	q.Remove(1, 18)
	jobs, err = q.Jobs()
	if err != nil || len(jobs) != 0 {
		t.Fatal("removal must not send a clear command")
	}
}

func TestRejectBadJSON(t *testing.T) {
	for _, data := range []string{
		`null`, `{}`, `{"version":2,"device":"1189:8890"}`, `{"version":1,"device":"1189:8840"}`,
		`{"version":1,"device":"1189:8890","led":3}`, `{"version":1,"device":"1189:8890","unknown":true}`,
		`{"version":1,"device":"1189:8890","bindings":[{"layer":1,"slot":1,"kind":"keyboard","value":"a"},{"layer":1,"slot":1,"kind":"keyboard","value":"b"}]}`,
		`{"version":1,"device":"1189:8890","bindings":[{"layer":1,"slot":19,"kind":"keyboard","value":"a"}]}`,
		`{"version":1,"device":"1189:8890"} {}`, `{"version":1,"device":"1189:8890"} garbage`,
	} {
		if _, err := Decode(strings.NewReader(data)); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
}

func TestInvalidSavePreservesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	p := New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "keyboard", Value: "invalid"})
	if err := p.Save(path); err == nil {
		t.Fatal("accepted invalid binding")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatal("overwrote existing file")
	}
}

func TestLauncherRejectsConflictingKeyboardChord(t *testing.T) {
	p := New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "chromium"}})
	p.Set(protocol.Binding{Layer: 1, Slot: 2, Kind: "keyboard", Value: protocol.Trigger(1, 1)})
	if _, err := p.Jobs(); err == nil {
		t.Fatal("keyboard action can trigger a launcher")
	}
}

func TestLayoutRetainsHiddenBindingsButRefusesUSBJobs(t *testing.T) {
	p := New()
	p.Set(protocol.Binding{Layer: 1, Slot: 12, Kind: "keyboard", Value: "a"})
	l := layout.Presets()[0].Layout
	p.Layout = &l
	path := filepath.Join(t.TempDir(), "small.json")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.Get(1, 12); !ok {
		t.Fatal("layout switch lost binding")
	}
	if _, err := q.Jobs(); err == nil {
		t.Fatal("sent hidden slot")
	}
	q.Layout = nil
	if _, err := q.Jobs(); err != nil {
		t.Fatal("legacy layout rejected", err)
	}
}
func TestCloneAndCandidateIsolation(t *testing.T) {
	p := New()
	led := 1
	p.LED = &led
	l := layout.Default()
	p.Layout = &l
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "chromium"}})
	q := p.Clone()
	*q.LED = 2
	q.Layout.Slots[0] = 12
	q.Bindings[0].Launch.Command = "changed"
	if *p.LED != 1 || p.Layout.Slots[0] != 1 || p.Bindings[0].Launch.Command != "chromium" {
		t.Fatal("clone aliases original")
	}
	if _, err := p.WithBinding(protocol.Binding{Layer: 1, Slot: 2, Kind: "keyboard", Value: protocol.Trigger(1, 1)}); err == nil {
		t.Fatal("candidate conflict accepted")
	}
	if len(p.Bindings) != 1 {
		t.Fatal("rejected candidate mutated profile")
	}
	q, err := p.WithBinding(protocol.Binding{Layer: 1, Slot: 2, Kind: "keyboard", Value: "ctrl+pageup"})
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Bindings) != 2 || len(p.Bindings) != 1 {
		t.Fatal("candidate was not isolated")
	}
}
func TestCustomLayoutRoundTripAndInvalidMapping(t *testing.T) {
	p := New()
	l := layout.Presets()[1].Layout
	l.Slots = []int{3, 6, 2, 5, 1, 4}
	p.Layout = &l
	p.Set(protocol.Binding{Layer: 1, Slot: 13, Kind: "media", Value: "mute"})
	path := filepath.Join(t.TempDir(), "layout.json")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.Layout.Slots[0] != 3 || q.Layout.Knobs[0].Left != 13 {
		t.Fatal("mapping lost")
	}
	jobs, err := q.Jobs()
	if err != nil || len(jobs) != 1 {
		t.Fatal(err)
	}
	l.Slots = []int{1, 1}
	if p.Validate() == nil {
		t.Fatal("duplicate mapping accepted")
	}
}
func TestSaveReplacementAndFailedRenameCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	p := New()
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "occupied")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "sentinel")
	if err := os.WriteFile(sentinel, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if p.Save(target) == nil {
		t.Fatal("rename to directory succeeded")
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "original" {
		t.Fatal("failed save destroyed target")
	}
	temps, _ := filepath.Glob(filepath.Join(dir, ".key-short-*.json"))
	if len(temps) != 0 {
		t.Fatal("temporary files leaked")
	}
}

func TestRelocateLauncherClearsSourceAndPreservesLaunchOptions(t *testing.T) {
	p := New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Value: "Chromium debug", Launch: &protocol.LaunchSpec{Mode: "command", Command: `chromium --remote-debugging-port=9222 --user-data-dir="/tmp/debug profile"`, ReuseWindow: true}})
	q, err := p.RelocateBinding(1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := q.Get(1, 1)
	target, _ := q.Get(1, 2)
	if source.Kind != "keyboard" || source.Value != "none" || source.Launch != nil {
		t.Fatal("source not explicitly disabled", source)
	}
	original, _ := p.Get(1, 1)
	if target.Kind != "launcher" || target.Value != original.Value || target.Launch.Command != original.Launch.Command || !target.Launch.ReuseWindow {
		t.Fatal("launch options lost", target)
	}
	if _, err := q.Jobs(); err != nil {
		t.Fatal(err)
	}
	q.Bindings[1].Launch.Command = "changed"
	if original.Launch.Command == "changed" {
		t.Fatal("move aliases original")
	}
	if len(p.Bindings) != 1 || original.Kind != "launcher" {
		t.Fatal("move mutated original")
	}
}
func TestRelocateSwapsBindingsOnlyInCurrentLayer(t *testing.T) {
	p := New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "keyboard", Value: "ctrl+c"})
	p.Set(protocol.Binding{Layer: 1, Slot: 13, Kind: "media", Value: "mute"})
	p.Set(protocol.Binding{Layer: 2, Slot: 1, Kind: "keyboard", Value: "b"})
	q, err := p.RelocateBinding(1, 1, 13)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := q.Get(1, 1)
	b, _ := q.Get(1, 13)
	other, _ := q.Get(2, 1)
	if a.Kind != "media" || a.Value != "mute" || b.Value != "ctrl+c" || other.Value != "b" || len(q.Bindings) != 3 {
		t.Fatal("swap lost or changed assignments", q)
	}
}
func TestRelocateRejectsNewLauncherReservationConflict(t *testing.T) {
	p := New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "chromium"}})
	p.Set(protocol.Binding{Layer: 1, Slot: 3, Kind: "keyboard", Value: protocol.Trigger(1, 2)})
	if _, err := p.Jobs(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RelocateBinding(1, 1, 2); err == nil {
		t.Fatal("move accepted a conflicting new trigger")
	}
	if len(p.Bindings) != 2 {
		t.Fatal("rejected move mutated profile")
	}
	original, _ := p.Get(1, 1)
	if original.Kind != "launcher" {
		t.Fatal("rejected move erased source")
	}
	if _, err := p.RelocateBinding(1, 2, 3); err == nil {
		t.Fatal("empty source moved")
	}
	l := layout.Presets()[0].Layout
	p.Layout = &l
	if _, err := p.RelocateBinding(1, 1, 12); err == nil {
		t.Fatal("hidden destination accepted")
	}
}
