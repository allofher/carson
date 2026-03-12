package scheduler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBundleStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBundleStore(dir)
	if err != nil {
		t.Fatalf("NewBundleStore: %v", err)
	}

	b := &Bundle{
		ID:        "evt_test001",
		CreatedAt: time.Now(),
		FiresAt:   time.Now().Add(1 * time.Hour),
		Status:    StatusPending,
		Prompt:    "Test prompt",
		Context:   json.RawMessage(`{"key": "value"}`),
		MaxRetries: 2,
		Chain:     []string{"evt_test001"},
	}

	if err := store.Save(b); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("evt_test001")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ID != b.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, b.ID)
	}
	if loaded.Prompt != b.Prompt {
		t.Errorf("Prompt = %q, want %q", loaded.Prompt, b.Prompt)
	}
	if loaded.Status != StatusPending {
		t.Errorf("Status = %q, want pending", loaded.Status)
	}
	var ctx map[string]string
	json.Unmarshal(loaded.Context, &ctx)
	if ctx["key"] != "value" {
		t.Errorf("Context key = %q, want %q", ctx["key"], "value")
	}
}

func TestBundleStore_Archive(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBundleStore(dir)
	if err != nil {
		t.Fatalf("NewBundleStore: %v", err)
	}

	b := &Bundle{
		ID:      "evt_archive",
		Status:  StatusPending,
		Prompt:  "Archive me",
		FiresAt: time.Now().Add(1 * time.Hour),
		Chain:   []string{"evt_archive"},
	}
	store.Save(b)

	// Archive it.
	if err := store.Archive("evt_archive"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	// Should not exist in active dir.
	if _, err := os.Stat(filepath.Join(dir, "evt_archive.json")); !os.IsNotExist(err) {
		t.Error("bundle still in active dir after archive")
	}

	// Should exist in completed dir.
	if _, err := os.Stat(filepath.Join(dir, "completed", "evt_archive.json")); err != nil {
		t.Error("bundle not found in completed dir")
	}

	// Load should still find it.
	loaded, err := store.Load("evt_archive")
	if err != nil {
		t.Fatalf("Load after archive: %v", err)
	}
	if loaded.ID != "evt_archive" {
		t.Errorf("loaded wrong bundle: %s", loaded.ID)
	}
}

func TestBundleStore_ListPending(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBundleStore(dir)
	if err != nil {
		t.Fatalf("NewBundleStore: %v", err)
	}

	// Create 3 bundles: 2 pending, 1 completed.
	for i, status := range []Status{StatusPending, StatusPending, StatusCompleted} {
		b := &Bundle{
			ID:      generateID(),
			Status:  status,
			Prompt:  "test",
			FiresAt: time.Now().Add(time.Duration(i) * time.Hour),
			Chain:   []string{},
		}
		store.Save(b)
	}

	pending, err := store.ListPending()
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("ListPending returned %d, want 2", len(pending))
	}
}

func TestBundleStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBundleStore(dir)
	if err != nil {
		t.Fatalf("NewBundleStore: %v", err)
	}

	b := &Bundle{
		ID:     "evt_delete",
		Status: StatusPending,
		Prompt: "delete me",
		Chain:  []string{},
	}
	store.Save(b)
	store.Delete("evt_delete")

	_, err = store.Load("evt_delete")
	if err == nil {
		t.Error("expected error loading deleted bundle")
	}
}

func TestBundle_IsExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	b1 := &Bundle{ExpireAfter: &past}
	if !b1.IsExpired() {
		t.Error("expected expired with past time")
	}

	b2 := &Bundle{ExpireAfter: &future}
	if b2.IsExpired() {
		t.Error("expected not expired with future time")
	}

	b3 := &Bundle{} // no expiry
	if b3.IsExpired() {
		t.Error("expected not expired with nil expiry")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if len(id1) < 10 {
		t.Errorf("ID too short: %s", id1)
	}
	if id1[:4] != "evt_" {
		t.Errorf("ID missing prefix: %s", id1)
	}
}
