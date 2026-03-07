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
	openaiDefaultURL   = "https://api.openai.com"
	openaiDefaultModel = "gpt-4o"
)

type openaiProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func newOpenAI(cfg *config.Config) (*openaiProvider, error) {
	if cfg.LLMAPIKey == "" {
		return nil, fmt.Errorf("openai provider requires CARSON_LLM_API_KEY")
	}
	return newOpenAILike(cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMBaseURL, openaiDefaultModel, openaiDefaultURL), nil
}

func newOpenAILike(apiKey, model, baseURL, defaultModel, defaultBaseURL string) *openaiProvider {
	if model == "" {
		model = defaultModel
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &openaiProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (p *openaiProvider) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	body := map[string]any{
		"model":    p.model,
		"messages": p.convertMessages(messages),
	}
	if len(tools) > 0 {
		body["tools"] = p.convertTools(tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, respBody)
	}

	return p.parseResponse(respBody)
}

func (p *openaiProvider) convertMessages(msgs []Message) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			out = append(out, map[string]any{"role": "user", "content": m.Content})
		case RoleAssistant:
			msg := map[string]any{"role": "assistant"}
			if m.Content != "" {
				msg["content"] = m.Content
			}
			if len(m.ToolCalls) > 0 {
				var tcs []map[string]any
				for _, tc := range m.ToolCalls {
					tcs = append(tcs, map[string]any{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]any{
							"name":      tc.Name,
							"arguments": string(tc.Input),
						},
					})
				}
				msg["tool_calls"] = tcs
			}
			out = append(out, msg)
		case RoleTool:
			out = append(out, map[string]any{
				"role":         "tool",
				"tool_call_id": m.ToolResultID,
				"content":      m.Content,
			})
		}
	}
	return out
}

func (p *openaiProvider) convertTools(tools []Tool) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		var schema any
		json.Unmarshal(t.InputSchema, &schema)
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  schema,
			},
		})
	}
	return out
}

func (p *openaiProvider) parseResponse(data []byte) (*Response, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content   *string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing openai response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}

	choice := raw.Choices[0]
	resp := &Response{}
	if choice.Message.Content != nil {
		resp.Content = *choice.Message.Content
	}
	for _, tc := range choice.Message.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	switch choice.FinishReason {
	case "stop":
		resp.StopReason = StopEndTurn
	case "tool_calls":
		resp.StopReason = StopToolUse
	case "length":
		resp.StopReason = StopMaxToks
	default:
		resp.StopReason = StopUnknown
	}

	return resp, nil
}
