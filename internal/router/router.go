package router

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/allofher/carson/internal/harness"
	"github.com/allofher/carson/internal/watcher"
)

// Router receives batched file events and invokes the agent with a summary prompt.
type Router struct {
	harness *harness.Harness
	logger  *slog.Logger
	sem     chan struct{} // concurrency guard — buffer of 1
}

// New creates a new event router.
func New(h *harness.Harness, logger *slog.Logger) *Router {
	return &Router{
		harness: h,
		logger:  logger,
		sem:     make(chan struct{}, 1),
	}
}

// ConsumeLoop reads batches from the channel and handles each one.
// It blocks until the channel is closed or ctx is cancelled.
func (r *Router) ConsumeLoop(ctx context.Context, batches <-chan []watcher.FileEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-batches:
			if !ok {
				return
			}
			r.HandleBatch(ctx, batch)
		}
	}
}

// HandleBatch builds a summary prompt from the events and invokes the agent.
// Uses a semaphore to prevent concurrent agent invocations — if the agent is
// busy and the queue is full, the batch is dropped.
func (r *Router) HandleBatch(ctx context.Context, events []watcher.FileEvent) {
	if len(events) == 0 {
		return
	}

	select {
	case r.sem <- struct{}{}:
		// Acquired — run the agent.
		go func() {
			defer func() { <-r.sem }()
			r.invoke(ctx, events)
		}()
	default:
		r.logger.Warn("agent busy, dropping file event batch", "events", len(events))
	}
}

func (r *Router) invoke(ctx context.Context, events []watcher.FileEvent) {
	prompt := BuildPrompt(events)
	r.logger.Info("invoking agent for file changes", "events", len(events))

	_, err := r.harness.Run(ctx, prompt)
	if err != nil {
		r.logger.Error("agent invocation failed", "error", err)
	}
}

// BuildPrompt constructs a prompt summarizing the file changes.
func BuildPrompt(events []watcher.FileEvent) string {
	var b strings.Builder
	b.WriteString("The following file changes were detected in the brain folder:\n\n")

	for _, ev := range events {
		annotation := ""
		if ev.Category == watcher.CategoryStatic {
			annotation = " [read-only]"
		}
		fmt.Fprintf(&b, "- %s (%s)%s\n", ev.Path, ev.Op, annotation)
	}

	b.WriteString("\nReview these changes. ")
	b.WriteString("For new files in static/, consider generating a metadata sidecar in .meta/. ")
	b.WriteString("For TODO.md changes, check if new items need scheduling. ")
	b.WriteString("For other changes, decide if any action is warranted.")

	return b.String()
}
