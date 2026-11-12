package window

import "context"

type Window struct {
	ID     string `json:"id"`
	PID    int    `json:"pid"`
	Class  string `json:"class"`
	AppID  string `json:"app_id"`
	Active bool   `json:"active"`
}
type Backend interface {
	List(context.Context) ([]Window, error)
	Activate(context.Context, string) error
	Close() error
}
