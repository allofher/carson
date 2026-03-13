package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsFileChanges(t *testing.T) {
	// Create a temp brain dir with required structure.
	brainDir := t.TempDir()
	os.MkdirAll(filepath.Join(brainDir, "notes"), 0755)
	os.MkdirAll(filepath.Join(brainDir, ".brain"), 0755)
	os.MkdirAll(filepath.Join(brainDir, "static"), 0755)

	w, err := New(Config{
		BrainDir:      brainDir,
		DebounceDelay: 50 * time.Millisecond,
		BatchWindow:   200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batches := w.Run(ctx)

	// Give fsnotify time to set up.
	time.Sleep(100 * time.Millisecond)

	// Create a file — should be detected.
	testFile := filepath.Join(brainDir, "notes", "hello.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	select {
	case batch := <-batches:
		found := false
		for _, ev := range batch {
			if ev.Path == "notes/hello.md" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected event for notes/hello.md, got %v", batch)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for batch")
	}
}

func TestWatcherIgnoresIgnoredDirs(t *testing.T) {
	brainDir := t.TempDir()
	os.MkdirAll(filepath.Join(brainDir, ".brain"), 0755)
	os.MkdirAll(filepath.Join(brainDir, "notes"), 0755)

	w, err := New(Config{
		BrainDir:      brainDir,
		DebounceDelay: 50 * time.Millisecond,
		BatchWindow:   200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batches := w.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Write to .brain — should be ignored.
	os.WriteFile(filepath.Join(brainDir, ".brain", "state.json"), []byte("{}"), 0644)

	// Write to notes — should be detected.
	os.WriteFile(filepath.Join(brainDir, "notes", "real.md"), []byte("real"), 0644)

	select {
	case batch := <-batches:
		for _, ev := range batch {
			if ev.Category == CategoryIgnored {
				t.Errorf("got ignored event in batch: %v", ev)
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for batch")
	}
}
