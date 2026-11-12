package window

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var enumWindows = user32.NewProc("EnumWindows")
var isVisible = user32.NewProc("IsWindowVisible")
var getPID = user32.NewProc("GetWindowThreadProcessId")
var getClass = user32.NewProc("GetClassNameW")
var foreground = user32.NewProc("GetForegroundWindow")
var showWindow = user32.NewProc("ShowWindowAsync")
var isWindow = user32.NewProc("IsWindow")
var setForeground = user32.NewProc("SetForegroundWindow")
var bringToTop = user32.NewProc("BringWindowToTop")
var attachInput = user32.NewProc("AttachThreadInput")

type nativeWindows struct {
	mu               sync.Mutex
	ctx              context.Context
	result           []Window
	active, callback uintptr
}

func New() (Backend, error) {
	b := &nativeWindows{}
	b.callback = windows.NewCallback(b.visit)
	return b, nil
}
func (b *nativeWindows) visit(hwnd, unused uintptr) uintptr {
	if b.ctx.Err() != nil {
		return 0
	}
	visible, _, _ := isVisible.Call(hwnd)
	if visible == 0 {
		return 1
	}
	var pid uint32
	getPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return 1
	}
	var class [256]uint16
	getClass.Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
	appID := ""
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err == nil {
		var path [32768]uint16
		n := uint32(len(path))
		if windows.QueryFullProcessImageName(h, 0, &path[0], &n) == nil {
			appID = strings.TrimSuffix(strings.ToLower(filepath.Base(windows.UTF16ToString(path[:n]))), ".exe")
		}
		windows.CloseHandle(h)
	}
	b.result = append(b.result, Window{ID: fmt.Sprintf("0x%x", hwnd), PID: int(pid), Class: windows.UTF16ToString(class[:]), AppID: appID, Active: hwnd == b.active})
	return 1
}
func (b *nativeWindows) List(ctx context.Context) ([]Window, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ctx = ctx
	b.result = nil
	b.active, _, _ = foreground.Call()
	enumWindows.Call(b.callback, 0)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(b.result)-1; i < j; i, j = i+1, j-1 {
		b.result[i], b.result[j] = b.result[j], b.result[i]
	}
	return b.result, nil
}
func (*nativeWindows) Activate(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := strconv.ParseUint(id, 0, 64)
	if err != nil {
		return err
	}
	hwnd := uintptr(n)
	ok, _, _ := isWindow.Call(hwnd)
	if ok == 0 {
		return fmt.Errorf("ウィンドウがありません")
	}
	showWindow.Call(hwnd, 9)
	current := uintptr(windows.GetCurrentThreadId())
	active, _, _ := foreground.Call()
	thread, _, _ := getPID.Call(active, 0)
	if thread != 0 && thread != current {
		attached, _, _ := attachInput.Call(current, thread, 1)
		if attached != 0 {
			defer attachInput.Call(current, thread, 0)
		}
	}
	bringToTop.Call(hwnd)
	ok, _, _ = setForeground.Call(hwnd)
	if ok == 0 {
		return fmt.Errorf("Windows によって前面化が拒否されました。同じ権限のアプリとして起動してください")
	}
	return nil
}
func (*nativeWindows) Close() error { return nil }
