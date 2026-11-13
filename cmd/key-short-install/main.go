package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"key-short/internal/autostart"
	"key-short/packaging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("install / install-autostart / uninstall を指定してください")
	}
	m, err := autostart.New()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if os.Args[1] == "uninstall" {
		if err = m.Uninstall(ctx); err != nil {
			return err
		}
		fmt.Println("INFO 監視デーモンと自動起動設定、アプリを削除しました。ユーザーの JSON と履歴は保持しています。")
		return nil
	}
	if os.Args[1] != "install" && os.Args[1] != "install-autostart" {
		return fmt.Errorf("不明なインストール操作です")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	root := filepath.Dir(filepath.Dir(self))
	if err = m.InstallApp(filepath.Join(root, "bin", "key-short"), packaging.Icon); err != nil {
		return err
	}
	fmt.Println("INFO インストールしました:", m.Binary)
	if os.Args[1] == "install-autostart" {
		if err = m.Enable(ctx, os.Getenv("KEY_SHORT_PROFILE"), os.Getenv("KEY_SHORT_AUTOSTART")); err != nil {
			return err
		}
		fmt.Println("INFO 次回のデスクトップログインから監視を自動起動します。今すぐ開始するには key-short -autostart start を実行してください。")
	}
	return nil
}
