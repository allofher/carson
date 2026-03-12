package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/allofher/carson/internal/brain"
	"github.com/allofher/carson/internal/chat"
	"github.com/allofher/carson/internal/config"
	"github.com/allofher/carson/internal/daemon"
	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/harness/tools"
	"github.com/allofher/carson/internal/llm"
	"github.com/allofher/carson/internal/lookout"
	"github.com/allofher/carson/internal/scheduler"
	"github.com/allofher/carson/internal/service"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error

	switch os.Args[1] {
	case "init":
		err = initBrain()
	case "start":
		if hasFlag("--foreground") {
			err = start()
		} else {
			err = startDaemon()
		}
	case "stop":
		err = stopDaemon()
	case "restart":
		err = restartDaemon()
	case "status":
		err = statusDaemon()
	case "uninstall":
		err = uninstallDaemon()
	case "run-scheduled":
		err = runScheduled()
	case "chat":
		err = runChat()
	case "lookout":
		err = runLookout()
	case "version":
		fmt.Println("carson v0.1.0-dev")
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args[2:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func flagValue(flag string) string {
	for i, arg := range os.Args[2:] {
		if arg == flag && i+1 < len(os.Args[2:]) {
			return os.Args[i+3] // offset: os.Args[0]=binary, [1]=subcommand, [2+i]=flag, [2+i+1]=value
		}
	}
	return ""
}

func initBrain() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: carson init <path>\n  example: carson init ~/brain")
	}
	brainPath := os.Args[2]

	// Expand ~ and resolve to absolute.
	if brainPath == "~" || len(brainPath) > 1 && brainPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory: %w", err)
		}
		brainPath = filepath.Join(home, brainPath[1:])
	}
	abs, err := filepath.Abs(brainPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	brainPath = abs

	// Initialize brain folder structure.
	if err := brain.Init(brainPath); err != nil {
		return err
	}

	// Write user config.
	configDir := config.UserConfigDir
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	data, err := config.DefaultInitConfig(brainPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Create system prompt file if it doesn't exist.
	promptPath := filepath.Join(configDir, "system-prompt.md")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		if err := os.WriteFile(promptPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("creating system prompt: %w", err)
		}
		fmt.Printf("System prompt created at %s\n", promptPath)
	}

	// Create .env file if it doesn't exist.
	envPath := filepath.Join(configDir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		envContent := `# Carson API keys — add your provider key here.
# CARSON_ANTHROPIC_API_KEY=
# CARSON_OPENAI_API_KEY=
# CARSON_GEMINI_API_KEY=
`
		if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
			return fmt.Errorf("creating .env: %w", err)
		}
		fmt.Printf("Env file created at %s (add your API key here)\n", envPath)
	}

	// Install service file for daemon management.
	mgr, err := service.NewManager(config.UserConfigDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not set up service file: %v\n", err)
	} else {
		if err := mgr.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not install service file: %v\n", err)
		} else {
			fmt.Println("Service file installed for background daemon management.")
		}
	}

	fmt.Printf("Brain initialized at %s\n", brainPath)
	fmt.Printf("Config saved to %s\n", configPath)
	return nil
}

func start() error {
	cfg, err := config.Load(mustCwd())
	if err != nil {
		return err
	}
	return daemon.Run(cfg)
}

func startDaemon() error {
	// Validate config exists before trying to start.
	_, err := config.Load(mustCwd())
	if err != nil {
		return err
	}

	mgr, err := service.NewManager(config.UserConfigDir)
	if err != nil {
		return fmt.Errorf("initializing service manager: %w", err)
	}

	// Check if already running.
	st, err := mgr.Status()
	if err != nil {
		return err
	}
	if st.Running {
		return fmt.Errorf("Carson is already running (PID %d). Use `carson restart` to restart.", st.PID)
	}

	// Ensure service is installed.
	if err := mgr.Install(); err != nil {
		return fmt.Errorf("installing service: %w", err)
	}

	// Start via service manager.
	if err := mgr.Start(); err != nil {
		return fmt.Errorf("starting service: %w", err)
	}

	// Wait briefly and check if it started.
	time.Sleep(1 * time.Second)
	st, _ = mgr.Status()
	if st.Running {
		fmt.Printf("Carson started (PID %d)\n", st.PID)
	} else {
		fmt.Println("Carson service started. Check `carson lookout` for logs.")
	}
	return nil
}

func stopDaemon() error {
	pid, err := service.ReadPID(config.UserConfigDir)
	if err != nil {
		return err
	}
	if pid == 0 || !service.IsRunning(pid) {
		fmt.Println("Carson is not running.")
		return nil
	}

	fmt.Printf("Stopping Carson (PID %d)...\n", pid)
	if err := service.StopProcess(pid); err != nil {
		return fmt.Errorf("stopping Carson: %w", err)
	}

	// Wait for process to exit (up to 10 seconds).
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if !service.IsRunning(pid) {
			service.RemovePID(config.UserConfigDir)
			fmt.Println("Carson stopped.")
			return nil
		}
	}
	return fmt.Errorf("Carson (PID %d) did not stop within 10 seconds", pid)
}

func restartDaemon() error {
	// Stop if running (ignore "not running" case).
	pid, _ := service.ReadPID(config.UserConfigDir)
	if pid > 0 && service.IsRunning(pid) {
		if err := stopDaemon(); err != nil {
			return err
		}
	}
	return startDaemon()
}

func statusDaemon() error {
	pid, err := service.ReadPID(config.UserConfigDir)
	if err != nil {
		return err
	}
	if pid == 0 || !service.IsRunning(pid) {
		fmt.Println("Carson is not running.")
		return nil
	}

	fmt.Printf("Carson is running (PID %d)\n", pid)

	// Try health check.
	cfg, err := config.Load(mustCwd())
	if err == nil {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", cfg.DaemonPort))
		if err == nil {
			defer resp.Body.Close()
			var health map[string]string
			json.NewDecoder(resp.Body).Decode(&health)
			if p := health["provider"]; p != "" {
				fmt.Printf("  Provider: %s\n", p)
			}
			if m := health["model"]; m != "" {
				fmt.Printf("  Model: %s\n", m)
			}
		}
	}
	return nil
}

func uninstallDaemon() error {
	// Stop if running.
	pid, _ := service.ReadPID(config.UserConfigDir)
	if pid > 0 && service.IsRunning(pid) {
		fmt.Printf("Stopping Carson (PID %d)...\n", pid)
		_ = service.StopProcess(pid)
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			if !service.IsRunning(pid) {
				break
			}
		}
	}

	mgr, err := service.NewManager(config.UserConfigDir)
	if err != nil {
		return fmt.Errorf("initializing service manager: %w", err)
	}

	if err := mgr.Uninstall(); err != nil {
		return fmt.Errorf("uninstalling service: %w", err)
	}

	fmt.Println("Carson service uninstalled. The service file and login item have been removed.")
	return nil
}

func runScheduled() error {
	eventID := flagValue("--event")
	if eventID == "" {
		return fmt.Errorf("usage: carson run-scheduled --event <event_id>")
	}

	cfg, err := config.Load(mustCwd())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Set up scheduler.
	schedDir := filepath.Join(config.UserConfigDir, "scheduled")
	store, err := scheduler.NewBundleStore(schedDir)
	if err != nil {
		return fmt.Errorf("opening bundle store: %w", err)
	}

	carsonBin, _ := os.Executable()
	crontab := scheduler.NewCrontabManager(carsonBin)
	sched := scheduler.New(store, crontab, logger)

	// Load the bundle.
	bundle, err := sched.LoadForExecution(eventID)
	if err != nil {
		return fmt.Errorf("loading event %s: %w", eventID, err)
	}
	if bundle == nil {
		logger.Info("event expired, skipping", "id", eventID)
		return nil
	}

	logger.Info("executing scheduled event",
		"id", eventID,
		"prompt_len", len(bundle.Prompt),
		"chain_depth", len(bundle.Chain),
	)

	// Build the user prompt with context.
	userMessage := bundle.Prompt
	if len(bundle.Context) > 0 && string(bundle.Context) != "null" {
		userMessage = fmt.Sprintf("%s\n\nContext:\n```json\n%s\n```", bundle.Prompt, string(bundle.Context))
	}

	// Set up LLM provider.
	if cfg.LLMProvider == "" {
		err := fmt.Errorf("no LLM provider configured")
		sched.MarkFailed(eventID, err)
		return err
	}
	provider, err := llm.New(cfg)
	if err != nil {
		sched.MarkFailed(eventID, err)
		return fmt.Errorf("initializing LLM provider: %w", err)
	}

	// Build registry with scheduling tools that carry the parent chain.
	registry := harness.NewRegistry()
	registry.Register(tools.ReadFile(cfg.BrainDir))
	registry.Register(tools.WriteFile(cfg.BrainDir))
	registry.Register(tools.Bash(cfg.BrainDir))
	registry.Register(tools.ScheduleEvent(sched, bundle.Chain))
	registry.Register(tools.ListScheduledEvents(sched))
	registry.Register(tools.CancelScheduledEvent(sched))

	// Load system prompt.
	var systemPrompt string
	if data, err := os.ReadFile(cfg.SystemPromptPath); err == nil {
		systemPrompt = string(data)
	}

	h := harness.New(harness.Config{
		Provider:     provider,
		Registry:     registry,
		SystemPrompt: systemPrompt,
		BrainDir:     cfg.BrainDir,
		Logger:       logger,
	})

	// Execute with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, execErr := h.Run(ctx, userMessage)
	if execErr != nil {
		sched.MarkFailed(eventID, execErr)
		return fmt.Errorf("execution failed: %w", execErr)
	}

	sched.MarkCompleted(eventID)

	// Opportunistic cleanup of old completed bundles.
	if removed, err := sched.Cleanup(scheduler.DefaultArchiveTTL); err == nil && removed > 0 {
		logger.Info("cleaned up old bundles", "removed", removed)
	}

	return nil
}

func runChat() error {
	cfg, err := config.Load(mustCwd())
	if err != nil {
		return err
	}
	return chat.Run(cfg.DaemonPort, config.UserConfigDir)
}

func runLookout() error {
	cfg, err := config.Load(mustCwd())
	if err != nil {
		return err
	}
	return lookout.Run(cfg.LogDir, cfg.DaemonPort, 50)
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return cwd
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: carson <command>

Commands:
  init <path>      Set a directory as your brain folder
  start            Start the Carson daemon (background by default)
  stop             Stop the Carson daemon
  restart          Restart the Carson daemon
  status           Show daemon status
  uninstall        Remove the Carson service file and login item
  chat             Open the terminal chat (The Study)
  lookout          Stream daemon logs with colored output
  run-scheduled    Execute a scheduled event (called by cron)
  version          Print version information
  help             Show this help message

Flags:
  start --foreground           Run the daemon in the foreground (for development)
  run-scheduled --event <id>   Execute a specific scheduled event
`)
}
