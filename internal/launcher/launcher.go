// Package launcher starts applications and remembers their process generations
// and windows across GUI or CLI invocations.
package launcher

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"key-short/internal/desktop"
	"key-short/internal/filelock"
	"key-short/internal/protocol"
	"key-short/internal/window"
)

type windowRef struct {
	ID      string  `json:"id"`
	Process process `json:"process"`
}
type record struct {
	Session string      `json:"session"`
	Root    process     `json:"root"`
	Windows []windowRef `json:"windows"`
	Token   string      `json:"token,omitempty"`
}
type Result struct {
	Message                   string
	Focused, Started, Pending bool
}
type Manager struct {
	stateDir string
	mu       sync.Mutex
	backend  window.Backend
	start    func([]string, string, string) (process, <-chan error, error)
	wait     time.Duration
}

func StateDir() (string, error) {
	root, err := stateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "key-short"), nil
}
func New(dir string) (*Manager, error) {
	if dir == "" {
		var err error
		dir, err = StateDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Manager{stateDir: dir, start: startProcess, wait: 8 * time.Second}, nil
}
func (m *Manager) StateDir() string { return m.stateDir }
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.backend != nil {
		return m.backend.Close()
	}
	return nil
}
func (m *Manager) windows() (window.Backend, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.backend == nil {
		b, err := window.New()
		if err != nil {
			return nil, err
		}
		m.backend = b
	}
	return m.backend, nil
}

func startProcess(args []string, dir string, token string) (process, <-chan error, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "KEY_SHORT_LAUNCH_ID="+token)
	// Inherit the session environment, not the lifetime of the input listener.
	// Stopping the listener must not terminate applications it has launched.
	if err := cmd.Start(); err != nil {
		return process{}, nil, err
	}
	p, _, err := processInfo(cmd.Process.Pid)
	if err != nil {
		p = process{PID: cmd.Process.Pid}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); close(done) }()
	return p, done, nil
}

func signature(spec protocol.LaunchSpec) string {
	spec.ReuseWindow = false
	data, _ := json.Marshal(spec)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (m *Manager) Run(ctx context.Context, spec protocol.LaunchSpec) (Result, error) {
	if err := spec.Validate(); err != nil {
		return Result{}, err
	}
	key := signature(spec)
	lock, locked, err := filelock.Acquire(filepath.Join(m.stateDir, key+".lock"))
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	if !locked {
		return Result{Message: "このアプリは起動処理中です", Pending: true}, nil
	}
	args, err := protocol.CommandArgs(spec.Command)
	var app desktop.Application
	if spec.Mode == "application" {
		app, err = desktop.Read(spec.DesktopFile)
		if err == nil {
			args, err = desktop.Command(app)
		}
	}
	if err != nil {
		return Result{}, err
	}
	statePath := filepath.Join(m.stateDir, key+".json")
	var previous record
	if data, err := os.ReadFile(statePath); err == nil {
		if err = json.Unmarshal(data, &previous); err != nil {
			slog.Warn("起動履歴を読み直します", "error", err)
		}
	}
	if previous.Session != session() {
		previous = record{}
	}
	var backend window.Backend
	var before []window.Window
	if spec.ReuseWindow {
		backend, err = m.windows()
		if err != nil {
			return Result{}, err
		}
		before, err = backend.List(ctx)
		if err != nil {
			return Result{}, fmt.Errorf("起動済みウィンドウの確認に失敗しました: %w", err)
		}
		if w, ok := findWindow(before, previous, spec, app, args, nil); ok {
			if err = backend.Activate(ctx, w.ID); err != nil {
				return Result{}, fmt.Errorf("起動済みウィンドウを前面にできません: %w", err)
			}
			m.remember(statePath, previous.Root, w, previous.Token)
			return Result{Message: "起動済みのウィンドウを前面にしました", Focused: true}, nil
		}
		if len(previous.Windows) == 0 && (alive(previous.Root) || tokenRunning(previous.Token)) {
			return Result{Message: "プロセスは起動済みですが、前面にできるウィンドウがありません", Pending: true}, nil
		}
	}
	var nonce [24]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return Result{}, err
	}
	token := hex.EncodeToString(nonce[:])
	root, done, err := m.start(args, spec.WorkingDir, token)
	if err != nil {
		return Result{}, fmt.Errorf("起動できません: %w", err)
	}
	m.remember(statePath, root, window.Window{}, token)
	slog.Info("アプリを起動しました", "pid", root.PID)
	if !spec.ReuseWindow {
		return Result{Message: "新しく起動しました", Started: true}, nil
	}
	baseline := map[string]bool{}
	for _, w := range before {
		baseline[w.ID] = true
	}
	deadline := time.NewTimer(m.wait)
	defer deadline.Stop()
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	current := record{Root: root, Token: token}
	for {
		wins, listErr := backend.List(ctx)
		if listErr != nil {
			return Result{Started: true}, fmt.Errorf("起動しましたがウィンドウを確認できません: %w", listErr)
		}
		if w, ok := findWindow(wins, current, spec, app, args, baseline); ok {
			m.remember(statePath, root, w, token)
			if err = backend.Activate(ctx, w.ID); err != nil {
				return Result{Started: true}, fmt.Errorf("起動しましたが前面化に失敗しました: %w", err)
			}
			return Result{Message: "起動してウィンドウを記録しました", Started: true}, nil
		}
		select {
		case e, ok := <-done:
			if ok && e != nil {
				return Result{Started: true}, fmt.Errorf("起動したコマンドがエラーで終了しました: %w", e)
			}
			done = nil
		case <-tick.C:
		case <-ctx.Done():
			return Result{Started: true}, ctx.Err()
		case <-deadline.C:
			return Result{Message: "起動しました。ウィンドウはまだ検出されていません", Started: true}, nil
		}
	}
}

func findWindow(wins []window.Window, r record, spec protocol.LaunchSpec, app desktop.Application, args []string, baseline map[string]bool) (window.Window, bool) {
	// Reverse stacking order gives the most recently used matching window priority.
	for i := len(wins) - 1; i >= 0; i-- {
		w := wins[i]
		for _, ref := range r.Windows {
			if ref.ID == w.ID && ref.Process.PID == w.PID && alive(ref.Process) {
				return w, true
			}
		}
	}
	for i := len(wins) - 1; i >= 0; i-- {
		w := wins[i]
		for _, ref := range r.Windows {
			if ref.Process.PID == w.PID && alive(ref.Process) {
				return w, true
			}
		}
	}
	for i := len(wins) - 1; i >= 0; i-- {
		w := wins[i]
		if belongs(w.PID, r.Root) || hasToken(w.PID, r.Token) {
			return w, true
		}
	}
	for i := len(wins) - 1; i >= 0; i-- {
		w := wins[i]
		if spec.WindowClass != "" && strings.EqualFold(spec.WindowClass, w.Class) {
			return w, true
		}
		if spec.Mode == "application" && ((w.AppID != "" && strings.EqualFold(w.AppID, app.ID)) || (app.Class != "" && strings.EqualFold(w.Class, app.Class))) {
			return w, true
		}
		if spec.Mode == "command" && matchesCommand(w.PID, args) {
			return w, true
		}
		// A command may hand off to an already running application and exit.
		// Adopt only a newly created window, never an arbitrary existing one.
		if baseline != nil && !baseline[w.ID] && spec.Mode == "command" {
			name := filepath.Base(args[0])
			if strings.EqualFold(filepath.Ext(name), ".exe") {
				name = strings.TrimSuffix(name, filepath.Ext(name))
			}
			if strings.EqualFold(w.Class, name) || strings.EqualFold(w.AppID, name) || strings.HasSuffix(strings.ToLower(w.AppID), "."+strings.ToLower(name)) {
				return w, true
			}
		}
	}
	return window.Window{}, false
}

func (m *Manager) remember(path string, root process, w window.Window, token ...string) {
	r := record{Session: session(), Root: root}
	if len(token) > 0 {
		r.Token = token[0]
	}
	if w.ID != "" {
		if p, _, err := processInfo(w.PID); err == nil {
			r.Windows = []windowRef{{w.ID, p}}
		}
	}
	data, err := json.Marshal(r)
	if err == nil {
		err = atomicWrite(path, data)
	}
	if err != nil {
		slog.Warn("起動履歴を保存できません。再起動後の前面化に影響します", "error", err)
	}
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".launch-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
