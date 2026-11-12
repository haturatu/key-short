package protocol

import "golang.org/x/sys/windows"

func parseCommand(command string) ([]string, bool, error) {
	args, err := windows.DecomposeCommandLine(command)
	return args, false, err
}
