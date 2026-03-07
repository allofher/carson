# Patterns & Conventions

This document captures the recurring patterns and conventions used across the Carson codebase. Update it as new patterns emerge.

## General Principles

- **Explicit over implicit** — Configuration is loaded from well-known env vars, never inferred from the environment. Tool definitions are declared statically, not discovered.
- **Minimal abstraction** — Prefer straightforward code over clever indirection. A few similar functions are better than a premature generic framework.
- **Platform parity** — Every feature must work on both macOS and Linux. Platform-specific code is isolated behind a thin adapter interface and selected at build time or startup.

## Project Structure (planned)

```
carson/
├── cmd/                  # CLI entry points (start, stop, status, rollback, restore, backup, chat)
├── internal/
│   ├── config/           # Env-based configuration loading
│   ├── daemon/           # Daemon lifecycle (start, stop, signal handling)
│   ├── watcher/          # Folder watcher (FSEvents / inotify adapters)
│   ├── brain/            # Brain folder initialization, static/ enforcement, .meta/ management
│   ├── cron/             # Crontab read/write/namespace management
│   ├── scheduling/       # ScheduledPrompt bundles, chain tracking, safety limits
│   ├── versioning/       # Jujutsu wrapper, rollback logic
│   ├── backup/           # Nightly snapshot, retention policy, backup log
│   ├── router/           # Event routing and dispatch
│   ├── harness/          # Agent harness — tool schema, context, LLM bridge
│   │   ├── tools/        # Dedicated tool implementations
│   │   └── skills/       # Skill protocol dispatch
│   ├── chat/             # Terminal chat client (carson chat)
│   └── platform/         # OS-specific adapters (launchd, systemd, fs events)
├── .env.example
├── .gitignore
├── ARCHITECTURE.md
├── BRAIN.md
├── FRONTEND.md
├── PATTERNS.md
├── PROJECT.md
├── QUESTIONS.md
├── SCHEDULING.md
├── VERSIONING.md
└── README.md
```

## Error Handling

- Return errors rather than panicking. Reserve panics for truly unrecoverable states during startup.
- Wrap errors with context at each call-site so stack traces are human-readable.
- Log at the boundary (where the error is handled), not at every intermediate layer.

## Logging

- Use structured logging (key-value pairs, not interpolated strings).
- Levels: `debug` for development trace, `info` for operational events, `warn` for recoverable issues, `error` for failures requiring attention.
- The `LOG_LEVEL` env var controls the threshold.

## Configuration

- Single source of truth: environment variables, optionally loaded from `.env`.
- Every config value has a sensible default where possible.
- Required values with no default cause a clear startup error listing what is missing.

## Tool Definitions

- Each dedicated tool is a self-contained module that exports a schema (name, description, parameters) and a handler function.
- Tools receive a scoped context object — never raw access to global state.
- Tool handlers return structured results (not free-form text) so the harness can inspect outcomes before passing them to the LLM.

## Skill Definitions

- Skills follow the standard skills protocol and are registered in a manifest.
- A skill can compose multiple dedicated tools in a single action.
- Skills are the preferred extension point for new agent capabilities.

## Testing

- Unit tests live alongside source files.
- Integration tests that touch the filesystem or network go in a dedicated test directory and are gated behind a build tag / test flag.
- The watch directory used in tests is always a temp directory, never a real user path.

## Security

- File operations are sandboxed: any path resolved outside the configured watch directory is rejected before I/O.
- **`static/` enforcement:** Within the watch directory, any write/edit/delete/move targeting a path under `static/` is rejected by a path-prefix check in the tool handler. This is the only permission boundary — no ownership tracking, no manifest.
- External API credentials are never logged, even at `debug` level.
- The agent's tool schema is frozen at startup. Runtime tool registration is not supported.
- System cron entries (`SYSTEM:` prefix) are protected from agent modification via `schedule_event` / `cancel_scheduled_event`.
- The agent cannot trigger rollbacks or restores. `carson rollback` and `carson restore` are user-facing CLI commands only.

## Brain File Conventions

- **Sidecar metadata** lives in `.meta/`, mirroring the brain's directory structure. One `.meta.json` per source file (e.g., `.meta/static/photos/vacation.jpg.meta.json`). Sidecars include `source_hash` for detecting renames/moves.
- **TODO.md** uses inline backtick-wrapped key-value pairs for machine-parseable metadata: `` `origin:...` `due:...` `priority:...` ``. Sections: `Active`, `Someday`, `Done (last 7 days)`.
- **Backup log** uses JSON lines (`.jsonl`) — one JSON object per line, append-only. No wrapping array.
- **Chat session history** uses JSON lines (`.jsonl`) with weekly compaction and 30-day purge of historical files.
