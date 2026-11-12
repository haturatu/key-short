// Package layout maps display positions to fixed slots in the 1189:8890 protocol.
// A preset describes a user-selected layout, not a detected hardware model.
package layout

import "fmt"

type Knob struct {
	Left  int `json:"left"`
	Press int `json:"press"`
	Right int `json:"right"`
}
type Layout struct {
	Preset  string `json:"preset"`
	Columns int    `json:"columns"`
	Slots   []int  `json:"slots"`
	Knobs   []Knob `json:"knobs"`
}
type Preset struct {
	ID, Name string
	Layout   Layout
}

func Presets() []Preset {
	return []Preset{
		makePreset("mini-3", "Mini 3 · 3キー / 1ノブ", 3, 3, 1),
		makePreset("mini-6", "Mini 6 · 6キー / 1ノブ", 2, 6, 1),
		makePreset("mini-9", "Mini 9 · 9キー / 2ノブ", 3, 9, 2),
		makePreset("mini-12", "Mini 12 · 12キー / 2ノブ", 4, 12, 2),
		makePreset("mini-12-vertical", "Mini 12（縦配置）· 12キー / 2ノブ", 3, 12, 2),
	}
}
func makePreset(id, name string, columns, keys, knobs int) Preset {
	l := Layout{Preset: id, Columns: columns, Slots: []int{}, Knobs: []Knob{}}
	for i := 1; i <= keys; i++ {
		l.Slots = append(l.Slots, i)
	}
	// The knob slots stay 13–18 even on a three-key unit.
	for i := 0; i < knobs; i++ {
		l.Knobs = append(l.Knobs, Knob{13 + i*3, 14 + i*3, 15 + i*3})
	}
	return Preset{id, name, l}
}
func Default() Layout { return Presets()[4].Layout }
func (l Layout) Clone() Layout {
	q := l
	if l.Slots != nil {
		q.Slots = make([]int, len(l.Slots))
		copy(q.Slots, l.Slots)
	}
	if l.Knobs != nil {
		q.Knobs = make([]Knob, len(l.Knobs))
		copy(q.Knobs, l.Knobs)
	}
	return q
}
func (l Layout) Contains(slot int) bool {
	for _, s := range l.Slots {
		if slot == s {
			return true
		}
	}
	for _, k := range l.Knobs {
		if slot == k.Left || slot == k.Press || slot == k.Right {
			return true
		}
	}
	return false
}
func (l Layout) Validate() error {
	if l.Columns < 1 || l.Columns > 12 || len(l.Slots) < 1 || len(l.Slots) > 12 || len(l.Knobs) > 2 {
		return fmt.Errorf("配置は1〜12キー、0〜2ノブ、1〜12列で指定してください")
	}
	known := l.Preset == "custom"
	for _, p := range Presets() {
		if l.Preset == p.ID {
			known = true
		}
	}
	if !known {
		return fmt.Errorf("不明な配置プリセット: %s", l.Preset)
	}
	seen := map[int]bool{}
	for _, s := range l.Slots {
		if s < 1 || s > 12 || seen[s] {
			return fmt.Errorf("キーのスロットは1〜12で重複なく指定してください")
		}
		seen[s] = true
	}
	for _, k := range l.Knobs {
		for _, s := range []int{k.Left, k.Press, k.Right} {
			if s < 13 || s > 18 || seen[s] {
				return fmt.Errorf("ノブのスロットは13〜18で重複なく指定してください")
			}
			seen[s] = true
		}
	}
	return nil
}
