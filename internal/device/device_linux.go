// Package device accesses only the configuration interface, leaving keyboard and
// mouse interfaces attached to their Linux drivers.
package device

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/gousb"
	"key-short/internal/protocol"
)

// List reads descriptors without opening or writing to any device.
func List() ([]Info, error) {
	c := gousb.NewContext()
	defer c.Close()
	var infos []Info
	_, err := c.OpenDevices(func(d *gousb.DeviceDesc) bool {
		if d.Vendor == 0x1189 && d.Product == 0x8890 {
			infos = append(infos, Info{Bus: d.Bus, Address: d.Address})
		}
		return false
	})
	return infos, err
}

func Write(id string, jobs []protocol.Job, progress func(string, int, int)) (err error) {
	if len(jobs) == 0 {
		return fmt.Errorf("書き込む設定がありません")
	}
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
	c := gousb.NewContext()
	defer c.Close()
	devs, openErr := c.OpenDevices(func(d *gousb.DeviceDesc) bool {
		return d.Vendor == 0x1189 && d.Product == 0x8890 && (Info{Bus: d.Bus, Address: d.Address}).ID() == id
	})
	defer func() {
		for _, d := range devs {
			if e := d.Close(); e != nil {
				slog.Warn("USB のクローズに失敗", "error", e)
			}
		}
	}()
	if openErr != nil {
		return fmt.Errorf("USB を開けません。sudo ./scripts/install-udev.sh を実行後、USB を接続し直してください: %w", openErr)
	}
	if len(devs) != 1 {
		return fmt.Errorf("USB %s が見つかりません。再検出してください", id)
	}
	d := devs[0]
	num, err := d.ActiveConfigNum()
	if err != nil {
		return err
	}
	if num != 1 {
		return fmt.Errorf("未対応の USB 構成: %d", num)
	}
	// Do not enable auto-detach: gousb would detach all keyboard interfaces.
	cfg, err := d.Config(num)
	if err != nil {
		return err
	}
	defer cfg.Close()
	intf, err := cfg.Interface(1, 0)
	if err != nil {
		return fmt.Errorf("設定用インターフェース1を取得できません: %w", err)
	}
	defer intf.Close()
	if intf.Setting.Class != gousb.ClassHID {
		return fmt.Errorf("設定用インターフェースが HID ではありません")
	}
	ep, err := intf.OutEndpoint(2)
	if err != nil {
		return err
	}
	if ep.Desc.TransferType != gousb.TransferTypeInterrupt {
		return fmt.Errorf("設定用エンドポイントが interrupt OUT ではありません")
	}
	done := 0
	for _, job := range jobs {
		for _, f := range job.Frames {
			slog.Debug("USB 送信", "device", id, "bytes", fmt.Sprintf("% x", f[:]))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			n, e := ep.WriteContext(ctx, f[:])
			cancel()
			if e == nil && n != len(f) {
				e = errors.New("転送したバイト数が不足しています")
			}
			if e != nil {
				return fmt.Errorf("%s の送信に失敗しました。一部だけ設定された可能性があります。再接続して再実行してください: %w", job.Label, e)
			}
			done++
			if progress != nil {
				progress(job.Label, done, total)
			}
			time.Sleep(30 * time.Millisecond)
		}
	}
	slog.Info("設定データを USB へ送信しました", "device", id, "items", len(jobs))
	return nil
}
