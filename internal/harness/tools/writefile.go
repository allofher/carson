package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/allofher/carson/internal/brain"
	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/llm"
)

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func WriteFile(brainDir string) harness.ToolDef {
	return harness.ToolDef{
		Schema: llm.Tool{
			Name:        "write_file",
			Description: "Create or overwrite a file in the brain folder. Parent directories are created automatically. The path is relative to the brain folder root. Cannot write to the static/ directory.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Relative path to the file within the brain folder"
					},
					"content": {
						"type": "string",
						"description": "The full content to write to the file"
					}
				},
				"required": ["path", "content"]
			}`),
		},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var in writeFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			if in.Path == "" {
				return "", fmt.Errorf("path is required")
			}

			absPath := filepath.Join(brainDir, in.Path)

			// Sandbox + static/ check.
			if err := brain.ValidateWritePath(brainDir, absPath); err != nil {
				return "", err
			}

			// Validate topofmind.md constraints.
			if brain.IsTopOfMindPath(in.Path) {
				if err := brain.ValidateTopOfMind(in.Content); err != nil {
					return "", fmt.Errorf("write rejected: %w", err)
				}
			}

			// Create parent directories.
			dir := filepath.Dir(absPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("creating directories: %w", err)
			}

			if err := os.WriteFile(absPath, []byte(in.Content), 0644); err != nil {
				return "", fmt.Errorf("writing file: %w", err)
			}

			return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), in.Path), nil
		},
	}
}
