// Package daemon controls only managed foreground listeners, never a GUI or an
// unrelated process. A private Unix socket avoids signaling a recycled PID.
package daemon

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"key-short/internal/filelock"
)

type Server struct {
	listener net.Listener
	lock     interface{ Close() error }
	done     chan struct{}
}

func socketPath(dir string) string {
	absolute, err := filepath.Abs(dir)
	if err == nil {
		dir = absolute
	}
	sum := sha256.Sum256([]byte(filepath.Clean(dir)))
	return fmt.Sprintf("@key-short-%d-%x", os.Geteuid(), sum[:16])
}
func Serve(dir string, cancel context.CancelFunc) (*Server, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock, ok, err := filelock.Acquire(filepath.Join(dir, "daemon.lock"))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("監視デーモンは起動済みです")
	}
	fail := func(err error) (*Server, error) { lock.Close(); return nil, err }
	listener, err := net.Listen("unix", socketPath(dir))
	if err != nil {
		return fail(err)
	}
	s := &Server{listener: listener, lock: lock, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		var clients sync.WaitGroup
		defer clients.Wait()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			clients.Add(1)
			go func() {
				defer clients.Done()
				defer conn.Close()
				if authenticate(conn) != nil {
					return
				}
				conn.SetDeadline(time.Now().Add(time.Second))
				line, _ := bufio.NewReaderSize(conn, 64).ReadSlice('\n')
				switch string(line) {
				case "ping\n":
					fmt.Fprintln(conn, "ok")
				case "stop\n":
					fmt.Fprintln(conn, "ok")
					cancel()
				}
			}()
		}
	}()
	return s, nil
}
func (s *Server) Close() { s.listener.Close(); <-s.done; s.lock.Close() }
func request(ctx context.Context, dir, command string) (bool, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath(dir))
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		// A missing socket or a stale one is not a running daemon. Permissions and
		// malformed endpoints are reported rather than silently deleting anything.
		if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return false, nil
		}
		return false, err
	}
	defer conn.Close()
	if err = authenticate(conn); err != nil {
		return false, err
	}
	deadline := time.Now().Add(time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)
	if _, err = fmt.Fprintln(conn, command); err != nil {
		return false, err
	}
	line, err := bufio.NewReaderSize(conn, 64).ReadSlice('\n')
	if err != nil {
		return false, err
	}
	if string(line) != "ok\n" {
		return false, fmt.Errorf("監視デーモンからの応答が不正です")
	}
	return true, nil
}
func Running(ctx context.Context, dir string) (bool, error) { return request(ctx, dir, "ping") }
func Stop(ctx context.Context, dir string) error {
	running, err := request(ctx, dir, "stop")
	if err != nil {
		return err
	}
	if !running {
		return nil
	}
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		lock, ok, err := filelock.Acquire(filepath.Join(dir, "daemon.lock"))
		if err != nil {
			return err
		}
		if ok {
			lock.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
