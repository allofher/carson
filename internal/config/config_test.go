package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FromEnv(t *testing.T) {
	tmp := t.TempDir()

	t.Setenv("CARSON_BRAIN_DIR", tmp)
	t.Setenv("CARSON_LOG_LEVEL", "debug")
	t.Setenv("CARSON_LLM_PROVIDER", "")
	t.Setenv("CARSON_LLM_API_KEY", "")

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

	t.Setenv("CARSON_BRAIN_DIR", tmp)
	t.Setenv("CARSON_LOG_LEVEL", "")
	t.Setenv("CARSON_LLM_PROVIDER", "")
	t.Setenv("CARSON_LLM_API_KEY", "")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoad_MissingBrainDir(t *testing.T) {
	t.Setenv("CARSON_BRAIN_DIR", "")
	// Point UserConfigDir to empty temp dir so no config.json is found.
	UserConfigDir = t.TempDir()

	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing brain path")
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	tmp := t.TempDir()
	brainDir := t.TempDir()

	envContent := "CARSON_BRAIN_DIR=" + brainDir + "\nCARSON_LOG_LEVEL=warn\n"
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CARSON_BRAIN_DIR", "")
	t.Setenv("CARSON_LOG_LEVEL", "")
	t.Setenv("CARSON_LLM_PROVIDER", "")
	t.Setenv("CARSON_LLM_API_KEY", "")

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

	uc := userConfig{BrainPath: brainDir, LogLevel: "debug"}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	// Point to our test config dir and clear env vars.
	UserConfigDir = configDir
	t.Setenv("CARSON_BRAIN_DIR", "")
	t.Setenv("CARSON_LOG_LEVEL", "")
	t.Setenv("CARSON_LLM_PROVIDER", "")
	t.Setenv("CARSON_LLM_API_KEY", "")

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
}

func TestLoad_EnvOverridesUserConfig(t *testing.T) {
	brainFromConfig := t.TempDir()
	brainFromEnv := t.TempDir()
	configDir := t.TempDir()

	uc := userConfig{BrainPath: brainFromConfig}
	data, _ := json.Marshal(uc)
	os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)

	UserConfigDir = configDir
	t.Setenv("CARSON_BRAIN_DIR", brainFromEnv)
	t.Setenv("CARSON_LOG_LEVEL", "")
	t.Setenv("CARSON_LLM_PROVIDER", "")
	t.Setenv("CARSON_LLM_API_KEY", "")

	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BrainDir != brainFromEnv {
		t.Errorf("BrainDir = %q, want %q (env should override user config)", cfg.BrainDir, brainFromEnv)
	}
}
