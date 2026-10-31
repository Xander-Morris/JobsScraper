package llm

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	maxCachedQueryEmbeddings = 1000
	queryEmbedTimeout        = 2 * time.Second
	// queryEmbedPause is how long typed search skips Jina after a failure.
	queryEmbedPause = 5 * time.Minute
)

var queryEmbeddings = struct {
	sync.Mutex
	cache       map[string][]float32
	pausedUntil time.Time
}{cache: map[string][]float32{}}

// EmbedQuery embeds a typed search query for semantic search. Results are cached, and
// after a failure Jina is skipped for a while so a dead key doesn't slow every search.
func EmbedQuery(ctx context.Context, query string) ([]float32, bool) {
	key := strings.ToLower(strings.TrimSpace(query))

	queryEmbeddings.Lock()
	vector, cached := queryEmbeddings.cache[key]
	paused := time.Now().Before(queryEmbeddings.pausedUntil)
	queryEmbeddings.Unlock()

	if cached {
		return vector, true
	}

	if key == "" || paused {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(ctx, queryEmbedTimeout)
	defer cancel()

	vectors, err := callJinaEmbed(ctx, []string{key})

	queryEmbeddings.Lock()
	defer queryEmbeddings.Unlock()

	if err != nil {
		slog.Warn("embed search query; keyword search only for a while", "error", err)
		queryEmbeddings.pausedUntil = time.Now().Add(queryEmbedPause)
		return nil, false
	}

	if len(queryEmbeddings.cache) >= maxCachedQueryEmbeddings {
		clear(queryEmbeddings.cache)
	}

	queryEmbeddings.cache[key] = vectors[0]

	return vectors[0], true
}
