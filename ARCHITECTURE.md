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
│  │  │  - schedule_event │   │                           │  │  │
│  │  │  - cancel/list    │   │                           │  │  │
│  │  └───────────────────┘   └───────────────────────────┘  │  │
│  └────────────────────────┬────────────────────────────────┘  │
│                           │                                   │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │              Versioning & Backup Layer                   │  │
│  │  Jujutsu (continuous change tracking) + Nightly backup  │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                              │
└──────────────────────────┬───────────────────────────────────┘
                           │
             ┌─────────────┼─────────────┐
             ▼             ▼             ▼
       ┌──────────┐ ┌───────────┐ ┌────────────┐
       │  LLM     │ │ External  │ │ Frontend   │
       │ Provider │ │ Services  │ │ (Tauri     │
       │ (API)    │ │ (GitHub,  │ │  desktop,  │
       │          │ │  Slack..) │ │  terminal, │
       │          │ │           │ │  iOS app)  │
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

### 2a. Brain Folder

The watch directory is structured as the agent's "brain." Key paths:

| Path | Agent Access | Purpose |
|---|---|---|
| `static/` | **Read-only** | User-protected files. Write/edit/delete/move blocked by tool handlers. |
| `.brain/` | Read/Write | Agent internals — indices, caches. |
| `.meta/` | Read/Write | Sidecar metadata files that annotate brain artifacts. |
| `TODO.md` | Read/Write | Unified task file — read by agent, frontend, and scheduling system. |
| `daily-summary/` | Read/Write | Agent-generated periodic summaries. |
| Everything else | Read/Write | Mutable space for both human and agent files. |

See [BRAIN.md](BRAIN.md) for the full design.

### 3. Cron Manager

Carson owns a namespaced section of the user's crontab. Entries point back to the service binary with subcommand flags (e.g., `carson run-scheduled --task poll-github`). The agent can create, update, and remove these entries through a dedicated tool.

Protected `SYSTEM:` entries (nightly backup, morning inspection) cannot be modified by the agent. See [SCHEDULING.md](SCHEDULING.md) for the recursive scheduling design and [VERSIONING.md](VERSIONING.md) for backup automation.

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
- `read_file` / `write_file` — scoped to the watch directory. Writes to `static/` are blocked at the handler level.
- `search_files` — glob and grep within the watch directory.
- `frontend_command` — emit a native OS push notification (text only) to the desktop app.
- `schedule_event` — schedule a future agent invocation. Creates a prompt bundle on disk and a crontab entry. See [SCHEDULING.md](SCHEDULING.md).
- `list_scheduled_events` / `cancel_scheduled_event` — inspect and manage pending scheduled events.

### 7. Skills

Follow the standard skills protocol. Used for higher-level, composable actions and external service integrations where latency is less critical.

## Configuration

All runtime config is loaded from environment variables (`.env` file supported). See `.env.example` for the full list.

## Versioning & Backup

The brain folder is a Jujutsu repository. Jujutsu continuously tracks working-copy changes (no explicit commits needed during the day), eliminating the risk of data loss from crashes. A nightly cron job creates a formal checkpoint commit, then snapshots the brain to compressed backup storage. The agent is not involved in backup execution — it only reads the backup log the next morning and posts status to `TODO.md`. See [VERSIONING.md](VERSIONING.md) for the full design.

## Security Boundaries

- The agent **cannot** execute arbitrary shell commands.
- File I/O is sandboxed to the configured watch directory. Within the watch directory, `static/` is read-only — write/edit/delete/move operations are rejected by a path-prefix check in each tool handler.
- External service calls require explicit credentials in config.
- The tool schema is statically defined — the agent cannot register new tools at runtime.
- System-level cron entries (`SYSTEM:` prefix) are protected from agent modification.
- The agent cannot trigger rollbacks or restores. Versioning commands (`carson rollback`, `carson restore`) are user-facing CLI only.
