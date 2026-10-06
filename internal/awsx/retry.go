package awsx

import (
	"context"
	"fmt"
	"time"
)

// Backoff describes how Retry waits between attempts: Initial, then double each time,
// capped at Max.
type Backoff struct {
	// Attempts is the total number of tries. Zero or one means no retry.
	Attempts int
	Initial  time.Duration
	// Max caps the delay. Zero means no cap.
	Max time.Duration

	sleep func(context.Context, time.Duration) error // replaced in tests
}

// Retry calls fn until it succeeds, returns an error that retryable rejects, runs out
// of attempts, or ctx is cancelled.
//
// The SDK already retries throttling and server errors on its own. Retry is for what
// the SDK cannot know: that an error such as "not found" is expected for a few seconds
// after a create, and worth waiting out. See IsNotYetConsistent.
func Retry(ctx context.Context, b Backoff, retryable func(error) bool, fn func(context.Context) error) error {
	sleep := b.sleep
	if sleep == nil {
		sleep = sleepContext
	}
	attempts := max(b.Attempts, 1)
	delay := b.Initial

	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn(ctx)
		if err == nil || !retryable(err) {
			return err
		}
		if attempt == attempts {
			return fmt.Errorf("gave up after %d attempts: %w", attempts, err)
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
		delay *= 2
		if b.Max > 0 {
			delay = min(delay, b.Max)
		}
	}
}

// sleepContext waits for d, or returns early with the context's error if it is
// cancelled.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
