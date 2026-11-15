package main

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	devlayout "key-short/internal/layout"
)

// Limit labels to their actual cells while retaining Fyne's native Button
// renderer, focus feedback, keyboard handling and theme behavior.
func fittedButtonText(text string, width float32, fontSize float32) string {
	available := fyne.Max(12, width-40)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if fyne.MeasureText(line, fontSize, fyne.TextStyle{Bold: true}).Width <= available {
			continue
		}
		runes := []rune(line)
		for len(runes) > 0 && fyne.MeasureText(string(runes)+"…", fontSize, fyne.TextStyle{Bold: true}).Width > available {
			runes = runes[:len(runes)-1]
		}
		lines[i] = string(runes) + "…"
	}
	return strings.Join(lines, "\n")
}

type compactButton struct {
	widget.Button
	fullText                string
	fittedWidth, fittedFont float32
}

func newCompactButton(label string, tapped func()) *compactButton {
	b := &compactButton{}
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	b.SetText(label)
	return b
}
func (b *compactButton) MinSize() fyne.Size { return fyne.NewSize(150, 36) }
func (b *compactButton) SetText(text string) {
	if text != b.fullText {
		b.fittedWidth = 0
	}
	b.fullText = text
	b.fitText()
}
func (b *compactButton) fitText() {
	width, font := fyne.Max(150, b.Size().Width), b.Theme().Size(theme.SizeNameText)
	if width == b.fittedWidth && font == b.fittedFont {
		return
	}
	b.fittedWidth, b.fittedFont = width, font
	text := fittedButtonText(b.fullText, width, font)
	if b.Text != text {
		b.Button.SetText(text)
	}
}
func (b *compactButton) Resize(size fyne.Size) { b.Button.Resize(size); b.fitText() }

// The diagram uses the available height, so all physical inputs remain visible.
// Its minimum does not grow with the number of rows in a custom layout.
type deviceDiagramLayout struct {
	layout      devlayout.Layout
	grid, knobs fyne.CanvasObject
}

func (l *deviceDiagramLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(240, 160) }
func (l *deviceDiagramLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	rows := (len(l.layout.Slots) + l.layout.Columns - 1) / l.layout.Columns
	n := len(l.layout.Knobs)
	gap := theme.Padding()
	if n == 0 {
		l.grid.Move(fyne.NewPos(0, 0))
		l.grid.Resize(size)
		return
	}
	if l.layout.KnobPosition == "left" || l.layout.KnobPosition == "right" {
		knobWidth := (size.Width - gap) * .46
		gridWidth := size.Width - gap - knobWidth
		gridX, knobX := float32(0), gridWidth+gap
		if l.layout.KnobPosition == "left" {
			knobX, gridX = 0, knobWidth+gap
		}
		l.grid.Move(fyne.NewPos(gridX, 0))
		l.grid.Resize(fyne.NewSize(gridWidth, size.Height))
		l.knobs.Move(fyne.NewPos(knobX, 0))
		l.knobs.Resize(fyne.NewSize(knobWidth, size.Height))
		return
	}
	unit := fyne.Min(86, (size.Height-gap*float32(rows+n-1))/float32(rows+n))
	gridHeight := unit*float32(rows) + gap*float32(rows-1)
	knobHeight := unit*float32(n) + gap*float32(n-1)
	top := (size.Height - gridHeight - knobHeight - gap) / 2
	gridY, knobY := top, top+gridHeight+gap
	if l.layout.KnobPosition == "top" {
		knobY, gridY = top, top+knobHeight+gap
	}
	l.grid.Move(fyne.NewPos(0, gridY))
	l.grid.Resize(fyne.NewSize(size.Width, gridHeight))
	l.knobs.Move(fyne.NewPos(0, knobY))
	l.knobs.Resize(fyne.NewSize(size.Width, knobHeight))
}
func (s *studio) deviceDiagram(l devlayout.Layout) fyne.CanvasObject {
	grid := container.NewGridWithColumns(l.Columns)
	for _, slot := range l.Slots {
		grid.Add(s.buttons[slot-1])
	}
	knobs := container.NewGridWithRows(max(1, len(l.Knobs)))
	for i, k := range l.Knobs {
		name := widget.NewLabelWithStyle("ノブ "+string(rune('1'+i)), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		knobs.Add(container.NewBorder(nil, nil, name, nil, container.NewGridWithColumns(3, s.buttons[k.Left-1], s.buttons[k.Press-1], s.buttons[k.Right-1])))
	}
	fitted := &deviceDiagramLayout{layout: l, grid: grid, knobs: knobs}
	return container.New(fitted, grid, knobs)
}
