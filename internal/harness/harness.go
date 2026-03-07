package harness

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/allofher/carson/internal/llm"
)

const defaultMaxIterations = 50

// Harness manages conversations with the LLM, dispatching tool calls
// through the registry and looping until the model finishes.
type Harness struct {
	provider      llm.Provider
	registry      *Registry
	systemPrompt  string
	maxIterations int
	logger        *slog.Logger
}

type Config struct {
	Provider      llm.Provider
	Registry      *Registry
	SystemPrompt  string
	MaxIterations int
	Logger        *slog.Logger
}

func New(cfg Config) *Harness {
	max := cfg.MaxIterations
	if max <= 0 {
		max = defaultMaxIterations
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Harness{
		provider:      cfg.Provider,
		registry:      cfg.Registry,
		systemPrompt:  cfg.SystemPrompt,
		maxIterations: max,
		logger:        logger,
	}
}

// Run sends a user message to the LLM and loops through tool calls
// until the model produces a final response or the iteration limit is hit.
// Returns the final text response from the model.
func (h *Harness) Run(ctx context.Context, userMessage string) (string, error) {
	messages := []llm.Message{}

	// Prepend system prompt as the first user message if set.
	// Most providers handle a system message as the first user turn.
	if h.systemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: h.systemPrompt + "\n\n" + userMessage,
		})
	} else {
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: userMessage,
		})
	}

	tools := h.registry.Schemas()

	for i := 0; i < h.maxIterations; i++ {
		h.logger.Debug("sending to LLM", "iteration", i+1, "messages", len(messages))

		resp, err := h.provider.Chat(ctx, messages, tools)
		if err != nil {
			return "", fmt.Errorf("LLM request failed (iteration %d): %w", i+1, err)
		}

		// If no tool calls, we're done.
		if len(resp.ToolCalls) == 0 {
			h.logger.Debug("LLM finished", "stop_reason", resp.StopReason)
			return resp.Content, nil
		}

		// Append the assistant's response (with tool calls) to history.
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append results.
		for _, tc := range resp.ToolCalls {
			h.logger.Info("tool call", "tool", tc.Name, "id", tc.ID)

			result, err := h.executeTool(ctx, tc)
			if err != nil {
				h.logger.Warn("tool error", "tool", tc.Name, "error", err)
				// Send the error back to the LLM so it can recover.
				result = fmt.Sprintf("error: %s", err)
			}

			messages = append(messages, llm.Message{
				Role:         llm.RoleTool,
				Content:      result,
				ToolResultID: tc.ID,
			})
		}
	}

	return "", fmt.Errorf("agent loop exceeded %d iterations", h.maxIterations)
}

func (h *Harness) executeTool(ctx context.Context, tc llm.ToolCall) (string, error) {
	def := h.registry.Get(tc.Name)
	if def == nil {
		return "", fmt.Errorf("unknown tool: %s", tc.Name)
	}
	return def.Handler(ctx, tc.Input)
}
