package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/keycapture"
)

type captureControl struct {
	widget.BaseWidget
	rec                            keycapture.Recorder
	title, held, value, errorLabel *widget.Label
	changed                        func()
}

func newCaptureControl() *captureControl {
	c := &captureControl{title: widget.NewLabel("ここをクリックし、割り当てたいキーを押してください"), held: widget.NewLabel("修飾キー: なし"), value: widget.NewLabel("未記録"), errorLabel: widget.NewLabel("")}
	c.value.Wrapping = fyne.TextWrapWord
	c.errorLabel.Wrapping = fyne.TextWrapWord
	c.ExtendBaseWidget(c)
	return c
}
func (c *captureControl) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewCard("キー入力の記録", "", container.NewVBox(c.title, c.held, c.value, c.errorLabel)))
}
func (c *captureControl) sync() {
	c.held.SetText("修飾キー: " + c.rec.Modifiers())
	v := c.rec.Value()
	if v == "" {
		v = "未記録"
	}
	c.value.SetText(v)
	c.errorLabel.SetText(c.rec.Error)
	if c.changed != nil {
		c.changed()
	}
}
func (c *captureControl) KeyDown(e *fyne.KeyEvent)    { c.rec.Down(string(e.Name)); c.sync() }
func (c *captureControl) KeyUp(e *fyne.KeyEvent)      { c.rec.Up(string(e.Name)); c.sync() }
func (c *captureControl) TypedKey(*fyne.KeyEvent)     {}
func (c *captureControl) TypedRune(rune)              {}
func (c *captureControl) TypedShortcut(fyne.Shortcut) {}
func (c *captureControl) AcceptsTab() bool            { return true }
func (c *captureControl) FocusGained() {
	c.rec.Release()
	c.title.SetText("記録中 · A や Ctrl + PageUp などを押してください")
	c.sync()
}
func (c *captureControl) FocusLost() {
	c.rec.Release()
	c.title.SetText("記録は一時停止中です。ここをクリックすると再開します")
	c.sync()
}
func (c *captureControl) Tapped(*fyne.PointEvent) {
	fyne.CurrentApp().Driver().CanvasForObject(c).Focus(c)
}

func (s *studio) recordKeys(target *widget.SelectEntry) {
	c := newCaptureControl()
	var d dialog.Dialog
	accept := widget.NewButton("割り当てに取り込む", func() {
		if c.rec.Value() != "" {
			target.SetText(c.rec.Value())
			d.Hide()
		}
	})
	accept.Importance = widget.HighImportance
	accept.Disable()
	c.changed = func() {
		if c.rec.Value() == "" {
			accept.Disable()
		} else {
			accept.Enable()
		}
	}
	sequence := widget.NewCheck("連続操作を記録（最大5操作）", func(v bool) { c.rec.Sequence = v; c.rec.Clear(); c.sync(); s.w.Canvas().Focus(c) })
	clear := widget.NewButton("クリアして記録し直す", func() { c.rec.Clear(); c.sync(); s.w.Canvas().Focus(c) })
	note := widget.NewLabel("この記録欄にフォーカスがある間だけ入力を受け付けます。OS が予約するショートカットは取得できない場合があります。")
	note.Wrapping = fyne.TextWrapWord
	d = dialog.NewCustom("キー入力から割り当てを記録", "キャンセル", container.NewVBox(c, sequence, note, container.NewHBox(clear, accept)), s.w)
	d.SetOnClosed(func() { c.rec.Release() })
	d.Resize(fyne.NewSize(680, 370))
	d.Show()
	s.w.Canvas().Focus(c)
}
