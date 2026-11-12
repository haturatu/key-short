package device

import "github.com/sstallion/go-hid"

func setWriteTimeout(d *hid.Device) { d.SetWriteTimeout(1000) }
