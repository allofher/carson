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
