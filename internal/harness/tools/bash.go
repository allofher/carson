package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/llm"
)

const bashTimeout = 30 * time.Second

// blockedCommands are commands the agent should not run.
// Keep this list short and intentional — the philosophy is trust
// with a small blacklist, not distrust with a whitelist.
var blockedCommands = []string{
	"rm -rf /",
	"mkfs",
	"dd if=",
	":(){ :|:& };:", // fork bomb
}

type bashInput struct {
	Command string `json:"command"`
}

func Bash(workDir string) harness.ToolDef {
	return harness.ToolDef{
		Schema: llm.Tool{
			Name:        "bash",
			Description: "Run a shell command. Use this for tasks like checking the calendar, managing files outside the brain folder, interacting with system tools, or any operation that benefits from shell access. Do not use this for software development tasks like compiling code, running package managers, or building projects.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "The shell command to execute"
					}
				},
				"required": ["command"]
			}`),
		},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var in bashInput
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			if in.Command == "" {
				return "", fmt.Errorf("command is required")
			}

			// Check blacklist.
			lower := strings.ToLower(in.Command)
			for _, blocked := range blockedCommands {
				if strings.Contains(lower, strings.ToLower(blocked)) {
					return "", fmt.Errorf("command blocked: contains %q", blocked)
				}
			}

			ctx, cancel := context.WithTimeout(ctx, bashTimeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
			cmd.Dir = workDir

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			var result strings.Builder
			if stdout.Len() > 0 {
				result.WriteString(stdout.String())
			}
			if stderr.Len() > 0 {
				if result.Len() > 0 {
					result.WriteString("\n")
				}
				result.WriteString("stderr: ")
				result.WriteString(stderr.String())
			}

			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return "", fmt.Errorf("command timed out after %s", bashTimeout)
				}
				// Return the output alongside the error — the agent often
				// needs to see stderr to understand what went wrong.
				if result.Len() > 0 {
					return result.String(), fmt.Errorf("command failed: %w", err)
				}
				return "", fmt.Errorf("command failed: %w", err)
			}

			if result.Len() == 0 {
				return "(no output)", nil
			}
			return result.String(), nil
		},
	}
}
