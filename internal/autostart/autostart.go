// Package autostart shares Linux login startup configuration between GUI, CLI
// and make installation. XDG autostart is independent of the system init.
package autostart

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"key-short/internal/daemon"
	"key-short/internal/filelock"
	"key-short/internal/profile"
)

type Config struct {
	Enabled  bool   `json:"enabled"`
	Backend  string `json:"backend"`
	Profile  string `json:"profile"`
	StateDir string `json:"state_dir"`
}
type Status struct {
	Config
	Running bool `json:"running"`
}
type Manager struct {
	Home, ConfigHome, DataHome, StateDir, Binary string
	// Systemctl is replaceable in installation tests. Successful commands do not
	// print the user's imported session environment.
	Systemctl func(context.Context, ...string) error
}

func New() (*Manager, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("自動起動の設定は Linux 用です")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := func(env, fallback string) (string, error) {
		p := os.Getenv(env)
		if p == "" {
			p = fallback
		}
		if !filepath.IsAbs(p) {
			return "", fmt.Errorf("%s は絶対パスで指定してください", env)
		}
		return p, nil
	}
	config, err := path("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err != nil {
		return nil, err
	}
	data, err := path("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	if err != nil {
		return nil, err
	}
	state, err := path("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	if err != nil {
		return nil, err
	}
	m := &Manager{Home: home, ConfigHome: config, DataHome: data, StateDir: filepath.Join(state, "key-short"), Binary: filepath.Join(home, ".local", "bin", "key-short")}
	m.Systemctl = func(ctx context.Context, args ...string) error {
		cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
		return cmd.Run()
	}
	return m, nil
}
func (m *Manager) configPath() string {
	return filepath.Join(m.ConfigHome, "key-short", "autostart.json")
}
func (m *Manager) unitPath() string {
	return filepath.Join(m.ConfigHome, "systemd", "user", "key-short.service")
}
func (m *Manager) desktopPath() string {
	return filepath.Join(m.ConfigHome, "autostart", "key-short.desktop")
}
func (m *Manager) read() (Config, error) {
	c := Config{Backend: "auto", StateDir: m.StateDir}
	data, err := os.ReadFile(m.configPath())
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("自動起動設定を読めません: %w", err)
	}
	if c.StateDir == "" {
		c.StateDir = m.StateDir
	}
	if !filepath.IsAbs(c.StateDir) {
		return c, fmt.Errorf("監視デーモンの状態フォルダーが不正です")
	}
	return c, nil
}
func (m *Manager) Status(ctx context.Context) (Status, error) {
	c, err := m.read()
	if err != nil {
		return Status{}, err
	}
	running, err := daemon.Running(ctx, c.StateDir)
	return Status{Config: c, Running: running}, err
}
func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".key-short-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func (m *Manager) save(c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(m.configPath(), append(data, '\n'), 0600)
}
func unitArg(s string) string {
	return strconv.Quote(strings.NewReplacer("%", "%%", "$", "$$").Replace(s))
}
func desktopArg(s string) string {
	s = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(s)
	return "\"" + strings.ReplaceAll(s, "\\", "\\\\") + "\""
}

// A fixed shell executable accepts paths that systemd prohibits as argv[0].
// The script is constant; every path is a separate argument to exec "$@".
// exec preserves the main PID and never evaluates user-provided shell text.
func (m *Manager) unit(c Config) []byte {
	return []byte(fmt.Sprintf(`[Unit]
Description=Key Short launcher monitor
PartOf=graphical-session.target
After=graphical-session-pre.target

[Service]
Type=simple
WorkingDirectory=%%h
ExecStart=/bin/sh -c %s key-short %s -daemon %s -daemon-state %s
Restart=on-failure
RestartSec=5s
TimeoutStopSec=20s
# Applications launched by the listener must survive a listener restart.
KillMode=process
StandardOutput=journal
StandardError=journal
SyslogIdentifier=key-short

[Install]
WantedBy=graphical-session.target
`, unitArg(`exec "$@"`), unitArg(m.Binary), unitArg(c.Profile), unitArg(c.StateDir)))
}
func (m *Manager) desktop(c Config) []byte {
	return []byte(fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Key Short Launcher
Comment=マクロパッドからのアプリ起動を監視
Exec=/bin/sh -c %s key-short %s -daemon %s -daemon-state %s -daemon-logs
Terminal=false
StartupNotify=false
`, desktopArg(`exec "$@"`), desktopArg(m.Binary), desktopArg(c.Profile), desktopArg(c.StateDir)))
}
func (m *Manager) systemdAvailable(ctx context.Context) bool {
	return m.Systemctl(ctx, "show-environment") == nil
}
func (m *Manager) importEnvironment(ctx context.Context) error {
	args := []string{"import-environment"}
	for _, name := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "XDG_CURRENT_DESKTOP", "XDG_SESSION_TYPE", "XDG_SESSION_ID", "KDE_FULL_SESSION", "PATH"} {
		if _, ok := os.LookupEnv(name); ok {
			args = append(args, name)
		}
	}
	if len(args) == 1 {
		return nil
	}
	return m.Systemctl(ctx, args...)
}
func (m *Manager) Enable(ctx context.Context, path, backend string) error {
	if path == "" {
		return fmt.Errorf("PROFILE または -profile で保存済みの JSON を指定してください")
	}
	if backend == "" {
		backend = "auto"
	}
	if backend != "auto" && backend != "systemd" && backend != "xdg" {
		return fmt.Errorf("起動方式は auto / systemd / xdg から選んでください")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for _, p := range []string{path, m.Binary, m.StateDir} {
		if strings.ContainsAny(p, "\n\r\x00") {
			return fmt.Errorf("自動起動のパスに改行や NUL は使用できません")
		}
	}
	for _, target := range []string{m.configPath(), m.unitPath(), m.desktopPath()} {
		same := path == target
		if a, e := os.Stat(path); e == nil {
			if b, e := os.Stat(target); e == nil && os.SameFile(a, b) {
				same = true
			}
		}
		if same {
			return fmt.Errorf("自動起動設定の保存先とは別の JSON を指定してください")
		}
	}
	p, err := profile.Load(path)
	if err != nil {
		return err
	}
	if _, err = p.Jobs(); err != nil {
		return err
	}
	launches := 0
	for _, b := range p.Bindings {
		if b.Kind == "launcher" {
			launches++
		}
	}
	if launches == 0 {
		return fmt.Errorf("ランチャーの割り当てを含む JSON を保存して指定してください")
	}
	info, err := os.Stat(m.Binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return fmt.Errorf("先に make install でアプリをインストールしてください")
	}
	if backend == "auto" {
		backend = "xdg"
		if m.systemdAvailable(ctx) && m.Systemctl(ctx, "is-active", "--quiet", "graphical-session.target") == nil {
			backend = "systemd"
		}
	}
	if backend == "systemd" && !m.systemdAvailable(ctx) {
		return fmt.Errorf("systemd のユーザーマネージャーに接続できません。xdg を選択してください")
	}
	// Reject invalid input before stopping an existing listener or replacing files.
	previous, err := m.Status(ctx)
	if err != nil {
		return err
	}
	if err = m.Disable(ctx); err != nil {
		return err
	}
	c := Config{Enabled: true, Backend: backend, Profile: path, StateDir: m.StateDir}
	if backend == "systemd" {
		if err = writeFile(m.unitPath(), m.unit(c), 0600); err != nil {
			return err
		}
		if err = m.importEnvironment(ctx); err == nil {
			err = m.Systemctl(ctx, "daemon-reload")
		}
		if err == nil {
			err = m.Systemctl(ctx, "enable", "key-short.service")
		}
	} else {
		err = writeFile(m.desktopPath(), m.desktop(c), 0600)
	}
	if err == nil {
		err = m.save(c)
	}
	if err != nil {
		if cleanupErr := m.Disable(ctx); cleanupErr != nil {
			return fmt.Errorf("%w（自動起動の解除にも失敗: %v）", err, cleanupErr)
		}
		return err
	}
	if previous.Running {
		return m.Start(ctx)
	}
	return nil
}
func remove(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (m *Manager) Stop(ctx context.Context) error {
	c, err := m.read()
	if err != nil {
		return err
	}
	if _, err := os.Stat(m.unitPath()); err == nil && m.systemdAvailable(ctx) {
		if err = m.Systemctl(ctx, "stop", "key-short.service"); err != nil {
			return fmt.Errorf("systemd サービスを停止できません: %w", err)
		}
	}
	return daemon.Stop(ctx, c.StateDir)
}
func (m *Manager) Disable(ctx context.Context) error {
	c, err := m.read()
	if err != nil {
		return err
	}
	if err = m.Stop(ctx); err != nil {
		return err
	}
	if _, err := os.Stat(m.unitPath()); err == nil && m.systemdAvailable(ctx) {
		if err = m.Systemctl(ctx, "disable", "key-short.service"); err != nil {
			return err
		}
	}
	for _, path := range []string{m.desktopPath(), filepath.Join(filepath.Dir(m.unitPath()), "graphical-session.target.wants", "key-short.service"), m.unitPath()} {
		if err = remove(path); err != nil {
			return err
		}
	}
	if m.systemdAvailable(ctx) {
		if err = m.Systemctl(ctx, "daemon-reload"); err != nil {
			return err
		}
	}
	c.Enabled = false
	return m.save(c)
}
func (m *Manager) Start(ctx context.Context) error {
	c, err := m.read()
	if err != nil {
		return err
	}
	if !c.Enabled {
		return fmt.Errorf("先に自動起動を有効にしてください")
	}
	if running, err := daemon.Running(ctx, c.StateDir); err != nil {
		return err
	} else if running {
		return nil
	}
	p, err := profile.Load(c.Profile)
	if err != nil {
		return err
	}
	if _, err = p.Jobs(); err != nil {
		return err
	}
	launches := false
	for _, b := range p.Bindings {
		if b.Kind == "launcher" {
			launches = true
		}
	}
	if !launches {
		return fmt.Errorf("ランチャーの割り当てがありません")
	}
	if err = os.MkdirAll(c.StateDir, 0700); err != nil {
		return err
	}
	lock, ok, err := filelock.Acquire(filepath.Join(c.StateDir, "listener.lock"))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("GUI または別の CLI で監視中です。停止してから開始してください")
	}
	lock.Close()
	if c.Backend == "systemd" {
		if err = m.importEnvironment(ctx); err != nil {
			return err
		}
		if err = m.Systemctl(ctx, "start", "key-short.service"); err != nil {
			return err
		}
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				running, err := daemon.Running(ctx, c.StateDir)
				if err != nil {
					return err
				}
				if running {
					return nil
				}
			}
		}
	}
	if c.Backend != "xdg" {
		return fmt.Errorf("自動起動方式が不正です")
	}
	stdout, err := os.OpenFile(filepath.Join(c.StateDir, "daemon.stdout.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(filepath.Join(c.StateDir, "daemon.stderr.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer stderr.Close()
	cmd := exec.Command(m.Binary, "-daemon", c.Profile, "-daemon-state", c.StateDir)
	cmd.Dir = m.Home
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err = cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return fmt.Errorf("監視デーモンが終了しました（daemon.stderr.log を確認）: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			running, err := daemon.Running(ctx, c.StateDir)
			if err != nil {
				return err
			}
			if running {
				return nil
			}
		}
	}
}
func (m *Manager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	return m.Start(ctx)
}
func (m *Manager) Uninstall(ctx context.Context) error {
	if err := m.Disable(ctx); err != nil {
		return err
	}
	for _, path := range []string{m.configPath(), m.Binary, filepath.Join(m.DataHome, "applications", "key-short.desktop"), filepath.Join(m.DataHome, "icons", "hicolor", "scalable", "apps", "key-short.svg")} {
		if err := remove(path); err != nil {
			return err
		}
	}
	return nil
}
func (m *Manager) InstallApp(binary string, icon []byte) error {
	f, err := os.Open(binary)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if err = writeFile(m.Binary, data, 0755); err != nil {
		return err
	}
	if err = writeFile(filepath.Join(m.DataHome, "icons", "hicolor", "scalable", "apps", "key-short.svg"), icon, 0644); err != nil {
		return err
	}
	desktop := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Key Short\nComment=MINI KeyBoard のキーとノブを設定\nExec=/bin/sh -c %s key-short %s\nIcon=key-short\nTerminal=false\nCategories=Settings;HardwareSettings;\nStartupNotify=true\n", desktopArg(`exec "$@"`), desktopArg(m.Binary))
	return writeFile(filepath.Join(m.DataHome, "applications", "key-short.desktop"), []byte(desktop), 0644)
}

func (m *Manager) Apply(ctx context.Context, action, path, backend string) error {
	switch action {
	case "status":
		return nil
	case "enable":
		if path == "" {
			return fmt.Errorf("-profile で保存済みの JSON を指定してください")
		}
		return m.Enable(ctx, path, backend)
	case "disable":
		return m.Disable(ctx)
	case "start":
		return m.Start(ctx)
	case "stop":
		return m.Stop(ctx)
	case "restart":
		return m.Restart(ctx)
	default:
		return fmt.Errorf("自動起動の操作は status / enable / disable / start / stop / restart です")
	}
}
