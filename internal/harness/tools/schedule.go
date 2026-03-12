package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/llm"
	"github.com/allofher/carson/internal/scheduler"
)

// ScheduleEvent returns the schedule_event tool definition.
// parentChain is the chain from the current execution context (nil for top-level).
func ScheduleEvent(sched *scheduler.Scheduler, parentChain []string) harness.ToolDef {
	return harness.ToolDef{
		Schema: llm.Tool{
			Name:        "schedule_event",
			Description: "Schedule a future prompt to be delivered to the agent at a specific time. The prompt will re-enter the agent harness as if it were a new event. Use this to set up follow-up actions, recurring checks, or any deferred work.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"at": {
						"type": "string",
						"description": "When to fire. Accepts ISO-8601 datetime (e.g. '2026-03-06T10:35:00') or relative shorthand (e.g. '+30m', '+2h', 'tomorrow 09:00')."
					},
					"prompt": {
						"type": "string",
						"description": "The natural-language prompt that will be delivered to the agent when this event fires. Should be specific and self-contained."
					},
					"context": {
						"type": "object",
						"description": "Structured key-value data attached to the prompt. Passed verbatim to the agent at fire time. Use this for IDs, URLs, file paths, or any data the future prompt will need.",
						"additionalProperties": true
					},
					"recurrence": {
						"type": "string",
						"description": "Optional cron expression for recurring events (e.g. '0 9 * * 1-5' for weekday mornings). If set, the event re-schedules itself after each execution. Omit for one-shot events."
					},
					"max_retries": {
						"type": "integer",
						"description": "How many times to retry if the scheduled execution fails. 0 = no retries.",
						"default": 2
					},
					"expire_after": {
						"type": "string",
						"description": "ISO-8601 datetime after which this event should be silently dropped. Safety net for one-shot events that become stale."
					}
				},
				"required": ["at", "prompt"]
			}`),
		},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var req scheduler.ScheduleRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			if req.Prompt == "" {
				return "", fmt.Errorf("prompt is required")
			}

			result, err := sched.Schedule(req, parentChain)
			if err != nil {
				return "", err
			}

			out, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("marshaling result: %w", err)
			}
			return string(out), nil
		},
	}
}

// ListScheduledEvents returns the list_scheduled_events tool definition.
func ListScheduledEvents(sched *scheduler.Scheduler) harness.ToolDef {
	return harness.ToolDef{
		Schema: llm.Tool{
			Name:        "list_scheduled_events",
			Description: "List all pending scheduled events, optionally filtered by status or time range.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"status": {
						"type": "string",
						"enum": ["pending", "executing", "completed", "failed", "all"],
						"default": "pending"
					}
				}
			}`),
		},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			if params.Status == "" {
				params.Status = "pending"
			}

			bundles, err := sched.List(params.Status)
			if err != nil {
				return "", err
			}

			// Return a summary rather than full bundles.
			type eventSummary struct {
				ID         string `json:"id"`
				FiresAt    string `json:"fires_at"`
				Status     string `json:"status"`
				Prompt     string `json:"prompt"`
				Recurrence string `json:"recurrence,omitempty"`
			}

			summaries := make([]eventSummary, len(bundles))
			for i, b := range bundles {
				summaries[i] = eventSummary{
					ID:         b.ID,
					FiresAt:    b.FiresAt.Format("2006-01-02T15:04:05Z07:00"),
					Status:     string(b.Status),
					Prompt:     b.Prompt,
					Recurrence: b.Recurrence,
				}
			}

			out, err := json.Marshal(summaries)
			if err != nil {
				return "", fmt.Errorf("marshaling events: %w", err)
			}
			return string(out), nil
		},
	}
}

// CancelScheduledEvent returns the cancel_scheduled_event tool definition.
func CancelScheduledEvent(sched *scheduler.Scheduler) harness.ToolDef {
	return harness.ToolDef{
		Schema: llm.Tool{
			Name:        "cancel_scheduled_event",
			Description: "Cancel a pending scheduled event by ID. Removes the crontab entry and marks the bundle as cancelled.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"id": {
						"type": "string",
						"description": "The event ID to cancel (e.g. 'evt_abc123')."
					}
				},
				"required": ["id"]
			}`),
		},
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			if params.ID == "" {
				return "", fmt.Errorf("id is required")
			}

			if err := sched.Cancel(params.ID); err != nil {
				return "", err
			}

			return fmt.Sprintf(`{"cancelled": true, "id": %q}`, params.ID), nil
		},
	}
}
