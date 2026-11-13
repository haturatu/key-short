package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"key-short/internal/autostart"
	"key-short/internal/daemon"
	"key-short/internal/device"
	"key-short/internal/host"
	"key-short/internal/launcher"
	"key-short/internal/logging"
	"key-short/internal/profile"
)

func main() { os.Exit(run()) }

func run() int {
	list := flag.Bool("list", false, "USB デバイスを表示（書き込みなし）")
	dry := flag.String("dry-run", "", "JSON 設定の送信データを表示（書き込みなし）")
	apply := flag.String("apply", "", "JSON 設定を USB に書き込む")
	id := flag.String("device", "", "対象 USB バス:アドレス（例: 002:003）")
	verbose := flag.Bool("verbose", false, "DEBUG ログを表示")
	startup := flag.String("autostart", "", "自動起動の status / enable / disable / start / stop / restart")
	backend := flag.String("autostart-backend", "auto", "自動起動方式: auto / systemd / xdg")
	daemonProfile := flag.String("daemon", "", "自動起動用の管理付き監視（フォアグラウンド実行）")
	daemonLogs := flag.Bool("daemon-logs", false, "管理付き監視の stdout/stderr を状態フォルダーのログに保存")
	daemonState := flag.String("daemon-state", "", "管理付き監視の状態フォルダー")
	listen := flag.String("listen", "", "JSON のランチャー割り当てを常駐監視")
	launch := flag.String("launch", "", "JSON の指定スロットの起動・前面化を実行")
	guiProfile := flag.String("profile", "", "GUI または -autostart enable で使用する JSON 設定")
	slot := flag.Int("slot", 1, "-launch の入力番号 (1〜18)")
	layer := flag.Int("layer", 1, "-launch のレイヤー (1〜3)")
	flag.Parse()
	modes := 0
	if *startup != "" {
		modes++
	}
	if *daemonProfile != "" {
		modes++
	}
	if *list {
		modes++
	}
	if *dry != "" {
		modes++
	}
	if *apply != "" {
		modes++
	}
	if *listen != "" {
		modes++
	}
	if *launch != "" {
		modes++
	}
	if modes > 1 || flag.NArg() > 0 || (*daemonState != "" && *daemonProfile == "") {
		fmt.Fprintln(os.Stderr, "ERROR 実行モードは1つだけ指定してください。-daemon-state は -daemon と併用します")
		return 1
	}
	if *daemonLogs {
		if *daemonProfile == "" || runtime.GOOS != "linux" {
			fmt.Fprintln(os.Stderr, "ERROR -daemon-logs は Linux の -daemon と併用してください")
			return 1
		}
		dir := *daemonState
		if dir == "" {
			var err error
			dir, err = launcher.StateDir()
			if err != nil {
				fmt.Fprintln(os.Stderr, "ERROR", err)
				return 1
			}
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR", err)
			return 1
		}
		stdout, err := os.OpenFile(filepath.Join(dir, "daemon.stdout.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR", err)
			return 1
		}
		defer stdout.Close()
		stderr, err := os.OpenFile(filepath.Join(dir, "daemon.stderr.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR", err)
			return 1
		}
		defer stderr.Close()
		os.Stdout = stdout
		os.Stderr = stderr
	}
	logging.Setup(*verbose)
	if *startup != "" {
		m, err := autostart.New()
		if err != nil {
			slog.Error("自動起動を設定できません", "error", err)
			return 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err = m.Apply(ctx, *startup, *guiProfile, *backend); err != nil {
			slog.Error("自動起動の操作に失敗しました", "error", err)
			return 1
		}
		status, err := m.Status(ctx)
		if err != nil {
			slog.Error("自動起動の状態を確認できません", "error", err)
			return 1
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
		return 0
	}
	if *daemonProfile != "" && runtime.GOOS != "linux" {
		slog.Error("管理付き監視は Linux 用です")
		return 1
	}
	if *listen != "" || *launch != "" || *daemonProfile != "" {
		path := *listen
		if *daemonProfile != "" {
			path = *daemonProfile
		}
		if path == "" {
			path = *launch
		}
		p, err := profile.Load(path)
		if err != nil {
			slog.Error("設定を読み込めません", "error", err)
			return 1
		}
		m, err := launcher.New(*daemonState)
		if err != nil {
			slog.Error("ランチャーを初期化できません", "error", err)
			return 1
		}
		defer m.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if *launch != "" {
			b, ok := p.Get(*layer, *slot)
			if !ok || b.Kind != "launcher" || b.Launch == nil {
				slog.Error("指定スロットにランチャーの割り当てがありません")
				return 1
			}
			result, err := m.Run(ctx, *b.Launch)
			if err != nil {
				slog.Error("起動・前面化に失敗", "error", err)
				return 1
			}
			slog.Info(result.Message)
			return 0
		}
		stop, done, err := host.StartWithDone(ctx, p, m, func(text string) { slog.Info(text) })
		if err != nil {
			slog.Error("監視を開始できません", "error", err)
			return 1
		}
		var control *daemon.Server
		defer func() {
			stop()
			if control != nil {
				control.Close()
			}
		}()
		if *daemonProfile != "" {
			control, err = daemon.Serve(m.StateDir(), cancel)
			if err != nil {
				slog.Error("監視の制御ソケットを開始できません", "error", err)
				return 1
			}
		}
		select {
		case <-ctx.Done():
		case err := <-done:
			if err != nil {
				slog.Error("入力監視を継続できません", "error", err)
				return 1
			}
		}
		return 0
	}
	if *list {
		infos, err := device.List()
		if err != nil {
			slog.Error("USB 検出に失敗", "error", err)
			return 1
		}
		if len(infos) == 0 {
			fmt.Println("1189:8890 は接続されていません")
		}
		for _, info := range infos {
			fmt.Printf("%s  1189:8890  CH57x\n", info.ID())
		}
		return 0
	}
	path := *dry
	if path == "" {
		path = *apply
	}
	if path != "" {
		p, err := profile.Load(path)
		if err != nil {
			slog.Error("設定を読み込めません", "error", err)
			return 1
		}
		jobs, err := p.Jobs()
		if err != nil {
			slog.Error("設定が不正です", "error", err)
			return 1
		}
		if len(jobs) == 0 {
			slog.Error("書き込む設定がありません")
			return 1
		}
		if *dry != "" {
			for _, job := range jobs {
				fmt.Println(job.Label)
				for _, f := range job.Frames {
					fmt.Printf("% x\n", f[:])
				}
			}
			return 0
		}
		if *id == "" {
			infos, err := device.List()
			if err != nil || len(infos) != 1 {
				slog.Error("-device で USB バス:アドレスを指定してください", "error", err)
				return 1
			}
			*id = infos[0].ID()
		}
		if err := device.Write(*id, jobs, nil); err != nil {
			slog.Error("書き込みに失敗", "error", err)
			return 1
		}
		return 0
	}
	if err := runGUI(*guiProfile); err != nil {
		slog.Error("アプリを起動できません", "error", err)
		return 1
	}
	return 0
}
