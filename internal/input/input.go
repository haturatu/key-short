package input

import (
	"fmt"
	"key-short/internal/protocol"
)

// Code uses Linux's stable F13..F24 numbers on every OS; native events are
// translated before detection so existing profiles remain compatible.
type Chord struct {
	Modifier byte
	Code     uint16
}
type Detector struct {
	mods     byte
	dropping bool
}

var modifiers = map[uint16]byte{29: 1, 42: 2, 56: 4, 125: 8, 97: 16, 54: 32, 100: 64, 126: 128}

func (d *Detector) Feed(kind, code uint16, value int32) (Chord, bool) {
	if kind == 0 && code == 3 {
		d.mods = 0
		d.dropping = true
		return Chord{}, false
	}
	if d.dropping {
		if kind == 0 && code == 0 {
			d.dropping = false
		}
		return Chord{}, false
	}
	if kind != 1 {
		return Chord{}, false
	}
	if mod, ok := modifiers[code]; ok {
		if value == 0 {
			d.mods &^= mod
		} else if value == 1 {
			d.mods |= mod
		}
		return Chord{}, false
	}
	if value != 1 || code < 183 || code > 194 {
		return Chord{}, false
	}
	return Chord{d.mods, code}, true
}
func ForBinding(b protocol.Binding) (Chord, error) {
	if b.Kind != "launcher" {
		return Chord{}, fmt.Errorf("ランチャーの割り当てではありません")
	}
	mod, key, err := protocol.Stroke(protocol.Trigger(b.Layer, b.Slot))
	if err != nil {
		return Chord{}, err
	}
	return Chord{mod, uint16(key) - 0x68 + 183}, nil
}
