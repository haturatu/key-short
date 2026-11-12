package window

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"
import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

type nativeMac struct{}

func New() (Backend, error) { return &nativeMac{}, nil }
func (*nativeMac) List(ctx context.Context) ([]Window, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data := C.ks_window_list()
	if data == nil {
		return nil, fmt.Errorf("macOS のウィンドウを取得できません")
	}
	defer C.free(unsafe.Pointer(data))
	var out []Window
	err := json.Unmarshal([]byte(C.GoString(data)), &out)
	return out, err
}
func (*nativeMac) Activate(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	part, _, _ := strings.Cut(id, ":")
	pid, err := strconv.Atoi(part)
	if err != nil {
		return err
	}
	switch C.ks_window_activate(C.int(pid)) {
	case 0:
		return nil
	case -2:
		return fmt.Errorf("システム設定のプライバシーとセキュリティで Key Short のアクセシビリティを許可してください")
	default:
		return fmt.Errorf("macOS のウィンドウを前面化できません")
	}
}
func (*nativeMac) Close() error { return nil }
