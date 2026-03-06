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
