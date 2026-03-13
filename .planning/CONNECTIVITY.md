# Remote Connectivity — Design Proposal

> **Audience:** Developer picking this up for implementation.
> **Status:** Architecture drafted, transport decision pending.

## The Problem

Carson's daemon runs on a Linux host. Everything else — the macOS laptop, the TUI, the eventual iPhone app — lives elsewhere. Today the API server binds to `127.0.0.1:7780`, which means it's invisible to any device that isn't the host itself. To pull down code and start testing from a laptop, we need the daemon to be reachable over a secure tunnel.

This document covers two things:
1. **What we need to build** in Carson to support remote connectivity.
2. **What transport layer** to use underneath (Tailscale, WireGuard, Headscale, etc.).

## Deployment Topology

```
┌──────────────────────────────────────────────────────┐
│  Linux Host (home server / work VM)                   │
│                                                       │
│  ┌──────────────┐  ┌────────────┐  ┌──────────────┐  │
│  │ Carson Daemon │  │ Scheduler  │  │ Brain Folder │  │
│  │ 127.0.0.1    │  │ (cron.d)   │  │              │  │
│  │ :7780        │  │            │  │              │  │
│  └──────┬───────┘  └────────────┘  └──────────────┘  │
│         │                                             │
│         │  Tunnel interface (e.g. wg0, tailscale0)    │
│         │  binds to tunnel IP on :7780                │
└─────────┼─────────────────────────────────────────────┘
          │
    ╔═════╧═══════════════════════╗
    ║   Encrypted tunnel          ║
    ║   (WireGuard / Headscale /  ║
    ║    Tailscale / NetBird)     ║
    ╚═════╤═══════════════════════╝
          │
    ┌─────┼──────────────────┐
    │     │                  │
    ▼     ▼                  ▼
┌────────┐ ┌───────────┐ ┌──────────┐
│ macOS  │ │ macOS TUI │ │ iPhone   │
│Desktop │ │ (carson   │ │ App      │
│App     │ │  chat)    │ │ (future) │
└────────┘ └───────────┘ └──────────┘
```

Two deployment targets:
- **Personal:** Linux home server, accessed from macOS laptop + iPhone over a tunnel.
- **Work:** Linux VM behind corporate VPN, accessed from work Mac through the VPN (tunnel not needed — VPN provides routing).

---

## Part 1: What We Need to Build

The tunnel itself is external to Carson — we don't implement WireGuard. But Carson needs changes to support being accessed over one.

### 1.1 Bind Address Configuration

**Current state:** The API server hardcodes `127.0.0.1:{port}`.

**Change:** Make the bind address configurable.

```
Config field:  DaemonBindAddr  (default: "127.0.0.1")
Env var:       CARSON_BIND_ADDR
config.json:   "bind_addr": "127.0.0.1"
```

When running behind a tunnel, the user sets `bind_addr` to the tunnel interface IP (e.g., `100.64.x.x` for Tailscale, `10.x.x.x` for WireGuard) or `0.0.0.0` to listen on all interfaces.

**Important:** Binding to anything other than `127.0.0.1` removes the implicit security of localhost-only access. This is acceptable *only* because:
- The tunnel already provides encryption and authentication at the network layer.
- We add API-level authentication (see 1.2) as defense in depth.

```go
// in config.go
DaemonBindAddr string // default "127.0.0.1"

// in server.go
addr := fmt.Sprintf("%s:%d", cfg.DaemonBindAddr, cfg.DaemonPort)
```

### 1.2 API Authentication (Bearer Token)

When the daemon is reachable beyond localhost, we need request-level authentication. A simple shared-secret bearer token is sufficient for a single-user system.

**Design:**
- A `CARSON_API_TOKEN` env var (or `.env` entry) holds a random token.
- If set, every request must include `Authorization: Bearer <token>`.
- If unset *and* bind addr is `127.0.0.1`, no auth is required (backwards compatible).
- If unset *and* bind addr is not localhost, the daemon refuses to start with a clear error message. This prevents accidentally exposing an unauthenticated API.
- SSE endpoints (`/logs`, `/events`, `/chat`) accept the token as a query parameter `?token=<token>` as a fallback, since some SSE clients can't set headers.

```go
// middleware
func (h *Handlers) requireAuth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if h.apiToken == "" {
            next(w, r)
            return
        }
        token := extractToken(r) // checks Authorization header, then ?token= query param
        if token != h.apiToken {
            writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
            return
        }
        next(w, r)
    }
}
```

**Token generation:** `carson init` generates a random 32-byte hex token and writes it to `.env`. The user copies this token to their client devices.

### 1.3 Client-Side Token Support

The `carson chat` TUI and `carson lookout` commands need to send the token when connecting to a remote daemon.

**Current state:** These commands connect to `http://127.0.0.1:{port}`.

**Change:** Add a `--host` flag and token resolution.

```
carson chat --host 100.64.1.5:7780
carson lookout --host 100.64.1.5:7780
```

Token resolution order:
1. `--token` flag (for scripts/testing)
2. `CARSON_API_TOKEN` env var
3. `~/.config/carson/.env` (same file the daemon reads)

When `--host` is provided, the client skips the local daemon health check and connects directly to the remote address.

### 1.4 CORS Headers for Desktop App

The Tauri desktop app will make HTTP requests to the daemon from a web view. When the daemon is remote, the browser's same-origin policy applies.

Add CORS middleware that allows requests from any origin when an API token is present. This is safe because the token itself is the access control, not the origin.

```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### 1.5 Health Check with Latency

Extend `/health` to include a `latency_ms` field when called from a remote client. The client measures round-trip time locally, but the server can also report its own view of responsiveness (e.g., time since last successful LLM call).

More importantly, remote clients need a fast way to know "am I connected?" before entering a chat session.

```
GET /health
Authorization: Bearer <token>

200 OK
{
  "status": "ok",
  "provider": "anthropic",
  "model": "claude-sonnet-4-20250514",
  "version": "0.8.0",
  "remote": true    // true when request came from non-loopback
}
```

### 1.6 Notification Queue for Reconnecting Clients

**Already identified in BACKLOG.md.** When a remote client disconnects (laptop sleeps, network blip, iPhone backgrounded), it misses SSE events. On reconnect, it needs to catch up.

**Design:** The daemon maintains a bounded in-memory ring buffer of recent events (default: 1000 entries, configurable). Each event gets a monotonic sequence ID. Clients include `Last-Event-ID` in their SSE reconnect, and the daemon replays missed events.

This is standard SSE reconnection behavior — the protocol already supports `Last-Event-ID`. We just need to buffer events server-side.

```go
type EventBuffer struct {
    mu     sync.RWMutex
    events []BufferedEvent
    nextID uint64
    maxLen int
}

type BufferedEvent struct {
    ID   uint64
    Type string // "log", "file_change", etc.
    Data []byte
    Time time.Time
}
```

This applies to both `/logs` and `/events` endpoints.

### 1.7 TLS (Optional, Depends on Transport)

If the transport layer provides encryption (WireGuard, Tailscale — both do), Carson doesn't need its own TLS. The API runs plain HTTP over an already-encrypted tunnel.

However, if someone wants to expose Carson over the open internet (not recommended but possible), or if the transport is a plain VPN that doesn't encrypt at the application layer, TLS support is useful.

**Design:** Optional. If `CARSON_TLS_CERT` and `CARSON_TLS_KEY` are set, the server uses `http.ListenAndServeTLS`. Otherwise, plain HTTP. This is a low-priority addition — all recommended transports encrypt at the tunnel layer.

### Summary: What to Build

| Change | Priority | Complexity | Files |
|---|---|---|---|
| Bind address config | **Must have** | Small | `config.go`, `server.go` |
| Bearer token auth | **Must have** | Small | `config.go`, `handlers.go`, `server.go` |
| Client `--host` flag | **Must have** | Small | `cmd/carson/main.go`, chat/lookout clients |
| CORS middleware | Should have | Trivial | `server.go` |
| Health check additions | Should have | Trivial | `handlers.go` |
| Notification queue | Should have | Medium | New: `internal/api/eventbuffer.go` |
| TLS support | Nice to have | Small | `server.go` |

### Implementation Order

1. **Bind address + bearer token + startup guard** — The minimum to safely listen beyond localhost.
2. **Client `--host` + token resolution** — So `carson chat` can reach the remote daemon.
3. **CORS + health check** — Prepare for the desktop app.
4. **Notification queue** — Reliability for intermittent connections.
5. **TLS** — Only if a use case demands it.

After step 2, you can start the daemon on the Linux host, set up a tunnel, and `carson chat --host <tunnel-ip>:7780` from your laptop.

---

## Part 2: Transport Layer Comparison

Six options evaluated. The goal: securely connect 3–5 personal devices (macOS, Linux, iPhone) to a daemon on a Linux host. Single user. No mandatory SSO. Minimal vendor lock-in.

### Option 1: Tailscale (Managed)

**What it is:** Managed mesh VPN built on WireGuard. Coordination server hosted by Tailscale Inc. Devices authenticate via an identity provider (Google, GitHub, Apple, Microsoft, or custom OIDC).

**Pros:**
- Easiest setup of any option — install, log in, done.
- Excellent iOS app — best in class.
- NAT traversal works out of the box (DERP relays).
- MagicDNS gives you hostnames instead of IPs.
- Free for personal use (3 users, 100 devices).
- Devices are peer-to-peer once connected — traffic doesn't route through Tailscale's servers.

**Cons:**
- **SSO is mandatory.** You must log in with Google, GitHub, Apple, or Microsoft. There is no "just a password" option. This is the core tension: it works perfectly but forces a third-party identity dependency.
- Control plane is proprietary. If Tailscale changes pricing or shuts down, your network stops working.
- No self-hosting the coordination server (that's what Headscale is for).

**Verdict:** The easiest path by far, but the SSO requirement means Google or GitHub is always in the loop. If that's acceptable, stop here — nothing else comes close on UX.

### Option 2: WireGuard (Raw)

**What it is:** Kernel-level VPN protocol. No coordination server, no accounts, no vendor. Pure public-key cryptography. You configure each device manually.

**Pros:**
- Zero dependencies. No vendor, no accounts, no SSO.
- In the Linux kernel — as battle-tested as networking gets.
- Solid iOS app (official, well-maintained).
- Simplest mental model: each device has a key pair, peers exchange public keys.
- Hub-and-spoke topology is perfect for "one server, N clients."

**Cons:**
- Manual key distribution and config management. For 3 devices this is ~15 minutes; for each new device, you edit configs on all peers.
- No NAT traversal. The Linux host needs a reachable IP (static public IP, port-forwarded, or a VPS with a known address). If the host is behind double NAT with no port forwarding, raw WireGuard won't work without building your own relay.
- No MagicDNS, no automatic key rotation, no web UI.
- Hub-and-spoke means all traffic routes through the Linux server. Fine for Carson (that's where we want to go anyway), but not a mesh.

**Verdict:** The strongest option *if* the Linux host has a reachable IP. Zero dependencies, maximum control, excellent security posture. The setup tax is real but one-time.

### Option 3: Headscale (Self-Hosted Tailscale)

**What it is:** Open-source replacement for Tailscale's coordination server. You run a single Go binary on a server you control. Clients use the stock Tailscale apps (including the iOS app).

**Pros:**
- **No mandatory SSO.** Supports pre-authenticated keys — you generate a key from the CLI, paste it into the Tailscale client. No Google, no GitHub, no OIDC. SSO is optional.
- Uses the official Tailscale iOS app (set "Alternate Coordination Server URL" in settings). Same excellent iOS experience.
- Single Go binary. Lighter to operate than it sounds.
- Full control over the coordination plane. No vendor lock-in.
- Gets you MagicDNS, NAT traversal (via self-hosted DERP relays), and the Tailscale client UX without the Tailscale SaaS dependency.

**Cons:**
- Requires running a coordination server somewhere with a public IP and a domain with TLS (Let's Encrypt works). A $5/month VPS handles it, or the Linux home server itself if it's reachable.
- Not yet at v1.0 (v0.26+ as of 2025). Actively developed, widely used, but not officially "stable."
- Missing some Tailscale features (Funnel, Serve, network flow logs).
- iOS setup requires a non-obvious step: going into Tailscale app settings to change the coordination server URL.

**Verdict:** Best of both worlds — Tailscale's UX without the SSO lock-in. The coordination server is an extra thing to run, but it's a single binary and can live on the same Linux host as Carson.

### Option 4: Nebula (Slack / Defined Networking)

**What it is:** Certificate-based overlay network. You run your own CA, issue certs to each device, and run "lighthouse" nodes for discovery.

**Pros:**
- Fully self-hosted, certificate-based auth. No SSO, no accounts.
- Battle-tested at Slack's scale.
- Solid engineering, clean design.

**Cons:**
- **iOS app is unreliable.** User reports of crashes, system hangs requiring device reboots. This is a dealbreaker for the iPhone use case.
- Higher setup complexity than all other options — CA management, per-device certs, YAML configs, lighthouse nodes.
- No web UI. All management is CLI/config files.

**Verdict:** Skip. The iOS app quality disqualifies it. Would be a strong contender for Linux-only or Linux+macOS deployments.

### Option 5: ZeroTier

**What it is:** Peer-to-peer overlay network with a hosted controller. Devices join networks by ID and are authorized by node ID.

**Pros:**
- No SSO required. Device identity is cryptographic key pairs.
- Decent iOS app.
- Low setup complexity — create network, join devices, authorize.

**Cons:**
- **Pricing instability.** Free tier shrank from 25 to 10 devices (Nov 2025). Concerning trend.
- Self-hosted controller was broken in v1.16.0 (Sep 2025) — fixed, but exposed that self-hosting isn't a priority for the company.
- Custom protocol (not WireGuard). Less audited.
- Fully replacing the root infrastructure is complex and poorly documented.

**Verdict:** Skip. The pricing instability and fragile self-hosting story make it the worst risk/reward. ZeroTier is fine for casual use but not something to build infrastructure on.

### Option 6: NetBird

**What it is:** Open-source mesh VPN built on WireGuard. Self-hostable management plane with built-in local user management (no external IdP required as of v0.62).

**Pros:**
- **Built-in local auth.** Create users with username/password directly. No SSO, no OIDC, no external identity provider at all. Best auth story for the "just my personal devices" case.
- Built on WireGuard underneath — same crypto guarantees.
- Fully self-hostable (management server + signal server + TURN relay).
- Web dashboard included.
- iOS app available and actively maintained.
- v0.65+ added a reverse proxy for exposing services with auth — could directly serve Carson's API.

**Cons:**
- Younger project (founded ~2022). Less battle-tested than Tailscale/WireGuard.
- Self-hosted stack has 3 components (management, signal, TURN) vs. Headscale's 1 binary.
- iOS app is newer and less polished than Tailscale's.
- Higher risk of breaking changes between versions.

**Verdict:** The most interesting option for the "no SSO, self-hosted, works on iPhone" requirements. The youth of the project is the main risk.

### Comparison Matrix

| | No SSO | iOS App | Self-Host | Maturity | Setup | Lock-in |
|---|---|---|---|---|---|---|
| **Tailscale** | No | Excellent | No | Very High | 5 min | Medium |
| **WireGuard** | Yes | Good | N/A | Very High | 30 min | None |
| **Headscale** | Yes | Excellent* | Yes (1 binary) | Medium-High | 1 hour | None |
| **Nebula** | Yes | Poor | Yes | High | 2 hours | None |
| **ZeroTier** | Yes | Decent | Fragile | High | 15 min | Medium |
| **NetBird** | Yes | Decent | Yes (3 parts) | Medium | 1 hour | None |

*\*Headscale uses the stock Tailscale iOS app*

### Recommendation

**For right now (getting testing started):** Use **raw WireGuard** if the Linux host has a reachable IP. It's 15 minutes of setup, zero dependencies, and you're connected. Generate keys on both devices, write two config files, start the tunnel, and `carson chat --host 10.0.0.1:7780` works immediately. No coordination server to run.

**For the longer term (iPhone, multiple devices, convenience):** Move to **Headscale**. It gives you:
- Tailscale's iOS app (the best available)
- No SSO requirement (pre-auth keys)
- Self-hosted coordination on the same Linux host
- NAT traversal via self-hosted DERP relays
- MagicDNS so you can `carson chat --host carson-server:7780` instead of remembering IPs

**Keep an eye on NetBird.** Its local auth model is exactly what you want, and if the project matures and the iOS app solidifies, it could become the better long-term choice. But today, Headscale has a stronger ecosystem (it inherits Tailscale's battle-tested clients).

### Migration Path

The beauty of all these options (except stock Tailscale) is that they use WireGuard underneath. The migration path is:

```
Raw WireGuard → Headscale → (optionally) NetBird
```

Carson doesn't care which tunnel it's behind. It just sees traffic arriving on a network interface. The bind address, bearer token, and client `--host` flag work identically regardless of transport. The only Carson-side decision is "what IP to bind to" — and that's a config value, not code.

### What Carson Should NOT Do

- **Don't embed a VPN client.** The tunnel is the user's responsibility. Carson is an application, not a network stack.
- **Don't assume a specific transport.** All the architecture in Part 1 works over any tunnel — or no tunnel at all (localhost, corporate VPN, etc.).
- **Don't auto-configure the tunnel.** A `carson init` step can *suggest* a WireGuard config, but should never silently modify network settings. Network config is high-stakes and varies wildly across environments.
- **Don't depend on MagicDNS or any transport-specific feature.** The client takes a plain `host:port`. Whether that's an IP, a DNS name, or a MagicDNS name is the user's concern.

---

## Architecture Decisions Log

| Decision | Choice | Why |
|---|---|---|
| Bind address | Configurable, default `127.0.0.1` | Must stay secure by default; explicit opt-in to expose |
| API auth | Bearer token from `.env` | Simple, sufficient for single-user, no session management needed |
| Auth enforcement | Refuse to start if exposed without token | Prevent accidental unauthenticated exposure |
| SSE auth | Query param fallback (`?token=`) | Some SSE clients can't set headers |
| CORS | Allow all origins when token is set | Token is the access control, not origin |
| Reconnection | Standard SSE `Last-Event-ID` with server-side buffer | Handles laptop sleep, network blips without custom protocol |
| TLS | Optional, not required with encrypted tunnels | Defense in depth, not primary security layer |
| Transport recommendation | WireGuard now, Headscale later | Fastest path to testing; best long-term UX without SSO |
| Transport coupling | None — Carson is transport-agnostic | Future-proof, no vendor lock-in at the application layer |
