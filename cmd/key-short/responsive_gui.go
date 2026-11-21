package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// Each pane belongs to exactly one visible host, preserving drafts and
// selection when the window changes between split and tabbed presentations.
type workspaceLayout struct {
	left, right                              fyne.CanvasObject
	split                                    *container.Split
	tabs                                     *container.AppTabs
	splitLeft, splitRight, tabLeft, tabRight *fyne.Container
	narrow, initialized                      bool
}

func newWorkspace(left, right fyne.CanvasObject) (*fyne.Container, *workspaceLayout) {
	l := &workspaceLayout{left: left, right: right, splitLeft: container.NewStack(), splitRight: container.NewStack(), tabLeft: container.NewStack(), tabRight: container.NewStack()}
	l.split = container.NewHSplit(l.splitLeft, l.splitRight)
	l.split.Offset = .48
	l.tabs = container.NewAppTabs(container.NewTabItem("キー配置", l.tabLeft), container.NewTabItem("キーの編集", l.tabRight))
	return container.New(l, l.split, l.tabs), l
}
func (l *workspaceLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(460, 240) }
func (l *workspaceLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	narrow := size.Width < 960
	if !l.initialized || narrow != l.narrow {
		l.initialized = true
		l.narrow = narrow
		l.splitLeft.Objects = nil
		l.splitRight.Objects = nil
		l.tabLeft.Objects = nil
		l.tabRight.Objects = nil
		if narrow {
			l.split.Hide()
			l.tabLeft.Objects = []fyne.CanvasObject{l.left}
			l.tabRight.Objects = []fyne.CanvasObject{l.right}
			l.tabs.Show()
		} else {
			l.tabs.Hide()
			l.splitLeft.Objects = []fyne.CanvasObject{l.left}
			l.splitRight.Objects = []fyne.CanvasObject{l.right}
			l.split.Show()
		}
		for _, host := range []*fyne.Container{l.splitLeft, l.splitRight, l.tabLeft, l.tabRight} {
			host.Refresh()
		}
	}
	if narrow {
		l.tabs.Move(fyne.NewPos(0, 0))
		l.tabs.Resize(size)
	} else {
		l.split.Move(fyne.NewPos(0, 0))
		l.split.Resize(size)
	}
}
