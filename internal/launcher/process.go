package launcher

type process struct {
	PID   int    `json:"pid"`
	Start string `json:"start"`
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
