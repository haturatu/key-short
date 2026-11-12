package input

import (
	"context"
	"encoding/binary"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"key-short/internal/protocol"
)

func TestAllLauncherChordsAreDistinct(t *testing.T) {
	seen := map[Chord]bool{}
	for layer := 1; layer <= 3; layer++ {
		for slot := 1; slot <= 18; slot++ {
			c, err := ForBinding(protocol.Binding{Layer: layer, Slot: slot, Kind: "launcher"})
			if err != nil {
				t.Fatal(err)
			}
			if seen[c] {
				t.Fatalf("duplicate chord: %+v", c)
			}
			seen[c] = true
		}
	}
	if len(seen) != 54 {
		t.Fatal(len(seen))
	}
}

func TestDetectorIgnoresRepeatsAndDroppedFrames(t *testing.T) {
	var d Detector
	d.Feed(1, 29, 1)
	d.Feed(1, 56, 1)
	d.Feed(1, 125, 1)
	c, ok := d.Feed(1, 183, 1)
	if !ok || c.Modifier != 13 {
		t.Fatalf("%+v %v", c, ok)
	}
	if _, ok = d.Feed(1, 183, 2); ok {
		t.Fatal("repeat triggered")
	}
	if _, ok = d.Feed(1, 183, 0); ok {
		t.Fatal("release triggered")
	}
	d.Feed(0, 3, 0)
	if _, ok = d.Feed(1, 183, 1); ok {
		t.Fatal("SYN_DROPPED triggered")
	}
	d.Feed(0, 0, 0)
	c, ok = d.Feed(1, 183, 1)
	if !ok || c.Modifier != 0 {
		t.Fatal("modifiers were not cleared")
	}
	d.Feed(1, 29, 1)
	d.Feed(1, 29, 0)
	c, _ = d.Feed(1, 183, 1)
	if c.Modifier != 0 {
		t.Fatal("modifier release ignored")
	}
}

func TestBinaryEventStreamAndCancellation(t *testing.T) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fds[0])
	defer unix.Close(fds[1])
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Chord, 1)
	done := make(chan error, 1)
	go func() { done <- readEvents(ctx, fds[0], func(c Chord) { events <- c }) }()
	offset := int(unsafe.Sizeof(unix.Timeval{}))
	var data []byte
	for _, e := range [][3]uint16{{29, 1, 1}, {56, 1, 1}, {125, 1, 1}, {183, 1, 1}} {
		event := make([]byte, offset+8)
		binary.NativeEndian.PutUint16(event[offset:], e[1])
		binary.NativeEndian.PutUint16(event[offset+2:], e[0])
		binary.NativeEndian.PutUint32(event[offset+4:], uint32(e[2]))
		data = append(data, event...)
	}
	// Split inside a struct to verify buffered decoding, not read boundaries.
	if _, err = unix.Write(fds[1], data[:7]); err != nil {
		t.Fatal(err)
	}
	if _, err = unix.Write(fds[1], data[7:]); err != nil {
		t.Fatal(err)
	}
	select {
	case c := <-events:
		if c.Modifier != 13 || c.Code != 183 {
			t.Fatal(c)
		}
	case <-time.After(time.Second):
		t.Fatal("event missing")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("reader did not stop")
	}
}
