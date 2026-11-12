package input

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation -framework ApplicationServices
#include "native_darwin.h"
*/
import "C"
import (
	"context"
	"fmt"
	"runtime"
)

func Monitor(ctx context.Context, fire func(Chord), status func(string)) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	result := C.ks_input_open()
	if result == -2 {
		return fmt.Errorf("システム設定のプライバシーとセキュリティで入力監視を許可し、監視を開始し直してください")
	}
	if result != 0 {
		return fmt.Errorf("IOHIDManager を開始できません")
	}
	defer C.ks_input_close()
	if status != nil {
		status("ランチャー監視中 · CH57x の HID 入力を待っています")
	}
	detectors := map[uint64]*Detector{}
	pressed := map[uint64]map[uint16]bool{}
	events := make([]C.ks_key, 256)
	for ctx.Err() == nil {
		n := int(C.ks_input_read(&events[0], C.int(len(events))))
		if n < 0 {
			detectors = map[uint64]*Detector{}
			pressed = map[uint64]map[uint16]bool{}
			continue
		}
		for _, event := range events[:n] {
			id := uint64(event.device)
			usage := uint16(event.usage)
			code := uint16(0)
			if usage >= 0x68 && usage <= 0x73 {
				code = 183 + usage - 0x68
			} else if usage >= 0xe0 && usage <= 0xe7 {
				code = []uint16{29, 42, 56, 125, 97, 54, 100, 126}[usage-0xe0]
			}
			if code == 0 {
				continue
			}
			if detectors[id] == nil {
				detectors[id] = &Detector{}
				pressed[id] = map[uint16]bool{}
			}
			down := event.pressed != 0
			if down && pressed[id][code] {
				continue
			}
			pressed[id][code] = down
			v := int32(0)
			if down {
				v = 1
			}
			if chord, ok := detectors[id].Feed(1, code, v); ok {
				fire(chord)
			}
		}
	}
	return nil
}
