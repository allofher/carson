package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionLogger writes conversation events to a JSONL file.
type SessionLogger struct {
	file *os.File
}

type sessionEntry struct {
	Timestamp string `json:"ts"`
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Tool      string `json:"tool,omitempty"`
	Status    string `json:"status,omitempty"`
}

// NewSessionLogger creates a JSONL session file in the sessions directory.
func NewSessionLogger(configDir, sessionID string) (*SessionLogger, error) {
	sessDir := filepath.Join(configDir, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%s_%s.jsonl", time.Now().Format("2006-01-02_15-04-05"), sessionID)
	f, err := os.OpenFile(filepath.Join(sessDir, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &SessionLogger{file: f}, nil
}

// Log writes an entry to the session file.
func (s *SessionLogger) Log(entryType, content, tool, status string) {
	entry := sessionEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Type:      entryType,
		Content:   content,
		Tool:      tool,
		Status:    status,
	}
	data, _ := json.Marshal(entry)
	s.file.Write(append(data, '\n'))
}

// Close closes the session file.
func (s *SessionLogger) Close() error {
	return s.file.Close()
}
