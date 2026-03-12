# Patterns & Conventions

This document captures the recurring patterns and conventions used across the Carson codebase. Update it as new patterns emerge.

## General Principles

- **Explicit over implicit** — Configuration is loaded from well-known sources (`config.json`, `.env`, env vars), never inferred from the environment. Tool definitions are declared statically, not discovered.
- **Minimal abstraction** — Prefer straightforward code over clever indirection. A few similar functions are better than a premature generic framework.
- **Platform parity** — Every feature must work on both macOS and Linux. Platform-specific code is isolated behind a thin adapter interface and selected at build time or startup.

## Project Structure

```
carson/
├── cmd/
│   └── carson/
│       └── main.go           # CLI entry point (init, start, chat, lookout, version, help)
├── internal/
│   ├── api/                  # HTTP API server, handlers, SSE streaming
│   ├── brain/                # Brain folder init, static/ enforcement, topofmind.md
│   ├── chat/                 # Terminal chat TUI client, HTTP+SSE client, session logger
│   ├── config/               # Layered config: config.json + .env + env vars
│   ├── daemon/               # Daemon lifecycle (start, signal handling, graceful shutdown)
│   ├── harness/              # Agent harness — tool registry, event types, LLM loop
│   │   └── tools/            # Dedicated tool implementations (read_file, write_file, bash)
│   ├── llm/                  # LLM provider interface + implementations (Anthropic, OpenAI, Gemini, Ollama)
│   ├── logging/              # JSON file logger with rotation, broadcast to SSE subscribers
│   ├── lookout/              # Log viewer — SSE stream or file tail with colored output
│   ├── service/              # OS service management (launchd/systemd), PID file utilities
│   └── session/              # Server-side session store for multi-turn conversations
├── .planning/                # Design documents and planning
│   ├── ARCHITECTURE.md
│   ├── BRAIN.md
│   ├── CHAT-MILESTONE.md
│   ├── CHANGES.md
│   ├── FRONTEND.md
│   ├── PATTERNS.md
│   ├── PROJECT.md
│   ├── QUESTIONS.md
│   ├── SCHEDULING.md
│   └── VERSIONING.md
├── go.mod
├── go.sum
└── .gitignore
```

## Error Handling

- Return errors rather than panicking. Reserve panics for truly unrecoverable states during startup.
- Wrap errors with context at each call-site so stack traces are human-readable.
- Log at the boundary (where the error is handled), not at every intermediate layer.

## Logging

- Use structured logging (key-value pairs, not interpolated strings).
- Levels: `debug` for development trace, `info` for operational events, `warn` for recoverable issues, `error` for failures requiring attention.
- The `CARSON_LOG_LEVEL` env var (or `log_level` in `config.json`) controls the threshold.

## Configuration

- Layered precedence: env vars (highest) > `.env` files (secrets) > `~/.config/carson/config.json` (preferences).
- `config.json` is created by `carson init` and stores non-secret preferences (brain path, provider, model, port, log level).
- `.env` files store API keys. Loaded from `~/.config/carson/.env` first, then CWD `.env` as override.
- Every config value has a sensible default where possible.
- Required values with no default (e.g., `brain_path`) cause a clear startup error listing what is missing.
- Default config values are embedded in the binary via `config.default.json`.

## Tool Definitions

- Each dedicated tool is a function that returns a `harness.ToolDef` — a bundle of an `llm.Tool` schema and a `ToolHandler` function.
- Tools are constructed with their dependencies (e.g., `brainDir`) via closure, not raw access to global state.
- Tool handlers return string results. The harness passes tool results as strings to the LLM.
- Tools are registered in a `Registry` at daemon startup. The registry provides schemas to the LLM and dispatches calls to handlers.
- The `bash` tool uses a blacklist approach for safety — a short list of known-dangerous patterns is blocked, everything else is allowed.

## Testing

- Unit tests live alongside source files.
- Integration tests that touch the filesystem or network go in a dedicated test directory and are gated behind a build tag / test flag.
- The watch directory used in tests is always a temp directory, never a real user path.

## Security

- File operations are sandboxed: any path resolved outside the brain directory is rejected before I/O.
- **`static/` enforcement:** Within the brain directory, any write targeting a path under `static/` is rejected by a path-prefix check in the tool handler (`brain.ValidateWritePath`). This is the only permission boundary — no ownership tracking, no manifest.
- **`bash` tool:** Shell access with a blacklist of dangerous commands (`rm -rf /`, `mkfs`, `dd if=`, fork bomb) and a 30-second timeout. The working directory is set to the brain folder.
- **`topofmind.md` enforcement:** Writes to `topofmind.md` are validated by the daemon: max 2 KB, max 30 lines, no fenced code blocks. Validation happens in the `write_file` tool handler.
- External API credentials are stored in `.env` files and resolved per-provider. Never logged, even at `debug` level.
- The agent's tool schema is frozen at startup. Runtime tool registration is not supported.
- The API server binds to `127.0.0.1` only — not network-accessible without a VPN/tunnel.

## Brain File Conventions

- **`topofmind.md`** — agent-managed context file at the brain root. Loaded by the daemon and combined with the system prompt on every harness invocation. Daemon-enforced constraints: max 2 KB, max 30 lines, no fenced code blocks.
- **Sidecar metadata** lives in `.meta/`, mirroring the brain's directory structure. One `.meta.json` per source file (e.g., `.meta/static/photos/vacation.jpg.meta.json`). Sidecars include `source_hash` for detecting renames/moves.
- **TODO.md** uses inline backtick-wrapped key-value pairs for machine-parseable metadata: `` `origin:...` `due:...` `priority:...` ``. Sections: `Active`, `Someday`, `Done (last 7 days)`.
- **Backup log** (planned) uses JSON lines (`.jsonl`) — one JSON object per line, append-only. No wrapping array.
- **Chat session history** uses JSON lines (`.jsonl`). Each session is a separate file in `~/.config/carson/sessions/`, named `{date}_{time}_{session_id}.jsonl`. Weekly compaction and 30-day purge planned but not yet implemented.
