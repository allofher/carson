package scheduler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the lifecycle state of a scheduled prompt.
type Status string

const (
	StatusPending   Status = "pending"
	StatusExecuting Status = "executing"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// Bundle is a self-contained scheduled prompt that tells Carson:
// "At time T, wake the agent with this prompt and this context."
type Bundle struct {
	ID            string          `json:"id"`
	CreatedAt     time.Time       `json:"created_at"`
	FiresAt       time.Time       `json:"fires_at"`
	Status        Status          `json:"status"`
	Prompt        string          `json:"prompt"`
	Context       json.RawMessage `json:"context,omitempty"`
	Recurrence    string          `json:"recurrence,omitempty"`
	MaxRetries    int             `json:"max_retries"`
	RetryCount    int             `json:"retry_count"`
	ExpireAfter   *time.Time      `json:"expire_after,omitempty"`
	ParentEventID string          `json:"parent_event_id,omitempty"`
	Chain         []string        `json:"chain"`
}

// IsExpired returns true if the bundle has passed its expiry time.
func (b *Bundle) IsExpired() bool {
	if b.ExpireAfter == nil {
		return false
	}
	return time.Now().After(*b.ExpireAfter)
}

// generateID creates a new event ID like "evt_a1b2c3d4".
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "evt_" + hex.EncodeToString(b)
}

// BundleStore manages scheduled prompt bundles on disk.
type BundleStore struct {
	dir          string // e.g. ~/.config/carson/scheduled
	completedDir string // e.g. ~/.config/carson/scheduled/completed
}

// NewBundleStore creates a store rooted at the given directory.
// It ensures the directory structure exists.
func NewBundleStore(dir string) (*BundleStore, error) {
	completedDir := filepath.Join(dir, "completed")
	if err := os.MkdirAll(completedDir, 0755); err != nil {
		return nil, fmt.Errorf("creating scheduled directory: %w", err)
	}
	return &BundleStore{dir: dir, completedDir: completedDir}, nil
}

func (s *BundleStore) bundlePath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *BundleStore) completedPath(id string) string {
	return filepath.Join(s.completedDir, id+".json")
}

// Save writes a bundle to disk as JSON. Always writes to the active directory.
// Use Archive to move a bundle to the completed directory.
func (s *BundleStore) Save(b *Bundle) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling bundle: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(s.bundlePath(b.ID), data, 0644); err != nil {
		return fmt.Errorf("writing bundle: %w", err)
	}
	return nil
}

// Load reads a bundle from disk by ID. Checks the active directory first,
// then the completed directory.
func (s *BundleStore) Load(id string) (*Bundle, error) {
	for _, path := range []string{s.bundlePath(id), s.completedPath(id)} {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading bundle: %w", err)
		}
		var b Bundle
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("parsing bundle %s: %w", id, err)
		}
		return &b, nil
	}
	return nil, fmt.Errorf("bundle %s not found", id)
}

// Archive moves a bundle from the active directory to the completed directory.
func (s *BundleStore) Archive(id string) error {
	src := s.bundlePath(id)
	dst := s.completedPath(id)
	if err := os.Rename(src, dst); err != nil {
		if os.IsNotExist(err) {
			return nil // already archived or never existed
		}
		return fmt.Errorf("archiving bundle: %w", err)
	}
	return nil
}

// Delete removes a bundle from both active and completed directories.
func (s *BundleStore) Delete(id string) error {
	for _, path := range []string{s.bundlePath(id), s.completedPath(id)} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting bundle: %w", err)
		}
	}
	return nil
}

// ListPending returns all bundles in the active directory with status "pending".
func (s *BundleStore) ListPending() ([]*Bundle, error) {
	return s.listDir(s.dir, func(b *Bundle) bool {
		return b.Status == StatusPending
	})
}

// ListAll returns all bundles from both active and completed directories,
// optionally filtered by status.
func (s *BundleStore) ListAll(status Status) ([]*Bundle, error) {
	var filter func(*Bundle) bool
	if status != "" && status != "all" {
		filter = func(b *Bundle) bool { return b.Status == status }
	}

	active, err := s.listDir(s.dir, filter)
	if err != nil {
		return nil, err
	}
	completed, err := s.listDir(s.completedDir, filter)
	if err != nil {
		return nil, err
	}
	return append(active, completed...), nil
}

// PendingCount returns the number of pending bundles.
func (s *BundleStore) PendingCount() (int, error) {
	pending, err := s.ListPending()
	if err != nil {
		return 0, err
	}
	return len(pending), nil
}

func (s *BundleStore) listDir(dir string, filter func(*Bundle) bool) ([]*Bundle, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing bundles: %w", err)
	}

	var bundles []*Bundle
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var b Bundle
		if err := json.Unmarshal(data, &b); err != nil {
			continue
		}
		if filter == nil || filter(&b) {
			bundles = append(bundles, &b)
		}
	}
	return bundles, nil
}

// CleanupCompleted removes archived bundles older than the given age.
func (s *BundleStore) CleanupCompleted(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(s.completedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("listing completed bundles: %w", err)
	}

	var removed int
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(s.completedDir, e.Name())
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}
