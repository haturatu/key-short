//go:build !windows

package protocol

import "github.com/mattn/go-shellwords"

func parseCommand(command string) ([]string, bool, error) {
	p := shellwords.NewParser()
	p.ParseEnv = false
	p.ParseBacktick = false
	args, err := p.Parse(command)
	return args, p.Position >= 0, err
}
