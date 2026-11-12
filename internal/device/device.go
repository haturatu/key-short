package device

import (
	"crypto/sha256"
	"fmt"
)

type Info struct {
	Bus, Address int
	Path         string
}

func (i Info) ID() string {
	if i.Path != "" {
		sum := sha256.Sum256([]byte(i.Path))
		return fmt.Sprintf("hid:%x", sum[:8])
	}
	return fmt.Sprintf("%03d:%03d", i.Bus, i.Address)
}
