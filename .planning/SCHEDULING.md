# Recursive Scheduled Events — Design Proposal

> **Audience:** Developer picking this up for implementation.
> **Status:** Decisions finalized — not yet implemented.

## The Problem

Carson's agent needs to do more than react to file changes and respond to direct prompts. It needs to **discover future events, schedule actions around them, and chain follow-up actions whose shape isn't known until runtime.**

A concrete example: the agent polls a calendar service, sees a meeting at 10:00–10:30, and decides on its own to check back at 10:35 for the transcript. When it retrieves the transcript, the agent decides what to do next — maybe it writes the transcript to the watch folder, maybe it also extracts action items into a TODO file, maybe it schedules *another* check for 11:00 to see if follow-up items were assigned. The key property is that **each scheduled callback re-enters the agent, and the agent's output can include scheduling more callbacks.** This is what makes it recursive.

## End-to-End Walkthrough

Before diving into components, here is the full lifecycle of one recursive scheduling chain:

```mermaid
sequenceDiagram
    participant Cron as System Crontab
    participant Carson as Carson Daemon
    participant Router as Event Router
    participant Harness as Agent Harness
    participant LLM as LLM Provider
    participant Cal as Calendar API
    participant Meet as Meeting Service API
    participant FS as Watch Folder

    Note over Cron,Carson: 1. Scheduled poll fires
    Cron->>Carson: carson run-scheduled --task poll-calendar
    Carson->>Router: ScheduledEvent{task: "poll-calendar"}
    Router->>Harness: forward to agent

    Note over Harness,Cal: 2. Agent discovers a meeting
    Harness->>LLM: system prompt + event context
    LLM-->>Harness: tool_call: poll_calendar_service()
    Harness->>Cal: GET /events?timeMin=now&timeMax=+24h
    Cal-->>Harness: [{summary: "Standup", start: "10:00", end: "10:30", meetingUrl: "..."}]
    Harness-->>LLM: tool_result: [meeting data]

    Note over LLM,Harness: 3. Agent decides to schedule a follow-up
    LLM-->>Harness: tool_call: schedule_event({at: "10:35", prompt: "Retrieve transcript for Standup meeting from Google Meet", context: {meetingUrl: "..."}})
    Harness->>Carson: create ScheduledPrompt
    Carson->>Carson: compile prompt bundle → /var/carson/scheduled/evt_abc123.json
    Carson->>Cron: write crontab entry: "35 10 * * * carson run-scheduled --event evt_abc123"
    Harness-->>LLM: tool_result: {scheduled: true, id: "evt_abc123", fires_at: "10:35"}
    LLM-->>Harness: "Done. I've scheduled a transcript retrieval for 10:35."

    Note over Cron,Carson: 4. Follow-up fires at 10:35
    Cron->>Carson: carson run-scheduled --event evt_abc123
    Carson->>Carson: load /var/carson/scheduled/evt_abc123.json
    Carson->>Router: ScheduledEvent{prompt: "Retrieve transcript...", context: {meetingUrl: "..."}}
    Router->>Harness: forward to agent

    Note over Harness,Meet: 5. Agent retrieves the transcript
    Harness->>LLM: system prompt + scheduled prompt + context
    LLM-->>Harness: tool_call: fetch_meeting_transcript({meetingUrl: "..."})
    Harness->>Meet: GET /meetings/{id}/transcript
    Meet-->>Harness: {transcript: "Alice: Let's ship the feature by Friday..."}
    Harness-->>LLM: tool_result: {transcript: "..."}

    Note over LLM,FS: 6. Agent decides what to do with the output
    LLM-->>Harness: tool_call: write_file({path: "meetings/2026-03-06-standup.md", content: "..."})
    Harness->>FS: write transcript file
    LLM-->>Harness: tool_call: write_file({path: "TODO.md", content: "- [ ] Ship feature by Friday\n- [ ] ..."})
    Harness->>FS: append action items

    Note over LLM,Harness: 7. Agent optionally schedules another follow-up
    LLM-->>Harness: tool_call: schedule_event({at: "17:00", prompt: "Check if TODO items from Standup were addressed today", context: {todoFile: "TODO.md"}})
    Harness->>Carson: create another ScheduledPrompt
    Carson->>Cron: write crontab entry for 17:00
    LLM-->>Harness: "Transcript saved. Action items extracted. I'll check back at 5 PM."
```

## Core Concept: The Scheduled Prompt

The unit of work is a **ScheduledPrompt** — a self-contained bundle that tells Carson: "At time T, wake the agent with this prompt and this context." When the agent runs, its output may include creating *more* ScheduledPrompts. That's the recursion.

```mermaid
graph TD
    A[Agent receives event] --> B{Agent decides action}
    B -->|Immediate| C[Execute tool / write file / call API]
    B -->|Deferred| D[Create ScheduledPrompt]
    D --> E[Compile prompt bundle to disk]
    E --> F[Write crontab entry]
    F --> G[Cron fires at scheduled time]
    G --> H[Carson loads prompt bundle]
    H --> A

    style A fill:#e1f0ff,stroke:#4a90d9
    style D fill:#fff3cd,stroke:#d4a017
    style G fill:#d4edda,stroke:#28a745
```

## The `schedule_event` Tool

This is the dedicated tool the agent calls to schedule a future action. It is the only way the agent can write to the crontab.

### Tool Schema

```json
{
  "name": "schedule_event",
  "description": "Schedule a future prompt to be delivered to the agent at a specific time. The prompt will re-enter the agent harness as if it were a new event. Use this to set up follow-up actions, recurring checks, or any deferred work.",
  "parameters": {
    "type": "object",
    "properties": {
      "at": {
        "type": "string",
        "description": "When to fire. Accepts ISO-8601 datetime (e.g. '2026-03-06T10:35:00') or relative shorthand (e.g. '+30m', '+2h', 'tomorrow 09:00')."
      },
      "prompt": {
        "type": "string",
        "description": "The natural-language prompt that will be delivered to the agent when this event fires. Should be specific and self-contained — the agent that receives it may not have the current conversation context."
      },
      "context": {
        "type": "object",
        "description": "Structured key-value data attached to the prompt. Passed verbatim to the agent at fire time. Use this for IDs, URLs, file paths, or any data the future prompt will need.",
        "additionalProperties": true
      },
      "recurrence": {
        "type": "string",
        "description": "Optional cron expression for recurring events (e.g. '0 9 * * 1-5' for weekday mornings). If set, the event re-schedules itself after each execution. Omit for one-shot events.",
        "default": null
      },
      "max_retries": {
        "type": "integer",
        "description": "How many times to retry if the scheduled execution fails (e.g. network error fetching a transcript). 0 = no retries.",
        "default": 2
      },
      "expire_after": {
        "type": "string",
        "description": "ISO-8601 datetime after which this event should be silently dropped rather than executed. Safety net for one-shot events that become stale.",
        "default": null
      }
    },
    "required": ["at", "prompt"]
  }
}
```

### Tool Return Value

```json
{
  "scheduled": true,
  "id": "evt_abc123",
  "fires_at": "2026-03-06T10:35:00-05:00",
  "cron_entry": "35 10 6 3 * carson run-scheduled --event evt_abc123",
  "bundle_path": "/var/carson/scheduled/evt_abc123.json"
}
```

## The ScheduledPrompt Bundle

When the agent calls `schedule_event`, Carson compiles a JSON file to disk. This is the source of truth — the crontab entry is just a trigger that points to it.

```json
{
  "id": "evt_abc123",
  "created_at": "2026-03-06T09:12:44Z",
  "fires_at": "2026-03-06T10:35:00-05:00",
  "status": "pending",
  "prompt": "Retrieve the transcript for the 'Standup' meeting that ended at 10:30 from Google Meet and save it to the watch folder. Extract any action items into TODO.md.",
  "context": {
    "meetingUrl": "https://meet.google.com/abc-defg-hij",
    "meetingSummary": "Standup",
    "meetingEnd": "2026-03-06T10:30:00-05:00"
  },
  "recurrence": null,
  "max_retries": 2,
  "retry_count": 0,
  "expire_after": "2026-03-06T18:00:00-05:00",
  "parent_event_id": null,
  "chain": ["evt_abc123"]
}
```

### Key Fields

| Field | Purpose |
|---|---|
| `prompt` | The full natural-language instruction for the agent. Must be self-contained. |
| `context` | Structured data the agent will need. Passed into the harness alongside the prompt. |
| `recurrence` | If set, the event is re-registered after execution instead of being marked complete. |
| `parent_event_id` | Links to the event that spawned this one. Enables chain tracing. |
| `chain` | Ordered list of all event IDs in this recursive chain. Enables depth limiting. |

## How Cron Entries Work

Carson manages a namespaced block in the user's crontab:

```crontab
# >>> CARSON MANAGED — DO NOT EDIT <<<
# [evt_abc123] Retrieve transcript for Standup meeting
35 10 6 3 * /usr/local/bin/carson run-scheduled --event evt_abc123
# [poll-calendar] Regular calendar poll
*/30 9-17 * * 1-5 /usr/local/bin/carson run-scheduled --task poll-calendar
# <<< CARSON MANAGED >>>
```

The `run-scheduled` subcommand:
1. Loads the bundle from disk.
2. Checks `expire_after` — drops silently if stale.
3. Sends the prompt + context into the Event Router.
4. On success: marks the bundle `completed` (or re-registers if `recurrence` is set).
5. On failure: increments `retry_count`, re-schedules with backoff if under `max_retries`.
6. Cleans up the crontab entry for completed one-shot events.

## Recursion & Safety

The recursive property — an agent invocation producing more scheduled invocations — needs guardrails.

```mermaid
flowchart TD
    A[Scheduled event fires] --> B[Load prompt bundle]
    B --> C{Expired?}
    C -->|Yes| D[Drop silently, clean up]
    C -->|No| E{Chain depth < MAX_CHAIN_DEPTH?}
    E -->|No| F[Log warning, execute but disable further scheduling]
    E -->|Yes| G[Execute: send prompt to agent harness]
    G --> H{Agent calls schedule_event?}
    H -->|No| I[Mark complete, clean up cron entry]
    H -->|Yes| J[Create child bundle]
    J --> K[Set parent_event_id, append to chain]
    K --> L[Write crontab entry for child]
    L --> I
    G --> M{Execution failed?}
    M -->|Yes| N{retry_count < max_retries?}
    N -->|Yes| O[Increment retry, reschedule with backoff]
    N -->|No| P[Mark failed, log error, clean up]

    style D fill:#f8d7da,stroke:#dc3545
    style F fill:#fff3cd,stroke:#d4a017
    style I fill:#d4edda,stroke:#28a745
    style P fill:#f8d7da,stroke:#dc3545
```

### Safety Limits

| Limit | Default | Purpose |
|---|---|---|
| `MAX_CHAIN_DEPTH` | 10 | Prevents infinite recursive scheduling. After this depth, the agent can still execute but `schedule_event` calls are rejected. |
| `MAX_PENDING_EVENTS` | 50 | Total number of pending scheduled events across all chains. Prevents runaway accumulation. |
| `MIN_SCHEDULE_DELAY` | 60 seconds | The agent cannot schedule something less than 60s in the future. Prevents tight loops. |
| `MAX_SCHEDULE_HORIZON` | 7 days | Events cannot be scheduled more than 7 days out. Forces the agent to use `recurrence` for long-lived patterns. |
| `RETRY_BACKOFF` | Exponential: 2m, 8m, 32m | Failed events are retried with increasing delay. |

## State Machine for a ScheduledPrompt

```mermaid
stateDiagram-v2
    [*] --> pending : schedule_event called
    pending --> executing : cron fires, bundle loaded
    pending --> expired : expire_after passed
    executing --> completed : agent finished successfully
    executing --> failed : execution error, retries exhausted
    executing --> pending : execution error, retries remaining (reschedule)
    completed --> [*]
    failed --> [*]
    expired --> [*]

    completed --> pending : recurrence set (re-register)
```

## Data Flow: Where Things Live

```mermaid
graph LR
    subgraph Disk
        A["/var/carson/scheduled/*.json<br/>Prompt bundles"]
        B["/var/carson/scheduled/completed/<br/>Archived bundles"]
        C["~/.carson/config<br/>Safety limits"]
    end

    subgraph Crontab
        D["User crontab<br/>CARSON MANAGED block"]
    end

    subgraph Runtime
        E["Event Router"]
        F["Agent Harness"]
        G["schedule_event tool"]
    end

    G -->|writes| A
    G -->|writes| D
    D -->|triggers| E
    E -->|loads| A
    E -->|forwards| F
    F -->|may call| G
    A -->|on completion| B
```

## Example: Full Meeting Transcript Chain

To make this concrete, here is exactly what happens when the calendar poll discovers a meeting:

### Step 1 — Calendar poll (recurring, runs every 30 min during work hours)

The agent receives the `poll-calendar` task. It calls the calendar skill, gets back a list of upcoming meetings, and schedules a follow-up for each one:

```
Agent prompt: "Poll the user's calendar for upcoming events in the next 2 hours."
Agent response:
  → tool_call: poll_calendar_service()
  → result: [{summary: "Standup", end: "10:30", meetingUrl: "..."}]
  → tool_call: schedule_event({
      at: "2026-03-06T10:35:00",
      prompt: "The user's 'Standup' meeting just ended. Retrieve the transcript from Google Meet and save it. Extract action items into TODO.md.",
      context: {meetingUrl: "...", meetingSummary: "Standup"},
      expire_after: "2026-03-06T18:00:00"
    })
  → "Scheduled transcript retrieval for 10:35."
```

### Step 2 — Transcript retrieval (one-shot, fires at 10:35)

```
Agent prompt: "The user's 'Standup' meeting just ended. Retrieve the transcript..."
Agent response:
  → tool_call: fetch_meeting_transcript({meetingUrl: "..."})
  → result: {transcript: "Alice: Let's ship by Friday..."}
  → tool_call: write_file({path: "meetings/2026-03-06-standup.md", content: "# Standup\n\n..."})
  → tool_call: write_file({path: "TODO.md", content: "- [ ] Ship feature by Friday\n..."})
  → tool_call: schedule_event({
      at: "2026-03-06T17:00:00",
      prompt: "End-of-day check: review TODO.md for items from today's Standup. Summarize what was completed and what carries over.",
      context: {todoFile: "TODO.md", originMeeting: "Standup"}
    })
  → "Transcript saved. Action items extracted. Scheduled EOD review for 5 PM."
```

### Step 3 — EOD review (one-shot, fires at 17:00)

```
Agent prompt: "End-of-day check: review TODO.md for items from today's Standup..."
Agent response:
  → tool_call: read_file({path: "TODO.md"})
  → tool_call: write_file({path: "daily-summary/2026-03-06.md", content: "## Standup Follow-up\n\nCompleted: ...\nCarries over: ..."})
  → tool_call: frontend_command({action: "notify", message: "Daily summary ready."})
  → (no further schedule_event — chain ends naturally)
```

## The `list_scheduled_events` Tool

The agent also needs visibility into what's already scheduled to avoid duplicates and reason about its queue.

```json
{
  "name": "list_scheduled_events",
  "description": "List all pending scheduled events, optionally filtered by status or time range.",
  "parameters": {
    "type": "object",
    "properties": {
      "status": {
        "type": "string",
        "enum": ["pending", "executing", "completed", "failed", "all"],
        "default": "pending"
      },
      "from": {
        "type": "string",
        "description": "ISO-8601 datetime. Only return events firing after this time."
      },
      "to": {
        "type": "string",
        "description": "ISO-8601 datetime. Only return events firing before this time."
      }
    }
  }
}
```

## The `cancel_scheduled_event` Tool

```json
{
  "name": "cancel_scheduled_event",
  "description": "Cancel a pending scheduled event by ID. Removes the crontab entry and marks the bundle as cancelled.",
  "parameters": {
    "type": "object",
    "properties": {
      "id": {
        "type": "string",
        "description": "The event ID to cancel (e.g. 'evt_abc123')."
      }
    },
    "required": ["id"]
  }
}
```

## Implementation Notes for the Developer

### What to build first

1. **ScheduledPrompt bundle format** — Define the JSON schema, write serialization/deserialization, add a storage directory with cleanup.
2. **`schedule_event` tool handler** — Accept the tool call, validate parameters against safety limits, write the bundle, write the crontab entry.
3. **`run-scheduled` CLI subcommand** — Load a bundle by ID, check expiry, send prompt+context to the Event Router, handle success/failure/retry.
4. **Crontab manager** — Read/write/delete entries within the `CARSON MANAGED` block. Must be idempotent and safe for concurrent access (file lock).
5. **`list_scheduled_events` / `cancel_scheduled_event`** — Read from the bundles directory, update status, remove crontab entries.
6. **Chain tracking and depth limiting** — Propagate `parent_event_id` and `chain` array through child events. Enforce `MAX_CHAIN_DEPTH`.

### Platform considerations

- **Crontab access:** Both macOS and Linux support `crontab -l` / `crontab -` for reading and writing. Carson should use these commands rather than editing crontab files directly.
- **File locking:** The bundles directory needs a lock file to prevent races between the daemon writing new bundles and `run-scheduled` reading/updating them.
- **Timezone handling:** All `fires_at` times must be stored with timezone offset. Crontab entries use system-local time. Carson must convert correctly.

### What NOT to build

- **A custom scheduler daemon.** Use cron. It's battle-tested, survives reboots, and the user can inspect it with standard tools.
- **Persistent conversation context across scheduled events.** Each scheduled prompt starts a fresh agent context. The `prompt` and `context` fields must carry everything the agent needs. This is intentional — it keeps scheduled events self-contained and debuggable.
- **A UI for managing scheduled events.** That's the frontend repo's job. Carson just exposes the tools and the bundle files.

### Decisions

All scheduling questions have been resolved. See [QUESTIONS.md](QUESTIONS.md) under **Recursive Scheduling** for the full decision log. Key decisions:

- **Fresh chain per recurrence** — Each recurring event gets a fresh `chain`. Logically independent.
- **No mutable pending events** — Cancel and re-create. Simpler to implement, easier to audit.
- **Archive completed bundles** — Archive to `completed/` with TTL-based cleanup (30 days).
