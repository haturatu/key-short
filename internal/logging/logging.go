package logging

import (
	"context"
	"log/slog"
	"os"
)

type split struct{ out, err slog.Handler }

func (s split) Enabled(ctx context.Context, l slog.Level) bool {
	if l >= slog.LevelWarn {
		return s.err.Enabled(ctx, l)
	}
	return s.out.Enabled(ctx, l)
}
func (s split) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		return s.err.Handle(ctx, r)
	}
	return s.out.Handle(ctx, r)
}
func (s split) WithAttrs(a []slog.Attr) slog.Handler {
	return split{s.out.WithAttrs(a), s.err.WithAttrs(a)}
}
func (s split) WithGroup(g string) slog.Handler { return split{s.out.WithGroup(g), s.err.WithGroup(g)} }
func Setup(verbose bool) {
	l := slog.LevelInfo
	if verbose {
		l = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: l}
	slog.SetDefault(slog.New(split{slog.NewTextHandler(os.Stdout, opts), slog.NewTextHandler(os.Stderr, opts)}))
}
