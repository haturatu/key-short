package host

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"key-short/internal/filelock"
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

// StartWithDone lets CLI callers distinguish an unavailable native input API
// from a listener that is simply waiting for the USB device to reconnect.
func StartWithDone(parent context.Context, p profile.Profile, manager *launcher.Manager, status func(string)) (func(), <-chan error, error) {
	done := make(chan error, 1)
	stop, err := start(parent, p, manager, status, func(ctx context.Context, fire func(input.Chord), report func(string)) error {
		err := input.Monitor(ctx, fire, report)
		done <- err
		close(done)
		return err
	})
	return stop, done, err
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
	lock, locked, err := filelock.Acquire(filepath.Join(manager.StateDir(), "listener.lock"))
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("別の Key Short がランチャーを監視中です")
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	events := make(chan input.Chord, 32)
	var launches sync.WaitGroup
	go func() {
		defer close(done)
		defer lock.Close()
		monitorDone := make(chan struct{})
		go func() {
			defer close(monitorDone)
			monitorErr := monitor(ctx, func(c input.Chord) {
				select {
				case events <- c:
				default:
					slog.Warn("ランチャーの入力が多すぎるためスキップしました")
				}
			}, status)
			if monitorErr != nil && ctx.Err() == nil {
				slog.Error("入力監視に失敗しました", "error", monitorErr)
				status(monitorErr.Error())
				cancel()
			}
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
