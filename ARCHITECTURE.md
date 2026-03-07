# Architecture

## System Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                        Carson Service                        │
│                                                              │
│  ┌─────────────┐   ┌──────────────┐   ┌──────────────────┐  │
│  │  Folder      │   │  Cron        │   │  Service         │  │
│  │  Watcher     │   │  Manager     │   │  Health/Status   │  │
│  └──────┬───────┘   └──────┬───────┘   └──────────────────┘  │
│         │                  │                                  │
│         ▼                  ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                   Event Router                          │  │
│  └────────────────────────┬────────────────────────────────┘  │
│                           │                                   │
│                           ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                   Agent Harness                         │  │
│  │                                                         │  │
│  │  ┌───────────────────┐   ┌───────────────────────────┐  │  │
│  │  │  Dedicated Tools  │   │  Skills (Skills Protocol) │  │  │
│  │  │  - file read/write│   │  - composable actions     │  │  │
│  │  │  - search/grep    │   │  - external service calls │  │  │
│  │  │  - frontend cmds  │   │  - multi-step workflows   │  │  │
│  │  └───────────────────┘   └───────────────────────────┘  │  │
│  └────────────────────────┬────────────────────────────────┘  │
│                           │                                   │
└───────────────────────────┼───────────────────────────────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
        ┌──────────┐ ┌───────────┐ ┌────────────┐
        │  LLM     │ │ External  │ │ Frontend   │
        │ Provider │ │ Services  │ │ (other     │
        │ (API)    │ │ (GitHub,  │ │  repo)     │
        │          │ │  Slack..) │ │            │
        └──────────┘ └───────────┘ └────────────┘
```

## Core Components

### 1. Daemon Lifecycle

The service runs as a background process managed by the OS-native mechanism:
- **macOS** — `launchd` plist
- **Linux** — `systemd` unit file

A small CLI wraps install/uninstall/start/stop/status commands so the user never hand-edits service definitions.

### 2. Folder Watcher

Uses OS-level file-system events (`FSEvents` on macOS, `inotify` on Linux) to detect changes in the watch directory. Events are debounced and batched before being forwarded to the Event Router.

### 3. Cron Manager

Carson owns a namespaced section of the user's crontab. Entries point back to the service binary with subcommand flags (e.g., `carson run-scheduled --task poll-github`). The agent can create, update, and remove these entries through a dedicated tool.

### 4. Event Router

Central dispatch that receives events from the Folder Watcher, Cron Manager, and inbound HTTP requests, then decides whether they should be forwarded to the Agent Harness for LLM processing or handled locally.

### 5. Agent Harness

The core abstraction. It:
- Maintains the conversation context with the upstream LLM.
- Exposes a fixed tool schema — the agent can only call tools defined here.
- Routes tool calls to either a **dedicated tool** (local, fast) or a **skill** (protocol-based, potentially multi-step).
- Enforces scope constraints (e.g., file operations are sandboxed to the watch directory).

### 6. Dedicated Tools

Natively implemented for speed. Examples:
- `read_file` / `write_file` — scoped to the watch directory.
- `search_files` — glob and grep within the watch directory.
- `frontend_command` — emit a structured JSON command for the companion frontend.

### 7. Skills

Follow the standard skills protocol. Used for higher-level, composable actions and external service integrations where latency is less critical.

## Configuration

All runtime config is loaded from environment variables (`.env` file supported). See `.env.example` for the full list.

## Security Boundaries

- The agent **cannot** execute arbitrary shell commands.
- File I/O is sandboxed to the configured watch directory.
- External service calls require explicit credentials in config.
- The tool schema is statically defined — the agent cannot register new tools at runtime.
