package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"key-short/internal/desktop"
	"key-short/internal/protocol"
	"key-short/internal/window"
)

type fakeBackend struct {
	windows   []window.Window
	activated []string
	err       error
	onList    func()
}

func (f *fakeBackend) List(context.Context) ([]window.Window, error) {
	if f.onList != nil {
		f.onList()
	}
	return append([]window.Window(nil), f.windows...), f.err
}
func (f *fakeBackend) Activate(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.activated = append(f.activated, id)
	return nil
}
func (f *fakeBackend) Close() error { return nil }

func self(t *testing.T) process {
	t.Helper()
	p, _, err := processInfo(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func commandSpec() protocol.LaunchSpec {
	return protocol.LaunchSpec{Mode: "command", Command: `key-short-test-program --profile "two words" --remote-debugging-port=9222`, ReuseWindow: true}
}

func TestLaunchThenFocusAcrossManagers(t *testing.T) {
	dir := t.TempDir()
	p := self(t)
	backend := &fakeBackend{}
	started := 0
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.backend = backend
	m.wait = 30 * time.Millisecond
	m.start = func(args []string, dir string, token string) (process, <-chan error, error) {
		started++
		want := []string{"key-short-test-program", "--profile", "two words", "--remote-debugging-port=9222"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("argv: %#v", args)
		}
		backend.windows = []window.Window{{ID: "owned-window", PID: p.PID, Class: "test"}}
		return p, make(chan error), nil
	}
	spec := commandSpec()
	result, err := m.Run(context.Background(), spec)
	if err != nil || !result.Started || started != 1 {
		t.Fatalf("start: %+v %v", result, err)
	}
	// A different Manager reads the on-disk process generation and window ID.
	second, _ := New(dir)
	second.backend = backend
	second.start = m.start
	result, err = second.Run(context.Background(), spec)
	if err != nil || !result.Focused || started != 1 {
		t.Fatalf("focus: %+v %v starts=%d", result, err, started)
	}
	// If the original window closed, another window of its recorded process works.
	backend.windows = []window.Window{{ID: "another-window", PID: p.PID}}
	result, err = second.Run(context.Background(), spec)
	if err != nil || !result.Focused || started != 1 {
		t.Fatalf("replacement window: %+v %v", result, err)
	}
}

func TestAlwaysLaunchDoesNotFocus(t *testing.T) {
	m, _ := New(t.TempDir())
	calls := 0
	m.start = func([]string, string, string) (process, <-chan error, error) {
		calls++
		return self(t), make(chan error), nil
	}
	spec := commandSpec()
	spec.ReuseWindow = false
	for i := 0; i < 2; i++ {
		result, err := m.Run(context.Background(), spec)
		if err != nil || !result.Started {
			t.Fatalf("%+v %v", result, err)
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestPIDReuseAndClosedWindowDoNotActivate(t *testing.T) {
	p := self(t)
	backend := &fakeBackend{windows: []window.Window{{ID: "recycled", PID: p.PID}}}
	spec := commandSpec()
	stale := record{Root: process{p.PID, "wrong-generation"}, Windows: []windowRef{{"recycled", process{p.PID, "wrong-generation"}}}}
	if _, ok := findWindow(backend.windows, stale, spec, desktop.Application{}, []string{"key-short-test-program"}, nil); ok {
		t.Fatal("recycled PID or stale ID was reused")
	}
}

func TestFocusFailureNeverStartsDuplicate(t *testing.T) {
	m, _ := New(t.TempDir())
	m.backend = &fakeBackend{err: errors.New("desktop unavailable")}
	starts := 0
	m.start = func([]string, string, string) (process, <-chan error, error) { starts++; return process{}, nil, nil }
	if _, err := m.Run(context.Background(), commandSpec()); err == nil {
		t.Fatal("expected desktop error")
	}
	if starts != 0 {
		t.Fatal("started duplicate after focus/list error")
	}
}

func TestArgumentsHaveIndependentHistory(t *testing.T) {
	normal := protocol.LaunchSpec{Mode: "command", Command: "chromium", ReuseWindow: true}
	debug := normal
	debug.Command = "chromium --remote-debugging-port=9222"
	if signature(normal) == signature(debug) {
		t.Fatal("different command arguments share history")
	}
	normal.ReuseWindow = false
	if signature(normal) != signature(protocol.LaunchSpec{Mode: "command", Command: "chromium", ReuseWindow: true}) {
		t.Fatal("toggling reuse lost history")
	}
}

func TestRapidConcurrentPressIsNotQueued(t *testing.T) {
	m, _ := New(t.TempDir())
	m.backend = &fakeBackend{}
	m.wait = time.Second
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	m.start = func([]string, string, string) (process, <-chan error, error) {
		once.Do(func() { close(entered) })
		<-release
		return self(t), make(chan error), nil
	}
	spec := commandSpec()
	spec.ReuseWindow = false
	done := make(chan error, 1)
	go func() { _, err := m.Run(context.Background(), spec); done <- err }()
	<-entered
	second, _ := New(m.stateDir)
	second.start = func([]string, string, string) (process, <-chan error, error) {
		t.Error("duplicate spawn")
		return process{}, nil, nil
	}
	result, err := second.Run(context.Background(), spec)
	if err != nil || !result.Pending {
		t.Errorf("%+v %v", result, err)
	}
	close(release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPendingProcessWithoutWindowDoesNotRespawn(t *testing.T) {
	m, _ := New(t.TempDir())
	m.backend = &fakeBackend{}
	spec := commandSpec()
	m.remember(filepath.Join(m.stateDir, signature(spec)+".json"), self(t), window.Window{})
	m.start = func([]string, string, string) (process, <-chan error, error) {
		t.Fatal("duplicate start")
		return process{}, nil, nil
	}
	result, err := m.Run(context.Background(), spec)
	if err != nil || !result.Pending {
		t.Fatalf("%+v %v", result, err)
	}
}
