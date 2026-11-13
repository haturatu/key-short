package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagedListenerControlAndLongStatePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("x", 120))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := Serve(dir, cancel)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	go func() { <-ctx.Done(); s.Close(); close(closed) }()
	requestCtx, requestCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer requestCancel()
	running, err := Running(requestCtx, dir)
	if err != nil || !running {
		t.Fatal("not running", err)
	}
	if _, err := Serve(dir, cancel); err == nil {
		t.Fatal("duplicate daemon accepted")
	}
	if err := Stop(requestCtx, dir); err != nil {
		t.Fatal(err)
	}
	<-closed
	if running, err := Running(requestCtx, dir); err != nil || running {
		t.Fatal("still running", err)
	}
	if err := Stop(requestCtx, dir); err != nil {
		t.Fatal("repeated stop", err)
	}
	// A subsequent daemon can use the same address without a stale PID/socket.
	s, err = Serve(dir, func() {})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}
func TestControlDoesNotDeleteUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.sock")
	if err := os.WriteFile(path, []byte("user data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Stop(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	s, err := Serve(dir, func() {})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	data, _ := os.ReadFile(path)
	if string(data) != "user data" {
		t.Fatal("control removed unrelated file")
	}
}
