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

type readFileInput struct {
	Path string `json:"path"`
}

func ReadFile(brainDir string) harness.ToolDef {
	return harness.ToolDef{
		Schema: llm.Tool{
			Name:        "read_file",
			Description: "Read the contents of a file in the brain folder. The path is relative to the brain folder root.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Relative path to the file within the brain folder"
					}
				},
				"required": ["path"]
			}`),
		},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var in readFileInput
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			if in.Path == "" {
				return "", fmt.Errorf("path is required")
			}

			absPath := filepath.Join(brainDir, in.Path)

			// Sandbox check: must be inside brain folder.
			inside, err := brain.IsInsideBrain(brainDir, absPath)
			if err != nil {
				return "", err
			}
			if !inside {
				return "", fmt.Errorf("path %q is outside the brain folder", in.Path)
			}

			data, err := os.ReadFile(absPath)
			if err != nil {
				return "", fmt.Errorf("reading file: %w", err)
			}
			return string(data), nil
		},
	}
}
