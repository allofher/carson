package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/allofher/carson/internal/config"
)

// Role identifies the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// StopReason indicates why the model stopped generating.
type StopReason string

const (
	StopEndTurn  StopReason = "end_turn"
	StopToolUse  StopReason = "tool_use"
	StopMaxToks  StopReason = "max_tokens"
	StopUnknown  StopReason = "unknown"
)

// Message represents a single turn in a conversation.
type Message struct {
	Role         Role
	Content      string
	ToolCalls    []ToolCall
	ToolResultID string // set when Role == RoleTool
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Tool describes a tool the model can call.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage // JSON Schema object
}

// Response is the model's reply to a chat request.
type Response struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason StopReason
}

// Provider is the interface every LLM backend implements.
type Provider interface {
	// Chat sends a conversation to the model and returns its response.
	// tools may be nil if no tools are available for this request.
	Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
}

// New creates a Provider from the given config.
func New(cfg *config.Config) (Provider, error) {
	switch cfg.LLMProvider {
	case "anthropic":
		return newAnthropic(cfg)
	case "openai":
		return newOpenAI(cfg)
	case "gemini":
		return newGemini(cfg)
	case "ollama":
		return newOllama(cfg)
	case "":
		return nil, fmt.Errorf("no LLM provider configured — set llm_provider in config or CARSON_LLM_PROVIDER")
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (supported: anthropic, openai, gemini, ollama)", cfg.LLMProvider)
	}
}
