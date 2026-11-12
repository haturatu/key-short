//go:build windows || darwin

package launcher

import (
	"fmt"
	"github.com/shirou/gopsutil/v4/host"
	"os"
	"runtime"
)

func stateRoot() (string, error) { return os.UserConfigDir() }
func session() string {
	boot, _ := host.BootTime()
	return fmt.Sprintf("%s|%d|%s", runtime.GOOS, boot, os.Getenv("SESSIONNAME"))
}
