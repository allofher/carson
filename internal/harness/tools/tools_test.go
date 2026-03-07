package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile(t *testing.T) {
	brain := t.TempDir()
	os.WriteFile(filepath.Join(brain, "hello.txt"), []byte("hello world"), 0644)

	tool := ReadFile(brain)
	input, _ := json.Marshal(readFileInput{Path: "hello.txt"})
	result, err := tool.Handler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("result = %q, want %q", result, "hello world")
	}
}

func TestReadFile_OutsideBrain(t *testing.T) {
	brain := t.TempDir()
	tool := ReadFile(brain)
	input, _ := json.Marshal(readFileInput{Path: "../../etc/passwd"})
	_, err := tool.Handler(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for path outside brain")
	}
}

func TestReadFile_StaticAllowed(t *testing.T) {
	brain := t.TempDir()
	os.MkdirAll(filepath.Join(brain, "static"), 0755)
	os.WriteFile(filepath.Join(brain, "static", "doc.txt"), []byte("protected"), 0644)

	tool := ReadFile(brain)
	input, _ := json.Marshal(readFileInput{Path: "static/doc.txt"})
	result, err := tool.Handler(context.Background(), input)
	if err != nil {
		t.Fatalf("reads from static/ should be allowed: %v", err)
	}
	if result != "protected" {
		t.Errorf("result = %q, want %q", result, "protected")
	}
}

func TestWriteFile(t *testing.T) {
	brain := t.TempDir()
	tool := WriteFile(brain)

	input, _ := json.Marshal(writeFileInput{Path: "notes/new.md", Content: "# Notes"})
	result, err := tool.Handler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "7 bytes") {
		t.Errorf("result = %q, expected byte count", result)
	}

	// Verify file was written and parent dir was created.
	data, err := os.ReadFile(filepath.Join(brain, "notes", "new.md"))
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != "# Notes" {
		t.Errorf("file content = %q, want %q", string(data), "# Notes")
	}
}

func TestWriteFile_StaticBlocked(t *testing.T) {
	brain := t.TempDir()
	os.MkdirAll(filepath.Join(brain, "static"), 0755)

	tool := WriteFile(brain)
	input, _ := json.Marshal(writeFileInput{Path: "static/secret.txt", Content: "hacked"})
	_, err := tool.Handler(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for write to static/")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error = %q, want mention of read-only", err.Error())
	}
}

func TestWriteFile_OutsideBrain(t *testing.T) {
	brain := t.TempDir()
	tool := WriteFile(brain)
	input, _ := json.Marshal(writeFileInput{Path: "../../tmp/evil.txt", Content: "bad"})
	_, err := tool.Handler(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for path outside brain")
	}
}

func TestBash(t *testing.T) {
	tool := Bash(t.TempDir())
	input, _ := json.Marshal(bashInput{Command: "echo hello"})
	result, err := tool.Handler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result) != "hello" {
		t.Errorf("result = %q, want %q", result, "hello\n")
	}
}

func TestBash_BlockedCommand(t *testing.T) {
	tool := Bash(t.TempDir())
	input, _ := json.Marshal(bashInput{Command: "rm -rf /"})
	_, err := tool.Handler(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for blocked command")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error = %q, want mention of blocked", err.Error())
	}
}

func TestBash_ReturnsStderr(t *testing.T) {
	tool := Bash(t.TempDir())
	input, _ := json.Marshal(bashInput{Command: "echo oops >&2"})
	result, err := tool.Handler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "oops") {
		t.Errorf("result = %q, expected stderr content", result)
	}
}

func TestBash_FailedCommand(t *testing.T) {
	tool := Bash(t.TempDir())
	input, _ := json.Marshal(bashInput{Command: "false"})
	_, err := tool.Handler(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for failed command")
	}
}
