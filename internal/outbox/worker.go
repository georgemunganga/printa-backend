package outbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// Handler processes one event. It must be idempotent because a delivered side
// effect can be retried if a process stops before MarkDelivered succeeds.
type Handler func(context.Context, Event) error

type Worker struct {
	Repository  *Repository
	Handlers    map[string]Handler
	PollEvery   time.Duration
	LeaseFor    time.Duration
	BatchSize   int
	MaxAttempts int
	Logger      *log.Logger
}

func (w *Worker) Run(ctx context.Context) error {
	if w.Repository == nil {
		return errors.New("outbox repository is required")
	}
	if w.PollEvery <= 0 {
		w.PollEvery = 2 * time.Second
	}
	if w.LeaseFor <= 0 {
		w.LeaseFor = 5 * time.Minute
	}
	if w.BatchSize < 1 || w.BatchSize > 100 {
		w.BatchSize = 25
	}
	if w.MaxAttempts < 1 {
		w.MaxAttempts = 5
	}
	if w.Logger == nil {
		w.Logger = log.Default()
	}

	ticker := time.NewTicker(w.PollEvery)
	defer ticker.Stop()
	for {
		if err := w.ProcessOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			w.Logger.Printf("outbox worker poll failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) error {
	events, err := w.Repository.ClaimPending(ctx, w.BatchSize, w.LeaseFor)
	if err != nil {
		return fmt.Errorf("claim outbox events: %w", err)
	}
	for _, event := range events {
		if err := w.processEvent(ctx, event); err != nil {
			w.Logger.Printf("outbox event %s (%s) failed: %v", event.ID, event.EventType, err)
		}
	}
	return nil
}

func (w *Worker) processEvent(ctx context.Context, event Event) error {
	handler := w.Handlers[event.EventType]
	if handler == nil {
		err := fmt.Errorf("no handler registered for event type %q", event.EventType)
		return w.Repository.MarkFailed(ctx, event.ID, err, retryDelay(event.Attempts), w.MaxAttempts)
	}
	if err := handler(ctx, event); err != nil {
		return w.Repository.MarkFailed(ctx, event.ID, err, retryDelay(event.Attempts), w.MaxAttempts)
	}
	return w.Repository.MarkDelivered(ctx, event.ID)
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second * time.Duration(1<<min(attempt-1, 8))
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
