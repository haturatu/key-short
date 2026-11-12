package host

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"key-short/internal/input"
	"key-short/internal/launcher"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

// Start holds a session listener lock until Stop returns. Reconnects are handled
// automatically; starting a second GUI/CLI listener cannot duplicate launches.
func Start(parent context.Context, p profile.Profile, manager *launcher.Manager, status func(string)) (stop func(), err error) {
	return start(parent, p, manager, status, input.Monitor)
}

func start(parent context.Context, p profile.Profile, manager *launcher.Manager, status func(string), monitor func(context.Context, func(input.Chord), func(string)) error) (stop func(), err error) {
	if status == nil {
		status = func(string) {}
	}
	if _, err = p.Jobs(); err != nil {
		return nil, err
	}
	bindings := map[input.Chord]protocol.Binding{}
	for _, b := range p.Bindings {
		if b.Kind == "launcher" {
			chord, e := input.ForBinding(b)
			if e != nil {
				return nil, e
			}
			bindings[chord] = b
		}
	}
	if len(bindings) == 0 {
		return nil, fmt.Errorf("アプリ起動またはコマンド実行の割り当てを追加してください")
	}
	lock, err := os.OpenFile(filepath.Join(manager.StateDir(), "listener.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("別の Key Short がランチャーを監視中です")
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	events := make(chan input.Chord, 32)
	var launches sync.WaitGroup
	go func() {
		defer close(done)
		defer lock.Close()
		defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		monitorDone := make(chan struct{})
		go func() {
			defer close(monitorDone)
			_ = monitor(ctx, func(c input.Chord) {
				select {
				case events <- c:
				default:
					slog.Warn("ランチャーの入力が多すぎるためスキップしました")
				}
			}, status)
		}()
		last := map[input.Chord]time.Time{}
		for {
			select {
			case <-ctx.Done():
				<-monitorDone
				launches.Wait()
				return
			case c := <-events:
				b, ok := bindings[c]
				if !ok || time.Since(last[c]) < 250*time.Millisecond {
					continue
				}
				last[c] = time.Now()
				launches.Add(1)
				go func(b protocol.Binding) {
					defer launches.Done()
					result, err := manager.Run(ctx, *b.Launch)
					if err != nil {
						if ctx.Err() == nil {
							slog.Error("ランチャーの実行に失敗", "error", err)
							status(err.Error())
						}
						return
					}
					status(protocol.SlotName(b.Slot) + ": " + result.Message)
				}(b)
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }, nil
}
