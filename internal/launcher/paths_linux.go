package launcher

import (
	"os"
	"path/filepath"
	"strings"
)

func stateRoot() (string, error) {
	root := os.Getenv("XDG_STATE_HOME")
	if root != "" {
		return root, nil
	}
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".local/state"), err
}
func session() string {
	boot, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(boot)) + "|" + os.Getenv("XDG_SESSION_ID") + "|" + os.Getenv("DISPLAY") + "|" + os.Getenv("WAYLAND_DISPLAY")
}
