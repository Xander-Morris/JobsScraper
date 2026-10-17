package schedule

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestOnceRunsJob(t *testing.T) {
	ran := false

	Once(t.Context(), "test", func(context.Context) { ran = true })

	if !ran {
		t.Error("Once did not run the job")
	}
}

func TestOnceRecoversPanic(t *testing.T) {
	Once(t.Context(), "test", func(context.Context) { panic("boom") })
}

func TestOncePassesContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "v")

	var got any
	Once(ctx, "test", func(c context.Context) { got = c.Value(key{}) })

	if got != "v" {
		t.Errorf("job got ctx value %v, want %q", got, "v")
	}
}

func runEvery(t *testing.T, ctx context.Context, interval time.Duration, job func(context.Context)) {
	t.Helper()

	done := make(chan struct{})

	go func() {
		Every(ctx, "test", interval, job)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Every did not return after cancel")
	}
}

func TestEveryRunsImmediatelyAndRepeats(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var runs atomic.Int32

	runEvery(t, ctx, 5*time.Millisecond, func(context.Context) {
		if runs.Add(1) == 3 {
			cancel()
		}
	})

	if n := runs.Load(); n < 3 {
		t.Errorf("runs = %d, want >= 3", n)
	}
}

func TestEverySurvivesPanickingRun(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var runs atomic.Int32

	runEvery(t, ctx, 5*time.Millisecond, func(context.Context) {
		if runs.Add(1) >= 2 {
			cancel()
			return
		}
		panic("first run fails")
	})

	if n := runs.Load(); n < 2 {
		t.Errorf("runs = %d, want >= 2", n)
	}
}

func TestEveryReturnsWhenAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var runs atomic.Int32

	runEvery(t, ctx, time.Hour, func(context.Context) { runs.Add(1) })

	if n := runs.Load(); n != 1 {
		t.Errorf("runs = %d, want 1 (the immediate run)", n)
	}
}
