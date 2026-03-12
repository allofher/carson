# Documentation Drift — Codebase vs. Docs

This file summarizes where the codebase diverged from the planning documents.
The codebase is the source of truth; all planning docs have been updated to match.

---

## ARCHITECTURE.md

1. **Bash tool exists** — The doc stated "the agent cannot execute arbitrary shell commands." The codebase includes a `bash` tool with a blacklist-based safety model (block known-dangerous patterns, allow everything else). Security boundaries section updated.

2. **No Event Router, Folder Watcher, or Cron Manager** — These components are described in the architecture diagram but do not exist in the codebase. The current flow is: API server receives requests -> harness runs the agent loop. The diagram and component descriptions have been updated to reflect what's built vs. planned.

3. **Configuration model changed** — The doc said "all runtime config is loaded from environment variables (`.env` file supported)." In practice, configuration uses a layered approach: `~/.config/carson/config.json` for preferences, `.env` for secrets (API keys), and env vars as the highest-precedence override. Updated.

4. **LLM provider abstraction** — The doc mentioned "upstream provider" generically. The codebase supports four providers (Anthropic, OpenAI, Gemini, Ollama) behind a `Provider` interface. Updated.

5. **API server exists** — Not shown in the original architecture. The daemon runs an HTTP API on `127.0.0.1:7780` with endpoints: `/health`, `/status`, `/chat` (SSE), `/invoke`, `/logs` (SSE). Added to architecture.

6. **Tool model simplified** — The "Dual tool model" (dedicated tools + skills protocol) is documented but only dedicated tools exist. Skills are not implemented. Updated to reflect current state.

## PATTERNS.md

1. **Project structure completely wrong** — Listed directories that don't exist (`watcher/`, `cron/`, `scheduling/`, `versioning/`, `backup/`, `router/`, `platform/`, `skills/`). Missing directories that do exist (`llm/`, `api/`, `logging/`, `lookout/`). Rebuilt from actual codebase.

2. **Doc file locations** — Docs moved from repo root to `.planning/`. Updated references.

3. **Configuration pattern changed** — Was "env vars only." Now JSON config file + .env + env var overrides. Updated.

4. **Tool definitions pattern updated** — Tools return string results, not "structured results." The harness inspects them but passes strings to the LLM. Updated.

5. **Bash tool not documented** — Added to tool listing.

6. **`topofmind.md` not documented as a pattern** — Added to brain file conventions.

## BRAIN.md

1. **Status was wrong** — Said "not yet implemented" but `brain.Init()`, `IsStaticPath()`, `IsInsideBrain()`, `ValidateWritePath()` are all implemented. Updated status.

2. **`topofmind.md` missing** — The brain folder now includes `topofmind.md` (agent-managed context file with daemon-enforced constraints). Added.

## CHAT-MILESTONE.md

1. **Status was wrong** — Said "Planning — not yet implemented" but the full chat surface is implemented: daemon API, streaming harness, TUI client, session logging, `carson lookout`.

2. **`topofmind.md` loading differs from spec** — Doc said it's prepended before the system prompt. In code, it's concatenated with the system prompt and user message into a single user message: `[topofmind] + [system prompt] + [user message]`. Updated to match code.

3. **Invoke endpoint** — Doc said it returns `invocation_id`. Code returns `{"accepted": true}` with no invocation ID. Updated.

4. **Session ID format** — Doc showed `sess_abc123`. Code uses `sess_{unix_millis}`. Updated.

5. **No server-side session history** — Doc said "the daemon holds separate message histories per `session_id`." In code, each `/chat` POST creates a fresh harness invocation — no multi-turn context. `session_id` is used for logging only. Updated.

6. **SSE tool_call event** — Doc showed `input` field in `tool_call` events. Code only sends `tool` and `id`. Updated.

7. **Session JSONL format** — Doc showed `input` field on `tool_call` entries. Code's `sessionEntry` struct only has `ts`, `type`, `content`, `tool`, `status`. Updated.

## PROJECT.md

1. **Configuration reference** — Mentioned `.env.example`. Config is now `~/.config/carson/config.json` + `.env`. Updated.

2. **Milestone progress** — Multiple milestones partially or fully complete. Updated status.

## FRONTEND.md, VERSIONING.md, SCHEDULING.md

- No code changes conflicting with these docs — they describe future work.
- Status lines remain "not yet implemented" which is correct.

## QUESTIONS.md

- No changes needed — decisions are historical and still accurate.
