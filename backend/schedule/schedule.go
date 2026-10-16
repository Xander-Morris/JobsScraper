// Package schedule runs a background job on an interval.
package schedule

import (
	"context"
	"log/slog"
	"time"
)

// Every runs job immediately, then once per interval until ctx is cancelled.
// Run it in its own goroutine. job gets ctx, so cancelling also cuts short a
// run in flight.
func Every(ctx context.Context, name string, interval time.Duration, job func(context.Context)) {
	Once(ctx, name, job)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			Once(ctx, name, job)
		}
	}
}

// Once runs job with panic recovery, so one bad run logs and the schedule
// survives instead of the goroutine dying and silently ending every later run.
func Once(ctx context.Context, name string, job func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("scheduled job panicked", "job", name, "panic", r)
		}
	}()

	job(ctx)
}
