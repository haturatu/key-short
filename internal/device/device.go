package device

import (
	"crypto/sha256"
	"fmt"
)

type Info struct {
	Bus, Address              int
	Serial, Product, Location string
	Path                      string
}

func (i Info) ID() string {
	if i.Path != "" {
		sum := sha256.Sum256([]byte(i.Path))
		return fmt.Sprintf("hid:%x", sum[:8])
	}
	return fmt.Sprintf("%03d:%03d", i.Bus, i.Address)
}

// LayoutKey deliberately uses a connection location as well as any serial:
// inexpensive pads can share a factory serial. Never key on VID/PID alone.
// Empty means no durable identification is available; JSON remains portable.
func (i Info) LayoutKey() string {
	location := i.Location
	if location == "" {
		location = i.Path
	}
	if location == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("1189:8890\x00" + location + "\x00" + i.Serial))
	return fmt.Sprintf("%x", sum[:16])
}
