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
	geminiDefaultURL   = "https://generativelanguage.googleapis.com"
	geminiDefaultModel = "gemini-2.5-flash"
)

type geminiProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func newGemini(cfg *config.Config) (*geminiProvider, error) {
	if cfg.LLMAPIKey == "" {
		return nil, fmt.Errorf("gemini provider requires CARSON_LLM_API_KEY")
	}
	model := cfg.LLMModel
	if model == "" {
		model = geminiDefaultModel
	}
	baseURL := cfg.LLMBaseURL
	if baseURL == "" {
		baseURL = geminiDefaultURL
	}
	return &geminiProvider{
		apiKey:  cfg.LLMAPIKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}, nil
}

func (p *geminiProvider) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	body := map[string]any{
		"contents": p.convertMessages(messages),
	}
	if len(tools) > 0 {
		body["tools"] = []map[string]any{
			{"function_declarations": p.convertTools(tools)},
		}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, respBody)
	}

	return p.parseResponse(respBody)
}

func (p *geminiProvider) convertMessages(msgs []Message) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		switch m.Role {
		case RoleUser:
			out = append(out, map[string]any{
				"role":  "user",
				"parts": []map[string]any{{"text": m.Content}},
			})
		case RoleAssistant:
			entry := map[string]any{"role": "model"}
			var parts []map[string]any
			if m.Content != "" {
				parts = append(parts, map[string]any{"text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				var args any
				json.Unmarshal(tc.Input, &args)
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": tc.Name,
						"args": args,
					},
				})
			}
			entry["parts"] = parts
			out = append(out, entry)
		case RoleTool:
			var result any
			json.Unmarshal([]byte(m.Content), &result)
			if result == nil {
				result = map[string]any{"result": m.Content}
			}
			out = append(out, map[string]any{
				"role": "user",
				"parts": []map[string]any{{
					"functionResponse": map[string]any{
						"name":     m.ToolResultID,
						"response": result,
					},
				}},
			})
		}
	}
	return out
}

func (p *geminiProvider) convertTools(tools []Tool) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		var params any
		json.Unmarshal(t.InputSchema, &params)
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  params,
		})
	}
	return out
}

func (p *geminiProvider) parseResponse(data []byte) (*Response, error) {
	var raw struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing gemini response: %w", err)
	}
	if len(raw.Candidates) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	cand := raw.Candidates[0]
	resp := &Response{}
	for i, part := range cand.Content.Parts {
		if part.Text != "" {
			resp.Content += part.Text
		}
		if part.FunctionCall != nil {
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:    fmt.Sprintf("call_%d", i),
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		}
	}

	switch cand.FinishReason {
	case "STOP":
		resp.StopReason = StopEndTurn
	case "MAX_TOKENS":
		resp.StopReason = StopMaxToks
	default:
		if len(resp.ToolCalls) > 0 {
			resp.StopReason = StopToolUse
		} else {
			resp.StopReason = StopUnknown
		}
	}

	return resp, nil
}
