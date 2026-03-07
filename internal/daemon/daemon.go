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
	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/harness/tools"
	"github.com/allofher/carson/internal/llm"
)

// Run starts the Carson daemon. It initializes the brain folder, LLM
// provider, and tool harness, then blocks until a termination signal
// is received.
func Run(cfg *config.Config) error {
	logger := newLogger(cfg.LogLevel)

	logger.Info("starting carson", "brain_dir", cfg.BrainDir)

	if err := brain.Init(cfg.BrainDir); err != nil {
		return fmt.Errorf("initializing brain folder: %w", err)
	}
	logger.Info("brain folder ready")

	// Load system prompt.
	systemPrompt := loadSystemPrompt(cfg.SystemPromptPath, logger)

	// Initialize LLM provider and harness if configured.
	if cfg.LLMProvider != "" {
		provider, err := llm.New(cfg)
		if err != nil {
			return fmt.Errorf("initializing LLM provider: %w", err)
		}
		logger.Info("LLM provider ready", "provider", cfg.LLMProvider, "model", cfg.LLMModel)

		// Build tool registry.
		registry := harness.NewRegistry()
		registry.Register(tools.ReadFile(cfg.BrainDir))
		registry.Register(tools.WriteFile(cfg.BrainDir))
		registry.Register(tools.Bash(cfg.BrainDir))

		h := harness.New(harness.Config{
			Provider:     provider,
			Registry:     registry,
			SystemPrompt: systemPrompt,
			Logger:       logger,
		})
		_ = h // ready for use by event router / chat
	} else {
		logger.Warn("no LLM provider configured — running without agent capabilities")
	}

	// Block until we receive SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("carson is running", "pid", os.Getpid())
	<-ctx.Done()
	logger.Info("shutting down")

	return nil
}

func loadSystemPrompt(path string, logger *slog.Logger) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("failed to read system prompt", "path", path, "error", err)
		}
		return ""
	}
	prompt := string(data)
	if prompt != "" {
		logger.Info("system prompt loaded", "path", path, "bytes", len(prompt))
	}
	return prompt
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
