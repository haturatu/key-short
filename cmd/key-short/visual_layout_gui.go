package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	devlayout "key-short/internal/layout"
)

type visualLayoutEditor struct {
	s           *studio
	draft       devlayout.Layout
	grid, knobs *fyne.Container
	selected    int
	tiles       []*layoutTile
	hint        *widget.Label
}

// A tile supports the same swap through a drag or two keyboard/click selections.
type layoutTile struct {
	widget.Button
	editor   *visualLayoutEditor
	index    int
	last     fyne.Position
	dragging bool
}

func (b *layoutTile) MinSize() fyne.Size        { return b.Button.MinSize().Max(fyne.NewSize(72, 64)) }
func (b *layoutTile) Dragged(e *fyne.DragEvent) { b.last = e.AbsolutePosition; b.dragging = true }
func (b *layoutTile) DragEnd() {
	if !b.dragging {
		return
	}
	b.dragging = false
	for _, target := range b.editor.tiles {
		if b.editor.s.pointInside(target, b.last) {
			b.editor.swap(b.index, target.index)
			return
		}
	}
}
func (e *visualLayoutEditor) choose(index int) {
	if e.selected < 0 {
		e.selected = index
		e.render()
		return
	}
	e.swap(e.selected, index)
}
func (e *visualLayoutEditor) swap(from, to int) {
	if from < 12 && to < 12 {
		if from >= len(e.draft.Slots) || to >= len(e.draft.Slots) {
			return
		}
		e.draft.Slots[from], e.draft.Slots[to] = e.draft.Slots[to], e.draft.Slots[from]
	} else if from >= 12 && to >= 12 {
		from -= 12
		to -= 12
		if from >= len(e.draft.Knobs) || to >= len(e.draft.Knobs) {
			return
		}
		e.draft.Knobs[from], e.draft.Knobs[to] = e.draft.Knobs[to], e.draft.Knobs[from]
	} else {
		e.hint.SetText("キー同士、またはノブ同士を選んでください。")
		e.selected = -1
		return
	}
	e.selected = -1
	e.render()
}
func (e *visualLayoutEditor) tile(index int, label string) *layoutTile {
	b := &layoutTile{editor: e, index: index}
	b.ExtendBaseWidget(b)
	b.SetText(label)
	b.OnTapped = func() { e.choose(index) }
	if e.selected == index {
		b.Importance = widget.HighImportance
	}
	e.tiles = append(e.tiles, b)
	return b
}
func (e *visualLayoutEditor) render() {
	e.grid.Layout = container.NewGridWithColumns(e.draft.Columns).Layout
	e.grid.Objects = nil
	e.knobs.Objects = nil
	e.tiles = nil
	for i, slot := range e.draft.Slots {
		e.grid.Add(e.tile(i, fmt.Sprintf("K%d", slot)))
	}
	for i, k := range e.draft.Knobs {
		index := i
		reverse := widget.NewButton("回転方向を反転", func() {
			k := &e.draft.Knobs[index]
			k.Left, k.Right = k.Right, k.Left
			e.render()
		})
		e.knobs.Add(container.NewVBox(e.tile(12+i, fmt.Sprintf("ノブ %d\n左 %d · 押す %d · 右 %d", i+1, k.Left, k.Press, k.Right)), reverse))
	}
	rows := (len(e.draft.Slots) + e.draft.Columns - 1) / e.draft.Columns
	hint := fmt.Sprintf("%dキー / %d列 × %d行 · 2つ選ぶかドラッグして交換", len(e.draft.Slots), e.draft.Columns, rows)
	if e.selected >= 0 {
		hint = "交換先を選択してください（同じ項目で選択解除）。"
	}
	e.hint.SetText(hint)
	e.grid.Refresh()
	e.knobs.Refresh()
}
func (s *studio) layoutDialog() {
	s.syncLayoutSelect()
	if s.busy {
		return
	}
	l := s.p.EffectiveLayout()
	e := &visualLayoutEditor{s: s, draft: l.Clone(), selected: -1, grid: container.NewGridWithColumns(l.Columns), knobs: container.NewVBox(), hint: widget.NewLabel("")}
	e.hint.Wrapping = fyne.TextWrapWord
	e.draft.Preset = "custom"
	options := func(n int) []string {
		out := []string{}
		for i := 1; i <= n; i++ {
			out = append(out, strconv.Itoa(i))
		}
		return out
	}
	keys := widget.NewSelect(options(12), nil)
	keys.SetSelected(strconv.Itoa(len(l.Slots)))
	cols := widget.NewSelect(options(12), nil)
	cols.SetSelected(strconv.Itoa(l.Columns))
	knobs := widget.NewSelect([]string{"0", "1", "2"}, nil)
	knobs.SetSelected(strconv.Itoa(len(l.Knobs)))
	positions := []string{"下", "上", "左", "右"}
	codes := []string{"bottom", "top", "left", "right"}
	position := widget.NewSelect(positions, nil)
	position.SetSelectedIndex(0)
	for i, code := range codes {
		if code == l.KnobPosition {
			position.SetSelectedIndex(i)
		}
	}
	keys.OnChanged = func(v string) {
		n, _ := strconv.Atoi(v)
		if n < len(e.draft.Slots) {
			e.draft.Slots = e.draft.Slots[:n]
		}
		for len(e.draft.Slots) < n {
			for slot := 1; slot <= 12; slot++ {
				found := false
				for _, s := range e.draft.Slots {
					if s == slot {
						found = true
					}
				}
				if !found {
					e.draft.Slots = append(e.draft.Slots, slot)
					break
				}
			}
		}
		e.selected = -1
		e.render()
	}
	cols.OnChanged = func(v string) { e.draft.Columns, _ = strconv.Atoi(v); e.render() }
	knobs.OnChanged = func(v string) {
		n, _ := strconv.Atoi(v)
		if n < len(e.draft.Knobs) {
			e.draft.Knobs = e.draft.Knobs[:n]
		}
		for len(e.draft.Knobs) < n {
			used := map[int]bool{}
			for _, k := range e.draft.Knobs {
				used[k.Left] = true
				used[k.Press] = true
				used[k.Right] = true
			}
			free := []int{}
			for slot := 13; slot <= 18; slot++ {
				if !used[slot] {
					free = append(free, slot)
				}
			}
			e.draft.Knobs = append(e.draft.Knobs, devlayout.Knob{Left: free[0], Press: free[1], Right: free[2]})
		}
		e.selected = -1
		e.render()
	}
	position.OnChanged = func(string) { e.draft.KnobPosition = codes[position.SelectedIndex()] }
	var d dialog.Dialog
	apply := widget.NewButton("この配置を使う", func() {
		if err := e.draft.Validate(); err != nil {
			e.hint.SetText(err.Error())
			return
		}
		d.Hide()
		s.requestLayout(e.draft)
	})
	apply.Importance = widget.HighImportance
	reset := widget.NewButton("リセット", func() {
		d.Hide()
		s.layoutDialog()
	})
	note := widget.NewLabel("表示上の配置を編集します。割り当ては移動しません。キー数を減らす場合は、隠れる割り当てを適用前に確認できます。")
	note.Wrapping = fyne.TextWrapWord
	form := widget.NewForm(widget.NewFormItem("キー数", keys), widget.NewFormItem("列数", cols), widget.NewFormItem("ノブ数", knobs), widget.NewFormItem("ノブの位置", position))
	e.render()
	d = dialog.NewCustom("ビジュアル配置エディター", "キャンセル", container.NewBorder(container.NewVBox(note, form, e.hint), container.NewGridWithColumns(2, reset, apply), nil, nil, container.NewVScroll(container.NewVBox(e.grid, e.knobs))), s.w)
	d.Resize(fyne.NewSize(650, 650))
	d.Show()
}
