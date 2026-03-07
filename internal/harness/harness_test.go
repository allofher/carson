package harness

import (
	"context"
	"encoding/json"
	"fmt"
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

type capturingProvider struct {
	inner    *mockProvider
	captured *[]llm.Message
}

func (c *capturingProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	*c.captured = append(*c.captured, messages...)
	return c.inner.Chat(ctx, messages, tools)
}
