package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
	listen := flag.String("listen", "", "JSON のランチャー割り当てを常駐監視")
	launch := flag.String("launch", "", "JSON の指定スロットの起動・前面化を実行")
	guiProfile := flag.String("profile", "", "GUI で開く JSON 設定")
	slot := flag.Int("slot", 1, "-launch の入力番号 (1〜18)")
	layer := flag.Int("layer", 1, "-launch のレイヤー (1〜3)")
	flag.Parse()
	logging.Setup(*verbose)
	modes := 0
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
	if modes > 1 || flag.NArg() > 0 {
		slog.Error("-list / -dry-run / -apply / -listen / -launch は1つだけ指定してください")
		return 1
	}
	if *listen != "" || *launch != "" {
		path := *listen
		if path == "" {
			path = *launch
		}
		p, err := profile.Load(path)
		if err != nil {
			slog.Error("設定を読み込めません", "error", err)
			return 1
		}
		m, err := launcher.New("")
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
		defer stop()
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
