package watcher

import (
	"testing"
	"time"
)

func TestDebouncerCollapsesRapidEvents(t *testing.T) {
	d := NewDebouncer(50*time.Millisecond, 200*time.Millisecond)

	// Submit 5 rapid events for the same path.
	for i := 0; i < 5; i++ {
		d.Submit(FileEvent{
			Path:     "notes/idea.md",
			Op:       "modify",
			Category: CategoryMutable,
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce + batch window.
	select {
	case batch := <-d.Batches():
		if len(batch) != 1 {
			t.Errorf("expected 1 collapsed event, got %d", len(batch))
		}
		if batch[0].Path != "notes/idea.md" {
			t.Errorf("unexpected path: %s", batch[0].Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for batch")
	}

	d.Stop()
}

func TestDebouncerBatchesMultiplePaths(t *testing.T) {
	d := NewDebouncer(20*time.Millisecond, 100*time.Millisecond)

	d.Submit(FileEvent{Path: "a.md", Op: "create", Category: CategoryMutable})
	d.Submit(FileEvent{Path: "b.md", Op: "create", Category: CategoryMutable})

	select {
	case batch := <-d.Batches():
		if len(batch) != 2 {
			t.Errorf("expected 2 events, got %d", len(batch))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for batch")
	}

	d.Stop()
}

func TestDebouncerStopCancelsTimers(t *testing.T) {
	d := NewDebouncer(1*time.Second, 5*time.Second)

	d.Submit(FileEvent{Path: "test.md", Op: "create", Category: CategoryMutable})
	d.Stop()

	// Channel should be closed.
	_, ok := <-d.Batches()
	if ok {
		t.Error("expected batches channel to be closed after Stop")
	}
}
