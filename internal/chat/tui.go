package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"

	tea "github.com/charmbracelet/bubbletea"
)

// Styles.
var (
	userStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	inputPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Render("> ")
)

// Message types for Bubble Tea.
type sseEventMsg SSEEvent
type errMsg struct{ err error }
type streamDoneMsg struct{}

// waitForEvent returns a tea.Cmd that waits for the next SSE event on the channel.
func waitForEvent(events <-chan SSEEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return streamDoneMsg{}
		}
		return sseEventMsg(ev)
	}
}

// model is the Bubble Tea model for the chat TUI.
type model struct {
	client    *Client
	session   *SessionLogger
	sessionID string

	messages  []chatMessage
	input     string
	streaming bool
	err       error

	width  int
	height int

	renderer *glamour.TermRenderer

	// Accumulated text for the current streaming response.
	currentText strings.Builder

	// Active SSE event channel (nil when not streaming).
	events <-chan SSEEvent
}

type chatMessage struct {
	role    string // "user", "assistant", "tool", "error"
	content string
}

// Run starts the chat TUI.
func Run(port int, configDir string) error {
	client := NewClient(port)

	// Check daemon health first.
	if err := client.Health(); err != nil {
		return fmt.Errorf("Carson daemon is not running. Start it with `carson start`.\n(%v)", err)
	}

	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixMilli())

	session, err := NewSessionLogger(configDir, sessionID)
	if err != nil {
		return fmt.Errorf("creating session logger: %w", err)
	}

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	m := model{
		client:    client,
		session:   session,
		sessionID: sessionID,
		renderer:  renderer,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	session.Close()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Recreate the renderer with the new width so text wraps properly.
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(msg.Width),
		)
		if err == nil {
			m.renderer = r
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.streaming || strings.TrimSpace(m.input) == "" {
				return m, nil
			}
			return m.sendMessage()
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
			} else if msg.Type == tea.KeySpace {
				m.input += " "
			}
			return m, nil
		}

	case startStreamMsg:
		m.events = msg.events
		return m.handleSSE(msg.first)

	case sseEventMsg:
		return m.handleSSE(SSEEvent(msg))

	case streamDoneMsg:
		if m.currentText.Len() > 0 {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: m.currentText.String()})
			m.session.Log("assistant", m.currentText.String(), "", "")
			m.currentText.Reset()
		}
		m.streaming = false
		m.events = nil
		return m, nil

	case errMsg:
		m.err = msg.err
		m.streaming = false
		m.events = nil
		m.messages = append(m.messages, chatMessage{role: "error", content: msg.err.Error()})
		return m, nil
	}

	return m, nil
}

func (m model) sendMessage() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input)
	m.input = ""
	m.streaming = true
	m.messages = append(m.messages, chatMessage{role: "user", content: text})
	m.session.Log("user", text, "", "")

	// Start the SSE stream.
	return m, func() tea.Msg {
		events, err := m.client.ChatStream(text, m.sessionID)
		if err != nil {
			return errMsg{err}
		}

		// Read the first event; store the channel for subsequent reads.
		ev, ok := <-events
		if !ok {
			return streamDoneMsg{}
		}
		// We need to pass the channel to the model. Use a wrapper message.
		return startStreamMsg{events: events, first: ev}
	}
}

// startStreamMsg carries the channel + first event into the model.
type startStreamMsg struct {
	events <-chan SSEEvent
	first  SSEEvent
}

func (m model) handleSSE(ev SSEEvent) (tea.Model, tea.Cmd) {
	switch ev.Event {
	case "text":
		var data struct{ Content string }
		json.Unmarshal([]byte(ev.Data), &data)
		m.currentText.WriteString(data.Content)

	case "tool_call":
		var data struct {
			Tool string `json:"tool"`
			ID   string `json:"id"`
		}
		json.Unmarshal([]byte(ev.Data), &data)
		// Flush any accumulated text before showing tool indicator.
		if m.currentText.Len() > 0 {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: m.currentText.String()})
			m.currentText.Reset()
		}
		m.messages = append(m.messages, chatMessage{
			role:    "tool",
			content: fmt.Sprintf("[calling %s...]", data.Tool),
		})
		m.session.Log("tool_call", "", data.Tool, "")

	case "tool_result":
		var data struct {
			Tool   string `json:"tool"`
			Status string `json:"status"`
		}
		json.Unmarshal([]byte(ev.Data), &data)
		m.session.Log("tool_result", "", data.Tool, data.Status)

	case "done":
		if m.currentText.Len() > 0 {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: m.currentText.String()})
			m.session.Log("assistant", m.currentText.String(), "", "")
			m.currentText.Reset()
		}
		m.streaming = false
		m.events = nil
		return m, nil

	case "error":
		var data struct{ Message string }
		json.Unmarshal([]byte(ev.Data), &data)
		m.messages = append(m.messages, chatMessage{role: "error", content: data.Message})
		m.streaming = false
		m.events = nil
		return m, nil
	}

	// Wait for next event.
	if m.events != nil {
		return m, waitForEvent(m.events)
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header.
	header := statusStyle.Render("The Study — Carson Chat")
	if m.streaming {
		header += statusStyle.Render("  (streaming...)")
	}
	b.WriteString(header + "\n")
	b.WriteString(statusStyle.Render(strings.Repeat("─", m.width)) + "\n\n")

	// Messages.
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			b.WriteString(userStyle.Render("you: ") + wordwrap.String(msg.content, m.width-6) + "\n\n")
		case "assistant":
			rendered, err := m.renderer.Render(msg.content)
			if err != nil {
				b.WriteString(wordwrap.String(msg.content, m.width) + "\n\n")
			} else {
				b.WriteString(strings.TrimSpace(rendered) + "\n\n")
			}
		case "tool":
			b.WriteString(toolStyle.Render(msg.content) + "\n")
		case "error":
			b.WriteString(errorStyle.Render("error: "+msg.content) + "\n\n")
		}
	}

	// Show in-progress streaming text.
	if m.currentText.Len() > 0 {
		rendered, err := m.renderer.Render(m.currentText.String())
		if err != nil {
			b.WriteString(wordwrap.String(m.currentText.String(), m.width))
		} else {
			b.WriteString(strings.TrimSpace(rendered))
		}
		b.WriteString("\n")
	}

	// Input area at the bottom.
	b.WriteString("\n")
	b.WriteString(statusStyle.Render(strings.Repeat("─", m.width)) + "\n")
	b.WriteString(inputPrompt + m.input)
	if !m.streaming {
		b.WriteString("█")
	}

	return b.String()
}
