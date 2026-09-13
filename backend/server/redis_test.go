package server

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

const testIP = "203.0.113.10"

func newTestRedisLimiter(t *testing.T, limit int64, window time.Duration) (*redisLimiter, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { client.Close() })

	return &redisLimiter{
		name:     "test",
		limit:    limit,
		window:   window,
		client:   func() *redis.Client { return client },
		fallback: newIPLimiter(rate.Every(time.Hour), 1),
	}, mr
}

func TestRedisLimiterBlocksPastLimit(t *testing.T) {
	l, _ := newTestRedisLimiter(t, 2, time.Minute)

	for i := range 2 {
		if !l.allow(testIP) {
			t.Fatalf("request %d within limit was blocked", i+1)
		}
	}

	if l.allow(testIP) {
		t.Errorf("request past limit was allowed")
	}

	if !l.allow("203.0.113.11") {
		t.Errorf("a second IP was limited by the first IP's budget")
	}
}

func TestRedisLimiterResetsAfterWindow(t *testing.T) {
	l, mr := newTestRedisLimiter(t, 1, time.Minute)

	l.allow(testIP)
	if l.allow(testIP) {
		t.Fatalf("request past limit was allowed")
	}

	mr.FastForward(time.Minute)

	if !l.allow(testIP) {
		t.Errorf("request after window expired was blocked")
	}
}

func TestRedisLimiterFallsBackWhenRedisDown(t *testing.T) {
	l, mr := newTestRedisLimiter(t, 100, time.Minute)
	mr.Close()

	if !l.allow(testIP) {
		t.Fatalf("first request was blocked by fallback")
	}

	if l.allow(testIP) {
		t.Errorf("fallback limit not enforced while redis is down")
	}
}

func TestRedisLimiterFallsBackWithoutRedis(t *testing.T) {
	l := &redisLimiter{
		name:     "test",
		limit:    100,
		window:   time.Minute,
		client:   func() *redis.Client { return nil },
		fallback: newIPLimiter(rate.Every(time.Hour), 1),
	}

	if !l.allow(testIP) {
		t.Fatalf("first request was blocked by fallback")
	}

	if l.allow(testIP) {
		t.Errorf("fallback limit not enforced without REDIS_URL")
	}
}
