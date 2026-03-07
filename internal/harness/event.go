package harness

import "encoding/json"

// EventType identifies the kind of streaming event.
type EventType int

const (
	EventText       EventType = iota // Agent text chunk
	EventToolCall                    // Agent is calling a tool
	EventToolResult                  // Tool finished
	EventDone                        // Stream complete
	EventError                       // Something went wrong
)

// Event is a single streaming event from the harness.
type Event struct {
	Type EventType

	// Text content (EventText).
	Content string

	// Tool call fields (EventToolCall, EventToolResult).
	ToolName string
	ToolID   string
	Input    json.RawMessage // EventToolCall only
	Status   string          // EventToolResult: "ok" or "error"

	// Stop reason (EventDone).
	StopReason string

	// Error message (EventError).
	Error string
}
