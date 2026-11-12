//go:build windows || darwin

package launcher

import (
	"fmt"
	ps "github.com/shirou/gopsutil/v4/process"
	"path/filepath"
	"runtime"
	"strings"
)

func processInfo(pid int) (process, int, error) {
	p, err := ps.NewProcess(int32(pid))
	if err != nil {
		return process{}, 0, err
	}
	start, err := p.CreateTime()
	if err != nil {
		return process{}, 0, err
	}
	parent, err := p.Ppid()
	if err != nil {
		return process{}, 0, err
	}
	running, err := p.IsRunning()
	if err != nil || !running {
		return process{}, 0, fmt.Errorf("終了済みプロセス")
	}
	return process{pid, fmt.Sprint(start)}, int(parent), nil
}
func hasToken(pid int, token string) bool {
	if token == "" {
		return false
	}
	p, err := ps.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	env, err := p.Environ()
	if err != nil {
		return false
	}
	for _, e := range env {
		if e == "KEY_SHORT_LAUNCH_ID="+token {
			return true
		}
	}
	return false
}
func tokenRunning(token string) bool {
	if token == "" {
		return false
	}
	pids, _ := ps.Pids()
	for _, pid := range pids {
		if hasToken(int(pid), token) {
			return true
		}
	}
	return false
}
func matchesCommand(pid int, want []string) bool {
	if len(want) == 0 {
		return false
	}
	p, err := ps.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	actual, err := p.CmdlineSlice()
	if err != nil || len(actual) == 0 {
		return false
	}
	normalize := func(s string) string {
		s = filepath.Base(s)
		if runtime.GOOS == "windows" {
			s = strings.TrimSuffix(strings.ToLower(s), ".exe")
		}
		return s
	}
	if normalize(want[0]) != normalize(actual[0]) {
		return false
	}
	if len(want) == 1 {
		return len(actual) == 1
	}
	j := 1
	for _, a := range actual[1:] {
		if j < len(want) && want[j] == a {
			j++
		}
	}
	return j == len(want)
}
