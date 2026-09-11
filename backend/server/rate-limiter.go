package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

func newIPLimiter(r rate.Limit, burst int) *ipLimiter {
	l := &ipLimiter{visitors: make(map[string]*visitor), rate: r, burst: burst}
	go l.cleanupLoop()

	return l
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()

	v, exists := l.visitors[ip]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	limiter := v.limiter

	l.mu.Unlock()

	return limiter.Allow()
}

func (l *ipLimiter) cleanupLoop() {
	for {
		time.Sleep(time.Minute)

		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// Sized for one SPA screen load, which fans out to several endpoints, then polls
// resume extraction status, while still capping scripted abuse.
var globalLimiter = newIPLimiter(10, 20)

var authLimiter = newIPLimiter(rate.Every(20*time.Second), 5)

// llmLimiter guards the endpoints that call out to OpenRouter/Jina. Each one
// can take up to a couple minutes, so globalLimiter alone won't stop one
// caller from hammering them.
var llmLimiter = newIPLimiter(rate.Every(10*time.Second), 3)

// clientIP trusts the first X-Forwarded-For entry since Caddy overwrites it
// with the real remote address before proxying (see Caddyfile). Don't expose
// the backend directly to the internet; that'd make these limits spoofable.
func clientIP(r *http.Request) (string, error) {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip := strings.TrimSpace(strings.Split(fwd, ",")[0]); ip != "" {
			return ip, nil
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)

	return ip, err
}

func limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, err := clientIP(r)
		if err != nil {
			slog.Error("rate limiter: resolve client ip", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !globalLimiter.allow(ip) {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func limitWith(l *ipLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, err := clientIP(r)
		if err != nil {
			slog.Error("rate limiter: resolve client ip", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if !l.allow(ip) {
			writeError(w, http.StatusTooManyRequests, "too many attempts, please try again later")
			return
		}

		next(w, r)
	}
}

func limitAuth(next http.HandlerFunc) http.HandlerFunc {
	return limitWith(authLimiter, next)
}

func limitLLM(next http.HandlerFunc) http.HandlerFunc {
	return limitWith(llmLimiter, next)
}
