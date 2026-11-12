package window

import (
	"context"
	"fmt"
	"os"
)

type Window struct {
	ID     string `json:"id"`
	PID    int    `json:"pid"`
	Class  string `json:"class"`
	AppID  string `json:"app_id"`
	Active bool   `json:"active"`
}

type Backend interface {
	List(context.Context) ([]Window, error)
	Activate(context.Context, string) error
	Close() error
}

func New() (Backend, error) {
	forced := os.Getenv("KEY_SHORT_WINDOW_BACKEND")
	if forced == "kwin" || (forced == "" && os.Getenv("XDG_SESSION_TYPE") == "wayland") {
		b, err := newKWin()
		if err != nil {
			return nil, fmt.Errorf("Wayland の前面化には KDE Plasma の KWin が必要です: %w", err)
		}
		return b, nil
	}
	if forced != "" && forced != "x11" {
		return nil, fmt.Errorf("未対応のウィンドウ制御: %s", forced)
	}
	return newX11()
}
