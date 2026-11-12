package daemon

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// Abstract sockets have no filesystem permissions; authenticate the UID in both
// directions using kernel-provided credentials before sending any commands.
func authenticate(conn net.Conn) error {
	c, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("Unix ソケットではありません")
	}
	raw, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var credentialErr error
	err = raw.Control(func(fd uintptr) {
		cred, e := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if e != nil {
			credentialErr = e
		} else if cred.Uid != uint32(os.Geteuid()) {
			credentialErr = fmt.Errorf("監視デーモンのユーザーが一致しません")
		}
	})
	if err != nil {
		return err
	}
	return credentialErr
}
