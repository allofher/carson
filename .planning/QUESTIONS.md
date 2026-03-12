# Design Questions — Decision Log

Design questions collected from across the project docs, now all resolved. Each entry links back to its source document. The corresponding design docs have been updated to reflect these decisions.

---

## Recursive Scheduling (`SCHEDULING.md`)

### Q1: Fresh chain per recurrence?

> Should recurring events get a fresh `chain` on each recurrence, or continue the same chain?

**Decision**: Recurring events get a fresh `chain` on each recurrence.

---

### Q2: Mutable pending events?

> Should the agent be able to modify a pending event (change its time or update the prompt)?

**Decision**: No. Cancel and re-create. Simpler to implement, easier to audit.

---

### Q3: Archive vs. delete completed bundles?

> Should completed ScheduledPrompt bundles be archived or deleted?

**Decision**: Archive to `completed/` with a TTL-based cleanup (e.g., 30 days). Useful for debugging and chain tracing.

---

## Brain / Watch Folder (`BRAIN.md`)

### Q4: ~~How should file ownership be tracked?~~ Resolved

> Replaced by the `static/` directory model. There is no per-file ownership tracking. The agent is blocked from writing to `brain/static/` via a path-prefix check in the tool handler. Everything else is mutable. No manifest, no ownership tags.

**Decision:** `static/` directory permission gate. See BRAIN.md.

---

### Q5: ~~Should the agent be allowed to *append* to human files?~~ Resolved

> Moot under the `static/` model. Files in `static/` cannot be modified at all (including appends). Files outside `static/` can be freely edited. If the user wants a file to be append-safe, they keep it in `static/` and the agent uses a sidecar in `.meta/` instead.

**Decision:** No special append rules. `static/` = read-only, everything else = fully mutable.

---

### Q6: Monolithic TODO.md or multiple TODO files?

> One `TODO.md` at the root is simple, but it could grow large. Should there be per-project or per-folder TODO files that get aggregated? If so, where does aggregation happen — at read time by the agent, or as a build step that writes a combined file?


**Decision**: 2 phase approach. Version1: We have just a monolothic TODO.md file and it houses everything. Simple to get going, not a problem for quite a while. Version2: break out all todos individually with metadata. This could be stored as a series of markdown files and some linking todo-overview.json file OR could be a full blown SQL database. 

---

### Q7: TODO format — structured frontmatter vs. inline metadata?

> The proposal uses inline backtick-wrapped key-value pairs (`origin:...`, `due:...`). An alternative is YAML frontmatter per section or a parallel JSON file. Which is most robust for both human readability and machine parsing?

**Decision:** Version 1 should just use Inline backtick pairs. 

---

### Q8: What happens to sidecar metadata when a file is moved or renamed?

> The `.meta/` directory mirrors the brain's folder structure. If a file moves from `static/photos/vacation.jpg` to `static/archive/vacation.jpg`, the sidecar at `.meta/static/photos/vacation.jpg.meta.json` becomes orphaned. Should the agent detect this and move the sidecar? Use content hashing to re-link? Or just mark it orphaned and let the agent decide?

**Decision:** Content hashing (`source_hash` in the sidecar) to detect renames/moves. The watcher sees a delete + create pair; if the new file's hash matches an orphaned sidecar, the agent re-links. Otherwise, mark orphaned and let the agent decide on cleanup.

---

### Q9: ~~Should the human be able to "claim" an agent file?~~ Resolved

> Moot under the `static/` model. There is no ownership to transfer. If the user wants to protect a file from agent edits, they move it into `static/`. If they're OK with the agent editing it, they leave it outside. No mixed-ownership state needed.

**Decision:** User moves files into/out of `static/` to control mutability. No ownership concept.

---

### Q10: How eagerly should the agent generate sidecar metadata for new files?

> When a file is added to the brain (whether in `static/` or elsewhere), should the agent proactively generate a sidecar immediately, or wait until the file is relevant to a task? Eager metadata generation is thorough but costs LLM calls. Lazy generation saves resources but means some files have no metadata when the agent needs it.

**Decision:** The point of the metadata is to allow the LLM to decide given unique context; for this I'd rather expose a simple repeatable routine (via a skill or script) and then trust the llm.

---

### Q11: ~~How does the agent distinguish "positive edits" to human files?~~ Resolved

> Moot under the `static/` model. There's no need to classify edits as positive or destructive. Files in `static/` are fully read-only — no edits of any kind. Files outside `static/` are fully mutable. The agent uses sidecars in `.meta/` to annotate static files without touching them.

**Decision:** No edit classification needed. `static/` = no edits, everything else = any edits.

---

### Q12: Brain folder discovery and initialization

> How does the brain folder get set up? Does the user point Carson at an existing directory? Does Carson create the `.brain/` and `.meta/` directories on first run? What if the user's folder already has a `.meta/` directory for something else?

**Decision:** Carson creates `.brain/` and `.meta/` on first run only if they don't exist. If `.meta/` already exists, assume it was created for the purpose of Carson.

---

### Q13: Conflict resolution between TODO.md and the frontend

> Both the frontend and the agent write to `TODO.md`. If the frontend marks an item complete while the agent is mid-invocation and also writing to the file, what happens? Do we need file-level locking, operational transforms, or a "last write wins" policy?

**Decision:** Front-end should trump agent. But we don't want to build a complex change queueing procedure until we have to. So for now, last file write wins.

---

## Frontend Surfaces (`FRONTEND.md`)

### Q14: Desktop app toolkit — native per-platform or cross-platform framework?

> macOS and Linux both need a native app. Options: (a) two separate native apps (SwiftUI + GTK/Qt), (b) a cross-platform framework (Tauri, Electron, Flutter desktop), (c) a web app served locally by the daemon. Each has different trade-offs for performance, look-and-feel, and maintenance burden.

**Decision:** For the desktop app toolkit, we're going to use Tauri. If we run into issues with the app, particularly to do with the backend contract, then we'll use Wails (a golang web bundler).

---

### Q15: Desktop ↔ daemon communication — file watching, socket, or HTTP?

> The desktop app needs to know about task changes, agent notifications, and brain state. Options: (a) watch the brain folder directly (same as Carson), (b) connect to a Unix socket / named pipe, (c) local HTTP API, (d) a combination. File watching is simple but doesn't cover push notifications from the agent.

**Decision:** Hybrid — file watching for brain state (TODO.md, .meta/, summaries) plus a local socket for real-time notifications from `frontend_command`. Keeps the brain folder as the source of truth while enabling push.

---

### Q16: Terminal chat — standalone binary or subcommand of Carson?

> Should the terminal chat be `carson chat` (subcommand of the main binary) or a separate `carson-chat` binary? Subcommand keeps distribution simple; separate binary allows independent versioning and lighter dependencies.

**Decision:** Subcommand (`carson chat`). Simpler to ship, and the chat client is lightweight enough to not bloat the main binary. If this becomes a problem later when we're adding chat features, we can break them out and redesign the underlying communication between the TUI surface. 

---

### Q17: Terminal chat — session persistence model?

> How should chat history be stored? Options: (a) plain text/markdown log files, (b) SQLite database, (c) JSON lines. Needs to support search, resume, and eventually context replay for the agent.

**Decision:** JSON lines (.jsonl) is the simplest and grep-friendly. We will just need to add some session history maintenance QOL (like at the end of the week compacting many sessions into a historical file); and after 30 days purging a historical file, etc.

---

### Q18: Mobile sync — how does the iOS app communicate with a local daemon?

> Carson runs on the user's desktop/server, not in the cloud. The mobile app needs to reach it. Options: (a) Tailscale/WireGuard tunnel to the local machine, (b) a lightweight relay service (cloud-hosted, just passes messages), (c) iCloud or shared file sync (Syncthing, etc.), (d) the user exposes Carson on their LAN and the phone connects over WiFi only.

**Decision:** Let's design for Tailscale/WireGuard for now, and where we make intentional decisions that prevent a relay service drop in, let's document that as tech debt. Ultimate goal is users choice between WireGuard/Tailscale or a relay service.

---

### Q19: Mobile voice drop — on-device or server-side transcription?

> Voice drops need transcription. Options: (a) on-device (Apple Speech framework — free, private, works offline), (b) server-side via Carson (Whisper or API — higher quality, costs money/compute), (c) hybrid (on-device first, server refines).

**Decision:** On-device first (Apple Speech). It's free, private, and instant. The transcript is what gets sent to Carson — audio never leaves the phone unless the user opts in.

---

### Q20: Mobile context cards — who generates them?

> Cards need calendar data + brain context + relevance ranking. Options: (a) Carson generates cards proactively via scheduled events and syncs them to mobile, (b) the mobile app pulls raw data and assembles cards locally, (c) the mobile app requests cards on-demand from Carson.

**Decision:** Carson generates them. The agent already has calendar access (SCHEDULING.md) and brain context. It can pre-compute cards and sync the result. Keeps the mobile app thin. Schedule chains can help Carson manage relevance of cards in app.

---

### Q21: Should the desktop app have *any* chat capability?

> The current proposal gives chat exclusively to the terminal. But some users may want a lightweight inline prompt in the desktop app — e.g., "summarize this file" while browsing the brain. Is that scope creep, or a natural extension?

**Decision:** No chat at all in desktop app.

---

### Q22: Notification protocol — what can the agent send to the desktop app?

> The `frontend_command` tool needs a defined schema. What notification types are supported? Just text alerts? Actionable notifications (with buttons)? Deep links into the brain browser? Rich content (markdown, images)?

**Decision:** Carson should be able to update files which the desktop app picks up changes of automatically (eg. TODO.md). Beyond that, the frontend_command should enable desktop native push notifications. Text content only.

---

### Q23: Should the mobile app work fully offline?

> If the sync layer is unavailable (no network, Carson daemon is off), should the mobile app still show cached context cards and allow task completion (queued for sync later)? Or should it degrade to "no connection" state?

**Decision:** Offline-capable with queued writes. Context cards are pre-generated and cacheable. Task completions are simple state changes that can be queued and synced later. Voice drops can be stored locally and sent when connectivity returns. Just give the user feedback that clearly depicts whether the app is offline because the phone is missing connectivity or if the app is offline because the carson service is down. 

---

## Versioning & Safety (`VERSIONING.md`)

### Q24: Should the user be able to trigger an ad-hoc backup?

> The nightly job handles the primary backup. But should there be a `carson backup --now` command for the user to snapshot before doing something risky (e.g., bulk-adding files, letting the agent run a large task)? If so, does the ad-hoc snapshot follow the same retention policy or is it kept separately?

**Decision:** Yes — `carson backup --now` creates an out-of-cycle snapshot tagged as `manual`. 

---

### Q25: Git commit granularity — once daily or more often?

> The proposal commits once per day (during the nightly job). But a single daily commit means intraday rollback relies on uncommitted diffs, which are lost if the machine crashes. Should there be periodic intraday commits (e.g., every 4 hours) to reduce the blast radius of a crash, at the cost of a noisier git log?

**Decision:** We're accepting jujutsu as a useful dependency. It will give us midday crash safe diffs, as well as hopefully simplify continual ongoing changes throughout the day. Then we have the nightly commit before backup as a clear checkpoint.

---

### Q26: What belongs in the backup snapshot vs. what's excluded?

> The brain folder may contain large binary files (photos, videos, PDFs). Including them in every nightly compressed snapshot could make backups very large and slow. Should there be a size threshold or file-type filter? Or should binary files be handled differently (e.g., deduplicated, rsynced separately)?

**Decision:** (iteration 1) back up everything, accept large snapshots; (iteration 2) use rsync-style incremental backups instead of full snapshots for large brains. Needs to be informed by realistic brain folder sizes.

---

### Q27: Should the agent ever be able to trigger a rollback?

> The proposal reserves rollback for the user (`carson rollback`). But should the agent be able to trigger a rollback of its own files if it detects it made a mistake? E.g., "I corrupted TODO.md, let me restore it from the last commit." This is self-healing but also means the agent can undo its own recent work.

**Decision:** Not now. At a later date yes, but we don't need this functionality initially.

---

### Q28: Backup log format — append-only JSON array or JSON lines?

> The proposal shows `.brain/backup-log.json` as a JSON array. But appending to a JSON array requires reading the whole file, parsing, appending, and rewriting. JSON lines (one JSON object per line, no wrapping array) is append-friendly and works with standard tools (`tail -1`, `jq -s`). Which format is better for a log that grows indefinitely?

**Decision:** JSON lines (`.jsonl`). Append-only, no parse-rewrite cycle, trivially streamable. The agent can read the last line to get the most recent entry.

---

### Q29: How long should git history be retained?

> The git repo in the brain folder will accumulate daily commits indefinitely. After a year, that's 365 commits. After 5 years, 1,825. Should git history be pruned (e.g., squash commits older than 6 months into monthly snapshots)? Or is linear history cheap enough to keep forever?

**Decision:** Not a pressing concern for v1. v2 yes, we will address version history size and efficiency.

---

### Q30: Should backup status be surfaced beyond TODO.md?

> The proposal posts backup status to `TODO.md` for the agent and desktop app to render. Should backup failures also trigger: (a) an OS-level notification via the desktop app, (b) an email/SMS alert, (c) a persistent warning banner in the desktop app until resolved? How urgent is a single failed backup vs. multiple consecutive failures?

**Decision:** Single failure: TODO item + desktop notification. Two consecutive failures: high-priority TODO + persistent desktop banner. Three+: all of the above. Email/SMS is out of scope for now but the notification protocol (Q22) should be extensible enough to support it later.

---

### Q31: Should the soft versioning layer use git or something lighter?

> Git is powerful but heavyweight for what is essentially "track file changes and allow rollback." Alternatives: (a) a simple file-copy snapshot mechanism, (b) BTRFS/ZFS snapshots (if available), (c) a purpose-built tool like restic or borg for incremental snapshots, (d) Jujutsu (jj) which has a simpler mental model than git and better handles uncommitted work. Is git the right tool here, or are we importing complexity we don't need?

**Decision:** We're going to accept jujutsu as a useful dependency. It's constant support for uncommitted changes throughout the day serves as a better 'soft backup' model

---

### Q32: What happens during a restore — does the daemon stay running?

> If the user runs `carson restore --from 2026-02-28`, the brain folder's contents change underneath the running daemon and its watcher. Should the daemon: (a) be stopped before restore and restarted after, (b) stay running and the watcher picks up the changes naturally, (c) enter a "maintenance mode" that pauses agent invocations during restore?

**Decision:** Maintenance mode. The restore command signals the daemon to pause the watcher and reject new agent invocations, performs the restore, then signals the daemon to resume. This prevents the agent from reacting to the flood of file-change events during restore.

---

### Q33: Should backups be encrypted?

> If backups are stored on a remote target (S3, rsync host), should they be encrypted at rest? The brain contains personal data. Options: (a) always encrypt remote backups (age, GPG, or built-in), (b) encrypt only if the user configures a key, (c) never encrypt (rely on transport/storage-level encryption).

**Decision:** v1 no encryption. v2 Encrypt remote backups by default using `age` (simple, no GPG keyring complexity). Local backups are unencrypted (the disk's own encryption covers them). The user provides or generates an `age` key during remote backup setup; Carson stores the public key in config and the user safeguards the private key.
