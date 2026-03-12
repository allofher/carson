package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/allofher/carson/internal/llm"
)

// mockProvider is a fake LLM that returns pre-programmed responses.
type mockProvider struct {
	responses []llm.Response
	calls     int
}

func (m *mockProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if m.calls >= len(m.responses) {
		return nil, fmt.Errorf("mock: no more responses (got %d calls)", m.calls+1)
	}
	resp := m.responses[m.calls]
	m.calls++
	return &resp, nil
}

func TestHarness_SimpleResponse(t *testing.T) {
	mock := &mockProvider{
		responses: []llm.Response{
			{Content: "Hello!", StopReason: llm.StopEndTurn},
		},
	}
	reg := NewRegistry()
	h := New(Config{Provider: mock, Registry: reg})

	result, err := h.Run(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello!" {
		t.Errorf("result = %q, want %q", result, "Hello!")
	}
}

func TestHarness_ToolCallLoop(t *testing.T) {
	// Simulate: LLM calls a tool, gets result, then responds.
	mock := &mockProvider{
		responses: []llm.Response{
			{
				Content: "Let me check.",
				ToolCalls: []llm.ToolCall{
					{ID: "tc_1", Name: "test_tool", Input: json.RawMessage(`{"value":"hello"}`)},
				},
				StopReason: llm.StopToolUse,
			},
			{
				Content:    "The tool said: HELLO",
				StopReason: llm.StopEndTurn,
			},
		},
	}

	reg := NewRegistry()
	reg.Register(ToolDef{
		Schema: llm.Tool{Name: "test_tool"},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var in struct{ Value string }
			json.Unmarshal(input, &in)
			return "HELLO", nil
		},
	})

	h := New(Config{Provider: mock, Registry: reg})
	result, err := h.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// RunStream emits all text events, so Run concatenates them all.
	if result != "Let me check.The tool said: HELLO" {
		t.Errorf("result = %q, want %q", result, "Let me check.The tool said: HELLO")
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 LLM calls, got %d", mock.calls)
	}
}

func TestHarness_ToolError(t *testing.T) {
	// Tool returns an error — the harness should send it back to the LLM.
	mock := &mockProvider{
		responses: []llm.Response{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "tc_1", Name: "failing_tool", Input: json.RawMessage(`{}`)},
				},
				StopReason: llm.StopToolUse,
			},
			{
				Content:    "The tool failed, sorry.",
				StopReason: llm.StopEndTurn,
			},
		},
	}

	reg := NewRegistry()
	reg.Register(ToolDef{
		Schema: llm.Tool{Name: "failing_tool"},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			return "", fmt.Errorf("something went wrong")
		},
	})

	h := New(Config{Provider: mock, Registry: reg})
	result, err := h.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "The tool failed, sorry." {
		t.Errorf("result = %q", result)
	}
}

func TestHarness_UnknownTool(t *testing.T) {
	mock := &mockProvider{
		responses: []llm.Response{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "tc_1", Name: "nonexistent", Input: json.RawMessage(`{}`)},
				},
				StopReason: llm.StopToolUse,
			},
			{
				Content:    "That tool doesn't exist.",
				StopReason: llm.StopEndTurn,
			},
		},
	}

	reg := NewRegistry()
	h := New(Config{Provider: mock, Registry: reg})
	result, err := h.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "That tool doesn't exist." {
		t.Errorf("result = %q", result)
	}
}

func TestHarness_MaxIterations(t *testing.T) {
	// LLM always calls tools — should hit the limit.
	infiniteToolCall := llm.Response{
		ToolCalls: []llm.ToolCall{
			{ID: "tc_1", Name: "loop_tool", Input: json.RawMessage(`{}`)},
		},
		StopReason: llm.StopToolUse,
	}
	responses := make([]llm.Response, 100)
	for i := range responses {
		responses[i] = infiniteToolCall
	}
	mock := &mockProvider{responses: responses}

	reg := NewRegistry()
	reg.Register(ToolDef{
		Schema: llm.Tool{Name: "loop_tool"},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			return "ok", nil
		},
	})

	h := New(Config{Provider: mock, Registry: reg, MaxIterations: 3})
	_, err := h.Run(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for exceeding max iterations")
	}
}

func TestHarness_SystemPrompt(t *testing.T) {
	var capturedMessages []llm.Message
	mock := &mockProvider{
		responses: []llm.Response{
			{Content: "ok", StopReason: llm.StopEndTurn},
		},
	}
	// Wrap to capture messages.
	wrapper := &capturingProvider{inner: mock, captured: &capturedMessages}

	reg := NewRegistry()
	h := New(Config{
		Provider:     wrapper,
		Registry:     reg,
		SystemPrompt: "You are Carson.",
	})
	h.Run(context.Background(), "Hello")

	if len(capturedMessages) == 0 {
		t.Fatal("no messages captured")
	}
	first := capturedMessages[0]
	if first.Role != llm.RoleUser {
		t.Errorf("first message role = %q, want user", first.Role)
	}
	if first.Content != "You are Carson.\n\nHello" {
		t.Errorf("first message = %q, want system prompt + user message", first.Content)
	}
}

func TestHarness_MultiTurn(t *testing.T) {
	// Track all messages sent to the provider across calls.
	var call1Messages, call2Messages []llm.Message

	callCount := 0
	provider := &funcProvider{
		chatFn: func(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
			callCount++
			switch callCount {
			case 1:
				// First turn: capture messages, return a response.
				call1Messages = make([]llm.Message, len(messages))
				copy(call1Messages, messages)
				return &llm.Response{Content: "Hello! How can I help?", StopReason: llm.StopEndTurn}, nil
			case 2:
				// Second turn: capture messages, return a response.
				call2Messages = make([]llm.Message, len(messages))
				copy(call2Messages, messages)
				return &llm.Response{Content: "Sure, I can do that.", StopReason: llm.StopEndTurn}, nil
			default:
				return nil, fmt.Errorf("unexpected call %d", callCount)
			}
		},
	}

	reg := NewRegistry()
	h := New(Config{
		Provider:     provider,
		Registry:     reg,
		SystemPrompt: "You are Carson.",
	})

	// First turn: no history.
	events1 := make(chan Event, 64)
	history := h.RunStream(context.Background(), "Hi there", nil, events1)
	// Drain events.
	for range events1 {
	}

	// Verify first turn had system prompt injected.
	if len(call1Messages) != 1 {
		t.Fatalf("first turn: expected 1 message, got %d", len(call1Messages))
	}
	if !strings.Contains(call1Messages[0].Content, "You are Carson.") {
		t.Errorf("first turn: expected system prompt in message, got %q", call1Messages[0].Content)
	}
	if !strings.Contains(call1Messages[0].Content, "Hi there") {
		t.Errorf("first turn: expected user message in message, got %q", call1Messages[0].Content)
	}

	// Verify history contains the full conversation (user + assistant).
	if len(history) != 2 {
		t.Fatalf("expected history length 2, got %d", len(history))
	}
	if history[0].Role != llm.RoleUser {
		t.Errorf("history[0].Role = %q, want user", history[0].Role)
	}
	if history[1].Role != llm.RoleAssistant {
		t.Errorf("history[1].Role = %q, want assistant", history[1].Role)
	}
	if history[1].Content != "Hello! How can I help?" {
		t.Errorf("history[1].Content = %q", history[1].Content)
	}

	// Second turn: pass history from first turn.
	events2 := make(chan Event, 64)
	history2 := h.RunStream(context.Background(), "Tell me more", history, events2)
	for range events2 {
	}

	// Verify second turn received the full conversation history.
	if len(call2Messages) != 3 {
		t.Fatalf("second turn: expected 3 messages, got %d", len(call2Messages))
	}
	// First message should be the original (with system prompt).
	if call2Messages[0].Role != llm.RoleUser {
		t.Errorf("second turn msg[0].Role = %q, want user", call2Messages[0].Role)
	}
	if !strings.Contains(call2Messages[0].Content, "You are Carson.") {
		t.Errorf("second turn msg[0] should contain system prompt")
	}
	// Second message should be the assistant's first response.
	if call2Messages[1].Role != llm.RoleAssistant {
		t.Errorf("second turn msg[1].Role = %q, want assistant", call2Messages[1].Role)
	}
	// Third message should be the new user message (plain, no system prompt).
	if call2Messages[2].Role != llm.RoleUser {
		t.Errorf("second turn msg[2].Role = %q, want user", call2Messages[2].Role)
	}
	if call2Messages[2].Content != "Tell me more" {
		t.Errorf("second turn msg[2].Content = %q, want plain user message", call2Messages[2].Content)
	}

	// Verify final history has all 4 messages.
	if len(history2) != 4 {
		t.Fatalf("expected final history length 4, got %d", len(history2))
	}
}

type capturingProvider struct {
	inner    *mockProvider
	captured *[]llm.Message
}

func (c *capturingProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	*c.captured = append(*c.captured, messages...)
	return c.inner.Chat(ctx, messages, tools)
}

// funcProvider allows using a function as an llm.Provider for tests.
type funcProvider struct {
	chatFn func(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error)
}

func (f *funcProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	return f.chatFn(ctx, messages, tools)
}
