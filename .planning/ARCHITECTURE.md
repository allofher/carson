# Architecture

## System Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                        Carson Daemon                         │
│                                                              │
│  ┌──────────────────┐   ┌──────────────────────────────┐    │
│  │  HTTP API Server  │   │  Logging (JSON file + SSE)   │    │
│  │  127.0.0.1:7780   │   │  RotatingWriter + Broadcast  │    │
│  └────────┬─────────┘   └──────────────────────────────┘    │
│           │                                                  │
│           ▼                                                  │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                   Agent Harness                         │ │
│  │  - LLM Provider interface (Anthropic/OpenAI/Gemini/    │ │
│  │    Ollama)                                              │ │
│  │  - Tool Registry                                        │ │
│  │  - topofmind.md context injection                       │ │
│  │  - Streaming event loop (RunStream)                     │ │
│  │                                                         │ │
│  │  ┌───────────────────┐                                  │ │
│  │  │  Dedicated Tools  │                                  │ │
│  │  │  - read_file      │                                  │ │
│  │  │  - write_file     │                                  │ │
│  │  │  - bash           │                                  │ │
│  │  └───────────────────┘                                  │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                   Brain Folder                          │ │
│  │  .brain/ .meta/ static/ daily-summary/ topofmind.md     │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                              │
└──────────────────────────┬───────────────────────────────────┘
                           │
             ┌─────────────┼─────────────┐
             ▼             ▼             ▼
       ┌──────────┐ ┌───────────┐ ┌────────────┐
       │  LLM     │ │  Terminal  │ │  Lookout   │
       │ Provider │ │  Chat TUI  │ │  Log Tail  │
       │ (API)    │ │  (client)  │ │  (client)  │
       └──────────┘ └───────────┘ └────────────┘
```

### Planned (not yet implemented)

The following components are designed but not yet built:
- **Folder Watcher** — OS-level file-system events for detecting brain folder changes
- **Cron Manager** — Namespaced crontab management for recursive scheduling
- **Event Router** — Central dispatch for watcher, cron, and HTTP events
- **Skills** — Higher-level composable actions via a skills protocol
- **Versioning & Backup Layer** — Jujutsu + nightly compressed snapshots

## Core Components

### 1. Daemon Lifecycle

The daemon is managed as an OS service. `carson start` daemonizes by default via `launchd` (macOS) or `systemd` (Linux). `carson start --foreground` runs in the current process for development.

CLI commands: `carson start`, `carson stop`, `carson restart`, `carson status`.

The daemon writes its PID to `~/.config/carson/carson.pid` on startup and removes it on shutdown. Service files are installed to standard OS locations (`~/Library/LaunchAgents/com.carson.agent.plist` on macOS, `~/.config/systemd/user/carson.service` on Linux).

### 2. HTTP API Server

The daemon runs an HTTP server bound to `127.0.0.1:{port}` (default 7780). Endpoints:

| Endpoint | Method | Purpose |
|---|---|---|
| `/health` | GET | Health check — returns provider and model info |
| `/status` | GET | Daemon status — brain dir, provider, uptime |
| `/chat` | POST | Conversational chat — returns SSE stream of agent response |
| `/invoke` | POST | Fire-and-forget prompt — returns 202 Accepted |
| `/logs` | GET | Real-time log stream via SSE |

### 3. Brain Folder

The watch directory is structured as the agent's "brain." Key paths:

| Path | Agent Access | Purpose |
|---|---|---|
| `static/` | **Read-only** | User-protected files. Write/edit/delete/move blocked by tool handlers. |
| `.brain/` | Read/Write | Agent internals — indices, caches. |
| `.meta/` | Read/Write | Sidecar metadata files that annotate brain artifacts. |
| `TODO.md` | Read/Write | Unified task file — read by agent, frontend, and scheduling system. |
| `topofmind.md` | Read/Write | Agent-managed context file — prepended to every harness invocation. Daemon-enforced constraints (2 KB, 30 lines, no code blocks). |
| `daily-summary/` | Read/Write | Agent-generated periodic summaries. |
| Everything else | Read/Write | Mutable space for both human and agent files. |

See [BRAIN.md](BRAIN.md) for the full design.

### 4. Agent Harness

The core abstraction. It:
- Maintains the conversation context with the LLM provider.
- Loads `topofmind.md` and the system prompt, combining them with the user message.
- Exposes a fixed tool schema via a Registry — the agent can only call registered tools.
- Executes tool calls through registered handlers and loops until the model finishes or the iteration limit (default 50) is reached.
- Supports both blocking (`Run`) and streaming (`RunStream`) modes.
- Enforces scope constraints (file operations sandboxed to brain directory, `static/` read-only, `topofmind.md` validation).

### 5. LLM Providers

The harness communicates with LLMs through a `Provider` interface. Four providers are implemented:
- **Anthropic** — Claude models
- **OpenAI** — GPT models
- **Gemini** — Google models
- **Ollama** — Local models

Provider selection and API keys are configured via `config.json` and `.env`.

### 6. Dedicated Tools

Currently implemented:
- `read_file` — reads files within the brain folder. Sandboxed to brain directory.
- `write_file` — creates/overwrites files in the brain folder. Writes to `static/` are blocked. Writes to `topofmind.md` are validated against structural constraints (2 KB, 30 lines, no code blocks).
- `bash` — runs shell commands with a blacklist of dangerous patterns (`rm -rf /`, `mkfs`, `dd if=`, fork bomb) and a 30-second timeout.

**Planned tools** (not yet implemented):
- `search_files` — glob and grep within the watch directory.
- `frontend_command` — emit a native OS push notification.
- `schedule_event` / `list_scheduled_events` / `cancel_scheduled_event` — recursive scheduling. See [SCHEDULING.md](SCHEDULING.md).

### 7. Skills (planned)

Not yet implemented. Will follow a standard skills protocol for higher-level, composable actions and external service integrations.

### 8. Logging & Observability

The daemon writes structured logs to both stderr (text format) and a JSON log file (`~/.config/carson/logs/carson.log`). The log file rotates at 10 MB, keeping the last 3 rotated files. A `Broadcaster` fans out JSON log lines to SSE subscribers in real time.

`carson lookout` streams logs from the daemon via SSE, falling back to tailing the log file if the daemon is unreachable.

## Configuration

Configuration uses a layered approach with the following precedence (highest first):
1. **Environment variables** — `CARSON_BRAIN_DIR`, `CARSON_LOG_LEVEL`, `CARSON_DAEMON_PORT`, etc.
2. **`.env` file** — loaded from `~/.config/carson/.env` and the current working directory. Secrets only (API keys).
3. **`~/.config/carson/config.json`** — preferences (brain path, provider, model, port, log level, system prompt path).

The `carson init <path>` command creates the initial config file, `.env` template, and system prompt file.

## Versioning & Backup (planned)

Not yet implemented. The brain folder will become a Jujutsu repository for continuous change tracking. Nightly compressed snapshots for disaster recovery. See [VERSIONING.md](VERSIONING.md) for the full design.

## Security Boundaries

- The `bash` tool provides shell access with a blacklist of dangerous commands and a 30-second timeout. The philosophy is trust with a small blacklist, not distrust with a whitelist.
- File I/O is sandboxed to the configured brain directory. Within the brain directory, `static/` is read-only — write operations are rejected by a path-prefix check in the tool handler.
- `topofmind.md` writes are validated by the daemon: max 2 KB, max 30 lines, no fenced code blocks.
- API keys are stored in `.env` files, never in `config.json`.
- The tool schema is statically defined at startup — the agent cannot register new tools at runtime.
- The API server binds to `127.0.0.1` only — not reachable from the network without a VPN/tunnel.
