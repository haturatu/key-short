package window

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
)

type x11 struct {
	x  *xgbutil.XUtil
	mu sync.Mutex
}

func newX11() (*x11, error) {
	x, err := xgbutil.NewConn()
	if err != nil {
		return nil, fmt.Errorf("X11 に接続できません: %w", err)
	}
	return &x11{x: x}, nil
}
func (x *x11) List(ctx context.Context) ([]Window, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	wins, err := ewmh.ClientListStackingGet(x.x)
	if err != nil {
		wins, err = ewmh.ClientListGet(x.x)
	}
	if err != nil {
		return nil, err
	}
	active, _ := ewmh.ActiveWindowGet(x.x)
	var result []Window
	for _, id := range wins {
		pid, err := ewmh.WmPidGet(x.x, id)
		if err != nil || pid == 0 {
			continue
		}
		class := ""
		if c, err := icccm.WmClassGet(x.x, id); err == nil {
			class = c.Class
		}
		result = append(result, Window{ID: fmt.Sprintf("0x%x", uint32(id)), PID: int(pid), Class: class, Active: id == active})
	}
	return result, nil
}
func (x *x11) Activate(ctx context.Context, id string) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := strconv.ParseUint(id, 0, 32)
	if err != nil {
		return err
	}
	win := xproto.Window(n)
	if _, err = xproto.GetWindowAttributes(x.x.Conn(), win).Reply(); err != nil {
		return fmt.Errorf("ウィンドウがありません: %w", err)
	}
	if desktop, err := ewmh.WmDesktopGet(x.x, win); err == nil && desktop != 0xffffffff {
		if err = ewmh.CurrentDesktopReq(x.x, int(desktop)); err != nil {
			return err
		}
	}
	if err = xproto.MapWindowChecked(x.x.Conn(), win).Check(); err != nil {
		return err
	}
	return ewmh.ActiveWindowReqExtra(x.x, win, 2, 0, 0)
}
func (x *x11) Close() error { x.x.Conn().Close(); return nil }
