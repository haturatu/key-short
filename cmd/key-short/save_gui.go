package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"key-short/internal/profile"
)

func (s *studio) saveSnapshot(path string, snapshot profile.Profile) error {
	if s.busy {
		return fmt.Errorf("USB への送信完了を待ってください")
	}
	if s.hasDraft() {
		return fmt.Errorf("キーの変更を適用してから保存してください")
	}
	if err := s.p.Validate(); err != nil {
		return err
	}
	if profileSignature(s.p) != profileSignature(snapshot) {
		return fmt.Errorf("保存の準備中に設定が変わりました。保存し直してください")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// Profile.Save validates before creating a temporary file, syncs and closes
	// it, then renames it over the destination. No destination writer is opened.
	if err = snapshot.Save(path); err != nil {
		return err
	}
	s.profilePath = path
	s.savedSignature = profileSignature(snapshot)
	s.dirty = false
	s.app.Preferences().SetString("profile-path", path)
	if s.monitorStop != nil {
		s.app.Preferences().SetBool("launcher-enabled", true)
	}
	s.refresh()
	s.status.SetText("JSON に保存しました: " + path)
	return nil
}

func (s *studio) save() {
	if s.hasDraft() {
		s.fail(fmt.Errorf("キーの変更を適用してから保存してください"))
		return
	}
	if err := s.p.Validate(); err != nil {
		s.fail(err)
		return
	}
	if s.profilePath != "" {
		if err := s.saveSnapshot(s.profilePath, s.p.Clone()); err != nil {
			s.fail(err)
		}
		return
	}
	s.saveAs()
}

func (s *studio) saveAs() {
	if s.busy || s.hasDraft() {
		s.fail(fmt.Errorf("編集内容を適用し、USB 送信が終わってから保存してください"))
		return
	}
	if err := s.p.Validate(); err != nil {
		s.fail(err)
		return
	}
	snapshot := s.p.Clone()
	folder := widget.NewEntry()
	name := widget.NewEntry()
	initial := s.profilePath
	if initial == "" {
		home, _ := os.UserHomeDir()
		initial = filepath.Join(home, "key-short.json")
	}
	folder.SetText(filepath.Dir(initial))
	name.SetText(filepath.Base(initial))
	preview := widget.NewLabel("")
	preview.Wrapping = fyne.TextWrapWord
	validation := widget.NewLabel("")
	validation.Wrapping = fyne.TextWrapWord
	var d dialog.Dialog
	pathValue := func() (string, error) {
		n := strings.TrimSpace(name.Text)
		if n == "" || n == "." || n == ".." || filepath.Base(n) != n || strings.ContainsAny(n, "/\\") {
			return "", fmt.Errorf("フォルダーとファイル名を分けて指定してください")
		}
		if !strings.HasSuffix(strings.ToLower(n), ".json") {
			n += ".json"
		}
		if !filepath.IsAbs(folder.Text) {
			return "", fmt.Errorf("保存先フォルダーは絶対パスで指定してください")
		}
		return filepath.Join(folder.Text, n), nil
	}
	commit := func(path string) {
		if err := s.saveSnapshot(path, snapshot); err != nil {
			validation.SetText(err.Error())
			return
		}
		d.Hide()
	}
	save := widget.NewButton("この名前で保存", func() {
		path, err := pathValue()
		if err != nil {
			validation.SetText(err.Error())
			return
		}
		if err = s.p.Validate(); err != nil {
			validation.SetText(err.Error())
			return
		}
		if profileSignature(s.p) != profileSignature(snapshot) {
			validation.SetText("設定が変わりました。保存し直してください。")
			return
		}
		info, err := os.Stat(folder.Text)
		if err != nil || !info.IsDir() {
			validation.SetText("保存先のフォルダーが見つかりません。")
			return
		}
		info, err = os.Stat(path)
		if err == nil {
			if info.IsDir() {
				validation.SetText("同じ名前のフォルダーがあります。")
				return
			}
			dialog.ShowCustomConfirm("既存の JSON を置き換える", "置き換えて保存", "戻る", widget.NewLabelWithStyle(filepath.Base(path)+" を置き換えますか？", fyne.TextAlignLeading, fyne.TextStyle{}), func(ok bool) {
				if ok {
					commit(path)
				}
			}, s.w)
			return
		}
		if !os.IsNotExist(err) {
			validation.SetText(err.Error())
			return
		}
		commit(path)
	})
	save.Importance = widget.HighImportance
	update := func(string) {
		path, err := pathValue()
		if err != nil {
			validation.SetText(err.Error())
			save.Disable()
			preview.SetText("")
			return
		}
		validation.SetText("")
		preview.SetText(path)
		save.Enable()
	}
	folder.OnChanged = update
	name.OnChanged = update
	browse := widget.NewButton("フォルダーを選択…", func() {
		picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				validation.SetText(err.Error())
				return
			}
			if uri != nil {
				folder.SetText(uri.Path())
			}
		}, s.w)
		if uri, err := storage.ListerForURI(storage.NewFileURI(folder.Text)); err == nil {
			picker.SetLocation(uri)
		}
		picker.Show()
	})
	content := container.NewVBox(widget.NewForm(widget.NewFormItem("保存先フォルダー", folder)), browse, widget.NewForm(widget.NewFormItem("ファイル名", name)), preview, validation, save)
	d = dialog.NewCustom("JSON に名前を付けて保存", "キャンセル", content, s.w)
	d.Resize(fyne.NewSize(680, 380))
	update("")
	d.Show()
	s.w.Canvas().Focus(name)
}
