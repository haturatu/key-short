package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/autostart"
)

func (s *studio) autostartControls() fyne.CanvasObject {
	if runtime.GOOS != "linux" {
		return container.NewVBox()
	}
	s.autostartStatus = widget.NewLabel("自動起動の状態を確認できます。")
	s.autostartStatus.Wrapping = fyne.TextWrapWord
	s.autostartBackend = widget.NewSelect([]string{"自動選択", "systemd ユーザーサービス", "XDG 自動起動（init 共通）"}, nil)
	s.autostartBackend.SetSelectedIndex(0)
	enable := widget.NewButton("この JSON で自動起動を有効にする", func() { s.manageAutostart("enable") })
	disable := widget.NewButton("自動起動を無効にする", func() { s.manageAutostart("disable") })
	start := widget.NewButton("デーモンを開始", func() { s.manageAutostart("start") })
	stop := widget.NewButton("デーモンを停止", func() { s.manageAutostart("stop") })
	restart := widget.NewButton("再読み込み・再起動", func() { s.manageAutostart("restart") })
	refresh := widget.NewButton("状態を更新", func() { s.manageAutostart("status") })
	s.autostartWidgets = []fyne.Disableable{s.autostartBackend, enable, disable, start, stop, restart, refresh}
	note := widget.NewLabel("保存済み JSON を指定して、次回ログインからの自動起動を設定します。変更した JSON はデーモンの再起動で読み込みます。XDG は OpenRC・runit・s6・dinit などでも使用できます。")
	note.Wrapping = fyne.TextWrapWord
	return container.NewVBox(widget.NewSeparator(), widget.NewLabelWithStyle("監視の自動起動", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), note, s.autostartStatus, s.autostartBackend, enable, disable, container.NewGridWithColumns(2, start, stop), restart, refresh)
}
func (s *studio) updateAutostartControls() {
	for _, w := range s.autostartWidgets {
		if s.busy || s.autostartBusy {
			w.Disable()
		} else {
			w.Enable()
		}
	}
}
func (s *studio) manageAutostart(action string) {
	if s.busy || s.autostartBusy || s.closing {
		return
	}
	if action == "enable" && (s.profilePath == "" || s.dirty || s.hasDraft()) {
		s.fail(fmt.Errorf("キーの編集を適用し、JSON に保存してから自動起動を有効にしてください"))
		return
	}
	if (action == "start" || action == "restart") && s.hasDraft() {
		s.fail(fmt.Errorf("キーの編集を適用するか戻してから開始してください"))
		return
	}
	if action == "start" || action == "restart" {
		if s.monitorStop != nil {
			s.stopMonitoring()
		}
	}
	backend := "auto"
	switch s.autostartBackend.SelectedIndex() {
	case 1:
		backend = "systemd"
	case 2:
		backend = "xdg"
	}
	path := s.profilePath
	s.autostartBusy = true
	s.updateAutostartControls()
	s.launchWG.Add(1)
	go func() {
		defer s.launchWG.Done()
		ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
		defer cancel()
		m, err := autostart.New()
		var status autostart.Status
		if err == nil {
			err = m.Apply(ctx, action, path, backend)
		}
		if err == nil {
			status, err = m.Status(ctx)
		}
		fyne.Do(func() {
			if s.closing {
				return
			}
			s.autostartBusy = false
			defer s.refresh()
			s.updateAutostartControls()
			if err != nil {
				s.autostartStatus.SetText("自動起動の操作に失敗: " + err.Error())
				return
			}
			s.daemonRunning = status.Running
			state := "無効"
			if status.Enabled {
				state = "有効"
			}
			running := "停止中"
			if status.Running {
				running = "実行中"
			}
			text := fmt.Sprintf("自動起動: %s · %s\n監視デーモン: %s", state, status.Backend, running)
			if status.Profile != "" {
				text += "\n使用する JSON: " + status.Profile
			}
			s.autostartStatus.SetText(text)
			if status.Enabled {
				s.app.Preferences().SetBool("launcher-enabled", false)
			}
			if s.monitorStop == nil {
				if status.Running {
					s.monitorButton.SetText("監視デーモンを停止")
					s.monitorStatus.SetText("ランチャー監視デーモン実行中")
				} else {
					s.monitorButton.SetText("ランチャー監視を開始")
					s.monitorStatus.SetText("ランチャー停止中")
				}
			}
		})
	}()
}

func (s *studio) watchAutostart() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				fyne.Do(func() {
					if !s.closing && !s.autostartBusy && !s.busy {
						s.manageAutostart("status")
					}
				})
			}
		}
	}()
}
