# Project Overview

## Vision

Carson is a personal, locally-run agent service. It sits between an LLM provider and the user's machine, giving the model a carefully scoped set of capabilities while the user retains full visibility and control.

The service is designed to be the backend half of a two-repo system: Carson handles inference, scheduling, file management, and external service calls; a separate frontend repo provides the UI the agent can drive.

## Goals

1. **Cross-platform daemon** — Runs as a background service on macOS and Linux with a single codebase. No Windows support required at this stage.
2. **Folder watcher** — Monitors a user-designated directory for file changes and surfaces them to the agent context.
3. **Self-scheduling via cron** — Manages its own crontab entries to trigger polling, maintenance, and user-defined recurring tasks.
4. **Constrained agent harness** — The LLM's tool surface is explicitly defined and limited. The agent cannot escape the harness.
5. **Dual tool model** — Agent capabilities are provided through:
   - **Skills** — Higher-level, composable actions using the standard skills protocol.
   - **Dedicated tools** — Fast, local operations (file I/O, search, shell commands) implemented natively for performance.
6. **External service integration** — Authenticated outbound calls to third-party APIs (GitHub, Slack, custom endpoints, etc.) mediated through tool definitions.
7. **Frontend control surface** — A set of write tools that emit structured commands consumed by the companion frontend repo.

## Non-Goals (for now)

- Multi-user / multi-tenant operation
- Cloud-hosted deployment
- Mobile clients
- Training or fine-tuning models locally

## Milestones

| # | Milestone | Description |
|---|---|---|
| 1 | Skeleton | Repo scaffolding, config loading, basic daemon lifecycle (start / stop / status). |
| 2 | Watcher | File-system watcher on the designated folder with debounced change events. |
| 3 | LLM Bridge | Connect to upstream provider, send/receive messages, handle streaming. |
| 4 | Tool Harness | Define the agent tool schema, implement dedicated tools, wire up skill dispatch. |
| 5 | Cron Manager | Read/write crontab entries that call back into the service binary. |
| 6 | External Services | Add authenticated outbound integrations behind tool definitions. |
| 7 | Frontend Protocol | Define the structured command format the frontend repo will consume. |
