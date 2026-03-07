package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/allofher/carson/internal/api"
	"github.com/allofher/carson/internal/brain"
	"github.com/allofher/carson/internal/config"
	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/harness/tools"
	"github.com/allofher/carson/internal/llm"
	"github.com/allofher/carson/internal/logging"
)

// Run starts the Carson daemon. It initializes the brain folder, LLM
// provider, tool harness, and API server, then blocks until a termination
// signal is received.
func Run(cfg *config.Config) error {
	level := logging.ParseLevel(cfg.LogLevel)
	logger, broadcaster, err := logging.Setup(cfg.LogDir, level)
	if err != nil {
		return fmt.Errorf("initializing logging: %w", err)
	}

	logger.Info("starting carson", "brain_dir", cfg.BrainDir)

	if err := brain.Init(cfg.BrainDir); err != nil {
		return fmt.Errorf("initializing brain folder: %w", err)
	}
	logger.Info("brain folder ready")

	// Load system prompt.
	systemPrompt := loadSystemPrompt(cfg.SystemPromptPath, logger)

	// Initialize LLM provider and harness if configured.
	var h *harness.Harness
	if cfg.LLMProvider != "" {
		provider, err := llm.New(cfg)
		if err != nil {
			return fmt.Errorf("initializing LLM provider: %w", err)
		}
		logger.Info("LLM provider ready", "provider", cfg.LLMProvider, "model", cfg.LLMModel)

		registry := harness.NewRegistry()
		registry.Register(tools.ReadFile(cfg.BrainDir))
		registry.Register(tools.WriteFile(cfg.BrainDir))
		registry.Register(tools.Bash(cfg.BrainDir))

		h = harness.New(harness.Config{
			Provider:     provider,
			Registry:     registry,
			SystemPrompt: systemPrompt,
			BrainDir:     cfg.BrainDir,
			Logger:       logger,
		})
	} else {
		logger.Warn("no LLM provider configured — running without agent capabilities")
	}

	// Start the API server.
	handlers := &api.Handlers{
		Config:      cfg,
		Harness:     h,
		Broadcaster: broadcaster,
		Logger:      logger,
		StartTime:   time.Now(),
	}
	srv := api.NewServer(cfg.DaemonPort, handlers)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", "error", err)
		}
	}()

	// Block until we receive SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("carson is running", "pid", os.Getpid(), "port", cfg.DaemonPort)
	<-ctx.Done()
	logger.Info("shutting down")

	// Graceful shutdown of the API server.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

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
