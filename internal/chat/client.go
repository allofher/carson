package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SSEEvent represents a parsed Server-Sent Event.
type SSEEvent struct {
	Event string
	Data  string
}

// Client connects to the Carson daemon HTTP API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a chat client pointing at the daemon.
func NewClient(port int) *Client {
	return &Client{
		baseURL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		httpClient: &http.Client{},
	}
}

// Health checks the daemon health endpoint.
func (c *Client) Health() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("cannot connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}
	return nil
}

// ChatStream sends a message and returns a channel of SSE events.
// The channel is closed when the stream ends.
func (c *Client) ChatStream(message, sessionID string) (<-chan SSEEvent, error) {
	body, _ := json.Marshal(map[string]string{
		"message":    message,
		"session_id": sessionID,
	})

	resp, err := c.httpClient.Post(c.baseURL+"/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	events := make(chan SSEEvent, 16)
	go func() {
		defer resp.Body.Close()
		defer close(events)

		scanner := bufio.NewScanner(resp.Body)
		var event, data string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				event = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data = strings.TrimPrefix(line, "data: ")
			} else if line == "" && event != "" {
				events <- SSEEvent{Event: event, Data: data}
				event = ""
				data = ""
			}
		}
	}()

	return events, nil
}
