package llm

import (
	"fmt"

	"github.com/allofher/carson/internal/config"
)

const (
	ollamaDefaultURL   = "http://localhost:11434"
	ollamaDefaultModel = "llama3.1"
)

// Ollama exposes an OpenAI-compatible API at /v1/chat/completions,
// so we reuse the openaiProvider with Ollama defaults.
func newOllama(cfg *config.Config) (*openaiProvider, error) {
	baseURL := cfg.LLMBaseURL
	if baseURL == "" {
		baseURL = ollamaDefaultURL
	}
	model := cfg.LLMModel
	if model == "" {
		model = ollamaDefaultModel
	}
	// Verify connectivity hint — Ollama doesn't require an API key.
	if cfg.LLMAPIKey != "" {
		fmt.Println("note: ollama does not require an API key, ignoring CARSON_LLM_API_KEY")
	}
	return newOpenAILike("", model, baseURL, ollamaDefaultModel, ollamaDefaultURL), nil
}
