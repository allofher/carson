# Carson

A cross-platform (macOS & Linux) background service that bridges a local filesystem workspace with upstream LLM inference through a managed, tool-scoped agent harness.

## What It Does

Carson watches a designated "brain" folder on your machine, polls external services on configurable intervals, and manages its own crontab to schedule recursive agent invocations. At its core it maintains a connection to an LLM provider and exposes a constrained set of tools to the agent — giving the model controlled ability to:

- Read and write files in the brain folder (with `static/` as a read-only zone)
- Schedule future invocations of itself via cron
- Annotate files with sidecar metadata
- Manage a unified TODO list
- Push native notifications to the desktop app
- Call out to authenticated external services

## Key Concepts

| Concept | Description |
|---|---|
| **Brain Folder** | The watched directory. Contains a `static/` subdirectory that is read-only for the agent — everything else is mutable. Sidecar metadata in `.meta/`, agent internals in `.brain/`. See [BRAIN.md](BRAIN.md). |
| **`static/` Permission Gate** | The only permission boundary. A path-prefix check in each tool handler blocks writes to `static/`. No ownership tracking, no manifest. |
| **Recursive Scheduling** | The agent can call `schedule_event` to create future invocations. Each callback re-enters the harness and can schedule more callbacks. Chain depth and pending event limits prevent runaway recursion. See [SCHEDULING.md](SCHEDULING.md). |
| **Agent Harness** | The constrained tool surface exposed to the LLM — the "API" of the installation. Tools are statically defined at startup. |
| **Skills + Tools** | Agent capabilities split between *skills* (composable, protocol-based) and *dedicated tools* (fast local ops — file I/O, search, scheduling). |
| **Jujutsu Versioning** | The brain is a [Jujutsu](https://github.com/martinvonz/jj) repo for continuous change tracking. Nightly compressed snapshots for disaster recovery. See [VERSIONING.md](VERSIONING.md). |
| **Three Frontend Surfaces** | Desktop app (Tauri), terminal chat (`carson chat`), iOS mobile app. All share the brain folder as source of truth. See [FRONTEND.md](FRONTEND.md). |

## Dependencies

- **Jujutsu (`jj`)** — Used for continuous versioning of the brain folder. Must be installed on the host.
- **LLM provider credentials** — Configured via environment variables.

## Project Layout

| Document | Contents |
|---|---|
| [PROJECT.md](PROJECT.md) | Goals, non-goals, and milestones |
| [ARCHITECTURE.md](ARCHITECTURE.md) | System diagram and component descriptions |
| [BRAIN.md](BRAIN.md) | Brain folder structure, `static/` permission model, sidecar metadata, TODO format |
| [SCHEDULING.md](SCHEDULING.md) | Recursive scheduled events, prompt bundles, safety limits |
| [VERSIONING.md](VERSIONING.md) | Jujutsu soft versioning, nightly backup, restore scenarios |
| [FRONTEND.md](FRONTEND.md) | Desktop app, terminal chat, iOS mobile app |
| [PATTERNS.md](PATTERNS.md) | Code conventions, project structure, security patterns |
| [QUESTIONS.md](QUESTIONS.md) | Design decision log (all resolved) |

## Getting Started

```bash
cp .env.example .env
# Fill in your LLM provider credentials and config, then:
# (build & run instructions TBD)
```

## License

TBD
