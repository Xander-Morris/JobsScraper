package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"golang.org/x/time/rate"

	"main/utils"
)

var redisOnce sync.Once
var redisClient *redis.Client

func sharedRedis() *redis.Client {
	redisOnce.Do(func() {
		url := utils.GetEnv()["REDIS_URL"]
		if url == "" {
			return
		}

		opt, err := redis.ParseURL(url)
		if err != nil {
			slog.Error("redis: parse REDIS_URL", "error", err)
			return
		}

		opt.DisableIdentity = true
		opt.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}
		opt.DialTimeout = 2 * time.Second
		opt.ReadTimeout = time.Second
		opt.WriteTimeout = time.Second

		redisClient = redis.NewClient(opt)
	})

	return redisClient
}

// tokenBucket mirrors rate.Limiter.Allow; the key expires once the bucket would be full again.
var tokenBucket = redis.NewScript(`
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local state = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(state[1]) or burst
local ts = tonumber(state[2]) or now

tokens = math.min(burst, tokens + math.max(0, now - ts) / 1000 * rate)

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', math.max(ts, now))
redis.call('PEXPIRE', KEYS[1], math.ceil(burst / rate * 1000))
return allowed
`)

// redisLimiter shares a token bucket across serverless instances, falling back to memory if Redis is unset or down.
type redisLimiter struct {
	name     string
	rate     rate.Limit
	burst    int
	now      func() time.Time
	client   func() *redis.Client
	fallback *ipLimiter
}

func newRedisLimiter(name string, r rate.Limit, burst int) *redisLimiter {
	return &redisLimiter{name: name, rate: r, burst: burst, now: time.Now, client: sharedRedis, fallback: newIPLimiter(r, burst)}
}

func (l *redisLimiter) allow(ip string) bool {
	client := l.client()
	if client == nil {
		return l.fallback.allow(ip)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := "tokenbucket:" + l.name + ":" + ip
	allowed, err := tokenBucket.Run(ctx, client, []string{key}, float64(l.rate), l.burst, l.now().UnixMilli()).Int()
	if err != nil {
		slog.Error("rate limiter: redis", "limiter", l.name, "error", err)
		return l.fallback.allow(ip)
	}

	return allowed == 1
}
