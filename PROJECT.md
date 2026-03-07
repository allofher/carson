# Project Overview

## Vision

Carson is a personal, locally-run agent service. It sits between an LLM provider and the user's machine, giving the model a carefully scoped set of capabilities while the user retains full visibility and control.

The service is designed to be the backend half of a two-repo system: Carson handles inference, scheduling, file management, and external service calls; a separate frontend repo provides the UI the agent can drive.

## Goals

1. **Cross-platform daemon** — Runs as a background service on macOS and Linux with a single codebase. No Windows support required at this stage.
2. **Brain folder** — A watched directory with a `static/` subdirectory that is read-only for the agent. The agent can freely read/write everything else. Sidecar metadata in `.meta/`, unified task tracking in `TODO.md`. See [BRAIN.md](BRAIN.md).
3. **Recursive scheduling** — The agent can schedule future invocations of itself via `schedule_event`. Each callback re-enters the harness and can schedule more callbacks. Safety-limited by chain depth, pending event caps, and minimum delay. See [SCHEDULING.md](SCHEDULING.md).
4. **Constrained agent harness** — The LLM's tool surface is explicitly defined and limited. The agent cannot escape the harness.
5. **Dual tool model** — Agent capabilities are provided through:
   - **Skills** — Higher-level, composable actions using the standard skills protocol.
   - **Dedicated tools** — Fast, local operations (file I/O, search, scheduling) implemented natively for performance.
6. **Versioning & backup** — Jujutsu for continuous change tracking, nightly compressed snapshots for disaster recovery. The agent inspects backup logs but never executes backups or rollbacks. See [VERSIONING.md](VERSIONING.md).
7. **External service integration** — Authenticated outbound calls to third-party APIs (GitHub, Slack, custom endpoints, etc.) mediated through tool definitions.
8. **Three frontend surfaces** — Desktop app (Tauri), terminal chat (`carson chat`), and iOS mobile app. All share the brain folder as source of truth. See [FRONTEND.md](FRONTEND.md).

## Non-Goals (for now)

- Multi-user / multi-tenant operation
- Cloud-hosted deployment
- Training or fine-tuning models locally
- Android support

## Milestones

| # | Milestone | Description |
|---|---|---|
| 1 | Skeleton | Repo scaffolding, config loading, basic daemon lifecycle (start / stop / status). |
| 2 | Brain folder | Watch directory setup, `static/` permission gate in tool handlers, `.brain/` / `.meta/` initialization. |
| 3 | LLM Bridge | Connect to upstream provider, send/receive messages, handle streaming. |
| 4 | Tool Harness | Define the agent tool schema, implement dedicated tools (including `static/` enforcement), wire up skill dispatch. |
| 5 | Scheduling | `schedule_event` tool, prompt bundle format, cron manager, chain tracking, safety limits. |
| 6 | Versioning | Jujutsu repo initialization, `carson rollback` CLI, nightly backup script, backup-inspect scheduled task. |
| 7 | External Services | Add authenticated outbound integrations behind tool definitions. |
| 8 | Desktop App | Tauri-based desktop app — task board, brain browser, notifications via file watching + local socket. |
| 9 | Terminal Chat | `carson chat` subcommand — TUI chat client connecting to daemon over local transport. JSONL session persistence. |
| 10 | Mobile App | iOS app — task checkbox, voice drop, context cards. Sync via Tailscale/WireGuard. |
