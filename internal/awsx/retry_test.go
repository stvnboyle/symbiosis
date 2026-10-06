package awsx

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// recordedSleep returns a Backoff sleep that records delays and never waits.
func recordedSleep(delays *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, d time.Duration) error {
		*delays = append(*delays, d)
		return nil
	}
}

func always(error) bool { return true }

func TestRetrySucceedsFirstTimeWithoutSleeping(t *testing.T) {
	var delays []time.Duration
	calls := 0
	err := Retry(context.Background(), Backoff{Attempts: 5, Initial: time.Second, sleep: recordedSleep(&delays)}, always,
		func(context.Context) error { calls++; return nil })
	if err != nil || calls != 1 || len(delays) != 0 {
		t.Errorf("err=%v calls=%d sleeps=%d, want nil, 1, 0", err, calls, len(delays))
	}
}

func TestRetryBacksOffExponentiallyUpToMax(t *testing.T) {
	var delays []time.Duration
	calls := 0
	boom := errors.New("not yet")
	b := Backoff{Attempts: 6, Initial: time.Second, Max: 5 * time.Second, sleep: recordedSleep(&delays)}
	err := Retry(context.Background(), b, always, func(context.Context) error {
		calls++
		if calls < 6 {
			return boom
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	if !slices.Equal(delays, want) {
		t.Errorf("delays = %v, want %v", delays, want)
	}
}

func TestRetryGivesUpAfterAttempts(t *testing.T) {
	var delays []time.Duration
	calls := 0
	boom := errors.New("still not there")
	b := Backoff{Attempts: 3, Initial: time.Millisecond, sleep: recordedSleep(&delays)}
	err := Retry(context.Background(), b, always, func(context.Context) error { calls++; return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want it to wrap %v", err, boom)
	}
	if calls != 3 || len(delays) != 2 {
		t.Errorf("calls=%d sleeps=%d, want 3 and 2", calls, len(delays))
	}
}

func TestRetryDoesNotRetryOtherErrors(t *testing.T) {
	var delays []time.Duration
	calls := 0
	fatal := errors.New("validation failed")
	b := Backoff{Attempts: 5, Initial: time.Millisecond, sleep: recordedSleep(&delays)}
	err := Retry(context.Background(), b, func(error) bool { return false },
		func(context.Context) error { calls++; return fatal })
	if !errors.Is(err, fatal) || calls != 1 || len(delays) != 0 {
		t.Errorf("err=%v calls=%d sleeps=%d, want the error after 1 call and no sleep", err, calls, len(delays))
	}
}

func TestRetryStopsWhenCancelledWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	b := Backoff{Attempts: 5, Initial: time.Hour} // real sleep: must return on cancel, not after an hour
	done := make(chan error, 1)
	go func() {
		done <- Retry(ctx, b, always, func(context.Context) error {
			calls++
			cancel()
			return errors.New("not yet")
		})
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Retry did not return after cancellation")
	}
}

func TestRetryDoesNotStartWhenAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := Retry(ctx, Backoff{Attempts: 3, Initial: time.Millisecond}, always,
		func(context.Context) error { calls++; return nil })
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Errorf("err=%v calls=%d, want context.Canceled and 0", err, calls)
	}
}

func TestRetryZeroValueTriesOnce(t *testing.T) {
	calls := 0
	boom := errors.New("no")
	err := Retry(context.Background(), Backoff{}, always, func(context.Context) error { calls++; return boom })
	if !errors.Is(err, boom) || calls != 1 {
		t.Errorf("err=%v calls=%d, want the error after 1 call", err, calls)
	}
}
