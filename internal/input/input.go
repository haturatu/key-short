// Package input listens only to 1189:8890 evdev nodes. It never opens other
// keyboards, grabs devices, or reads text entered on the user's main keyboard.
package input

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"key-short/internal/protocol"
)

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

func Nodes() []string {
	paths, _ := filepath.Glob("/sys/class/input/event*/device/id/vendor")
	var nodes []string
	for _, path := range paths {
		vendor, _ := os.ReadFile(path)
		product, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "product"))
		if strings.TrimSpace(string(vendor)) != "1189" || strings.TrimSpace(string(product)) != "8890" {
			continue
		}
		nodes = append(nodes, filepath.Join("/dev/input", filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))))
	}
	return nodes
}

func Monitor(ctx context.Context, fire func(Chord), status func(string)) error {
	workers := map[string]bool{}
	finished := make(chan string, 32)
	ctx, cancel := context.WithCancel(ctx)
	var readers sync.WaitGroup
	defer func() { cancel(); readers.Wait() }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	lastStatus := ""
	report := func(text string) {
		if text != lastStatus {
			lastStatus = text
			if status != nil {
				status(text)
			}
		}
	}
	scan := func() {
		nodes := Nodes()
		if len(nodes) == 0 {
			report("ランチャー監視中 · USB の接続待ち")
		}
		for _, path := range nodes {
			if workers[path] {
				continue
			}
			fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if err != nil {
				report("入力デバイスを読めません。udev ルールを更新して USB を抜き差ししてください")
				continue
			}
			workers[path] = true
			readers.Add(1)
			report("ランチャー監視中 · USB ボタンを待っています")
			go func(path string, fd int) {
				defer readers.Done()
				defer unix.Close(fd)
				_ = readEvents(ctx, fd, fire)
				select {
				case finished <- path:
				case <-ctx.Done():
				}
			}(path, fd)
		}
	}
	scan()
	for {
		select {
		case <-ctx.Done():
			return nil
		case path := <-finished:
			delete(workers, path)
		case <-tick.C:
			scan()
		}
	}
}

func readEvents(ctx context.Context, fd int, fire func(Chord)) error {
	offset := int(unsafe.Sizeof(unix.Timeval{}))
	size := offset + 8
	buf := make([]byte, size*32)
	pending := []byte{}
	var d Detector
	for ctx.Err() == nil {
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		if _, err := unix.Poll(poll, 100); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}
		if poll[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return io.EOF
		}
		if poll[0].Revents&unix.POLLIN == 0 {
			continue
		}
		n, err := unix.Read(fd, buf)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.EOF
		}
		pending = append(pending, buf[:n]...)
		for len(pending) >= size {
			event := pending[offset:size]
			if chord, ok := d.Feed(binary.NativeEndian.Uint16(event), binary.NativeEndian.Uint16(event[2:]), int32(binary.NativeEndian.Uint32(event[4:]))); ok {
				fire(chord)
			}
			pending = pending[size:]
		}
	}
	return nil
}
