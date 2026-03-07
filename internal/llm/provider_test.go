package llm

import (
	"encoding/json"
	"testing"

	"github.com/allofher/carson/internal/config"
)

func TestNew_ValidProviders(t *testing.T) {
	tests := []struct {
		provider string
		needsKey bool
	}{
		{"anthropic", true},
		{"openai", true},
		{"gemini", true},
		{"ollama", false},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := &config.Config{
				LLMProvider: tt.provider,
				LLMAPIKey:   "test-key",
			}
			p, err := New(cfg)
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}
			if p == nil {
				t.Fatal("New() returned nil provider")
			}
		})
	}
}

func TestNew_UnknownProvider(t *testing.T) {
	cfg := &config.Config{LLMProvider: "banana"}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNew_EmptyProvider(t *testing.T) {
	cfg := &config.Config{}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
}

func TestNew_MissingAPIKey(t *testing.T) {
	for _, prov := range []string{"anthropic", "openai", "gemini"} {
		t.Run(prov, func(t *testing.T) {
			cfg := &config.Config{LLMProvider: prov}
			_, err := New(cfg)
			if err == nil {
				t.Fatalf("expected error for %s without API key", prov)
			}
		})
	}
}

func TestAnthropicParseResponse(t *testing.T) {
	p := &anthropicProvider{}
	raw := `{
		"content": [
			{"type": "text", "text": "Hello!"},
			{"type": "tool_use", "id": "tc_1", "name": "read_file", "input": {"path": "notes.md"}}
		],
		"stop_reason": "tool_use"
	}`
	resp, err := p.parseResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parseResponse error: %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello!")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("tool name = %q, want %q", resp.ToolCalls[0].Name, "read_file")
	}
	if resp.StopReason != StopToolUse {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, StopToolUse)
	}
}

func TestOpenAIParseResponse(t *testing.T) {
	p := &openaiProvider{}
	raw := `{
		"choices": [{
			"message": {
				"content": "Sure thing.",
				"tool_calls": [{
					"id": "call_abc",
					"type": "function",
					"function": {"name": "write_file", "arguments": "{\"path\":\"a.txt\",\"content\":\"hi\"}"}
				}]
			},
			"finish_reason": "tool_calls"
		}]
	}`
	resp, err := p.parseResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parseResponse error: %v", err)
	}
	if resp.Content != "Sure thing." {
		t.Errorf("Content = %q, want %q", resp.Content, "Sure thing.")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "write_file" {
		t.Errorf("tool name = %q, want %q", tc.Name, "write_file")
	}
	var input map[string]string
	json.Unmarshal(tc.Input, &input)
	if input["path"] != "a.txt" {
		t.Errorf("tool input path = %q, want %q", input["path"], "a.txt")
	}
}

func TestGeminiParseResponse(t *testing.T) {
	p := &geminiProvider{}
	raw := `{
		"candidates": [{
			"content": {
				"parts": [
					{"text": "Let me check."},
					{"functionCall": {"name": "search_files", "args": {"query": "*.md"}}}
				]
			},
			"finishReason": "STOP"
		}]
	}`
	resp, err := p.parseResponse([]byte(raw))
	if err != nil {
		t.Fatalf("parseResponse error: %v", err)
	}
	if resp.Content != "Let me check." {
		t.Errorf("Content = %q, want %q", resp.Content, "Let me check.")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "search_files" {
		t.Errorf("tool name = %q, want %q", resp.ToolCalls[0].Name, "search_files")
	}
}

func TestAnthropicParseResponse_TextOnly(t *testing.T) {
	p := &anthropicProvider{}
	raw := `{"content": [{"type": "text", "text": "Just text."}], "stop_reason": "end_turn"}`
	resp, err := p.parseResponse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Just text." {
		t.Errorf("Content = %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.StopReason != StopEndTurn {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, StopEndTurn)
	}
}
