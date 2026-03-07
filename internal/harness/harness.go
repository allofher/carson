package harness

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/allofher/carson/internal/brain"
	"github.com/allofher/carson/internal/llm"
)

const defaultMaxIterations = 50

// Harness manages conversations with the LLM, dispatching tool calls
// through the registry and looping until the model finishes.
type Harness struct {
	provider      llm.Provider
	registry      *Registry
	systemPrompt  string
	brainDir      string
	maxIterations int
	logger        *slog.Logger
}

type Config struct {
	Provider      llm.Provider
	Registry      *Registry
	SystemPrompt  string
	BrainDir      string // For loading topofmind.md
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
		brainDir:      cfg.BrainDir,
		maxIterations: max,
		logger:        logger,
	}
}

// Run sends a user message to the LLM and loops through tool calls
// until the model produces a final response or the iteration limit is hit.
// Returns the final text response from the model.
func (h *Harness) Run(ctx context.Context, userMessage string) (string, error) {
	events := make(chan Event, 64)

	var result strings.Builder
	var runErr error

	done := make(chan struct{})
	go func() {
		defer close(done)
		for ev := range events {
			switch ev.Type {
			case EventText:
				result.WriteString(ev.Content)
			case EventError:
				runErr = fmt.Errorf("%s", ev.Error)
			}
		}
	}()

	h.RunStream(ctx, userMessage, events)
	<-done

	if runErr != nil {
		return "", runErr
	}
	return result.String(), nil
}

// RunStream sends a user message to the LLM and streams events to the
// provided channel. The channel is closed when the stream is complete.
// Events: EventText, EventToolCall, EventToolResult, EventDone, EventError.
func (h *Harness) RunStream(ctx context.Context, userMessage string, events chan<- Event) {
	defer close(events)

	messages := []llm.Message{}

	// Build the full prompt: topofmind + system prompt + user message.
	var promptParts []string
	if h.brainDir != "" {
		if tom := brain.LoadTopOfMind(h.brainDir); tom != "" {
			promptParts = append(promptParts, tom)
		}
	}
	if h.systemPrompt != "" {
		promptParts = append(promptParts, h.systemPrompt)
	}
	promptParts = append(promptParts, userMessage)

	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: strings.Join(promptParts, "\n\n"),
	})

	tools := h.registry.Schemas()

	for i := 0; i < h.maxIterations; i++ {
		if ctx.Err() != nil {
			events <- Event{Type: EventError, Error: ctx.Err().Error()}
			return
		}

		h.logger.Debug("sending to LLM", "iteration", i+1, "messages", len(messages))

		resp, err := h.provider.Chat(ctx, messages, tools)
		if err != nil {
			events <- Event{Type: EventError, Error: fmt.Sprintf("LLM request failed (iteration %d): %s", i+1, err)}
			return
		}

		// Emit text content.
		if resp.Content != "" {
			events <- Event{Type: EventText, Content: resp.Content}
		}

		// If no tool calls, we're done.
		if len(resp.ToolCalls) == 0 {
			h.logger.Debug("LLM finished", "stop_reason", resp.StopReason)
			events <- Event{Type: EventDone, StopReason: string(resp.StopReason)}
			return
		}

		// Append the assistant's response to history.
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call.
		for _, tc := range resp.ToolCalls {
			h.logger.Info("tool call", "tool", tc.Name, "id", tc.ID)
			events <- Event{
				Type:     EventToolCall,
				ToolName: tc.Name,
				ToolID:   tc.ID,
				Input:    tc.Input,
			}

			result, err := h.executeTool(ctx, tc)
			status := "ok"
			if err != nil {
				h.logger.Warn("tool error", "tool", tc.Name, "error", err)
				result = fmt.Sprintf("error: %s", err)
				status = "error"
			}

			events <- Event{
				Type:     EventToolResult,
				ToolName: tc.Name,
				ToolID:   tc.ID,
				Status:   status,
			}

			messages = append(messages, llm.Message{
				Role:         llm.RoleTool,
				Content:      result,
				ToolResultID: tc.ID,
			})
		}
	}

	events <- Event{Type: EventError, Error: fmt.Sprintf("agent loop exceeded %d iterations", h.maxIterations)}
}

func (h *Harness) executeTool(ctx context.Context, tc llm.ToolCall) (string, error) {
	def := h.registry.Get(tc.Name)
	if def == nil {
		return "", fmt.Errorf("unknown tool: %s", tc.Name)
	}
	return def.Handler(ctx, tc.Input)
}
