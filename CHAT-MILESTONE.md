# Chat Milestone — Design & Implementation Plan

> **Audience:** Developer picking this up for implementation.
> **Status:** Planning — not yet implemented.

## Overview

Carson's terminal chat ("The Study") is a TUI client that connects to the Carson daemon over HTTP. The daemon exposes a local API; the chat is a rendering layer. Different surfaces (terminal, desktop, mobile) use different endpoints tailored to their needs.

```
┌────────────────────┐         ┌──────────────────────────────┐
│  carson chat (TUI) │◄──HTTP──►  Carson Daemon                │
│                    │   SSE   │  ┌──────────┐  ┌───────────┐ │
│  Bubble Tea        │         │  │ Local API │──│ Harness   │ │
│  Elm architecture  │         │  └──────────┘  └───────────┘ │
└────────────────────┘         └──────────────────────────────┘

┌────────────────────┐
│  carson lookout    │◄──SSE──── GET /logs (real-time log tail)
└────────────────────┘
```

## Architecture: Client-Server Split

The terminal chat does NOT run the agent harness. The daemon does. This is a hard constraint — there must never be two harness instances competing over the brain folder.

| Component | Responsibility |
|---|---|
| **Daemon** (`carson start`) | Runs the harness, tools, and local API server. Owns the brain folder. |
| **Chat client** (`carson chat`) | Connects to the daemon. Sends user messages. Renders streamed responses. Persists session logs locally. |
| **Log viewer** (`carson lookout`) | Connects to the daemon. Streams log lines in real time with color-coded levels. |

The chat client and log viewer are subcommands of the `carson` binary, not separate binaries.

## Transport: HTTP + SSE

### Why HTTP

The transport must work locally today and remotely tomorrow. HTTP is the only option that satisfies all three deployment modes without protocol changes:

| Mode | Address | What changes |
|---|---|---|
| **Local** | `http://127.0.0.1:7780` | Nothing — default |
| **Tailscale / WireGuard** | `http://100.x.y.z:7780` | Just the address |
| **Cloud relay** (future) | `https://relay.example.com/v1` | Address + TLS + auth header |

The daemon port is configured in `~/.config/carson/config.json`:

```json
{
  "daemon_port": 7780
}
```

### Why SSE for streaming

Server-Sent Events (SSE) over HTTP gives us streaming responses without WebSocket complexity. The chat sends a `POST`, the daemon responds with an SSE stream. Each event carries a chunk of the agent's response or a tool-call status update.

SSE is:
- One-directional (server → client), which is exactly what we need (the client sends a complete message, the server streams a response)
- HTTP-native — works through proxies, load balancers, and relay services
- Trivially debuggable with `curl`

### API Endpoints

#### Conversational (terminal chat)

##### `POST /chat`

Send a user message. Returns an SSE stream of the agent's response.

**Request:**

```json
{
  "message": "What's on my TODO list today?",
  "session_id": "sess_abc123"
}
```

**Response:** SSE stream with the following event types:

```
event: text
data: {"content": "Let me check your "}

event: text
data: {"content": "TODO list."}

event: tool_call
data: {"tool": "read_file", "id": "tc_1", "input": {"path": "TODO.md"}}

event: tool_result
data: {"tool": "read_file", "id": "tc_1", "status": "ok"}

event: text
data: {"content": "You have 3 active items..."}

event: done
data: {"stop_reason": "end_turn"}
```

Event types:

| Event | Purpose |
|---|---|
| `text` | Chunk of the agent's text response. Client appends to display. |
| `tool_call` | Agent is calling a tool. Client can display a status indicator. |
| `tool_result` | Tool finished. Includes `status` ("ok" or "error"). |
| `done` | Stream complete. Includes `stop_reason`. |
| `error` | Something went wrong. Includes `message`. |

The daemon holds separate message histories per `session_id`. Two concurrent `carson chat` instances get independent conversations — no shared state, no cross-session awareness. In practice this is almost never needed, but it's no harder to implement (a map of session ID → message history) and avoids arbitrary restrictions.

#### Fire-and-forget (mobile, scheduled events)

##### `POST /invoke`

Submit a prompt for async processing. Returns immediately with a 202 Accepted. No streaming, no response body beyond confirmation. Used by:
- Mobile voice drops
- Scheduled event execution (`carson run-scheduled`)
- Any non-interactive agent invocation

```json
// Request
{"prompt": "Retrieve transcript for Standup meeting", "context": {"meetingUrl": "..."}}

// Response: 202 Accepted
{"accepted": true, "invocation_id": "inv_xyz"}
```

#### Operational

##### `GET /health`

Health check. Returns 200 if the daemon is running and the harness is initialized.

```json
{"status": "ok", "provider": "anthropic", "model": "claude-sonnet-4-20250514"}
```

##### `GET /status`

Returns daemon status including brain path, provider, uptime.

```json
{
  "brain_dir": "/Users/liz/brain",
  "provider": "anthropic",
  "model": "claude-sonnet-4-20250514",
  "uptime_seconds": 3600
}
```

##### `GET /logs`

SSE stream of daemon log lines in real time. Used by `carson lookout`.

```
event: log
data: {"ts": "2026-03-07T14:22:01Z", "level": "info", "msg": "tool call", "tool": "read_file"}

event: log
data: {"ts": "2026-03-07T14:22:01Z", "level": "warn", "msg": "tool error", "error": "file not found"}
```

### Authentication

V0: None. The daemon binds to `127.0.0.1` only — not reachable from the network.

When Tailscale/WireGuard support is added, authentication will be required. Options (decided later):
- API key in header (`Authorization: Bearer ...`)
- mTLS (mutual TLS with client certificates)
- Tailscale's built-in identity (via `tsnet`)

The API should be designed so that adding an auth header later is a non-breaking change.

## Context Injection: `topofmind.md`

### The problem

The agent needs baseline context on every invocation — what's the user working on, what's urgent, what happened recently. This context could come from tool calls (`read_file("TODO.md")`), but tool calls are expensive: they consume reasoning steps, bloat the context window on every subsequent turn, and can fail.

### The solution

A single file — `topofmind.md` — that the agent writes and the daemon prepends to every harness invocation. It is the agent's own briefing note to its future self.

```
brain/
├── topofmind.md          ← agent-managed, always loaded
├── TODO.md               ← NOT auto-loaded (agent reads via tool when needed)
├── static/
├── .brain/
└── ...
```

### Properties

| Property | Detail |
|---|---|
| **Who writes it** | The agent, via the normal `write_file` tool |
| **Who reads it** | The daemon, automatically, before every harness invocation |
| **What it contains** | Whatever the agent decides is worth keeping top-of-mind — current priorities, active projects, upcoming deadlines, key context |
| **What it does NOT contain** | Full TODO lists, file contents, conversation history — those are accessible via tools |

### Why NOT `TODO.md`

`TODO.md` is a structured data file consumed by the frontend, scheduling system, and agent. It can grow large. Its format serves multiple consumers. Pinning it to every prompt would:
- Waste context on items that aren't relevant to the current conversation
- Create pressure to keep `TODO.md` small (which conflicts with its role as a comprehensive task list)
- Couple prompt engineering to task management format

`topofmind.md` is purpose-built for the prompt. The agent curates it based on what it actually references across sessions.

### Structural constraints (daemon-enforced)

The daemon validates `topofmind.md` on every write via the `write_file` tool. If the file violates constraints, the write is rejected and the agent receives an error explaining why.

| Constraint | Limit | Rationale |
|---|---|---|
| Max file size | 2 KB (~500 tokens) | Keeps context injection lightweight |
| Max lines | 30 | Forces conciseness |
| No fenced code blocks | — | This is a briefing note, not a code dump |
| Must be valid markdown | — | Parseable by the daemon and frontends |

These constraints are **deterministic and enforced by the daemon**, not by the LLM's judgment. The LLM controls the semantic content; the daemon controls the structural envelope.

### Lifecycle

1. `carson init` creates an empty `topofmind.md` in the brain folder.
2. Early on, the agent has little to write — the file stays sparse. This is fine.
3. Over time, the agent learns what it keeps needing and curates the file.
4. The agent can update `topofmind.md` at any point during a session.
5. If the agent writes something too long or structurally invalid, the write fails with a clear error, and the agent can retry with a corrected version.

### How it's loaded

The daemon reads `topofmind.md` from the brain folder and prepends it to the system prompt before every harness invocation:

```
[contents of topofmind.md]

[contents of system-prompt.md]

[user message]
```

If `topofmind.md` is empty or missing, nothing is prepended.

## Logging & Observability

### Log file

The daemon writes structured JSON logs to a persistent log file:

```
~/.config/carson/logs/carson.log
```

Log rotation: the daemon rotates the log file when it exceeds 10 MB, keeping the last 3 rotated files. Old log files are named `carson.log.1`, `carson.log.2`, etc.

The daemon continues to write human-readable logs to stderr (for `carson start` foreground mode), but the log file uses JSON format for machine parsing:

```json
{"ts":"2026-03-07T14:22:01Z","level":"info","msg":"tool call","tool":"read_file","path":"TODO.md"}
{"ts":"2026-03-07T14:22:01Z","level":"info","msg":"chat request","session":"sess_abc123"}
{"ts":"2026-03-07T14:22:02Z","level":"warn","msg":"write rejected","path":"topofmind.md","reason":"exceeds 30 lines"}
```

### `carson lookout`

A lightweight CLI subcommand that streams daemon logs in real time with color-coded levels.

```
$ carson lookout
14:22:01 INFO  tool call             tool=read_file path=TODO.md
14:22:01 INFO  chat request          session=sess_abc123
14:22:02 WARN  write rejected        path=topofmind.md reason="exceeds 30 lines"
14:22:05 ERROR LLM request failed    provider=anthropic status=429
```

Implementation: `carson lookout` connects to `GET /logs` on the daemon and renders the SSE stream with colored output. It's a read-only tail — no filtering or search in V0.

If the daemon isn't running, it falls back to tailing the log file directly (`~/.config/carson/logs/carson.log`).

## TUI Client: Bubble Tea

The chat client is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) using the Elm architecture.

### Design character (from FRONTEND.md)

> Minimal. Fast. No chrome. The terminal chat should feel like sitting across from someone competent and getting straight to the point. Keyboard-driven. Markdown-rendered output. Code blocks with syntax highlighting. No animations, no loading spinners longer than necessary — just a blinking cursor and then an answer.

### V0 Features

- Text input at the bottom of the screen
- Scrollable message history above
- Markdown rendering in agent responses (via [Glamour](https://github.com/charmbracelet/glamour))
- Tool call indicators (e.g., `[reading TODO.md...]`)
- Connection status indicator (connected / disconnected / reconnecting)

### V0 Non-Features

- No session resume or search
- No multi-line input (V0 — single line, enter to send)
- No file preview / image rendering
- No custom keybindings
- No themes

### Future: Custom Rendering Library

Bubble Tea is the starting point for fast iteration. The long-term plan is to replace it with a purpose-built rendering library that handles only what Carson needs. This is not V0 work — the Bubble Tea dependency is acceptable for now and will be deprecated when the custom library is ready.

## Session Persistence

Each `carson chat` invocation creates a new session. Sessions are logged to disk as JSONL for telemetry and debugging. No resume or search in V0.

### Session files

```
~/.config/carson/sessions/
├── 2026-03-07_14-22-01_sess_abc123.jsonl
├── 2026-03-07_15-10-44_sess_def456.jsonl
└── ...
```

Each line is a JSON object:

```json
{"ts": "2026-03-07T14:22:01Z", "type": "user", "content": "What's on my TODO?"}
{"ts": "2026-03-07T14:22:03Z", "type": "assistant", "content": "Let me check..."}
{"ts": "2026-03-07T14:22:03Z", "type": "tool_call", "tool": "read_file", "input": {"path": "TODO.md"}}
{"ts": "2026-03-07T14:22:04Z", "type": "tool_result", "tool": "read_file", "status": "ok"}
{"ts": "2026-03-07T14:22:05Z", "type": "assistant", "content": "You have 3 items..."}
```

### Retention

- Sessions are append-only during a conversation
- No compaction or resume in V0
- Retention policy TBD (30 days suggested in FRONTEND.md)

## Implementation Order

### Step 0: Logging Infrastructure

Before building the API, set up daemon logging so we can observe what's happening.

- `internal/logging/logger.go` — JSON file logger with rotation (10 MB, 3 files)
- `internal/logging/broadcast.go` — In-memory log broadcaster for SSE subscribers
- Update `internal/daemon/daemon.go` to write logs to both stderr and the log file
- `internal/api/handlers.go` — `GET /logs` SSE endpoint
- `cmd/carson/main.go` — add `lookout` subcommand (connects to `/logs` or tails log file)

### Step 1: Daemon API Server

Add an HTTP server to the daemon that starts alongside the existing signal handler.

- `internal/api/server.go` — HTTP server, binds to `127.0.0.1:{port}`
- `internal/api/handlers.go` — `/health`, `/status`, `/chat`, `/invoke`, `/logs` handlers
- `internal/api/sse.go` — SSE streaming helper
- Update `internal/daemon/daemon.go` to start the API server
- Update config for `daemon_port`

### Step 2: Streaming Harness

The harness currently returns a final string from `Run()`. For SSE, we need it to stream events as they happen.

- Add `RunStream(ctx, message, chan Event)` to the harness
- Events: `TextChunk`, `ToolCallStart`, `ToolCallResult`, `Done`, `Error`
- The existing `Run()` can wrap `RunStream()` for non-streaming use

### Step 3: `topofmind.md` Support

- Update `internal/brain/` — `topofmind.md` initialization, validation (size, line count, no code blocks)
- Update `write_file` tool handler to run `topofmind.md` validation on writes to that path
- Update harness to load and prepend `topofmind.md` before system prompt

### Step 4: Chat Client

Build the TUI client that connects to the daemon API.

- `internal/chat/client.go` — HTTP client, SSE reader
- `internal/chat/tui.go` — Bubble Tea model, view, update
- `internal/chat/session.go` — JSONL session logger
- `cmd/carson/main.go` — add `chat` subcommand

### Step 5: Connection Lifecycle

- `carson chat` checks daemon health on startup (`GET /health`)
- Clear error if daemon isn't running: "Carson daemon is not running. Start it with `carson start`."
- Graceful disconnect on Ctrl+C

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Transport protocol | HTTP + SSE | Works locally, over VPN, and through relay with zero protocol changes |
| Port | Configurable via `config.json`, default `7780` | Avoids conflicts, user can change |
| Context injection | `topofmind.md` — agent-written, daemon-enforced, always loaded | Saves context vs tool calls; agent manages relevance; daemon enforces structure |
| `TODO.md` in prompt | No — agent reads via tool when needed | Too large, wrong format, couples prompt to task management |
| Concurrent sessions | Yes, independent | Map of session → history; trivial to implement; avoids arbitrary restrictions |
| Non-chat endpoints | Separate (`/invoke` for fire-and-forget) | Different surfaces need different interaction patterns |
| Daemon logging | JSON log file + `GET /logs` SSE stream | Log file for post-mortem; SSE for live `carson lookout` |
| Log file location | `~/.config/carson/logs/` by default, overridable via `log_dir` in `config.json` | Logs are operational, not brain content |
| System prompt bootstrap | Starter system prompt references `topofmind.md` explicitly | Agent needs to know the file exists and how to use it |
| `carson lookout` filtering | Deferred — V0 is unfiltered tail | Later iteration: styled TUI for first-class daemon observation |
| Session persistence | New session per invocation, JSONL, no resume in V0 | Logging and telemetry only for now |

## Open Questions

None — all questions resolved. See Decisions table above.
