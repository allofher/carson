package api

import (
	"fmt"
	"net/http"
)

// sseWriter wraps an http.ResponseWriter to send Server-Sent Events.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	return &sseWriter{w: w, flusher: f}, nil
}

// Send writes a single SSE event.
func (s *sseWriter) Send(event string, data string) {
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data)
	s.flusher.Flush()
}

// SendData writes an SSE event with only data (no event type).
func (s *sseWriter) SendData(data string) {
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
}
