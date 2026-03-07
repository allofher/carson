package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// clearLLMEnv zeroes out all LLM-related env vars so tests are isolated.
func clearLLMEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"CARSON_BRAIN_DIR", "CARSON_LOG_LEVEL",
		"CARSON_LLM_PROVIDER", "CARSON_LLM_MODEL", "CARSON_LLM_BASE_URL",
		"CARSON_LLM_API_KEY",
		"CARSON_ANTHROPIC_API_KEY", "CARSON_OPENAI_API_KEY", "CARSON_GEMINI_API_KEY",
	} {
		t.Setenv(k, "")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	tmp := t.TempDir()
	clearLLMEnv(t)

	t.Setenv("CARSON_BRAIN_DIR", tmp)
	t.Setenv("CARSON_LOG_LEVEL", "debug")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BrainDir != tmp {
		t.Errorf("BrainDir = %q, want %q", cfg.BrainDir, tmp)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoad_DefaultLogLevel(t *testing.T) {
	tmp := t.TempDir()
	clearLLMEnv(t)
	t.Setenv("CARSON_BRAIN_DIR", tmp)

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoad_MissingBrainDir(t *testing.T) {
	clearLLMEnv(t)
	UserConfigDir = t.TempDir()

	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing brain path")
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	tmp := t.TempDir()
	brainDir := t.TempDir()
	clearLLMEnv(t)

	envContent := "CARSON_BRAIN_DIR=" + brainDir + "\nCARSON_LOG_LEVEL=warn\n"
	os.WriteFile(filepath.Join(tmp, ".env"), []byte(envContent), 0644)

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BrainDir != brainDir {
		t.Errorf("BrainDir = %q, want %q", cfg.BrainDir, brainDir)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "warn")
	}
}

func TestLoad_UserConfigFile(t *testing.T) {
	brainDir := t.TempDir()
	configDir := t.TempDir()
	clearLLMEnv(t)

	uc := userConfig{BrainPath: brainDir, LogLevel: "debug", LLMProvider: "anthropic"}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BrainDir != brainDir {
		t.Errorf("BrainDir = %q, want %q", cfg.BrainDir, brainDir)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.LLMProvider != "anthropic" {
		t.Errorf("LLMProvider = %q, want %q", cfg.LLMProvider, "anthropic")
	}
}

func TestLoad_EnvOverridesUserConfig(t *testing.T) {
	brainFromConfig := t.TempDir()
	brainFromEnv := t.TempDir()
	configDir := t.TempDir()
	clearLLMEnv(t)

	uc := userConfig{BrainPath: brainFromConfig}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir
	t.Setenv("CARSON_BRAIN_DIR", brainFromEnv)

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BrainDir != brainFromEnv {
		t.Errorf("BrainDir = %q, want %q (env should override user config)", cfg.BrainDir, brainFromEnv)
	}
}

func TestLoad_ProviderSpecificAPIKey(t *testing.T) {
	configDir := t.TempDir()
	brainDir := t.TempDir()
	clearLLMEnv(t)

	uc := userConfig{BrainPath: brainDir, LLMProvider: "anthropic"}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir
	t.Setenv("CARSON_ANTHROPIC_API_KEY", "sk-ant-test")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMAPIKey != "sk-ant-test" {
		t.Errorf("LLMAPIKey = %q, want %q", cfg.LLMAPIKey, "sk-ant-test")
	}
}

func TestLoad_SwitchProvider_GetsCorrectKey(t *testing.T) {
	configDir := t.TempDir()
	brainDir := t.TempDir()
	clearLLMEnv(t)

	// Config says openai, both keys are set.
	uc := userConfig{BrainPath: brainDir, LLMProvider: "openai"}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir
	t.Setenv("CARSON_ANTHROPIC_API_KEY", "sk-ant-wrong")
	t.Setenv("CARSON_OPENAI_API_KEY", "sk-oai-correct")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMAPIKey != "sk-oai-correct" {
		t.Errorf("LLMAPIKey = %q, want %q (should use openai key)", cfg.LLMAPIKey, "sk-oai-correct")
	}
}

func TestLoad_GenericKeyFallback(t *testing.T) {
	configDir := t.TempDir()
	brainDir := t.TempDir()
	clearLLMEnv(t)

	uc := userConfig{BrainPath: brainDir, LLMProvider: "anthropic"}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir
	// No provider-specific key, but generic fallback is set.
	t.Setenv("CARSON_LLM_API_KEY", "sk-generic")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMAPIKey != "sk-generic" {
		t.Errorf("LLMAPIKey = %q, want %q (generic fallback)", cfg.LLMAPIKey, "sk-generic")
	}
}

func TestLoad_OllamaNoKeyRequired(t *testing.T) {
	configDir := t.TempDir()
	brainDir := t.TempDir()
	clearLLMEnv(t)

	uc := userConfig{BrainPath: brainDir, LLMProvider: "ollama"}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMAPIKey != "" {
		t.Errorf("LLMAPIKey = %q, want empty for ollama", cfg.LLMAPIKey)
	}
	if cfg.LLMProvider != "ollama" {
		t.Errorf("LLMProvider = %q, want %q", cfg.LLMProvider, "ollama")
	}
}

func TestLoad_ModelAndBaseURLFromConfig(t *testing.T) {
	configDir := t.TempDir()
	brainDir := t.TempDir()
	clearLLMEnv(t)

	uc := userConfig{
		BrainPath:   brainDir,
		LLMProvider: "ollama",
		LLMModel:    "mistral",
		LLMBaseURL:  "http://gpu-box:11434",
	}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMModel != "mistral" {
		t.Errorf("LLMModel = %q, want %q", cfg.LLMModel, "mistral")
	}
	if cfg.LLMBaseURL != "http://gpu-box:11434" {
		t.Errorf("LLMBaseURL = %q, want %q", cfg.LLMBaseURL, "http://gpu-box:11434")
	}
}
