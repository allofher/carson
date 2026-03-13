# Backlog — Captured Items

Items identified during planning that don't belong to the current milestone but shouldn't be forgotten.

## Harness & Chat Improvements

- [x] **Multi-turn conversation context** — Server-side session store with TTL-based cleanup. Each session maintains full message history across turns. *(Implemented 2026-03-07)*
- [ ] **`search_files` tool** — Dedicated glob/grep tool for the brain folder. Currently the agent has to shell out via `bash` to search. Quick win once harness work resumes.
- [ ] **Skills protocol** — Higher-level composable actions. Designed but not implemented. Defer until patterns emerge from autonomous usage.

## Brain Folder

- [ ] **Folder watcher** — OS-level file-system events for detecting brain folder changes. Designed in BRAIN.md but not implemented. Enables event-driven agent invocations (new file detected, TODO.md changed, etc.). Important for the full scheduling + event router flow.

## Infrastructure

- [x] **Background service management** — `launchd` on macOS, `systemd` on Linux. `carson start` daemonizes by default, `--foreground` for dev. PID file tracking, `stop/restart/status` commands. *(Implemented 2026-03-07)*

## Frontend

- [ ] **Desktop app (Tauri)** — Task board, brain browser, notifications. Needs scheduling to exist first for scheduling visibility features. Significant planning still needed.
- [ ] **Mobile app (iOS)** — Task checkbox, voice drop, context cards. Needs Tailscale/WireGuard sync layer. Furthest out.

## Networking

- [ ] **Tailscale/WireGuard routing** — Required for mobile app connectivity and remote access. Authentication decisions deferred until this is picked up.

## Notifications

- [ ] **Notification queue** — Daemon-side queue for SSE events. When a frontend reconnects, it requests missed notifications since its last seen timestamp. Required for reliable desktop notifications in remote deployments.
- [ ] **iPhone push notifications** — iOS kills background SSE connections. Requires APNs integration or a relay service (ntfy, Pushover) as interim. Separate milestone from the desktop frontend.
