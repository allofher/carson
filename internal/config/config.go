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

	// LLMProvider is the upstream LLM provider name: anthropic, openai, gemini, ollama.
	LLMProvider string

	// LLMAPIKey is the resolved API key for the active provider.
	LLMAPIKey string

	// LLMModel is the model identifier (e.g., "claude-sonnet-4-20250514", "gpt-4o").
	// If empty, each provider uses its own default.
	LLMModel string

	// LLMBaseURL overrides the provider's default API endpoint.
	// Required for Ollama (e.g., "http://localhost:11434").
	LLMBaseURL string

	// SystemPromptPath is the absolute path to the system prompt markdown file.
	// Defaults to ~/.config/carson/system-prompt.md.
	SystemPromptPath string
}

// userConfig is the on-disk representation of ~/.config/carson/config.json.
// This holds preferences — never secrets.
type userConfig struct {
	BrainPath   string `json:"brain_path"`
	LogLevel    string `json:"log_level,omitempty"`
	LLMProvider string `json:"llm_provider,omitempty"`
	LLMModel    string `json:"llm_model,omitempty"`
	LLMBaseURL       string `json:"llm_base_url,omitempty"`
	SystemPromptPath string `json:"system_prompt_path,omitempty"`
}

// providerKeyEnvVars maps each provider to its specific API key env var.
var providerKeyEnvVars = map[string]string{
	"anthropic": "CARSON_ANTHROPIC_API_KEY",
	"openai":    "CARSON_OPENAI_API_KEY",
	"gemini":    "CARSON_GEMINI_API_KEY",
}

// Load reads configuration with the following precedence (highest first):
//  1. Environment variables
//  2. .env file in envDir (secrets only)
//  3. ~/.config/carson/config.json (preferences)
//
// Preferences (provider, model, base_url, brain_path) come from config.json.
// Secrets (API keys) come from env vars / .env.
// The API key is resolved based on the active provider.
func Load(envDir string) (*Config, error) {
	uc := loadUserConfig()
	loadDotEnv(filepath.Join(envDir, ".env"))

	cfg := &Config{
		BrainDir:    firstNonEmpty(os.Getenv("CARSON_BRAIN_DIR"), uc.BrainPath),
		LogLevel:    firstNonEmpty(os.Getenv("CARSON_LOG_LEVEL"), uc.LogLevel, "info"),
		LLMProvider: firstNonEmpty(os.Getenv("CARSON_LLM_PROVIDER"), uc.LLMProvider),
		LLMModel:    firstNonEmpty(os.Getenv("CARSON_LLM_MODEL"), uc.LLMModel),
		LLMBaseURL:  firstNonEmpty(os.Getenv("CARSON_LLM_BASE_URL"), uc.LLMBaseURL),
	}

	// Resolve the API key for the active provider.
	if cfg.LLMProvider != "" {
		cfg.LLMAPIKey = resolveAPIKey(cfg.LLMProvider)
	}

	// Resolve system prompt path.
	cfg.SystemPromptPath = firstNonEmpty(
		os.Getenv("CARSON_SYSTEM_PROMPT_PATH"),
		uc.SystemPromptPath,
		filepath.Join(UserConfigDir, "system-prompt.md"),
	)
	cfg.SystemPromptPath = expandHome(cfg.SystemPromptPath)

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

// resolveAPIKey finds the API key for the given provider by checking
// the provider-specific env var first, then the generic fallback.
func resolveAPIKey(provider string) string {
	if envVar, ok := providerKeyEnvVars[provider]; ok {
		if key := os.Getenv(envVar); key != "" {
			return key
		}
	}
	// Generic fallback for backwards compatibility.
	return os.Getenv("CARSON_LLM_API_KEY")
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
