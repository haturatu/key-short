package keycapture

import "testing"

func TestAAndControlPageUp(t *testing.T) {
	var r Recorder
	r.Down("A")
	if r.Value() != "a" {
		t.Fatal(r.Value())
	}
	r.Up("A")
	r.Down("LeftControl")
	if r.Value() != "a" {
		t.Fatal("modifier recorded alone")
	}
	r.Down("Prior")
	if r.Value() != "ctrl+pageup" {
		t.Fatal(r.Value())
	}
	r.Up("Prior")
	r.Up("LeftControl")
	r.Down("B")
	if r.Value() != "b" {
		t.Fatal("stuck modifier", r.Value())
	}
}
func TestRepeatSequenceAndFocusLoss(t *testing.T) {
	r := Recorder{Sequence: true}
	r.Down("LeftControl")
	r.Down("C")
	r.Down("C")
	r.Up("C")
	r.Down("V")
	if r.Value() != "ctrl+c,ctrl+v" {
		t.Fatal(r.Value())
	}
	r.Release()
	r.Down("Tab")
	if r.Value() != "ctrl+c,ctrl+v,tab" {
		t.Fatal(r.Value())
	}
	r.Up("Tab")
	r.Down("A")
	r.Up("A")
	r.Down("B")
	r.Up("B")
	r.Down("C")
	if len(r.Steps) != 5 || r.Error == "" {
		t.Fatal("macro limit not enforced")
	}
	r.Clear()
	r.Down("RightSuper")
	r.Down("S")
	if r.Value() != "rsuper+s" {
		t.Fatal(r.Value())
	}
}
