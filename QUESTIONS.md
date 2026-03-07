# Open Questions

Unresolved design questions collected from across the project docs. Each entry links back to its source and includes the current recommendation (if any). Update status as decisions are made.

---

## Recursive Scheduling (`SCHEDULING.md`)

### Q1: Fresh chain per recurrence?

> Should recurring events get a fresh `chain` on each recurrence, or continue the same chain?

**Recommendation:** Fresh chain. Each recurrence is logically independent.

**Status:** Undecided

---

### Q2: Mutable pending events?

> Should the agent be able to modify a pending event (change its time or update the prompt)?

**Recommendation:** No. Cancel and re-create. Simpler to implement, easier to audit.

**Status:** Undecided

---

### Q3: Archive vs. delete completed bundles?

> Should completed ScheduledPrompt bundles be archived or deleted?

**Recommendation:** Archive to `completed/` with a TTL-based cleanup (e.g., 30 days). Useful for debugging and chain tracing.

**Status:** Undecided

---

## Brain / Watch Folder (`BRAIN.md`)

### Q4: How should file ownership be tracked?

> Convention-based (agent files live under known directories), manifest-based (`.brain/manifest.json` lists every agent-created path), or sidecar-based (per-file marker)?

**Recommendation:** Hybrid — use directory convention as the fast path (anything under `.brain/`, `.meta/`, `daily-summary/`, and `TODO.md` is agent-owned) with a manifest as the authoritative fallback for agent files that live outside those directories. Keeps the common case fast and the edge cases correct.

**Status:** Undecided

---

### Q5: Should the agent be allowed to *append* to human files?

> The current rule is "never modify human files." But appending (e.g., adding a `## Agent Notes` section to a human-written markdown file) could be valuable. Is appending a special case of editing, or is it still off-limits?

**Recommendation:** None yet. Strong arguments both ways — appending is useful but blurs the ownership line. Sidecar metadata may be sufficient.

**Status:** Undecided

---

### Q6: Monolithic TODO.md or multiple TODO files?

> One `TODO.md` at the root is simple, but it could grow large. Should there be per-project or per-folder TODO files that get aggregated? If so, where does aggregation happen — at read time by the agent, or as a build step that writes a combined file?

**Recommendation:** None yet. Start with a single `TODO.md` and see if it becomes a bottleneck. If it does, consider a `TODO.md` per human-created subfolder with the agent maintaining an aggregated view.

**Status:** Undecided

---

### Q7: TODO format — structured frontmatter vs. inline metadata?

> The proposal uses inline backtick-wrapped key-value pairs (`origin:...`, `due:...`). An alternative is YAML frontmatter per section or a parallel JSON file. Which is most robust for both human readability and machine parsing?

**Recommendation:** Inline backtick pairs. They survive copy-paste, are visible in any markdown renderer, and don't require a separate parser. But this needs validation with the frontend team.

**Status:** Undecided

---

### Q8: What happens to sidecar metadata when a human file is moved or renamed?

> The `.meta/` directory mirrors the brain's folder structure. If the human moves `photos/vacation.jpg` to `archive/vacation.jpg`, the sidecar at `.meta/photos/vacation.jpg.meta.json` becomes orphaned. Should the agent detect this and move the sidecar? Use content hashing to re-link? Or just mark it orphaned and let the agent decide?

**Recommendation:** Content hashing (`source_hash` in the sidecar) to detect renames/moves. The watcher sees a delete + create pair; if the new file's hash matches an orphaned sidecar, the agent re-links. Otherwise, mark orphaned and let the agent decide on cleanup.

**Status:** Undecided

---

### Q9: Should the human be able to "claim" an agent file?

> If the human edits an agent-owned file (e.g., adds personal notes to `TODO.md`), does ownership change? Should there be a mixed-ownership state? Or does the human editing an agent file implicitly accept that the agent may overwrite their edits?

**Recommendation:** None yet. This is a fundamental UX question. Options: (a) agent files are always agent-owned, human edits at their own risk; (b) human edits promote the file to human-owned; (c) sections within a file can have different ownership.

**Status:** Undecided

---

### Q10: How much structure should the agent impose on new human files?

> When a human drops a raw file (photo, PDF, text) into the brain, should the agent proactively generate a sidecar immediately, or wait until the file is relevant to a task? Eager metadata generation is thorough but costs LLM calls. Lazy generation saves resources but means some files have no metadata when the agent needs it.

**Recommendation:** None yet. Likely depends on file type — images and PDFs benefit from eager analysis, while text files can be indexed cheaply without LLM calls.

**Status:** Undecided

---

### Q11: How does the agent distinguish "positive edits" to human files?

> The spec says the agent can edit human files "in a positive way (e.g., adding metadata to a picture)." Where is the line between a positive edit and a destructive one? Is adding EXIF data to an image positive? Is reformatting a markdown file positive? Who decides?

**Recommendation:** None yet. The safest initial implementation is to forbid all direct edits to human files and use sidecars exclusively. "Positive edits" could be a future opt-in capability with an explicit allowlist of operations (e.g., "add EXIF tags", "add frontmatter") that the user configures.

**Status:** Undecided

---

### Q12: Brain folder discovery and initialization

> How does the brain folder get set up? Does the user point Carson at an existing directory? Does Carson create the `.brain/` and `.meta/` directories on first run? What if the user's folder already has a `.meta/` directory for something else?

**Recommendation:** Carson creates `.brain/` and `.meta/` on first run only if they don't exist. If `.meta/` already exists, prompt the user to confirm or configure an alternative agent metadata path.

**Status:** Undecided

---

### Q13: Conflict resolution between TODO.md and the frontend

> Both the frontend and the agent write to `TODO.md`. If the frontend marks an item complete while the agent is mid-invocation and also writing to the file, what happens? Do we need file-level locking, operational transforms, or a "last write wins" policy?

**Recommendation:** None yet. File locking is the simplest correct solution but may cause the frontend to stall. A CRDT-like approach (each writer appends operations to a log, periodic compaction) is more robust but significantly more complex.

**Status:** Undecided

---

## Frontend Surfaces (`FRONTEND.md`)

### Q14: Desktop app toolkit — native per-platform or cross-platform framework?

> macOS and Linux both need a native app. Options: (a) two separate native apps (SwiftUI + GTK/Qt), (b) a cross-platform framework (Tauri, Electron, Flutter desktop), (c) a web app served locally by the daemon. Each has different trade-offs for performance, look-and-feel, and maintenance burden.

**Recommendation:** None yet. Tauri (Rust + web view) is the leading candidate — small binary, native-feeling, single codebase. But it needs evaluation against the "polished dinner service" design goal, especially on Linux where web views vary.

**Status:** Undecided

---

### Q15: Desktop ↔ daemon communication — file watching, socket, or HTTP?

> The desktop app needs to know about task changes, agent notifications, and brain state. Options: (a) watch the brain folder directly (same as Carson), (b) connect to a Unix socket / named pipe, (c) local HTTP API, (d) a combination. File watching is simple but doesn't cover push notifications from the agent.

**Recommendation:** Hybrid — file watching for brain state (TODO.md, .meta/, summaries) plus a local socket for real-time notifications from `frontend_command`. Keeps the brain folder as the source of truth while enabling push.

**Status:** Undecided

---

### Q16: Terminal chat — standalone binary or subcommand of Carson?

> Should the terminal chat be `carson chat` (subcommand of the main binary) or a separate `carson-chat` binary? Subcommand keeps distribution simple; separate binary allows independent versioning and lighter dependencies.

**Recommendation:** Subcommand (`carson chat`). Simpler to ship, and the chat client is lightweight enough to not bloat the main binary.

**Status:** Undecided

---

### Q17: Terminal chat — session persistence model?

> How should chat history be stored? Options: (a) plain text/markdown log files, (b) SQLite database, (c) JSON lines. Needs to support search, resume, and eventually context replay for the agent.

**Recommendation:** None yet. JSON lines is the simplest and grep-friendly. SQLite is more robust for search. Depends on whether we want full-text search across sessions.

**Status:** Undecided

---

### Q18: Mobile sync — how does the iOS app communicate with a local daemon?

> Carson runs on the user's desktop/server, not in the cloud. The mobile app needs to reach it. Options: (a) Tailscale/WireGuard tunnel to the local machine, (b) a lightweight relay service (cloud-hosted, just passes messages), (c) iCloud or shared file sync (Syncthing, etc.), (d) the user exposes Carson on their LAN and the phone connects over WiFi only.

**Recommendation:** None yet. This is the hardest infrastructure question. A relay service is the most reliable UX but introduces a cloud dependency. Tailscale is elegant but requires setup. LAN-only limits mobile usefulness to home/office.

**Status:** Undecided

---

### Q19: Mobile voice drop — on-device or server-side transcription?

> Voice drops need transcription. Options: (a) on-device (Apple Speech framework — free, private, works offline), (b) server-side via Carson (Whisper or API — higher quality, costs money/compute), (c) hybrid (on-device first, server refines).

**Recommendation:** On-device first (Apple Speech). It's free, private, and instant. The transcript is what gets sent to Carson — audio never leaves the phone unless the user opts in.

**Status:** Undecided

---

### Q20: Mobile context cards — who generates them?

> Cards need calendar data + brain context + relevance ranking. Options: (a) Carson generates cards proactively via scheduled events and syncs them to mobile, (b) the mobile app pulls raw data and assembles cards locally, (c) the mobile app requests cards on-demand from Carson.

**Recommendation:** Carson generates them. The agent already has calendar access (SCHEDULING.md) and brain context. It can pre-compute cards via scheduled events (e.g., "15 minutes before each meeting, generate a context card") and sync the result. Keeps the mobile app thin.

**Status:** Undecided

---

### Q21: Should the desktop app have *any* chat capability?

> The current proposal gives chat exclusively to the terminal. But some users may want a lightweight inline prompt in the desktop app — e.g., "summarize this file" while browsing the brain. Is that scope creep, or a natural extension?

**Recommendation:** None yet. Risk of the desktop app becoming a second chat client. A compromise: allow single-turn "ask about this" prompts contextual to what the user is looking at, but no persistent conversation.

**Status:** Undecided

---

### Q22: Notification protocol — what can the agent send to the desktop app?

> The `frontend_command` tool needs a defined schema. What notification types are supported? Just text alerts? Actionable notifications (with buttons)? Deep links into the brain browser? Rich content (markdown, images)?

**Recommendation:** None yet. Start minimal: text message + optional action (a single deep link into the brain browser). Expand based on real usage patterns.

**Status:** Undecided

---

### Q23: Should the mobile app work fully offline?

> If the sync layer is unavailable (no network, Carson daemon is off), should the mobile app still show cached context cards and allow task completion (queued for sync later)? Or should it degrade to "no connection" state?

**Recommendation:** Offline-capable with queued writes. Context cards are pre-generated and cacheable. Task completions are simple state changes that can be queued and synced later. Voice drops can be stored locally and sent when connectivity returns.

**Status:** Undecided
