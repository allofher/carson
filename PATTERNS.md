# Patterns & Conventions

This document captures the recurring patterns and conventions used across the Carson codebase. Update it as new patterns emerge.

## General Principles

- **Explicit over implicit** — Configuration is loaded from well-known env vars, never inferred from the environment. Tool definitions are declared statically, not discovered.
- **Minimal abstraction** — Prefer straightforward code over clever indirection. A few similar functions are better than a premature generic framework.
- **Platform parity** — Every feature must work on both macOS and Linux. Platform-specific code is isolated behind a thin adapter interface and selected at build time or startup.

## Project Structure (planned)

```
carson/
├── cmd/                  # CLI entry points (start, stop, status, run-scheduled)
├── internal/
│   ├── config/           # Env-based configuration loading
│   ├── daemon/           # Daemon lifecycle (start, stop, signal handling)
│   ├── watcher/          # Folder watcher (FSEvents / inotify adapters)
│   ├── cron/             # Crontab read/write/namespace management
│   ├── router/           # Event routing and dispatch
│   ├── harness/          # Agent harness — tool schema, context, LLM bridge
│   │   ├── tools/        # Dedicated tool implementations
│   │   └── skills/       # Skill protocol dispatch
│   └── platform/         # OS-specific adapters (launchd, systemd, fs events)
├── .env.example
├── .gitignore
├── ARCHITECTURE.md
├── PATTERNS.md
├── PROJECT.md
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
- External API credentials are never logged, even at `debug` level.
- The agent's tool schema is frozen at startup. Runtime tool registration is not supported.
