package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/allofher/carson/internal/brain"
	"github.com/allofher/carson/internal/config"
)

// Run starts the Carson daemon. It initializes the brain folder, then blocks
// until a termination signal is received.
func Run(cfg *config.Config) error {
	logger := newLogger(cfg.LogLevel)

	logger.Info("starting carson", "brain_dir", cfg.BrainDir)

	if err := brain.Init(cfg.BrainDir); err != nil {
		return fmt.Errorf("initializing brain folder: %w", err)
	}
	logger.Info("brain folder ready")

	// Block until we receive SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("carson is running", "pid", os.Getpid())
	<-ctx.Done()
	logger.Info("shutting down")

	return nil
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
}
