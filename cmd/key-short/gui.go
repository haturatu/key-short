package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	devlayout "key-short/internal/layout"
	"key-short/internal/profile"
	"key-short/internal/protocol"
	"key-short/packaging"
)

type studio struct {
	w                                      fyne.Window
	p                                      profile.Profile
	layer                                  int
	buttons                                []*assignmentButton
	status, count                          *widget.Label
	devices, led                           *widget.Select
	progress                               *widget.ProgressBar
	controls                               []fyne.Disableable
	dirty, busy                            bool
	app                                    fyne.App
	ctx                                    context.Context
	cancel                                 context.CancelFunc
	launchManager                          *launcher.Manager
	launchWG                               sync.WaitGroup
	monitorStop                            func()
	monitorButton                          *widget.Button
	monitorStatus                          *widget.Label
	resident                               *widget.Check
	closing                                bool
	loading                                bool
	selectedSlot                           int
	editor                                 *bindingEditor
	editorHost, keyHost                    *fyne.Container
	layers, layouts                        *widget.Select
	jsonState, usbState                    *widget.Label
	layoutNote                             *widget.Label
	saveButton, writeButton, previewButton *widget.Button
	saveAsButton                           *widget.Button
	profilePath, savedSignature            string
	usbFailedTarget                        string
	usbSignature, usbTarget                string
	usbFailed                              bool
	deviceInfos                            []device.Info
	monitorGeneration                      uint64
	dragTarget                             int
	keyScroll                              *container.Scroll
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
	a.SetIcon(fyne.NewStaticResource("key-short.svg", packaging.Icon))
	s := newStudio(a)
	defer s.shutdown()
	if profilePath != "" {
		if err := s.loadProfile(profilePath); err != nil {
			return err
		}
		a.Preferences().SetBool("launcher-enabled", false)
	}
	if a.Preferences().Bool("launcher-enabled") {
		s.startMonitoring()
	}
	s.w.ShowAndRun()
	return nil
}

func newStudio(a fyne.App) *studio {
	ctx, cancel := context.WithCancel(context.Background())
	s := &studio{app: a, ctx: ctx, cancel: cancel, w: a.NewWindow("Key Short"), p: profile.New(), layer: 1, selectedSlot: 1, loading: true}
	s.w.Resize(fyne.NewSize(1180, 860))
	s.status = widget.NewLabel("キーを選び、右側で編集して適用してください。USB への送信は別の操作です。")
	s.status.Wrapping = fyne.TextWrapWord
	s.count = widget.NewLabel("")
	s.jsonState = widget.NewLabel("")
	s.usbState = widget.NewLabel("")
	s.jsonState.Wrapping = fyne.TextWrapWord
	s.usbState.Wrapping = fyne.TextWrapWord
	s.devices = widget.NewSelect(nil, func(string) {
		if !s.loading {
			s.deviceSelected()
			s.refresh()
		}
	})
	s.devices.PlaceHolder = "デバイス未検出"
	rescan := widget.NewButtonWithIcon("再検出", theme.ViewRefreshIcon(), s.scan)
	s.layers = widget.NewSelect([]string{"レイヤー 1", "レイヤー 2", "レイヤー 3"}, func(v string) {
		if !s.loading {
			s.selectInput(int(v[len(v)-1]-'0'), s.selectedSlot)
		}
	})
	s.layers.SetSelectedIndex(0)
	for slot := 1; slot <= 18; slot++ {
		b := newAssignmentButton(s, slot)
		s.buttons = append(s.buttons, b)
		s.controls = append(s.controls, b)
	}
	s.layouts = widget.NewSelect(nil, func(v string) {
		if s.loading {
			return
		}
		for _, preset := range devlayout.Presets() {
			if preset.Name == v {
				s.requestLayout(preset.Layout)
				return
			}
		}
		if v == "カスタム配置…" {
			s.layoutDialog()
		}
	})
	for _, preset := range devlayout.Presets() {
		s.layouts.Options = append(s.layouts.Options, preset.Name)
	}
	s.layouts.Options = append(s.layouts.Options, "カスタム配置…")
	custom := widget.NewButton("スロットの並びを調整…", s.layoutDialog)
	s.keyHost = container.NewVBox()
	s.led = widget.NewSelect([]string{"変更しない", "0 · 消灯", "1 · モード1", "2 · モード2"}, func(v string) {
		if s.loading {
			return
		}
		if v == "変更しない" {
			s.p.LED = nil
		} else {
			mode := int(v[0] - '0')
			s.p.LED = &mode
		}
		s.changed()
	})
	s.led.SetSelectedIndex(0)
	launchControls := s.launcherControls()
	note := widget.NewLabel("割り当てをドラッグして別のキーへ移動できます。配置の変更だけでは USB に書き込みません。")
	note.Wrapping = fyne.TextWrapWord
	s.layoutNote = widget.NewLabel("機種未確定 · 配置を選択してください。")
	s.layoutNote.Wrapping = fyne.TextWrapWord
	keyArea := container.NewVBox(widget.NewLabelWithStyle("キー配置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), s.layouts, custom, s.layoutNote, note, s.keyHost,
		widget.NewAccordion(widget.NewAccordionItem("LED・ランチャー設定", container.NewVBox(widget.NewLabel("LED"), s.led, launchControls))))
	s.editorHost = container.NewVBox()
	s.keyScroll = container.NewVScroll(keyArea)
	split := container.NewHSplit(container.NewPadded(s.keyScroll), container.NewPadded(container.NewVScroll(s.editorHost)))
	split.Offset = 0.48
	open := widget.NewButtonWithIcon("JSON を開く", theme.FolderOpenIcon(), s.open)
	s.saveButton = widget.NewButtonWithIcon("保存", theme.DocumentSaveIcon(), s.save)
	saveAs := widget.NewButton("別名で保存", s.saveAs)
	s.saveAsButton = saveAs
	s.previewButton = widget.NewButton("送信データを確認", s.preview)
	s.writeButton = widget.NewButtonWithIcon("USB に書き込む", theme.UploadIcon(), s.write)
	s.writeButton.Importance = widget.HighImportance
	s.controls = append(s.controls, s.devices, rescan, s.layers, s.layouts, custom, s.led, open, s.saveButton, saveAs, s.previewButton, s.writeButton)
	s.progress = widget.NewProgressBar()
	s.progress.Hide()
	header := container.NewVBox(container.NewBorder(nil, nil, widget.NewLabelWithStyle("Key Short", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), s.layers, container.NewBorder(nil, nil, nil, rescan, s.devices)), s.count)
	footer := container.NewVBox(container.NewGridWithColumns(3, s.jsonState, s.usbState, s.monitorStatus), container.NewHBox(open, s.previewButton, layout.NewSpacer(), saveAs, s.saveButton, s.writeButton), s.progress, s.status)
	s.w.SetContent(container.NewPadded(container.NewBorder(header, footer, nil, nil, split)))
	s.w.SetCloseIntercept(func() { s.requestClose(false) })
	s.loading = false
	s.savedSignature = profileSignature(s.p)
	s.activateSelection(1, 1)
	s.rebuildLayout()
	s.refresh()
	// A saved profile is authoritative over a remembered device layout.
	if path := a.Preferences().String("profile-path"); path != "" {
		if err := s.loadProfile(path); err != nil {
			s.status.SetText("前回の JSON を読み込めません: " + err.Error())
		}
	}
	s.scan()
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
	if s.dirty || s.hasDraft() {
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

func profileSignature(p profile.Profile) string {
	data, _ := json.Marshal(p)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func wireSignature(p profile.Profile) string {
	jobs, err := p.Jobs()
	if err != nil {
		return ""
	}
	var frames []protocol.Frame
	for _, job := range jobs {
		frames = append(frames, job.Frames...)
	}
	data, _ := json.Marshal(frames)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func (s *studio) hasDraft() bool { return s.editor != nil && s.editor.dirty }
func (s *studio) changed() {
	if s.monitorStop != nil {
		s.stopMonitoring()
		s.monitorStatus.SetText("ランチャー停止中 · 設定変更のため停止")
	}
	s.dirty = profileSignature(s.p) != s.savedSignature
	s.refresh()
	if s.editor != nil {
		s.editor.validate()
	}
}
func (s *studio) refresh() {
	for i, b := range s.buttons {
		value := "未設定"
		if binding, ok := s.p.Get(s.layer, i+1); ok {
			value = binding.DisplayValue()
		}
		if runes := []rune(value); len(runes) > 22 {
			value = string(runes[:21]) + "…"
		}
		b.SetText(protocol.SlotName(i+1) + "\n" + value)
		b.Importance = widget.MediumImportance
		if s.selectedSlot == i+1 {
			b.Importance = widget.HighImportance
		}
		if s.dragTarget == i+1 {
			b.Importance = widget.SuccessImportance
		}
		b.Refresh()
	}
	name := "新規プロファイル"
	if s.profilePath != "" {
		name = filepath.Base(s.profilePath)
	}
	draft := ""
	if s.hasDraft() {
		draft = " · キーに未適用の変更あり"
	}
	s.count.SetText(fmt.Sprintf("%s · %d 件の割り当て%s", name, len(s.p.Bindings), draft))
	state := "JSON 保存済み"
	if s.profilePath == "" || s.dirty {
		state = "JSON 未保存"
	}
	s.jsonState.SetText(state)
	usb := "USB 未送信 / 現在の設定は不明"
	if s.usbSignature != "" && s.usbTarget == s.devices.Selected {
		usb = "USB 未反映の変更あり"
		if s.usbSignature == wireSignature(s.p) {
			usb = "USB 送信済み · 実機未検証"
		}
	}
	if s.usbFailed && s.usbFailedTarget == s.devices.Selected {
		usb = "USB 送信失敗 · 一部反映の可能性あり"
	}
	if s.busy {
		usb = "USB 送信中…"
	}
	s.usbState.SetText(usb)
	for _, c := range s.controls {
		if s.busy {
			c.Disable()
		} else {
			c.Enable()
		}
	}
	if s.hasDraft() {
		s.saveButton.Disable()
		s.saveAsButton.Disable()
		s.writeButton.Disable()
		s.previewButton.Disable()
		if s.monitorStop == nil {
			s.monitorButton.Disable()
		}
	}
	if jobs, err := s.p.Jobs(); err != nil {
		s.writeButton.Disable()
		s.previewButton.Disable()
		if !s.busy {
			s.status.SetText(err.Error())
		}
		if s.monitorStop == nil {
			s.monitorButton.Disable()
		}
	} else if len(jobs) == 0 {
		s.writeButton.Disable()
	}
	if s.devices.Selected == "" {
		s.writeButton.Disable()
	}
	if s.editor != nil {
		s.editor.updateControls()
	}
	if s.w.Content() != nil {
		s.w.Content().Refresh()
	}
}

func (s *studio) fail(err error) {
	slog.Error("操作に失敗しました", "error", err)
	s.status.SetText(err.Error())
	dialog.ShowError(err, s.w)
}

func (s *studio) scan() {
	if s.busy {
		return
	}
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
	s.loading = true
	s.deviceInfos = infos
	s.devices.ClearSelected()
	for _, id := range options {
		if id == old {
			s.devices.SetSelected(id)
		}
	}
	if s.devices.Selected == "" && len(options) == 1 {
		s.devices.SetSelected(options[0])
	}
	s.loading = false
	s.devices.Refresh()
	s.deviceSelected()
	s.refresh()
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
			s.installProfile(p, r.URI().Path())
			s.status.SetText("JSON を読み込みました。USB に送信する前に設定を確認してください。")
		}, s.w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		d.Show()
	}
	if s.dirty || s.hasDraft() {
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
	if s.busy || s.hasDraft() {
		return
	}
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
	if s.busy || s.hasDraft() {
		return
	}
	snapshot := s.p.Clone()
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
		if s.busy || s.hasDraft() || id != s.devices.Selected || profileSignature(snapshot) != profileSignature(s.p) {
			s.fail(fmt.Errorf("設定または対象デバイスが変わりました。確認し直してください"))
			return
		}
		s.busy = true
		s.refresh()
		s.progress.SetValue(0)
		s.progress.Show()
		s.status.SetText("設定データを送信しています…")
		go func() {
			err := device.Write(id, jobs, func(label string, done, total int) {
				fyne.Do(func() { s.progress.SetValue(float64(done) / float64(total)); s.status.SetText("送信中: " + label) })
			})
			fyne.Do(func() {
				s.busy = false

				s.progress.Hide()
				s.usbFailed = err != nil
				s.usbFailedTarget = id
				if err == nil {
					s.usbSignature = wireSignature(snapshot)
					s.usbTarget = id
				}
				s.refresh()
				if err != nil {
					s.fail(err)
				} else {
					s.status.SetText("設定データを送信しました。実際のキー・ノブ操作で動作を確認してください。")
				}
			})
		}()
	}, s.w)
}
