// Package schedule runs a background job on an interval.
package schedule

import (
	"context"
	"log/slog"
	"time"
)

// Every runs job immediately, then once per interval until ctx is cancelled.
// Run it in its own goroutine. On cancel it returns between runs rather than
// abandoning one mid-flight, so a write in progress finishes first.
func Every(ctx context.Context, name string, interval time.Duration, job func()) {
	Once(name, job)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			Once(name, job)
		}
	}
}

// Once runs job with panic recovery, so one bad run logs and the schedule
// survives instead of the goroutine dying and silently ending every later run.
func Once(name string, job func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("scheduled job panicked", "job", name, "panic", r)
		}
	}()

	job()
}
