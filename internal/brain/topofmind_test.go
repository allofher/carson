package brain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTopOfMind_Valid(t *testing.T) {
	content := "# Current Focus\n- Working on chat milestone\n- Review PR #42\n"
	if err := ValidateTopOfMind(content); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTopOfMind_TooLong(t *testing.T) {
	content := strings.Repeat("x", 2049)
	err := ValidateTopOfMind(content)
	if err == nil {
		t.Fatal("expected error for oversized content")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateTopOfMind_TooManyLines(t *testing.T) {
	lines := make([]string, 31)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")
	err := ValidateTopOfMind(content)
	if err == nil {
		t.Fatal("expected error for too many lines")
	}
}

func TestValidateTopOfMind_CodeBlock(t *testing.T) {
	content := "# Notes\n```go\nfmt.Println(\"hi\")\n```\n"
	err := ValidateTopOfMind(content)
	if err == nil {
		t.Fatal("expected error for code block")
	}
}

func TestValidateTopOfMind_TrailingNewline(t *testing.T) {
	// 30 lines + trailing newline should be fine.
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := ValidateTopOfMind(content); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadTopOfMind(t *testing.T) {
	dir := t.TempDir()
	Init(dir)

	// Should be empty initially.
	tom := LoadTopOfMind(dir)
	if tom != "" {
		t.Errorf("expected empty, got %q", tom)
	}

	// Write content and reload.
	os.WriteFile(filepath.Join(dir, TopOfMindFile), []byte("# Focus\n- Tests\n"), 0644)
	tom = LoadTopOfMind(dir)
	if tom != "# Focus\n- Tests" {
		t.Errorf("unexpected content: %q", tom)
	}
}
