package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/allofher/carson/internal/config"
)

const (
	anthropicDefaultURL   = "https://api.anthropic.com"
	anthropicDefaultModel = "claude-sonnet-4-20250514"
	anthropicAPIVersion   = "2023-06-01"
	anthropicMaxTokens    = 4096
)

type anthropicProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func newAnthropic(cfg *config.Config) (*anthropicProvider, error) {
	if cfg.LLMAPIKey == "" {
		return nil, fmt.Errorf("anthropic provider requires CARSON_LLM_API_KEY")
	}
	model := cfg.LLMModel
	if model == "" {
		model = anthropicDefaultModel
	}
	baseURL := cfg.LLMBaseURL
	if baseURL == "" {
		baseURL = anthropicDefaultURL
	}
	return &anthropicProvider{
		apiKey:  cfg.LLMAPIKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}, nil
}

func (p *anthropicProvider) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	body := map[string]any{
		"model":      p.model,
		"max_tokens": anthropicMaxTokens,
		"messages":   p.convertMessages(messages),
	}
	if len(tools) > 0 {
		body["tools"] = p.convertTools(tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Anthropic-Version", anthropicAPIVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, respBody)
	}

	return p.parseResponse(respBody)
}

func (p *anthropicProvider) convertMessages(msgs []Message) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			out = append(out, map[string]any{"role": "user", "content": m.Content})
		case RoleAssistant:
			msg := map[string]any{"role": "assistant"}
			if len(m.ToolCalls) > 0 {
				var content []map[string]any
				if m.Content != "" {
					content = append(content, map[string]any{"type": "text", "text": m.Content})
				}
				for _, tc := range m.ToolCalls {
					var input any
					json.Unmarshal(tc.Input, &input)
					content = append(content, map[string]any{
						"type":  "tool_use",
						"id":    tc.ID,
						"name":  tc.Name,
						"input": input,
					})
				}
				msg["content"] = content
			} else {
				msg["content"] = m.Content
			}
			out = append(out, msg)
		case RoleTool:
			out = append(out, map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":        "tool_result",
					"tool_use_id": m.ToolResultID,
					"content":     m.Content,
				}},
			})
		}
	}
	return out
}

func (p *anthropicProvider) convertTools(tools []Tool) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		var schema any
		json.Unmarshal(t.InputSchema, &schema)
		out = append(out, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": schema,
		})
	}
	return out
}

func (p *anthropicProvider) parseResponse(data []byte) (*Response, error) {
	var raw struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing anthropic response: %w", err)
	}

	resp := &Response{}
	for _, block := range raw.Content {
		switch block.Type {
		case "text":
			resp.Content += block.Text
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	switch raw.StopReason {
	case "end_turn":
		resp.StopReason = StopEndTurn
	case "tool_use":
		resp.StopReason = StopToolUse
	case "max_tokens":
		resp.StopReason = StopMaxToks
	default:
		resp.StopReason = StopUnknown
	}

	return resp, nil
}
