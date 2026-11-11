// Package protocol builds Report ID 3 frames without performing USB I/O.
// The wire format is derived from the supplied MINI KeyBoard.exe.
package protocol

import (
	"fmt"
	"strings"
)

type Frame [65]byte

type Binding struct {
	Layer int    `json:"layer"`
	Slot  int    `json:"slot"`
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Job struct {
	Label  string
	Frames []Frame
}

var Keys = map[string]byte{}
var Modifiers = map[string]byte{"ctrl": 1, "shift": 2, "alt": 4, "super": 8,
	"rctrl": 16, "rshift": 32, "ralt": 64, "rsuper": 128}
var Media = map[string]byte{"play_pause": 0xcd, "previous": 0xb6, "next": 0xb5,
	"stop": 0xb7, "mute": 0xe2, "volume_up": 0xe9, "volume_down": 0xea}
var Mouse = map[string][2]byte{"left_click": {1, 0}, "right_click": {2, 0},
	"middle_click": {4, 0}, "wheel_up": {0, 1}, "wheel_down": {0, 255}}

func init() {
	for i := 0; i < 26; i++ {
		Keys[string(rune('a'+i))] = byte(4 + i)
	}
	for i := 1; i <= 9; i++ {
		Keys[fmt.Sprint(i)] = byte(0x1d + i)
	}
	Keys["0"] = 0x27
	for i, name := range []string{"enter", "esc", "backspace", "tab", "space", "minus", "equal", "leftbrace", "rightbrace", "backslash", "nonushash", "semicolon", "apostrophe", "grave", "comma", "dot", "slash", "capslock"} {
		Keys[name] = byte(0x28 + i)
	}
	for i := 1; i <= 12; i++ {
		Keys[fmt.Sprintf("f%d", i)] = byte(0x39 + i)
	}
	for i, name := range []string{"printscreen", "scrolllock", "pause", "insert", "home", "pageup", "delete", "end", "pagedown", "right", "left", "down", "up", "numlock", "kp_divide", "kp_multiply", "kp_minus", "kp_plus", "kp_enter", "kp_1", "kp_2", "kp_3", "kp_4", "kp_5", "kp_6", "kp_7", "kp_8", "kp_9", "kp_0", "kp_dot", "nonusbackslash", "menu"} {
		Keys[name] = byte(0x46 + i)
	}
	for i := 13; i <= 24; i++ {
		Keys[fmt.Sprintf("f%d", i)] = byte(0x68 + i - 13)
	}
	for name, code := range map[string]byte{"none": 0, "ro": 0x87, "katakana": 0x88, "yen": 0x89, "henkan": 0x8a, "muhenkan": 0x8b} {
		Keys[name] = code
	}
}

func Stroke(text string) (byte, byte, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(text)), "+")
	aliases := map[string]string{"control": "ctrl", "win": "super", "gui": "super", "return": "enter", "escape": "esc", "pgup": "pageup", "pgdn": "pagedown", "del": "delete"}
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
		if a, ok := aliases[parts[i]]; ok {
			parts[i] = a
		}
	}
	var mod byte
	for _, p := range parts[:len(parts)-1] {
		m, ok := Modifiers[p]
		if !ok {
			return 0, 0, fmt.Errorf("不明な修飾キー: %q", p)
		}
		mod |= m
	}
	key := parts[len(parts)-1]
	if m, ok := Modifiers[key]; ok {
		return mod | m, 0, nil
	}
	if k, ok := Keys[key]; ok {
		return mod, k, nil
	}
	return 0, 0, fmt.Errorf("不明なキー: %q（例: ctrl+c、f13、comma）", key)
}

func report(payload ...byte) Frame {
	var f Frame
	f[0] = 3
	copy(f[1:], payload)
	return f
}

func (b Binding) Frames() ([]Frame, error) {
	if b.Layer < 1 || b.Layer > 3 || b.Slot < 1 || b.Slot > 18 {
		return nil, fmt.Errorf("レイヤーは1〜3、入力番号は1〜18で指定してください")
	}
	slot, layer := byte(b.Slot), byte(b.Layer)
	frames := []Frame{report(0xa1, layer)}
	switch b.Kind {
	case "keyboard":
		steps := strings.Split(b.Value, ",")
		if len(steps) > 5 {
			return nil, fmt.Errorf("キー操作は最大5つです（カンマ区切り）")
		}
		for i, step := range steps {
			mod, key, err := Stroke(step)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				frames = append(frames, report(slot, layer<<4|1, byte(len(steps)), 0, mod, 0))
			}
			frames = append(frames, report(slot, layer<<4|1, byte(len(steps)), byte(i+1), mod, key))
		}
	case "media":
		code, ok := Media[b.Value]
		if !ok {
			return nil, fmt.Errorf("不明なメディア操作: %q", b.Value)
		}
		frames = append(frames, report(slot, layer<<4|2, code, 0))
	case "mouse":
		parts := strings.Split(strings.ToLower(b.Value), "+")
		var mod byte
		for _, p := range parts[:len(parts)-1] {
			if p != "ctrl" && p != "shift" && p != "alt" {
				return nil, fmt.Errorf("マウスの修飾キーは ctrl / shift / alt です")
			}
			mod |= Modifiers[p]
		}
		action, ok := Mouse[parts[len(parts)-1]]
		if !ok {
			return nil, fmt.Errorf("不明なマウス操作: %q", b.Value)
		}
		frames = append(frames, report(slot, layer<<4|3, action[0], 0, 0, action[1], mod))
	default:
		return nil, fmt.Errorf("不明な操作の種類: %q", b.Kind)
	}
	return append(frames, report(0xaa, 0xaa)), nil
}

func LEDFrames(mode int) ([]Frame, error) {
	if mode < 0 || mode > 2 {
		return nil, fmt.Errorf("LED モードは0〜2です")
	}
	return []Frame{report(0xa1, 1), report(0xb0, 0x18, byte(mode)), report(0xaa, 0xa1)}, nil
}

func SlotName(slot int) string {
	if slot <= 12 {
		return fmt.Sprintf("KEY %02d", slot)
	}
	return fmt.Sprintf("ノブ %d · %s", (slot-13)/3+1, []string{"左回転", "押下", "右回転"}[(slot-13)%3])
}
