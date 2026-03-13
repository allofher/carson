package watcher

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/allofher/carson/internal/logging"
	"github.com/fsnotify/fsnotify"
)

// DefaultDebounceDelay is the per-path debounce delay.
const DefaultDebounceDelay = 500 * time.Millisecond

// DefaultBatchWindow is the batch accumulation window.
const DefaultBatchWindow = 5 * time.Minute

// Config configures the file watcher.
type Config struct {
	BrainDir      string
	DebounceDelay time.Duration
	BatchWindow   time.Duration
	Broadcaster   *logging.Broadcaster // for real-time SSE push
	Logger        *slog.Logger
}

// Watcher monitors the brain directory for file changes.
type Watcher struct {
	cfg       Config
	fsw       *fsnotify.Watcher
	debouncer *Debouncer
}

// New creates a new file watcher for the brain directory.
func New(cfg Config) (*Watcher, error) {
	if cfg.DebounceDelay == 0 {
		cfg.DebounceDelay = DefaultDebounceDelay
	}
	if cfg.BatchWindow == 0 {
		cfg.BatchWindow = DefaultBatchWindow
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Recursively add all non-ignored directories.
	if err := addRecursive(fsw, cfg.BrainDir, cfg.BrainDir); err != nil {
		fsw.Close()
		return nil, err
	}

	return &Watcher{
		cfg:       cfg,
		fsw:       fsw,
		debouncer: NewDebouncer(cfg.DebounceDelay, cfg.BatchWindow),
	}, nil
}

// Run starts the event loop and returns a channel of batched events.
// It blocks until ctx is cancelled; the returned channel is closed on exit.
func (w *Watcher) Run(ctx context.Context) <-chan []FileEvent {
	go w.loop(ctx)
	return w.debouncer.Batches()
}

func (w *Watcher) loop(ctx context.Context) {
	defer w.fsw.Close()
	defer w.debouncer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.cfg.Logger.Warn("fsnotify error", "error", err)
		}
	}
}

func (w *Watcher) handleEvent(ev fsnotify.Event) {
	fe := Classify(w.cfg.BrainDir, ev)

	if fe.Category == CategoryIgnored {
		return
	}

	// If a new directory was created, start watching it.
	if ev.Op.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			if err := addRecursive(w.fsw, w.cfg.BrainDir, ev.Name); err != nil {
				w.cfg.Logger.Warn("failed to watch new dir", "path", ev.Name, "error", err)
			}
		}
	}

	w.cfg.Logger.Debug("file event", "path", fe.Path, "op", fe.Op, "category", fe.Category.String())

	// Broadcast to SSE subscribers in real-time.
	if w.cfg.Broadcaster != nil {
		data, err := json.Marshal(fe)
		if err == nil {
			data = append(data, '\n')
			w.cfg.Broadcaster.Write(data)
		}
	}

	// Submit to debouncer for batching.
	w.debouncer.Submit(fe)
}

// addRecursive adds dir and all its non-ignored subdirectories to the watcher.
func addRecursive(fsw *fsnotify.Watcher, brainDir, dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible dirs
		}
		if !d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(brainDir, path)
		if err != nil {
			return nil
		}
		if rel != "." && IsIgnoredDir(rel) {
			return filepath.SkipDir
		}

		return fsw.Add(path)
	})
}
