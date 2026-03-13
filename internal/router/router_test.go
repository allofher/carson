package router

import (
	"strings"
	"testing"

	"github.com/allofher/carson/internal/watcher"
)

func TestBuildPrompt(t *testing.T) {
	events := []watcher.FileEvent{
		{Path: "notes/idea.md", Op: "create", Category: watcher.CategoryMutable},
		{Path: "TODO.md", Op: "modify", Category: watcher.CategoryTodo},
		{Path: "static/photos/new.jpg", Op: "create", Category: watcher.CategoryStatic},
	}

	prompt := BuildPrompt(events)

	// Check all files mentioned.
	if !strings.Contains(prompt, "notes/idea.md (create)") {
		t.Error("prompt missing notes/idea.md")
	}
	if !strings.Contains(prompt, "TODO.md (modify)") {
		t.Error("prompt missing TODO.md")
	}
	if !strings.Contains(prompt, "static/photos/new.jpg (create) [read-only]") {
		t.Error("prompt missing static file with read-only annotation")
	}

	// Non-static files should not have [read-only].
	lines := strings.Split(prompt, "\n")
	for _, line := range lines {
		if strings.Contains(line, "notes/idea.md") && strings.Contains(line, "[read-only]") {
			t.Error("mutable file should not have [read-only] annotation")
		}
	}
}

func TestBuildPromptEmpty(t *testing.T) {
	prompt := BuildPrompt(nil)
	if !strings.Contains(prompt, "file changes") {
		t.Error("even empty prompt should mention file changes")
	}
}
