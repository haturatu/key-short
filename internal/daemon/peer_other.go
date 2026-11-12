//go:build !linux

package daemon

import (
	"fmt"
	"net"
)

func authenticate(net.Conn) error {
	return fmt.Errorf("監視デーモンの管理は Linux 用です")
}
