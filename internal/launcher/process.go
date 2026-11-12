package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type process struct {
	PID   int    `json:"pid"`
	Start string `json:"start"`
}

func processInfo(pid int) (process, int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return process{}, 0, err
	}
	end := strings.LastIndex(string(data), ")")
	if end < 0 {
		return process{}, 0, fmt.Errorf("不正な /proc stat")
	}
	f := strings.Fields(string(data)[end+1:])
	if len(f) < 20 {
		return process{}, 0, fmt.Errorf("短い /proc stat")
	}
	if f[0] == "Z" {
		return process{}, 0, fmt.Errorf("終了済みプロセス")
	}
	parent, err := strconv.Atoi(f[1])
	return process{pid, f[19]}, parent, err
}

func alive(p process) bool {
	current, _, err := processInfo(p.PID)
	return err == nil && p.Start != "" && current.Start == p.Start
}

func belongs(pid int, root process) bool {
	if !alive(root) {
		return false
	}
	for depth := 0; pid > 1 && depth < 128; depth++ {
		if pid == root.PID {
			return true
		}
		_, parent, err := processInfo(pid)
		if err != nil || parent == pid {
			return false
		}
		pid = parent
	}
	return false
}

func hasToken(pid int, token string) bool {
	if token == "" {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return false
	}
	for _, entry := range strings.Split(string(data), "\x00") {
		if entry == "KEY_SHORT_LAUNCH_ID="+token {
			return true
		}
	}
	return false
}

func tokenRunning(token string) bool {
	if token == "" {
		return false
	}
	paths, _ := filepath.Glob("/proc/[0-9]*/environ")
	for _, path := range paths {
		pid, _ := strconv.Atoi(filepath.Base(filepath.Dir(path)))
		if hasToken(pid, token) {
			if _, _, err := processInfo(pid); err == nil {
				return true
			}
		}
	}
	return false
}

// Compare argv, including every requested flag. Class names alone must not merge
// a normal Chromium launch with a separately configured debugging invocation.
func matchesCommand(pid int, want []string) bool {
	if len(want) == 0 {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	actual := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(actual) == 0 {
		return false
	}
	wantedExe := filepath.Base(want[0])
	actualExe := filepath.Base(actual[0])
	if wantedExe != actualExe {
		return false
	}
	if len(want) == 1 {
		return len(actual) == 1
	}
	// Permit wrapper-added switches, while preserving requested argument order.
	j := 1
	for _, arg := range actual[1:] {
		if j < len(want) && want[j] == arg {
			j++
		}
	}
	return j == len(want)
}
