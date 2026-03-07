package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UserConfigDir is the directory for user-level configuration.
// Defaults to ~/.config/carson but can be overridden for testing.
var UserConfigDir = defaultConfigDir()

type Config struct {
	// BrainDir is the absolute path to the watched brain folder.
	BrainDir string

	// LogLevel controls logging verbosity: debug, info, warn, error.
	LogLevel string

	// LLMProvider is the upstream LLM provider name (e.g., "anthropic").
	LLMProvider string

	// LLMAPIKey is the API key for the LLM provider.
	LLMAPIKey string
}

// userConfig is the on-disk representation of ~/.config/carson/config.json.
type userConfig struct {
	BrainPath   string `json:"brain_path"`
	LogLevel    string `json:"log_level,omitempty"`
	LLMProvider string `json:"llm_provider,omitempty"`
}

// Load reads configuration with the following precedence (highest first):
//  1. Environment variables (CARSON_BRAIN_DIR, etc.)
//  2. .env file in envDir
//  3. ~/.config/carson/config.json
func Load(envDir string) (*Config, error) {
	uc := loadUserConfig()
	loadDotEnv(filepath.Join(envDir, ".env"))

	cfg := &Config{
		BrainDir:    firstNonEmpty(os.Getenv("CARSON_BRAIN_DIR"), uc.BrainPath),
		LogLevel:    firstNonEmpty(os.Getenv("CARSON_LOG_LEVEL"), uc.LogLevel, "info"),
		LLMProvider: firstNonEmpty(os.Getenv("CARSON_LLM_PROVIDER"), uc.LLMProvider),
		LLMAPIKey:   os.Getenv("CARSON_LLM_API_KEY"),
	}

	if cfg.BrainDir == "" {
		return nil, fmt.Errorf("brain path not configured — set it in %s or via CARSON_BRAIN_DIR",
			filepath.Join(UserConfigDir, "config.json"))
	}

	// Expand ~ to home directory.
	cfg.BrainDir = expandHome(cfg.BrainDir)

	abs, err := filepath.Abs(cfg.BrainDir)
	if err != nil {
		return nil, fmt.Errorf("resolving brain path: %w", err)
	}
	cfg.BrainDir = abs

	return cfg, nil
}

// loadUserConfig reads ~/.config/carson/config.json if it exists.
func loadUserConfig() userConfig {
	var uc userConfig
	data, err := os.ReadFile(filepath.Join(UserConfigDir, "config.json"))
	if err != nil {
		return uc
	}
	json.Unmarshal(data, &uc)
	return uc
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "carson")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "carson")
}

// loadDotEnv reads a .env file and sets any variables not already present
// in the environment. This is intentionally simple — no quoting, no multiline.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
