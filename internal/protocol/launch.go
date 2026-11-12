package protocol

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type LaunchSpec struct {
	Mode        string `json:"mode"`
	DesktopFile string `json:"desktop_file,omitempty"`
	Command     string `json:"command,omitempty"`
	ReuseWindow bool   `json:"reuse_window"`
	WindowClass string `json:"window_class,omitempty"`
	WorkingDir  string `json:"working_dir,omitempty"`
}

// CommandArgs supports quotes, but deliberately does not invoke a shell or
// expand environment variables, globs, or command substitutions.
func CommandArgs(command string) ([]string, error) {
	args, stopped, err := parseCommand(command)
	if err != nil {
		return nil, fmt.Errorf("コマンドの引用符などを確認してください: %w", err)
	}
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("実行するコマンドを入力してください")
	}
	if stopped {
		return nil, fmt.Errorf("パイプや複数コマンドには sh -c '…' を明示してください")
	}
	return args, nil
}

func (s LaunchSpec) Validate() error {
	switch s.Mode {
	case "application":
		ext := strings.ToLower(filepath.Ext(s.DesktopFile))
		valid := ext == ".desktop"
		if runtime.GOOS == "windows" {
			valid = ext == ".lnk" || ext == ".exe"
		}
		if runtime.GOOS == "darwin" {
			valid = ext == ".app"
		}
		if !filepath.IsAbs(s.DesktopFile) || !valid {
			return fmt.Errorf("この OS のアプリ一覧からアプリを選択してください")
		}
		if s.Command != "" {
			return fmt.Errorf("アプリ選択とコマンドは同時に指定できません")
		}
	case "command":
		if s.DesktopFile != "" {
			return fmt.Errorf("アプリ選択とコマンドは同時に指定できません")
		}
		if _, err := CommandArgs(s.Command); err != nil {
			return err
		}
	default:
		return fmt.Errorf("起動方法は application / command です")
	}
	if s.WorkingDir != "" && !filepath.IsAbs(s.WorkingDir) {
		return fmt.Errorf("作業フォルダーは絶対パスで指定してください")
	}
	return nil
}

// Unique for all 54 slots; only these generated chords are monitored on the pad.
func Trigger(layer, slot int) string {
	if layer < 1 || layer > 3 || slot < 1 || slot > 18 {
		return ""
	}
	mod := []string{"ctrl+alt+super+", "ctrl+shift+super+", "alt+shift+super+"}[layer-1]
	// Right Ctrl distinguishes the second group without exceeding F24.
	if slot > 12 {
		mod += "rctrl+"
	}
	return fmt.Sprintf("%sf%d", mod, 13+(slot-1)%12)
}

func (b Binding) DisplayValue() string {
	if b.Kind == "launcher" && b.Launch != nil {
		if b.Value != "" {
			return "起動: " + b.Value
		}
		if b.Launch.Mode == "application" {
			return "起動: " + filepath.Base(b.Launch.DesktopFile)
		}
		return "起動: " + b.Launch.Command
	}
	return b.Value
}
