package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/device"
	"key-short/internal/launcher"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

type studio struct {
	w             fyne.Window
	p             profile.Profile
	layer         int
	buttons       []*widget.Button
	status, count *widget.Label
	devices, led  *widget.Select
	progress      *widget.ProgressBar
	controls      []fyne.Disableable
	dirty, busy   bool
	app           fyne.App
	ctx           context.Context
	cancel        context.CancelFunc
	launchManager *launcher.Manager
	launchWG      sync.WaitGroup
	monitorStop   func()
	monitorButton *widget.Button
	monitorStatus *widget.Label
	resident      *widget.Check
	closing       bool
}

func runGUI(profilePath string) error {
	// Fyne uses a single font; prefer a Japanese-capable system TTF.
	if os.Getenv("FYNE_FONT") == "" {
		for _, path := range []string{"/usr/share/fonts/droid/DroidSansFallbackFull.ttf", "/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf", "/usr/share/fonts/truetype/noto/NotoSansJP-Regular.ttf"} {
			if _, err := os.Stat(path); err == nil {
				os.Setenv("FYNE_FONT", path)
				break
			}
		}
	}
	a := app.NewWithID("io.keyshort.configurator")
	s := newStudio(a)
	defer s.shutdown()
	if profilePath != "" {
		if err := s.loadProfile(profilePath); err != nil {
			return err
		}
	}
	if a.Preferences().Bool("launcher-enabled") {
		s.startMonitoring()
	}
	s.w.ShowAndRun()
	return nil
}

func newStudio(a fyne.App) *studio {
	ctx, cancel := context.WithCancel(context.Background())
	s := &studio{app: a, ctx: ctx, cancel: cancel, w: a.NewWindow("Key Short · MINI KeyBoard"), p: profile.New(), layer: 1}
	s.w.Resize(fyne.NewSize(1100, 780))
	s.status = widget.NewLabel("割り当てを追加し、JSON に保存するか USB に書き込んでください。")
	s.status.Wrapping = fyne.TextWrapWord
	s.count = widget.NewLabel("")
	s.devices = widget.NewSelect(nil, nil)
	s.devices.PlaceHolder = "デバイス未検出"
	refresh := widget.NewButtonWithIcon("再検出", theme.ViewRefreshIcon(), s.scan)
	s.controls = append(s.controls, s.devices, refresh)
	layers := widget.NewSelect([]string{"レイヤー 1", "レイヤー 2", "レイヤー 3"}, func(value string) { s.layer = int(value[len(value)-1] - '0'); s.refresh() })
	layers.SetSelectedIndex(0)
	s.controls = append(s.controls, layers)
	grid := container.NewGridWithColumns(3)
	knobs := container.NewGridWithColumns(2)
	for slot := 1; slot <= 18; slot++ {
		n := slot
		b := widget.NewButton("", func() { s.edit(n) })
		s.buttons = append(s.buttons, b)
		s.controls = append(s.controls, b)
		if slot <= 12 {
			grid.Add(b)
		}
	}
	for knob := 0; knob < 2; knob++ {
		box := container.NewVBox(widget.NewLabelWithStyle(fmt.Sprintf("ノブ %d", knob+1), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
		for i := 0; i < 3; i++ {
			box.Add(s.buttons[12+knob*3+i])
		}
		knobs.Add(box)
	}
	keyArea := container.NewVBox(widget.NewLabelWithStyle("キーを選んで編集", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), grid, widget.NewSeparator(), knobs)
	s.led = widget.NewSelect([]string{"変更しない", "0 · 消灯", "1 · モード1", "2 · モード2"}, func(value string) {
		if value == "変更しない" {
			s.p.LED = nil
		} else {
			mode := int(value[0] - '0')
			s.p.LED = &mode
		}
		s.changed()
	})
	s.led.SetSelectedIndex(0)
	s.dirty = false
	s.controls = append(s.controls, s.led)
	help := widget.NewLabel("12キー / 2ノブ\n\nキーとノブは同じ方法で設定できます。\n\nショートカット: ctrl+shift+s\n連続操作: ctrl+c,ctrl+v\n最大5操作 / F1〜F24 対応\n\n未設定の項目は書き込みません。\n本体の現在の割り当ては読み出せません。\n\nLED の見え方やレイヤー対応は\nファームウェアにより異なります。")
	help.Wrapping = fyne.TextWrapWord
	side := container.NewVBox(widget.NewLabelWithStyle("MINI KeyBoard", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewLabel("1189:8890 · CH57x"), widget.NewSeparator(), s.launcherControls(), widget.NewSeparator(), help, widget.NewSeparator(), widget.NewLabel("LED"), s.led)
	split := container.NewHSplit(container.NewPadded(container.NewVScroll(keyArea)), container.NewPadded(container.NewVScroll(side)))
	split.Offset = 0.68
	open := widget.NewButtonWithIcon("JSON を開く", theme.FolderOpenIcon(), s.open)
	save := widget.NewButtonWithIcon("JSON に保存", theme.DocumentSaveIcon(), s.save)
	preview := widget.NewButton("送信データを確認", s.preview)
	write := widget.NewButtonWithIcon("USB に書き込む", theme.UploadIcon(), s.write)
	write.Importance = widget.HighImportance
	s.controls = append(s.controls, open, save, preview, write)
	s.progress = widget.NewProgressBar()
	s.progress.Hide()
	header := container.NewBorder(nil, nil, widget.NewLabelWithStyle("Key Short", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), layers, container.NewBorder(nil, nil, nil, refresh, s.devices))
	footer := container.NewVBox(s.count, container.NewHBox(open, save, preview, layout.NewSpacer(), write), s.progress, s.status)
	s.w.SetContent(container.NewPadded(container.NewBorder(header, footer, nil, nil, split)))
	s.w.SetCloseIntercept(func() {
		if s.busy {
			dialog.ShowInformation("送信中", "USB への送信完了を待ってください。", s.w)
			return
		}
		s.requestClose(false)
	})
	s.refresh()
	s.scan()
	if path := a.Preferences().String("profile-path"); path != "" {
		if err := s.loadProfile(path); err != nil {
			s.status.SetText("前回の JSON を読み込めません: " + err.Error())
		}
	}
	return s
}

func (s *studio) requestClose(force bool) {
	if s.busy {
		s.w.Show()
		dialog.ShowInformation("送信中", "USB への送信完了を待ってください。", s.w)
		return
	}
	if !force && s.monitorStop != nil && s.resident.Checked {
		s.w.Hide()
		return
	}
	if s.dirty {
		dialog.ShowConfirm("未保存の設定", "JSON に保存していない変更があります。終了しますか？", func(ok bool) {
			if ok {
				s.shutdown()
				s.w.SetCloseIntercept(nil)
				s.app.Quit()
			}
		}, s.w)
		return
	}
	s.shutdown()
	s.w.SetCloseIntercept(nil)
	s.app.Quit()
}

func (s *studio) changed() {
	if s.monitorStop != nil {
		s.stopMonitoring()
		s.monitorStatus.SetText("設定を変更したため停止しました。USB 書き込み後に監視を再開してください。")
	}
	s.dirty = true
	s.refresh()
}
func (s *studio) refresh() {
	for i, b := range s.buttons {
		value := "未設定"
		if binding, ok := s.p.Get(s.layer, i+1); ok {
			value = binding.DisplayValue()
		}
		if runes := []rune(value); len(runes) > 24 {
			value = string(runes[:23]) + "…"
		}
		b.SetText(protocol.SlotName(i+1) + "\n" + value)
	}
	mark := ""
	if s.dirty {
		mark = " · JSON 未保存"
	}
	s.count.SetText(fmt.Sprintf("%d 件の割り当て%s", len(s.p.Bindings), mark))
}

func (s *studio) fail(err error) {
	slog.Error("操作に失敗しました", "error", err)
	s.status.SetText(err.Error())
	dialog.ShowError(err, s.w)
}

func (s *studio) scan() {
	infos, err := device.List()
	if err != nil {
		s.fail(err)
		return
	}
	old := s.devices.Selected
	options := []string{}
	for _, info := range infos {
		options = append(options, info.ID())
	}
	s.devices.Options = options
	s.devices.ClearSelected()
	for _, id := range options {
		if id == old {
			s.devices.SetSelected(id)
		}
	}
	if s.devices.Selected == "" && len(options) == 1 {
		s.devices.SetSelected(options[0])
	}
	s.devices.Refresh()
	if len(options) == 0 {
		s.status.SetText("1189:8890 が見つかりません。USB を接続し、再検出してください。")
	} else {
		s.status.SetText("USB を検出しました。キーまたはノブを選んで編集できます。")
	}
}

func sortedNames[T any](m map[string]T) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *studio) edit(slot int) {
	layer := s.layer
	value := widget.NewSelectEntry(nil)
	kind := widget.NewSelect([]string{"キーボード", "メディア", "マウス", "ランチャー"}, nil)
	existing, _ := s.p.Get(layer, slot)
	launchLabel := ""
	if existing.Kind == "launcher" {
		launchLabel = existing.Value
	}
	launchFields := s.newLaunchEditor(existing.Launch, launchLabel)
	var d dialog.Dialog
	var content *fyne.Container
	launchFields.box.Hide()
	valueForm := widget.NewForm(widget.NewFormItem("割り当て", value))
	note := widget.NewLabel("")
	note.Wrapping = fyne.TextWrapWord
	kind.OnChanged = func(v string) {
		launchFields.box.Hide()
		valueForm.Show()
		note.Show()
		switch v {
		case "キーボード":
			value.SetOptions([]string{"ctrl+c", "ctrl+v", "ctrl+shift+s", "super+e", "f13", "f14", "f15", "ctrl+c,ctrl+v"})
			note.SetText("キー名を入力。修飾キーは +、連続操作は , で区切ります（最大5操作）。\n記号は comma / dot / slash / equal などの名前を使います。")
		case "メディア":
			value.SetOptions(sortedNames(protocol.Media))
			note.SetText("volume_down / volume_up / mute / play_pause / previous / next / stop")
		case "マウス":
			value.SetOptions(sortedNames(protocol.Mouse))
			note.SetText("left_click / right_click / middle_click / wheel_up / wheel_down\nctrl+wheel_up のような修飾キーも指定できます。")
		case "ランチャー":
			launchFields.box.Show()
			valueForm.Hide()
			note.Hide()
		}
		if content != nil {
			content.Refresh()
		}
		if d != nil {
			d.Refresh()
		}
	}
	if b, ok := s.p.Get(layer, slot); ok {
		kind.SetSelected(map[string]string{"keyboard": "キーボード", "media": "メディア", "mouse": "マウス", "launcher": "ランチャー"}[b.Kind])
		value.SetText(b.Value)
	} else {
		kind.SetSelectedIndex(0)
	}
	validation := widget.NewLabel("")
	validation.Wrapping = fyne.TextWrapWord
	makeBinding := func() protocol.Binding {
		if kind.Selected == "ランチャー" {
			spec := launchFields.spec()
			return protocol.Binding{Layer: layer, Slot: slot, Kind: "launcher", Value: launchFields.label.Text, Launch: &spec}
		}
		return protocol.Binding{Layer: layer, Slot: slot, Kind: map[string]string{"キーボード": "keyboard", "メディア": "media", "マウス": "mouse"}[kind.Selected], Value: strings.TrimSpace(value.Text)}
	}
	add := widget.NewButton("設定に追加", func() {
		b := makeBinding()
		if _, err := b.Frames(); err != nil {
			validation.SetText(err.Error())
			return
		}
		s.p.Set(b)
		s.changed()
		s.status.SetText("設定に追加しました。USB への送信は「USB に書き込む」で実行します。")
		d.Hide()
	})
	add.Importance = widget.HighImportance
	value.OnChanged = func(string) { validation.SetText("") }
	value.OnSubmitted = func(string) { add.OnTapped() }
	remove := widget.NewButton("書き込み対象から外す", func() { s.p.Remove(layer, slot); s.changed(); d.Hide() })
	content = container.NewVBox(widget.NewForm(widget.NewFormItem("操作", kind)), valueForm, note, launchFields.box, validation, container.NewHBox(remove, layout.NewSpacer(), add))
	launchFields.onChanged = func() {
		content.Refresh()
		if d != nil {
			d.Refresh()
		}
	}
	d = dialog.NewCustom(fmt.Sprintf("レイヤー %d / %s", layer, protocol.SlotName(slot)), "閉じる", content, s.w)
	d.Resize(fyne.NewSize(750, 560))
	d.Show()
	if kind.Selected == "ランチャー" {
		s.w.Canvas().Focus(launchFields.command)
	} else {
		s.w.Canvas().Focus(value)
	}
}

func (s *studio) save() {
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			s.fail(err)
			return
		}
		if w == nil {
			return
		}
		path := w.URI().Path()
		if err = w.Close(); err != nil {
			s.fail(err)
			return
		}
		if err = s.p.Save(path); err != nil {
			s.fail(err)
			return
		}
		s.dirty = false
		s.app.Preferences().SetString("profile-path", path)
		if s.monitorStop != nil {
			s.app.Preferences().SetBool("launcher-enabled", true)
		}
		s.refresh()
		s.status.SetText("JSON に保存しました: " + path)
	}, s.w)
	d.SetFileName("key-short.json")
	d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	d.Show()
}

func (s *studio) open() {
	show := func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				s.fail(err)
				return
			}
			if r == nil {
				return
			}
			defer r.Close()
			p, err := profile.Decode(r)
			if err != nil {
				s.fail(err)
				return
			}
			s.p = p
			if s.monitorStop != nil {
				s.stopMonitoring()
			}
			if p.LED == nil {
				s.led.SetSelectedIndex(0)
			} else {
				s.led.SetSelectedIndex(*p.LED + 1)
			}
			s.dirty = false
			s.app.Preferences().SetString("profile-path", r.URI().Path())
			s.refresh()
			s.status.SetText("JSON を読み込みました。USB に送信する前に設定を確認してください。")
		}, s.w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		d.Show()
	}
	if s.dirty {
		dialog.ShowConfirm("未保存の設定", "編集中の設定を JSON ファイルの内容に置き換えますか？", func(ok bool) {
			if ok {
				show()
			}
		}, s.w)
	} else {
		show()
	}
}

func (s *studio) preview() {
	jobs, err := s.p.Jobs()
	if err != nil {
		s.fail(err)
		return
	}
	var text strings.Builder
	for _, job := range jobs {
		fmt.Fprintln(&text, job.Label)
		for _, f := range job.Frames {
			fmt.Fprintf(&text, "% x\n", f[:])
		}
	}
	if len(jobs) == 0 {
		text.WriteString("設定は空です。キーやノブを選んで割り当ててください。")
	}
	entry := widget.NewMultiLineEntry()
	entry.SetText(text.String())
	entry.Wrapping = fyne.TextWrapOff
	d := dialog.NewCustom("送信データ（この画面では送信しません）", "閉じる", container.NewScroll(entry), s.w)
	d.Resize(fyne.NewSize(900, 500))
	d.Show()
}

func (s *studio) write() {
	jobs, err := s.p.Jobs()
	if err != nil {
		s.fail(err)
		return
	}
	if len(jobs) == 0 {
		s.fail(fmt.Errorf("書き込む設定がありません"))
		return
	}
	id := s.devices.Selected
	if id == "" {
		s.fail(fmt.Errorf("対象の USB デバイスを選んでください"))
		return
	}
	dialog.ShowConfirm("USB に書き込む", fmt.Sprintf("USB %s に %d 件を送信します。対象の割り当ては上書きされます。続行しますか？", id, len(jobs)), func(ok bool) {
		if !ok {
			return
		}
		s.busy = true
		for _, c := range s.controls {
			c.Disable()
		}
		s.progress.SetValue(0)
		s.progress.Show()
		s.status.SetText("設定データを送信しています…")
		go func() {
			err := device.Write(id, jobs, func(label string, done, total int) {
				fyne.Do(func() { s.progress.SetValue(float64(done) / float64(total)); s.status.SetText("送信中: " + label) })
			})
			fyne.Do(func() {
				s.busy = false
				for _, c := range s.controls {
					c.Enable()
				}
				s.progress.Hide()
				if err != nil {
					s.fail(err)
				} else {
					s.status.SetText("設定データを送信しました。実際のキー・ノブ操作で動作を確認してください。")
				}
			})
		}()
	}, s.w)
}
