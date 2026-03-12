# Project Overview

## Vision

Carson is a personal, locally-run agent service. It sits between an LLM provider and the user's machine, giving the model a carefully scoped set of capabilities while the user retains full visibility and control.

The service is designed to be the backend half of a two-repo system: Carson handles inference, scheduling, file management, and external service calls; a separate frontend repo provides the UI the agent can drive.

## Goals

1. **Cross-platform daemon** — Runs as a foreground process on macOS and Linux with a single Go codebase. Background service management planned. No Windows support required at this stage.
2. **Brain folder** — A directory with a `static/` subdirectory that is read-only for the agent. The agent can freely read/write everything else. `topofmind.md` for context injection. Sidecar metadata in `.meta/`, unified task tracking in `TODO.md`. See [BRAIN.md](BRAIN.md).
3. **Recursive scheduling** — The agent can schedule future invocations of itself via `schedule_event`. Each callback re-enters the harness and can schedule more callbacks. Safety-limited by chain depth, pending event caps, and minimum delay. See [SCHEDULING.md](SCHEDULING.md).
4. **Constrained agent harness** — The LLM's tool surface is explicitly defined via a Registry. The agent can only call registered tools. Streaming and blocking execution modes.
5. **Multi-provider LLM support** — Anthropic, OpenAI, Gemini, and Ollama behind a unified Provider interface.
6. **Tool model** — Agent capabilities provided through dedicated tools (file I/O, bash) implemented natively. Skills protocol planned for higher-level composable actions.
7. **Versioning & backup** — Jujutsu for continuous change tracking, nightly compressed snapshots for disaster recovery. The agent inspects backup logs but never executes backups or rollbacks. See [VERSIONING.md](VERSIONING.md).
8. **External service integration** — Authenticated outbound calls to third-party APIs (GitHub, Slack, custom endpoints, etc.) mediated through tool definitions.
9. **Three frontend surfaces** — Desktop app (Tauri), terminal chat (`carson chat`), and iOS mobile app. All share the brain folder as source of truth. See [FRONTEND.md](FRONTEND.md).

## Non-Goals (for now)

- Multi-user / multi-tenant operation
- Cloud-hosted deployment
- Training or fine-tuning models locally
- Android support

## Milestones

| # | Milestone | Status | Description |
|---|---|---|---|
| 1 | Skeleton | **Done** | Repo scaffolding, config loading, basic daemon lifecycle, `carson init`. |
| 2 | Brain folder | **Partial** | Brain init, `static/` permission gate, `topofmind.md`. Folder watcher not yet built. |
| 3 | LLM Bridge | **Done** | Multi-provider support (Anthropic, OpenAI, Gemini, Ollama), streaming. |
| 4 | Tool Harness | **Partial** | Tool registry, `read_file`, `write_file`, `bash` tools. Multi-turn conversation context. Skills not yet built. |
| 5 | Logging & Observability | **Done** | JSON log file with rotation, SSE broadcast, `carson lookout`. |
| 6 | Terminal Chat | **Done** | `carson chat` TUI, SSE streaming, JSONL session persistence, multi-turn sessions. |
| 6.5 | Daemon Management | **Done** | Background daemon via launchd/systemd. `carson start/stop/restart/status`. PID file tracking. |
| 7 | Scheduling | Not started | `schedule_event` tool, prompt bundle format, cron manager, chain tracking. |
| 8 | Versioning | Not started | Jujutsu repo init, `carson rollback` CLI, nightly backup, backup-inspect. |
| 9 | External Services | Not started | Authenticated outbound integrations behind tool definitions. |
| 10 | Desktop App | Not started | Tauri-based desktop app — task board, brain browser, notifications. |
| 11 | Mobile App | Not started | iOS app — task checkbox, voice drop, context cards. Sync via Tailscale/WireGuard. |
