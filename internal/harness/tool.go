package harness

import (
	"context"
	"encoding/json"

	"github.com/allofher/carson/internal/llm"
)

// ToolHandler executes a tool call and returns the result as a string.
type ToolHandler func(ctx context.Context, input json.RawMessage) (string, error)

// ToolDef bundles a tool's LLM-facing schema with its server-side handler.
type ToolDef struct {
	Schema  llm.Tool
	Handler ToolHandler
}

// Registry holds the set of tools available to the agent.
type Registry struct {
	tools map[string]ToolDef
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]ToolDef)}
}

func (r *Registry) Register(def ToolDef) {
	r.tools[def.Schema.Name] = def
}

// Schemas returns the tool schemas for passing to the LLM.
func (r *Registry) Schemas() []llm.Tool {
	out := make([]llm.Tool, 0, len(r.tools))
	for _, def := range r.tools {
		out = append(out, def.Schema)
	}
	return out
}

// Get returns the tool definition for the given name, or nil if not found.
func (r *Registry) Get(name string) *ToolDef {
	def, ok := r.tools[name]
	if !ok {
		return nil
	}
	return &def
}
