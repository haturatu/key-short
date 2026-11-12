package host

import (
	"context"
	"testing"

	"key-short/internal/input"
	"key-short/internal/launcher"
	"key-short/internal/profile"
	"key-short/internal/protocol"
)

func TestListenerExclusionAndRestart(t *testing.T) {
	m, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	p := profile.New()
	p.Set(protocol.Binding{Layer: 1, Slot: 1, Kind: "launcher", Launch: &protocol.LaunchSpec{Mode: "command", Command: "chromium"}})
	status := func(string) {}
	monitor := func(ctx context.Context, _ func(input.Chord), _ func(string)) error { <-ctx.Done(); return nil }
	stop, err := start(context.Background(), p, m, status, monitor)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if second, err := start(context.Background(), p, m, status, monitor); err == nil {
		second()
		t.Fatal("second listener was accepted")
	}
	stop()
	stop() // stopping twice is safe, and releases the lock synchronously
	again, err := start(context.Background(), p, m, status, monitor)
	if err != nil {
		t.Fatal(err)
	}
	again()
}
