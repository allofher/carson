# Carson

A cross-platform (macOS & Linux) background service that bridges a local filesystem workspace with upstream LLM inference through a managed, tool-scoped agent harness.

## What It Does

Carson watches a designated folder on your machine, polls external services on configurable intervals, and manages its own crontab to schedule recurring tasks. At its core it maintains a persistent connection to an LLM provider and exposes a constrained set of "write" tools to the agent — giving the model controlled ability to:

- Edit files in the watched folder
- Drive a separately-hosted frontend via structured commands
- Call out to authenticated external services

## Key Concepts

| Concept | Description |
|---|---|
| **Watch Folder** | A user-designated directory Carson monitors for changes and where the agent can read/write files. |
| **Agent Harness** | The constrained tool surface exposed to the LLM — effectively the "API" of the installation. |
| **Cron Self-Scheduling** | Carson manages a crontab that calls back into itself on set schedules for polling and maintenance tasks. |
| **Skills + Tools** | Agent capabilities are split between *skills* (using the standard skills protocol) and *dedicated tools* (fast local ops like file reads and shell search). |

## Project Layout

See [PROJECT.md](PROJECT.md) for goals and roadmap, [ARCHITECTURE.md](ARCHITECTURE.md) for system design, and [PATTERNS.md](PATTERNS.md) for conventions used across the codebase.

## Getting Started

```bash
cp .env.example .env
# Fill in your LLM provider credentials and config, then:
# (build & run instructions TBD)
```

## License

TBD
