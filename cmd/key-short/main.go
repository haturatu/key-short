package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"key-short/internal/device"
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
	if modes > 1 || flag.NArg() > 0 {
		slog.Error("-list / -dry-run / -apply は1つだけ指定してください")
		return 1
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
	if err := runGUI(); err != nil {
		slog.Error("アプリを起動できません", "error", err)
		return 1
	}
	return 0
}
