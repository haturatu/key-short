package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
