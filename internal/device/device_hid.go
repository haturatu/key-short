//go:build windows || darwin

package device

import (
	"fmt"
	"github.com/sstallion/go-hid"
	"key-short/internal/protocol"
	"log/slog"
	"sync"
	"time"
)

var initOnce sync.Once
var initError error

func initialize() error { initOnce.Do(func() { initError = hid.Init() }); return initError }
func List() ([]Info, error) {
	if err := initialize(); err != nil {
		return nil, err
	}
	var infos []Info
	err := hid.Enumerate(0x1189, 0x8890, func(d *hid.DeviceInfo) error {
		if d.UsagePage == 0xff00 && d.Usage == 1 && (d.InterfaceNbr == 1 || d.InterfaceNbr < 0) {
			infos = append(infos, Info{Path: d.Path, Serial: d.SerialNbr, Product: d.ProductStr})
		}
		return nil
	})
	return infos, err
}
func Write(id string, jobs []protocol.Job, progress func(string, int, int)) error {
	total := 0
	for _, job := range jobs {
		if len(job.Frames) == 0 {
			return fmt.Errorf("空の送信データです")
		}
		for _, f := range job.Frames {
			if f[0] != 3 {
				return fmt.Errorf("Report ID 3 のみ対応しています")
			}
			total++
		}
	}
	if total == 0 {
		return fmt.Errorf("書き込む設定がありません")
	}
	infos, err := List()
	if err != nil {
		return err
	}
	path := ""
	for _, info := range infos {
		if info.ID() == id {
			path = info.Path
		}
	}
	if path == "" {
		return fmt.Errorf("設定用 HID インターフェースがありません。USB を接続し直してください")
	}
	d, err := hid.OpenPath(path)
	if err != nil {
		return fmt.Errorf("HID を開けません: %w", err)
	}
	defer d.Close()
	setWriteTimeout(d)
	done := 0
	for _, job := range jobs {
		for _, f := range job.Frames {
			n, err := d.Write(f[:])
			if err != nil {
				return fmt.Errorf("%s の送信に失敗しました。一部だけ設定された可能性があります: %w", job.Label, err)
			}
			if n != len(f) {
				return fmt.Errorf("HID の転送バイト数が不足しています")
			}
			done++
			if progress != nil {
				progress(job.Label, done, total)
			}
			time.Sleep(30 * time.Millisecond)
		}
	}
	slog.Info("設定データを HID へ送信しました", "device", id, "items", len(jobs))
	return nil
}
