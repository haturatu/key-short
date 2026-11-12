package layout

import "testing"

func TestPresetSlots(t *testing.T) {
	for _, p := range Presets() {
		if err := p.Layout.Validate(); err != nil {
			t.Fatal(err)
		}
		if !p.Layout.Contains(13) || p.Layout.Knobs[0].Left != 13 {
			t.Fatalf("renumbered knob for %s", p.ID)
		}
	}
	l := Presets()[0].Layout
	if l.Contains(4) || l.Contains(16) {
		t.Fatal("mini3 includes missing controls")
	}
	l.Slots = []int{1, 1}
	if l.Validate() == nil {
		t.Fatal("accepted duplicate")
	}
	l = Default()
	l.Knobs[0].Left = 1
	if l.Validate() == nil {
		t.Fatal("accepted keyboard slot as knob")
	}
	l = Default()
	clone := l.Clone()
	clone.Slots[0] = 12
	if l.Slots[0] != 1 {
		t.Fatal("clone shares slots")
	}
}
