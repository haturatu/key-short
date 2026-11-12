// Package keycapture records only key events explicitly delivered to the focused
// recording control. It never installs global hooks or logs keyboard input.
package keycapture

import (
	"fmt"
	"key-short/internal/protocol"
	"strings"
)

type Recorder struct {
	Sequence bool
	Steps    []string
	Held     map[string]bool
	Error    string
}

var modifiers = map[string]string{"LeftControl": "ctrl", "RightControl": "rctrl", "LeftShift": "shift", "RightShift": "rshift", "LeftAlt": "alt", "RightAlt": "ralt", "LeftSuper": "super", "RightSuper": "rsuper"}
var order = []string{"LeftControl", "LeftShift", "LeftAlt", "LeftSuper", "RightControl", "RightShift", "RightAlt", "RightSuper"}
var names = map[string]string{"Prior": "pageup", "Next": "pagedown", "Return": "enter", "KP_Enter": "kp_enter", "Escape": "esc", "BackSpace": "backspace", "PrintScreen": "printscreen", ",": "comma", ".": "dot", "/": "slash", ";": "semicolon", "'": "apostrophe", "`": "grave", "-": "minus", "=": "equal", "[": "leftbrace", "]": "rightbrace", "\\": "backslash"}

func (r *Recorder) Down(name string) {
	if r.Held == nil {
		r.Held = map[string]bool{}
	}
	if r.Held[name] {
		return
	}
	r.Held[name] = true
	if _, ok := modifiers[name]; ok {
		return
	}
	key := strings.ToLower(name)
	if alias, ok := names[name]; ok {
		key = alias
	}
	chord := append(r.modifiers(), key)
	step := strings.Join(chord, "+")
	if _, _, err := protocol.Stroke(step); err != nil {
		r.Error = fmt.Sprintf("このキーは記録できません: %s", name)
		return
	}
	if r.Sequence && len(r.Steps) >= 5 {
		r.Error = "連続操作は5操作までです。クリアして記録し直してください。"
		return
	}
	r.Error = ""
	if !r.Sequence {
		r.Steps = nil
	}
	r.Steps = append(r.Steps, step)
}
func (r *Recorder) Up(name string) { delete(r.Held, name) }
func (r *Recorder) Release()       { r.Held = map[string]bool{} }
func (r *Recorder) Clear()         { r.Steps = nil; r.Error = ""; r.Release() }
func (r *Recorder) modifiers() []string {
	var out []string
	for _, name := range order {
		if r.Held[name] {
			out = append(out, modifiers[name])
		}
	}
	return out
}
func (r *Recorder) Modifiers() string { return strings.Join(r.modifiers(), "+") }
func (r *Recorder) Value() string     { return strings.Join(r.Steps, ",") }
