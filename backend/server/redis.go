package server

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"

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

// fixedWindow increments a key, starting its expiry on the first hit.
var fixedWindow = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return n
`)

// redisLimiter shares counts across serverless instances, falling back to memory if Redis is unset or down.
type redisLimiter struct {
	name     string
	limit    int64
	window   time.Duration
	client   func() *redis.Client
	fallback *ipLimiter
}

func newRedisLimiter(name string, limit int64, window time.Duration, fallback *ipLimiter) *redisLimiter {
	return &redisLimiter{name: name, limit: limit, window: window, client: sharedRedis, fallback: fallback}
}

func (l *redisLimiter) allow(ip string) bool {
	client := l.client()
	if client == nil {
		return l.fallback.allow(ip)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	n, err := fixedWindow.Run(ctx, client, []string{"ratelimit:" + l.name + ":" + ip}, l.window.Milliseconds()).Int64()
	if err != nil {
		slog.Error("rate limiter: redis", "limiter", l.name, "error", err)
		return l.fallback.allow(ip)
	}

	return n <= l.limit
}
