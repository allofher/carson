package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/allofher/carson/internal/brain"
	"github.com/allofher/carson/internal/chat"
	"github.com/allofher/carson/internal/config"
	"github.com/allofher/carson/internal/daemon"
	"github.com/allofher/carson/internal/lookout"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		if err := initBrain(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "start":
		if err := start(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "chat":
		if err := runChat(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "lookout":
		if err := runLookout(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("carson v0.1.0-dev")
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
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
  init <path>  Set a directory as your brain folder
  start        Start the Carson daemon in the foreground
  chat         Open the terminal chat (The Study)
  lookout      Stream daemon logs with colored output
  version      Print version information
  help         Show this help message
`)
}
