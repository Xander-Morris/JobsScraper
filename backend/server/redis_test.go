package server

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

const testIP = "203.0.113.10"

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newTestRedisLimiter(t *testing.T, r rate.Limit, burst int) (*redisLimiter, *miniredis.Miniredis, *fakeClock) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { client.Close() })

	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}

	return &redisLimiter{
		name:     "test",
		rate:     r,
		burst:    burst,
		now:      clock.now,
		client:   func() *redis.Client { return client },
		fallback: newIPLimiter(rate.Every(time.Hour), 1),
	}, mr, clock
}

func TestRedisLimiterBlocksPastBurst(t *testing.T) {
	l, _, _ := newTestRedisLimiter(t, rate.Every(time.Minute), 2)

	for i := range 2 {
		if !l.allow(testIP) {
			t.Fatalf("request %d within burst was blocked", i+1)
		}
	}

	if l.allow(testIP) {
		t.Errorf("request past burst was allowed")
	}

	if !l.allow("203.0.113.11") {
		t.Errorf("a second IP was limited by the first IP's bucket")
	}
}

func TestRedisLimiterRefillsOneTokenAtATime(t *testing.T) {
	l, _, clock := newTestRedisLimiter(t, rate.Every(time.Minute), 3)

	for range 3 {
		l.allow(testIP)
	}
	if l.allow(testIP) {
		t.Fatalf("request past burst was allowed")
	}

	clock.t = clock.t.Add(time.Minute)

	if !l.allow(testIP) {
		t.Fatalf("request after one refill interval was blocked")
	}
	if l.allow(testIP) {
		t.Errorf("more than one token refilled after one interval")
	}
}

func TestRedisLimiterRefillCapsAtBurst(t *testing.T) {
	l, _, clock := newTestRedisLimiter(t, rate.Every(time.Minute), 2)

	l.allow(testIP)
	clock.t = clock.t.Add(time.Hour)

	for i := range 2 {
		if !l.allow(testIP) {
			t.Fatalf("request %d after full refill was blocked", i+1)
		}
	}
	if l.allow(testIP) {
		t.Errorf("bucket refilled past burst")
	}
}

func TestRedisLimiterKeyExpiresWhenFull(t *testing.T) {
	l, mr, _ := newTestRedisLimiter(t, rate.Every(time.Minute), 2)

	l.allow(testIP)

	key := "tokenbucket:test:" + testIP
	if ttl := mr.TTL(key); ttl != 2*time.Minute {
		t.Errorf("ttl = %v, want time to refill the full bucket (2m)", ttl)
	}
}

func TestRedisLimiterFallsBackWhenRedisDown(t *testing.T) {
	l, mr, _ := newTestRedisLimiter(t, rate.Every(time.Second), 100)
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
		rate:     rate.Every(time.Second),
		burst:    100,
		now:      time.Now,
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
