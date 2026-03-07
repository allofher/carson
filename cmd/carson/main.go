package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/allofher/carson/internal/brain"
	"github.com/allofher/carson/internal/config"
	"github.com/allofher/carson/internal/daemon"
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
	cfg := map[string]string{"brain_path": brainPath}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, append(data, '\n'), 0644); err != nil {
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

	fmt.Printf("Brain initialized at %s\n", brainPath)
	fmt.Printf("Config saved to %s\n", configPath)
	return nil
}

func start() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	return daemon.Run(cfg)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: carson <command>

Commands:
  init <path>  Set a directory as your brain folder
  start        Start the Carson daemon in the foreground
  version      Print version information
  help         Show this help message
`)
}
