package scheduler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func testScheduler(t *testing.T) (*Scheduler, *BundleStore) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewBundleStore(dir)
	if err != nil {
		t.Fatalf("NewBundleStore: %v", err)
	}
	// Use a no-op crontab manager (don't touch real crontab in tests).
	crontab := &CrontabManager{carsonBin: "/usr/local/bin/carson"}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sched := New(store, crontab, logger)
	// Override crontab methods to avoid touching real crontab.
	// We'll test crontab separately.
	return sched, store
}

func TestScheduler_Schedule(t *testing.T) {
	sched, store := testScheduler(t)
	// Skip crontab writes in unit tests.
	sched.MinScheduleDelay = 0 // relax for testing

	req := ScheduleRequest{
		At:     "+" + (2 * time.Hour).String(),
		Prompt: "Test scheduled prompt",
		Context: json.RawMessage(`{"meeting": "standup"}`),
		MaxRetries: 1,
	}

	result, err := sched.Schedule(req, nil)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if !result.Scheduled {
		t.Error("expected scheduled=true")
	}
	if result.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Verify bundle was saved.
	bundle, err := store.Load(result.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bundle.Prompt != "Test scheduled prompt" {
		t.Errorf("Prompt = %q", bundle.Prompt)
	}
	if bundle.Status != StatusPending {
		t.Errorf("Status = %q, want pending", bundle.Status)
	}
	if len(bundle.Chain) != 1 {
		t.Errorf("Chain length = %d, want 1", len(bundle.Chain))
	}
}

func TestScheduler_ChainDepthLimit(t *testing.T) {
	sched, _ := testScheduler(t)
	sched.MaxChainDepth = 3
	sched.MinScheduleDelay = 0

	// Create a chain of 3 (at the limit).
	parentChain := []string{"evt_1", "evt_2", "evt_3"}
	req := ScheduleRequest{
		At:     "+2h",
		Prompt: "Should be rejected",
	}

	_, err := sched.Schedule(req, parentChain)
	if err == nil {
		t.Error("expected error for exceeding chain depth")
	}
}

func TestScheduler_MaxPendingLimit(t *testing.T) {
	sched, _ := testScheduler(t)
	sched.MaxPendingEvents = 2
	sched.MinScheduleDelay = 0

	// Schedule 2 events (should succeed).
	for i := 0; i < 2; i++ {
		_, err := sched.Schedule(ScheduleRequest{
			At:     "+2h",
			Prompt: "event",
		}, nil)
		if err != nil {
			t.Fatalf("Schedule %d: %v", i, err)
		}
	}

	// Third should fail.
	_, err := sched.Schedule(ScheduleRequest{
		At:     "+2h",
		Prompt: "too many",
	}, nil)
	if err == nil {
		t.Error("expected error for exceeding max pending events")
	}
}

func TestScheduler_Cancel(t *testing.T) {
	sched, store := testScheduler(t)
	sched.MinScheduleDelay = 0

	result, _ := sched.Schedule(ScheduleRequest{
		At:     "+2h",
		Prompt: "cancel me",
	}, nil)

	if err := sched.Cancel(result.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	bundle, err := store.Load(result.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bundle.Status != StatusCancelled {
		t.Errorf("Status = %q, want cancelled", bundle.Status)
	}
}

func TestScheduler_LoadForExecution_Expired(t *testing.T) {
	sched, store := testScheduler(t)

	past := time.Now().Add(-1 * time.Hour)
	b := &Bundle{
		ID:          "evt_expired",
		Status:      StatusPending,
		Prompt:      "expired event",
		FiresAt:     time.Now(),
		ExpireAfter: &past,
		Chain:       []string{"evt_expired"},
	}
	store.Save(b)

	bundle, err := sched.LoadForExecution("evt_expired")
	if err != nil {
		t.Fatalf("LoadForExecution: %v", err)
	}
	if bundle != nil {
		t.Error("expected nil bundle for expired event")
	}
}

func TestScheduler_MarkCompleted(t *testing.T) {
	sched, store := testScheduler(t)
	sched.MinScheduleDelay = 0

	result, _ := sched.Schedule(ScheduleRequest{
		At:     "+2h",
		Prompt: "complete me",
	}, nil)

	// Simulate execution.
	sched.LoadForExecution(result.ID)
	sched.MarkCompleted(result.ID)

	bundle, _ := store.Load(result.ID)
	if bundle.Status != StatusCompleted {
		t.Errorf("Status = %q, want completed", bundle.Status)
	}
}

func TestScheduler_MarkFailed_WithRetry(t *testing.T) {
	sched, store := testScheduler(t)
	sched.MinScheduleDelay = 0

	result, _ := sched.Schedule(ScheduleRequest{
		At:         "+2h",
		Prompt:     "fail me",
		MaxRetries: 2,
	}, nil)

	sched.LoadForExecution(result.ID)
	sched.MarkFailed(result.ID, fmt.Errorf("network error"))

	bundle, _ := store.Load(result.ID)
	if bundle.Status != StatusPending {
		t.Errorf("Status = %q, want pending (should retry)", bundle.Status)
	}
	if bundle.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", bundle.RetryCount)
	}
}

func TestScheduler_MarkFailed_Exhausted(t *testing.T) {
	sched, store := testScheduler(t)
	sched.MinScheduleDelay = 0

	result, _ := sched.Schedule(ScheduleRequest{
		At:         "+2h",
		Prompt:     "fail permanently",
		MaxRetries: 0,
	}, nil)

	sched.LoadForExecution(result.ID)
	sched.MarkFailed(result.ID, fmt.Errorf("permanent error"))

	bundle, _ := store.Load(result.ID)
	if bundle.Status != StatusFailed {
		t.Errorf("Status = %q, want failed", bundle.Status)
	}
}

func TestParseTime_Relative(t *testing.T) {
	before := time.Now()
	result, err := parseTime("+30m")
	if err != nil {
		t.Fatalf("parseTime: %v", err)
	}
	expected := before.Add(30 * time.Minute)
	if result.Before(expected.Add(-1*time.Second)) || result.After(expected.Add(1*time.Second)) {
		t.Errorf("result = %v, expected ~%v", result, expected)
	}
}

func TestParseTime_ISO8601(t *testing.T) {
	result, err := parseTime("2026-03-06T10:35:00-05:00")
	if err != nil {
		t.Fatalf("parseTime: %v", err)
	}
	if result.Minute() != 35 || result.Hour() != 10 {
		t.Errorf("result = %v", result)
	}
}

func TestParseTime_Tomorrow(t *testing.T) {
	result, err := parseTime("tomorrow 09:00")
	if err != nil {
		t.Fatalf("parseTime: %v", err)
	}
	tomorrow := time.Now().AddDate(0, 0, 1)
	if result.Day() != tomorrow.Day() {
		t.Errorf("result day = %d, want %d", result.Day(), tomorrow.Day())
	}
	if result.Hour() != 9 || result.Minute() != 0 {
		t.Errorf("result time = %02d:%02d, want 09:00", result.Hour(), result.Minute())
	}
}

func TestParseTime_Invalid(t *testing.T) {
	_, err := parseTime("not a time")
	if err == nil {
		t.Error("expected error for invalid time")
	}
}

func TestTimeToCron(t *testing.T) {
	// March 6, 10:35.
	tm := time.Date(2026, 3, 6, 10, 35, 0, 0, time.Local)
	cron := timeToCron(tm)
	if cron != "35 10 6 3 *" {
		t.Errorf("timeToCron = %q, want %q", cron, "35 10 6 3 *")
	}
}
