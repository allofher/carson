package scheduler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Safety limits — defaults from the design doc.
const (
	DefaultMaxChainDepth    = 10
	DefaultMaxPendingEvents = 50
	DefaultMinScheduleDelay = 60 * time.Second
	DefaultMaxScheduleHoriz = 7 * 24 * time.Hour // 7 days
	DefaultArchiveTTL       = 30 * 24 * time.Hour // 30 days
)

// RetryBackoffs is the exponential backoff sequence for retries.
var RetryBackoffs = []time.Duration{2 * time.Minute, 8 * time.Minute, 32 * time.Minute}

// Scheduler manages the lifecycle of scheduled prompts.
type Scheduler struct {
	store   *BundleStore
	crontab *CrontabManager
	logger  *slog.Logger

	MaxChainDepth    int
	MaxPendingEvents int
	MinScheduleDelay time.Duration
	MaxScheduleHoriz time.Duration
}

// New creates a Scheduler with the given store, crontab manager, and logger.
func New(store *BundleStore, crontab *CrontabManager, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		store:            store,
		crontab:          crontab,
		logger:           logger,
		MaxChainDepth:    DefaultMaxChainDepth,
		MaxPendingEvents: DefaultMaxPendingEvents,
		MinScheduleDelay: DefaultMinScheduleDelay,
		MaxScheduleHoriz: DefaultMaxScheduleHoriz,
	}
}

// ScheduleRequest is the input for creating a new scheduled event.
type ScheduleRequest struct {
	At          string          `json:"at"`
	Prompt      string          `json:"prompt"`
	Context     json.RawMessage `json:"context,omitempty"`
	Recurrence  string          `json:"recurrence,omitempty"`
	MaxRetries  int             `json:"max_retries"`
	ExpireAfter string          `json:"expire_after,omitempty"`
}

// ScheduleResult is returned to the agent after scheduling.
type ScheduleResult struct {
	Scheduled  bool   `json:"scheduled"`
	ID         string `json:"id"`
	FiresAt    string `json:"fires_at"`
	CronEntry  string `json:"cron_entry,omitempty"`
	BundlePath string `json:"bundle_path,omitempty"`
}

// Schedule creates a new scheduled prompt from the agent's request.
// parentChain is the chain array from the calling event (nil for top-level).
func (s *Scheduler) Schedule(req ScheduleRequest, parentChain []string) (*ScheduleResult, error) {
	// Parse the firing time.
	firesAt, err := parseTime(req.At)
	if err != nil {
		return nil, fmt.Errorf("invalid 'at' value %q: %w", req.At, err)
	}

	// Validate safety limits.
	if err := s.validateSchedule(firesAt, parentChain); err != nil {
		return nil, err
	}

	// Parse optional expiry.
	var expireAfter *time.Time
	if req.ExpireAfter != "" {
		t, err := time.Parse(time.RFC3339, req.ExpireAfter)
		if err != nil {
			return nil, fmt.Errorf("invalid expire_after: %w", err)
		}
		expireAfter = &t
	}

	id := generateID()
	chain := append([]string{}, parentChain...)
	chain = append(chain, id)

	var parentID string
	if len(parentChain) > 0 {
		parentID = parentChain[len(parentChain)-1]
	}

	bundle := &Bundle{
		ID:            id,
		CreatedAt:     time.Now(),
		FiresAt:       firesAt,
		Status:        StatusPending,
		Prompt:        req.Prompt,
		Context:       req.Context,
		Recurrence:    req.Recurrence,
		MaxRetries:    req.MaxRetries,
		RetryCount:    0,
		ExpireAfter:   expireAfter,
		ParentEventID: parentID,
		Chain:         chain,
	}

	// Write bundle to disk.
	if err := s.store.Save(bundle); err != nil {
		return nil, fmt.Errorf("saving bundle: %w", err)
	}

	// Write crontab entry.
	if bundle.Recurrence != "" {
		if err := s.crontab.AddRecurring(bundle); err != nil {
			s.store.Delete(id) // clean up on failure
			return nil, fmt.Errorf("writing crontab entry: %w", err)
		}
	} else {
		if err := s.crontab.AddEvent(bundle); err != nil {
			s.store.Delete(id) // clean up on failure
			return nil, fmt.Errorf("writing crontab entry: %w", err)
		}
	}

	s.logger.Info("scheduled event",
		"id", id,
		"fires_at", firesAt.Format(time.RFC3339),
		"chain_depth", len(chain),
	)

	return &ScheduleResult{
		Scheduled: true,
		ID:        id,
		FiresAt:   firesAt.Format(time.RFC3339),
	}, nil
}

// Cancel cancels a pending scheduled event.
func (s *Scheduler) Cancel(id string) error {
	bundle, err := s.store.Load(id)
	if err != nil {
		return err
	}
	if bundle.Status != StatusPending {
		return fmt.Errorf("cannot cancel event %s: status is %s", id, bundle.Status)
	}

	bundle.Status = StatusCancelled
	if err := s.store.Save(bundle); err != nil {
		return fmt.Errorf("updating bundle status: %w", err)
	}

	// Remove from active dir and crontab.
	s.store.Archive(id)
	s.crontab.Remove(id)

	s.logger.Info("cancelled event", "id", id)
	return nil
}

// List returns scheduled events, optionally filtered by status.
func (s *Scheduler) List(status string) ([]*Bundle, error) {
	return s.store.ListAll(Status(status))
}

// LoadForExecution loads a bundle and prepares it for execution.
// Returns nil if the bundle is expired (and handles cleanup).
func (s *Scheduler) LoadForExecution(id string) (*Bundle, error) {
	bundle, err := s.store.Load(id)
	if err != nil {
		return nil, err
	}

	if bundle.IsExpired() {
		bundle.Status = StatusExpired
		s.store.Save(bundle)
		s.store.Archive(id)
		s.crontab.Remove(id)
		s.logger.Info("event expired, dropping", "id", id)
		return nil, nil
	}

	bundle.Status = StatusExecuting
	s.store.Save(bundle)
	return bundle, nil
}

// MarkCompleted marks an event as completed and handles recurrence.
func (s *Scheduler) MarkCompleted(id string) error {
	bundle, err := s.store.Load(id)
	if err != nil {
		return err
	}

	if bundle.Recurrence != "" {
		// Re-register: reset status, compute next fire time.
		bundle.Status = StatusPending
		bundle.RetryCount = 0
		bundle.Chain = []string{bundle.ID} // fresh chain per recurrence
		s.store.Save(bundle)
		s.logger.Info("recurring event re-registered", "id", id)
		return nil
	}

	bundle.Status = StatusCompleted
	s.store.Save(bundle)
	s.store.Archive(id)
	s.crontab.Remove(id)
	s.logger.Info("event completed", "id", id)
	return nil
}

// MarkFailed handles a failed execution — retries with backoff or marks failed.
func (s *Scheduler) MarkFailed(id string, execErr error) error {
	bundle, err := s.store.Load(id)
	if err != nil {
		return err
	}

	bundle.RetryCount++
	s.logger.Warn("event execution failed",
		"id", id,
		"retry_count", bundle.RetryCount,
		"max_retries", bundle.MaxRetries,
		"error", execErr,
	)

	if bundle.RetryCount <= bundle.MaxRetries {
		// Reschedule with backoff.
		backoffIdx := bundle.RetryCount - 1
		if backoffIdx >= len(RetryBackoffs) {
			backoffIdx = len(RetryBackoffs) - 1
		}
		delay := RetryBackoffs[backoffIdx]
		bundle.FiresAt = time.Now().Add(delay)
		bundle.Status = StatusPending
		s.store.Save(bundle)

		// Update crontab with new fire time.
		s.crontab.Remove(id)
		s.crontab.AddEvent(bundle)

		s.logger.Info("event rescheduled for retry",
			"id", id,
			"fires_at", bundle.FiresAt.Format(time.RFC3339),
			"backoff", delay,
		)
		return nil
	}

	// Retries exhausted.
	bundle.Status = StatusFailed
	s.store.Save(bundle)
	s.store.Archive(id)
	s.crontab.Remove(id)
	s.logger.Error("event failed permanently", "id", id, "error", execErr)
	return nil
}

// Cleanup removes old completed bundles.
func (s *Scheduler) Cleanup(maxAge time.Duration) (int, error) {
	return s.store.CleanupCompleted(maxAge)
}

// validateSchedule checks all safety limits.
func (s *Scheduler) validateSchedule(firesAt time.Time, parentChain []string) error {
	now := time.Now()

	// Check minimum delay.
	delay := firesAt.Sub(now)
	if delay < s.MinScheduleDelay {
		return fmt.Errorf("schedule delay %v is below minimum %v", delay.Round(time.Second), s.MinScheduleDelay)
	}

	// Check maximum horizon.
	if delay > s.MaxScheduleHoriz {
		return fmt.Errorf("schedule horizon %v exceeds maximum %v — use recurrence for long-lived patterns",
			delay.Round(time.Hour), s.MaxScheduleHoriz)
	}

	// Check chain depth.
	if len(parentChain) >= s.MaxChainDepth {
		return fmt.Errorf("chain depth %d would exceed maximum %d", len(parentChain)+1, s.MaxChainDepth)
	}

	// Check pending count.
	count, err := s.store.PendingCount()
	if err != nil {
		return fmt.Errorf("checking pending count: %w", err)
	}
	if count >= s.MaxPendingEvents {
		return fmt.Errorf("pending event count %d has reached maximum %d", count, s.MaxPendingEvents)
	}

	return nil
}

// parseTime parses an "at" value — supports ISO-8601 and relative shorthands.
func parseTime(s string) (time.Time, error) {
	// Relative: +30m, +2h, +1h30m
	if strings.HasPrefix(s, "+") {
		d, err := time.ParseDuration(s[1:])
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing relative time: %w", err)
		}
		return time.Now().Add(d), nil
	}

	// "tomorrow HH:MM"
	if strings.HasPrefix(s, "tomorrow ") {
		parts := strings.TrimPrefix(s, "tomorrow ")
		t, err := time.Parse("15:04", parts)
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing time in 'tomorrow HH:MM': %w", err)
		}
		tomorrow := time.Now().AddDate(0, 0, 1)
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}

	// ISO-8601 with timezone.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// ISO-8601 without timezone (assume local).
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local); err == nil {
		return t, nil
	}

	// Just time today: "14:30"
	if t, err := time.Parse("15:04", s); err == nil {
		now := time.Now()
		candidate := time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local)
		if candidate.Before(now) {
			candidate = candidate.AddDate(0, 0, 1) // next day if past
		}
		return candidate, nil
	}

	return time.Time{}, fmt.Errorf("unrecognized time format %q — use ISO-8601 or relative (+30m)", s)
}

