package input

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

var rawUser32 = windows.NewLazySystemDLL("user32.dll")
var rawDefault = rawUser32.NewProc("DefWindowProcW")
var rawRead = rawUser32.NewProc("GetRawInputData")
var rawInfo = rawUser32.NewProc("GetRawInputDeviceInfoW")
var rawRegister = rawUser32.NewProc("RegisterRawInputDevices")
var rawStates = struct {
	sync.Mutex
	m map[uintptr]*rawState
}{m: map[uintptr]*rawState{}}
var rawCallback = windows.NewCallback(rawDispatch)

type rawHeader struct {
	Type, Size     uint32
	Device, WParam uintptr
}
type rawKeyboard struct {
	MakeCode, Flags, Reserved, VKey uint16
	Message, Extra                  uint32
}
type rawDevice struct {
	Page, Usage uint16
	Flags       uint32
	Window      uintptr
}
type rawClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type rawMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}
type rawState struct {
	fire      func(Chord)
	detectors map[uintptr]*Detector
	pressed   map[uintptr]map[uint16]bool
}

func rawDispatch(hwnd, msg, wparam, lparam uintptr) uintptr {
	rawStates.Lock()
	s := rawStates.m[hwnd]
	rawStates.Unlock()
	if s != nil && msg == 0xfe && wparam == 2 {
		delete(s.detectors, lparam)
		delete(s.pressed, lparam)
	}
	if s != nil && msg == 0xff {
		size := uint32(0)
		headerSize := uintptr(unsafe.Sizeof(rawHeader{}))
		rawRead.Call(lparam, 0x10000003, 0, uintptr(unsafe.Pointer(&size)), headerSize)
		if size >= uint32(headerSize)+uint32(unsafe.Sizeof(rawKeyboard{})) && size <= 4096 {
			data := make([]byte, size)
			n, _, _ := rawRead.Call(lparam, 0x10000003, uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)), headerSize)
			if n != ^uintptr(0) {
				h := (*rawHeader)(unsafe.Pointer(&data[0]))
				if h.Type == 1 && isPad(h.Device) {
					k := (*rawKeyboard)(unsafe.Pointer(&data[headerSize]))
					code := nativeCode(k.VKey, k.MakeCode, k.Flags)
					if code != 0 {
						if s.detectors[h.Device] == nil {
							s.detectors[h.Device] = &Detector{}
							s.pressed[h.Device] = map[uint16]bool{}
						}
						value := int32(1)
						if k.Flags&1 != 0 {
							value = 0
						}
						repeat := value == 1 && s.pressed[h.Device][code]
						s.pressed[h.Device][code] = value == 1
						if !repeat {
							if c, ok := s.detectors[h.Device].Feed(1, code, value); ok {
								s.fire(c)
							}
						}
					}
				}
			}
		}
	}
	result, _, _ := rawDefault.Call(hwnd, msg, wparam, lparam)
	return result
}
func isPad(device uintptr) bool {
	var size uint32
	rawInfo.Call(device, 0x20000007, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 || size > 32768 {
		return false
	}
	name := make([]uint16, size+1)
	n, _, _ := rawInfo.Call(device, 0x20000007, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&size)))
	if n == ^uintptr(0) {
		return false
	}
	path := strings.ToLower(windows.UTF16ToString(name))
	return strings.Contains(path, "vid_1189") && strings.Contains(path, "pid_8890")
}
func nativeCode(key, scan, flags uint16) uint16 {
	if key >= 0x7c && key <= 0x87 {
		return 183 + key - 0x7c
	}
	switch key {
	case 0x11:
		if flags&2 != 0 {
			return 97
		}
		return 29
	case 0x10:
		if scan == 0x36 {
			return 54
		}
		return 42
	case 0x12:
		if flags&2 != 0 {
			return 100
		}
		return 56
	case 0x5b:
		return 125
	case 0x5c:
		return 126
	case 0xa0:
		return 42
	case 0xa1:
		return 54
	case 0xa2:
		return 29
	case 0xa3:
		return 97
	case 0xa4:
		return 56
	case 0xa5:
		return 100
	}
	return 0
}
func Monitor(ctx context.Context, fire func(Chord), status func(string)) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	instance, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	name, _ := windows.UTF16PtrFromString(fmt.Sprintf("KeyShortRawInput-%d", os.Getpid()))
	class := rawClass{Size: uint32(unsafe.Sizeof(rawClass{})), Proc: rawCallback, Instance: instance, Name: name}
	registered, _, err := rawUser32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if registered == 0 {
		return fmt.Errorf("入力監視ウィンドウの登録: %w", err)
	}
	defer rawUser32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), instance)
	hwnd, _, err := rawUser32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("入力監視ウィンドウの作成: %w", err)
	}
	defer rawUser32.NewProc("DestroyWindow").Call(hwnd)
	rawStates.Lock()
	rawStates.m[hwnd] = &rawState{fire: fire, detectors: map[uintptr]*Detector{}, pressed: map[uintptr]map[uint16]bool{}}
	rawStates.Unlock()
	defer func() { rawStates.Lock(); delete(rawStates.m, hwnd); rawStates.Unlock() }()
	device := rawDevice{Page: 1, Usage: 6, Flags: 0x100 | 0x2000, Window: hwnd}
	ok, _, err := rawRegister.Call(uintptr(unsafe.Pointer(&device)), 1, unsafe.Sizeof(device))
	if ok == 0 {
		return fmt.Errorf("Raw Input を登録できません: %w", err)
	}
	defer func() {
		remove := rawDevice{Page: 1, Usage: 6, Flags: 1}
		rawRegister.Call(uintptr(unsafe.Pointer(&remove)), 1, unsafe.Sizeof(remove))
	}()
	done := make(chan struct{})
	defer close(done)
	thread := windows.GetCurrentThreadId()
	go func() {
		select {
		case <-ctx.Done():
			rawUser32.NewProc("PostThreadMessageW").Call(uintptr(thread), 0x12, 0, 0)
		case <-done:
		}
	}()
	if status != nil {
		status("ランチャー監視中 · CH57x の Raw Input を待っています")
	}
	for {
		var msg rawMessage
		r, _, err := rawUser32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			return err
		}
		if r == 0 {
			return nil
		}
		rawUser32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		rawUser32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
}
