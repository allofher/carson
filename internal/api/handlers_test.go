package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/allofher/carson/internal/config"
	"github.com/allofher/carson/internal/logging"
)

func testHandlers() *Handlers {
	return &Handlers{
		Config: &config.Config{
			BrainDir:    "/tmp/test-brain",
			LLMProvider: "anthropic",
			LLMModel:    "claude-sonnet-4-20250514",
		},
		Broadcaster: logging.NewBroadcaster(),
		Logger:      slog.Default(),
		StartTime:   time.Now(),
	}
}

func TestHealth(t *testing.T) {
	h := testHandlers()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %q", resp["status"])
	}
	if resp["provider"] != "anthropic" {
		t.Errorf("expected provider anthropic, got %q", resp["provider"])
	}
}

func TestStatus(t *testing.T) {
	h := testHandlers()
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["brain_dir"] != "/tmp/test-brain" {
		t.Errorf("unexpected brain_dir: %v", resp["brain_dir"])
	}
}

func TestChatNoHarness(t *testing.T) {
	h := testHandlers()
	body := `{"message": "hello", "session_id": "test"}`
	req := httptest.NewRequest("POST", "/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestChatBadMethod(t *testing.T) {
	h := testHandlers()
	req := httptest.NewRequest("GET", "/chat", nil)
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestChatEmptyMessage(t *testing.T) {
	h := testHandlers()
	body := `{"message": "", "session_id": "test"}`
	req := httptest.NewRequest("POST", "/chat", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestInvokeNoHarness(t *testing.T) {
	h := testHandlers()
	body := `{"prompt": "do something"}`
	req := httptest.NewRequest("POST", "/invoke", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Invoke(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
