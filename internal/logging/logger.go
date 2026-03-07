package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Setup creates a logger that writes to both stderr (text) and a JSON log file
// (with rotation), plus a broadcaster that SSE subscribers can listen to.
func Setup(logDir string, level slog.Level) (*slog.Logger, *Broadcaster, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}

	fw, err := NewRotatingWriter(filepath.Join(logDir, "carson.log"), 10*1024*1024, 3)
	if err != nil {
		return nil, nil, err
	}

	bc := NewBroadcaster()

	// JSON output goes to both file and broadcaster.
	jsonWriter := io.MultiWriter(fw, bc)

	handler := &multiHandler{
		handlers: []slog.Handler{
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}),
			slog.NewJSONHandler(jsonWriter, &slog.HandlerOptions{Level: level}),
		},
	}

	return slog.New(handler), bc, nil
}

// multiHandler fans out log records to multiple slog.Handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(nil, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: hs}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: hs}
}

// ParseLevel converts a string level name to slog.Level.
func ParseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
