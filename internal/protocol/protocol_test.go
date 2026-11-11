package protocol

import (
	"bytes"
	"testing"
)

// Expected prefixes are the vendor app's payloads plus Report ID 3.
func TestVendorFrames(t *testing.T) {
	tests := []struct {
		name string
		b    Binding
		want [][]byte
	}{
		{"ctrl+c", Binding{1, 1, "keyboard", "ctrl+c"}, [][]byte{{3, 0xa1, 1}, {3, 1, 0x11, 1, 0, 1, 0}, {3, 1, 0x11, 1, 1, 1, 6}, {3, 0xaa, 0xaa}}},
		{"macro-layer3-knob2", Binding{3, 18, "keyboard", "alt+f4,super+e"}, [][]byte{{3, 0xa1, 3}, {3, 18, 0x31, 2, 0, 4, 0}, {3, 18, 0x31, 2, 1, 4, 0x3d}, {3, 18, 0x31, 2, 2, 8, 8}, {3, 0xaa, 0xaa}}},
		{"knob2-volume", Binding{1, 16, "media", "volume_down"}, [][]byte{{3, 0xa1, 1}, {3, 16, 0x12, 0xea, 0}, {3, 0xaa, 0xaa}}},
		{"ctrl-scroll", Binding{2, 12, "mouse", "ctrl+wheel_down"}, [][]byte{{3, 0xa1, 2}, {3, 12, 0x23, 0, 0, 0, 255, 1}, {3, 0xaa, 0xaa}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames, err := tt.b.Frames()
			if err != nil {
				t.Fatal(err)
			}
			if len(frames) != len(tt.want) {
				t.Fatalf("%d frames", len(frames))
			}
			for i, prefix := range tt.want {
				want := make([]byte, 65)
				copy(want, prefix)
				if !bytes.Equal(frames[i][:], want) {
					t.Fatalf("frame %d: % x; want % x", i, frames[i], want)
				}
			}
		})
	}
}

func TestInvalidBindings(t *testing.T) {
	for _, b := range []Binding{{0, 1, "keyboard", "a"}, {4, 1, "keyboard", "a"}, {1, 0, "keyboard", "a"}, {1, 19, "keyboard", "a"}, {1, 1, "keyboard", ""}, {1, 1, "keyboard", "ctrl++"}, {1, 1, "keyboard", "a,b,c,d,e,f"}, {1, 1, "keyboard", "unknown+a"}, {1, 1, "media", "nope"}, {1, 1, "mouse", "super+wheel_up"}, {1, 1, "mouse", "nope"}, {1, 1, "bad", "a"}} {
		if _, err := b.Frames(); err == nil {
			t.Errorf("accepted %+v", b)
		}
	}
}

func TestSpecialKeys(t *testing.T) {
	for name, want := range map[string]byte{"f13": 0x68, "f24": 0x73, "yen": 0x89, "muhenkan": 0x8b, "kp_plus": 0x57} {
		_, code, err := Stroke(name)
		if err != nil || code != want {
			t.Errorf("%s: %x, %v", name, code, err)
		}
	}
	mod, code, err := Stroke("Ctrl+Shift+Alt+Super")
	if err != nil || mod != 15 || code != 0 {
		t.Fatalf("modifier-only: %x %x %v", mod, code, err)
	}
}

func TestLEDCommit(t *testing.T) {
	frames, err := LEDFrames(2)
	if err != nil {
		t.Fatal(err)
	}
	if frames[1][1] != 0xb0 || frames[1][2] != 0x18 || frames[1][3] != 2 || frames[2][2] != 0xa1 {
		t.Fatalf("bad LED sequence: % x", frames)
	}
	if _, err := LEDFrames(3); err == nil {
		t.Fatal("accepted undefined LED mode")
	}
}
