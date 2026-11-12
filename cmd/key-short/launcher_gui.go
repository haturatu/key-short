package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynedesktop "fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/desktop"
	"key-short/internal/host"
	"key-short/internal/launcher"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

func (s *studio) manager() (*launcher.Manager, error) {
	if s.launchManager == nil {
		m, err := launcher.New("")
		if err != nil {
			return nil, err
		}
		s.launchManager = m
	}
	return s.launchManager, nil
}

func (s *studio) launcherControls() fyne.CanvasObject {
	s.monitorStatus = widget.NewLabel("ランチャー停止中")
	s.monitorStatus.Wrapping = fyne.TextWrapWord
	s.monitorButton = widget.NewButton("ランチャー監視を開始", func() {
		if s.monitorStop != nil {
			s.stopMonitoring()
		} else {
			s.startMonitoring()
		}
	})
	s.controls = append(s.controls, s.monitorButton)
	s.resident = widget.NewCheck("閉じてもトレイに常駐", func(v bool) { s.app.Preferences().SetBool("resident", v) })
	s.resident.SetChecked(s.app.Preferences().Bool("resident"))
	if a, ok := s.app.(fynedesktop.App); ok {
		a.SetSystemTrayMenu(fyne.NewMenu("Key Short", fyne.NewMenuItem("設定画面を開く", func() { s.w.Show(); s.w.RequestFocus() }), fyne.NewMenuItem("監視を停止", s.stopMonitoring), fyne.NewMenuItem("終了", func() { s.requestClose(true) })))
	} else {
		s.resident.Disable()
	}
	help := widget.NewLabel("アプリ起動には常駐監視が必要です。JSON の保存と USB への書き込み後に開始してください。")
	help.Wrapping = fyne.TextWrapWord
	return container.NewVBox(widget.NewLabelWithStyle("アプリランチャー", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), help, s.monitorButton, s.resident, s.monitorStatus)
}

func (s *studio) startMonitoring() {
	if s.monitorStop != nil {
		return
	}
	m, err := s.manager()
	if err != nil {
		s.fail(err)
		return
	}
	stop, err := host.Start(s.ctx, s.p, m, func(text string) {
		fyne.Do(func() {
			if s.monitorStop != nil && !s.closing {
				s.monitorStatus.SetText(text)
			}
		})
	})
	if err != nil {
		s.fail(err)
		return
	}
	s.monitorStop = stop
	s.monitorButton.SetText("ランチャー監視を停止")
	s.monitorStatus.SetText("ランチャー監視を開始しています…")
	s.app.Preferences().SetBool("launcher-enabled", !s.dirty && s.app.Preferences().String("profile-path") != "")
}

func (s *studio) stopMonitoring() {
	if s.monitorStop != nil {
		stop := s.monitorStop
		s.monitorStop = nil
		stop()
	}
	s.monitorButton.SetText("ランチャー監視を開始")
	s.monitorStatus.SetText("ランチャー停止中")
	s.app.Preferences().SetBool("launcher-enabled", false)
}

func (s *studio) shutdown() {
	if s.closing {
		return
	}
	s.closing = true
	s.cancel()
	if s.monitorStop != nil {
		stop := s.monitorStop
		s.monitorStop = nil
		stop()
	}
	s.launchWG.Wait()
	if s.launchManager != nil {
		s.launchManager.Close()
	}
}

func (s *studio) loadProfile(path string) error {
	p, err := profile.Load(path)
	if err != nil {
		return err
	}
	if s.monitorStop != nil {
		s.stopMonitoring()
	}
	s.p = p
	if p.LED == nil {
		s.led.SetSelectedIndex(0)
	} else {
		s.led.SetSelectedIndex(*p.LED + 1)
	}
	s.dirty = false
	s.refresh()
	s.status.SetText("JSON を読み込みました: " + path)
	s.app.Preferences().SetString("profile-path", path)
	return nil
}

type launchEditor struct {
	box                        *fyne.Container
	mode                       *widget.Select
	command, label, class, dir *widget.Entry
	reuse                      *widget.Check
	app                        desktop.Application
	onChanged                  func()
}

func (e *launchEditor) spec() protocol.LaunchSpec {
	s := protocol.LaunchSpec{ReuseWindow: e.reuse.Checked, WindowClass: strings.TrimSpace(e.class.Text), WorkingDir: strings.TrimSpace(e.dir.Text)}
	if e.mode.Selected == "アプリを選択" {
		s.Mode = "application"
		s.DesktopFile = e.app.Path
	} else {
		s.Mode = "command"
		s.Command = e.command.Text
	}
	return s
}

func (s *studio) newLaunchEditor(initial *protocol.LaunchSpec, label string) *launchEditor {
	e := &launchEditor{command: widget.NewEntry(), label: widget.NewEntry(), class: widget.NewEntry(), dir: widget.NewEntry()}
	e.label.SetPlaceHolder("例: Chromium · デバッグ")
	e.label.SetText(label)
	e.command.SetPlaceHolder("chromium --remote-allow-origins=* --remote-debugging-port=9222")
	e.reuse = widget.NewCheck("起動済みなら、そのウィンドウを前面にする", nil)
	selected := widget.NewLabel("アプリを選択してください")
	selected.Wrapping = fyne.TextWrapWord
	choose := widget.NewButton("アプリを選択…", func() {
		s.pickApplication(func(a desktop.Application) {
			e.app = a
			selected.SetText(a.Name)
			if e.label.Text == "" {
				e.label.SetText(a.Name)
			}
		})
	})
	appBox := container.NewBorder(nil, nil, nil, choose, selected)
	commandBox := widget.NewForm(widget.NewFormItem("コマンド", e.command))
	commandHelp := widget.NewLabel("引数と引用符を含むコマンドを入力できます。通常の起動は chromium だけでも指定できます。シェルの展開は行いません。")
	commandHelp.Wrapping = fyne.TextWrapWord
	e.mode = widget.NewSelect([]string{"アプリを選択", "コマンドラインで実行"}, func(v string) {
		if v == "アプリを選択" {
			appBox.Show()
			commandBox.Hide()
			commandHelp.Hide()
		} else {
			appBox.Hide()
			commandBox.Show()
			commandHelp.Show()
		}
		if e.box != nil {
			e.box.Refresh()
		}
		if e.onChanged != nil {
			e.onChanged()
		}
	})
	e.class.SetPlaceHolder("通常は空欄。必要な場合のみ WM_CLASS / resourceClass")
	e.dir.SetPlaceHolder("通常は空欄。指定する場合は絶対パス")
	advanced := widget.NewAccordion(widget.NewAccordionItem("詳細設定", widget.NewForm(widget.NewFormItem("ウィンドウクラス", e.class), widget.NewFormItem("作業フォルダー", e.dir))))
	message := widget.NewLabel("")
	message.Wrapping = fyne.TextWrapWord
	var testButton *widget.Button
	testButton = widget.NewButton("起動をテスト", func() {
		spec := e.spec()
		if err := spec.Validate(); err != nil {
			message.SetText(err.Error())
			return
		}
		m, err := s.manager()
		if err != nil {
			message.SetText(err.Error())
			return
		}
		testButton.Disable()
		message.SetText("起動・前面化を確認しています…")
		s.launchWG.Add(1)
		go func() {
			defer s.launchWG.Done()
			result, err := m.Run(s.ctx, spec)
			fyne.Do(func() {
				if s.closing {
					return
				}
				testButton.Enable()
				if err != nil {
					message.SetText(err.Error())
				} else {
					message.SetText(result.Message)
				}
			})
		}()
	})
	e.box = container.NewVBox(widget.NewForm(widget.NewFormItem("起動方法", e.mode), widget.NewFormItem("表示名", e.label)), appBox, commandBox, commandHelp, e.reuse, advanced, container.NewHBox(testButton, layout.NewSpacer()), message)
	if initial != nil {
		e.command.SetText(initial.Command)
		e.reuse.SetChecked(initial.ReuseWindow)
		e.class.SetText(initial.WindowClass)
		e.dir.SetText(initial.WorkingDir)
		if initial.Mode == "application" {
			e.app, _ = desktop.Read(initial.DesktopFile)
			e.app.Path = initial.DesktopFile
			name := e.app.Name
			if name == "" {
				name = initial.DesktopFile
			}
			selected.SetText(name)
			e.mode.SetSelectedIndex(0)
		} else {
			e.mode.SetSelectedIndex(1)
		}
	} else {
		e.mode.SetSelectedIndex(0)
	}
	return e
}

func (s *studio) pickApplication(onSelected func(desktop.Application)) {
	apps := desktop.Applications()
	filtered := append([]desktop.Application(nil), apps...)
	search := widget.NewEntry()
	search.SetPlaceHolder("アプリ名で検索")
	var d dialog.Dialog
	list := widget.NewList(func() int { return len(filtered) }, func() fyne.CanvasObject { return widget.NewLabel("") }, func(i widget.ListItemID, o fyne.CanvasObject) {
		o.(*widget.Label).SetText(fmt.Sprintf("%s  (%s)", filtered[i].Name, filtered[i].ID))
	})
	list.OnSelected = func(i widget.ListItemID) {
		if i >= 0 && i < len(filtered) {
			onSelected(filtered[i])
			d.Hide()
		}
	}
	search.OnChanged = func(text string) {
		list.UnselectAll()
		filtered = nil
		for _, a := range apps {
			if strings.Contains(strings.ToLower(a.Name+" "+a.ID), strings.ToLower(text)) {
				filtered = append(filtered, a)
			}
		}
		list.Refresh()
	}
	d = dialog.NewCustom("起動するアプリ", "閉じる", container.NewBorder(search, nil, nil, nil, list), s.w)
	d.Resize(fyne.NewSize(680, 460))
	d.Show()
	s.w.Canvas().Focus(search)
}
