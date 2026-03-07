package brain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	TopOfMindFile   = "topofmind.md"
	topOfMindMaxLen = 2048     // 2 KB
	topOfMindMaxLines = 30
)

// ValidateTopOfMind checks that content intended for topofmind.md meets
// the structural constraints. Returns nil if valid.
func ValidateTopOfMind(content string) error {
	if len(content) > topOfMindMaxLen {
		return fmt.Errorf("topofmind.md exceeds %d bytes (got %d)", topOfMindMaxLen, len(content))
	}

	lines := strings.Split(content, "\n")
	// Trailing newline produces an empty final element — don't count it.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > topOfMindMaxLines {
		return fmt.Errorf("topofmind.md exceeds %d lines (got %d)", topOfMindMaxLines, len(lines))
	}

	if strings.Contains(content, "```") {
		return fmt.Errorf("topofmind.md must not contain fenced code blocks")
	}

	return nil
}

// LoadTopOfMind reads topofmind.md from the brain folder.
// Returns empty string if the file doesn't exist or is empty.
func LoadTopOfMind(brainDir string) string {
	data, err := os.ReadFile(filepath.Join(brainDir, TopOfMindFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// IsTopOfMindPath reports whether the given path (relative to brain dir)
// is the topofmind.md file.
func IsTopOfMindPath(relPath string) bool {
	return relPath == TopOfMindFile
}
