package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/allofher/carson/internal/config"
	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/llm"
	"github.com/allofher/carson/internal/logging"
	"github.com/allofher/carson/internal/session"
)

// Handlers holds the dependencies for all API route handlers.
type Handlers struct {
	Config      *config.Config
	Harness     *harness.Harness
	Sessions    *session.Store
	Broadcaster *logging.Broadcaster
	Logger      *slog.Logger
	StartTime   time.Time
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status": "ok",
	}
	if h.Config.LLMProvider != "" {
		resp["provider"] = h.Config.LLMProvider
	}
	if h.Config.LLMModel != "" {
		resp["model"] = h.Config.LLMModel
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"brain_dir":      h.Config.BrainDir,
		"uptime_seconds": int(time.Since(h.StartTime).Seconds()),
	}
	if h.Config.LLMProvider != "" {
		resp["provider"] = h.Config.LLMProvider
	}
	if h.Config.LLMModel != "" {
		resp["model"] = h.Config.LLMModel
	}
	writeJSON(w, http.StatusOK, resp)
}

// chatRequest is the body of POST /chat.
type chatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

func (h *Handlers) Chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}

	if h.Harness == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no LLM provider configured"})
		return
	}

	h.Logger.Info("chat request", "session", req.SessionID)

	sse, err := newSSEWriter(w)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	// Get session history for multi-turn.
	var history []llm.Message
	if h.Sessions != nil && req.SessionID != "" {
		history = h.Sessions.Get(req.SessionID)
	}

	events := make(chan harness.Event, 64)
	resultCh := make(chan []llm.Message, 1)
	go func() {
		resultCh <- h.Harness.RunStream(r.Context(), req.Message, history, events)
	}()

	for ev := range events {
		switch ev.Type {
		case harness.EventText:
			data, _ := json.Marshal(map[string]string{"content": ev.Content})
			sse.Send("text", string(data))
		case harness.EventToolCall:
			data, _ := json.Marshal(map[string]string{"tool": ev.ToolName, "id": ev.ToolID})
			sse.Send("tool_call", string(data))
		case harness.EventToolResult:
			data, _ := json.Marshal(map[string]string{"tool": ev.ToolName, "id": ev.ToolID, "status": ev.Status})
			sse.Send("tool_result", string(data))
		case harness.EventDone:
			data, _ := json.Marshal(map[string]string{"stop_reason": ev.StopReason})
			sse.Send("done", string(data))
		case harness.EventError:
			data, _ := json.Marshal(map[string]string{"message": ev.Error})
			sse.Send("error", string(data))
		}
	}

	// Store updated session history.
	if h.Sessions != nil && req.SessionID != "" {
		finalMsgs := <-resultCh
		h.Sessions.Set(req.SessionID, finalMsgs)
	}
}

// invokeRequest is the body of POST /invoke.
type invokeRequest struct {
	Prompt  string         `json:"prompt"`
	Context map[string]any `json:"context,omitempty"`
}

func (h *Handlers) Invoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}

	if h.Harness == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no LLM provider configured"})
		return
	}

	h.Logger.Info("invoke request", "prompt_len", len(req.Prompt))

	// Fire-and-forget: run in background with a detached context
	// so it isn't cancelled when the HTTP response is sent.
	go func() {
		_, err := h.Harness.Run(context.Background(), req.Prompt)
		if err != nil {
			h.Logger.Error("invoke failed", "error", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
}

func (h *Handlers) Logs(w http.ResponseWriter, r *http.Request) {
	sse, err := newSSEWriter(w)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	sub := h.Broadcaster.Subscribe()
	defer sub.Unsubscribe()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.C:
			if !ok {
				return
			}
			sse.Send("log", string(msg))
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
