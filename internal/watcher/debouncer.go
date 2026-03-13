package watcher

import (
	"sync"
	"time"
)

// Debouncer provides two-stage event collapsing:
//  1. Per-path debounce: rapid events for the same path within debounceDelay
//     are collapsed into a single event (last write wins).
//  2. Batch window: settled events accumulate and flush as a batch after
//     batchWindow of inactivity (reset on each new event).
type Debouncer struct {
	debounceDelay time.Duration
	batchWindow   time.Duration

	mu       sync.Mutex
	pending  map[string]*pendingEvent // per-path debounce
	batch    []FileEvent              // accumulated settled events
	batches  chan []FileEvent
	batchTmr *time.Timer
	stopCh   chan struct{}
	stopped  bool
}

type pendingEvent struct {
	event FileEvent
	timer *time.Timer
}

// NewDebouncer creates a debouncer with the given per-path delay and batch window.
func NewDebouncer(debounceDelay, batchWindow time.Duration) *Debouncer {
	return &Debouncer{
		debounceDelay: debounceDelay,
		batchWindow:   batchWindow,
		pending:       make(map[string]*pendingEvent),
		batches:       make(chan []FileEvent, 8),
		stopCh:        make(chan struct{}),
	}
}

// Submit adds a file event to the debouncer. If an event for the same path
// is already pending, it resets the debounce timer and replaces the event.
func (d *Debouncer) Submit(ev FileEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	if p, ok := d.pending[ev.Path]; ok {
		p.timer.Stop()
		p.event = ev
		p.timer = time.AfterFunc(d.debounceDelay, func() {
			d.settle(ev.Path)
		})
		return
	}

	d.pending[ev.Path] = &pendingEvent{
		event: ev,
		timer: time.AfterFunc(d.debounceDelay, func() {
			d.settle(ev.Path)
		}),
	}
}

// settle moves a debounced event into the batch accumulator.
func (d *Debouncer) settle(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	p, ok := d.pending[path]
	if !ok {
		return
	}
	delete(d.pending, path)

	d.batch = append(d.batch, p.event)

	// Reset or start the batch timer.
	if d.batchTmr != nil {
		d.batchTmr.Stop()
	}
	d.batchTmr = time.AfterFunc(d.batchWindow, d.flush)
}

// flush sends the accumulated batch to the output channel.
func (d *Debouncer) flush() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.batch) == 0 || d.stopped {
		return
	}

	out := make([]FileEvent, len(d.batch))
	copy(out, d.batch)
	d.batch = d.batch[:0]

	select {
	case d.batches <- out:
	default:
		// Channel full — drop batch.
	}
}

// Batches returns the channel that receives settled, batched events.
func (d *Debouncer) Batches() <-chan []FileEvent {
	return d.batches
}

// Stop shuts down the debouncer, cancelling all pending timers.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}
	d.stopped = true

	for _, p := range d.pending {
		p.timer.Stop()
	}
	if d.batchTmr != nil {
		d.batchTmr.Stop()
	}

	close(d.stopCh)
	close(d.batches)
}
