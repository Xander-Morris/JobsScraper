package llm

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func resetQueryEmbeddings(t *testing.T) {
	t.Helper()

	clear(queryEmbeddings.cache)
	queryEmbeddings.pausedUntil = time.Time{}
	t.Cleanup(func() {
		clear(queryEmbeddings.cache)
		queryEmbeddings.pausedUntil = time.Time{}
	})
}

func embedOK(_ int32, input []string) (int, any) {
	return http.StatusOK, map[string]any{"data": []embedDatum{{Index: 0, Embedding: []float32{float32(len(input[0]))}}}}
}

func TestEmbedQueryCachesNormalizedQuery(t *testing.T) {
	resetQueryEmbeddings(t)
	calls := fakeJinaServer(t, embedOK)

	first, ok := EmbedQuery(context.Background(), "  Web Dev ")
	if !ok || len(first) != 1 || first[0] != 7 {
		t.Fatalf("EmbedQuery = %v, %v", first, ok)
	}

	if _, ok := EmbedQuery(context.Background(), "web dev"); !ok {
		t.Fatal("cached EmbedQuery not ok")
	}

	if calls.Load() != 1 {
		t.Errorf("jina calls = %d, want 1", calls.Load())
	}
}

func TestEmbedQueryEmpty(t *testing.T) {
	resetQueryEmbeddings(t)
	calls := fakeJinaServer(t, embedOK)

	if _, ok := EmbedQuery(context.Background(), "   "); ok {
		t.Error("empty query should not embed")
	}

	if calls.Load() != 0 {
		t.Errorf("jina calls = %d, want 0", calls.Load())
	}
}

func TestEmbedQueryPausesAfterFailure(t *testing.T) {
	resetQueryEmbeddings(t)
	calls := fakeJinaServer(t, func(int32, []string) (int, any) {
		return http.StatusForbidden, map[string]any{"detail": "insufficient balance"}
	})

	for range 2 {
		if _, ok := EmbedQuery(context.Background(), "go"); ok {
			t.Fatal("failed embed should not be ok")
		}
	}

	if calls.Load() != 1 {
		t.Errorf("jina calls = %d, want 1 (paused after first failure)", calls.Load())
	}
}

func TestEmbedQueryEvictsWhenFull(t *testing.T) {
	resetQueryEmbeddings(t)
	fakeJinaServer(t, embedOK)

	for i := range maxCachedQueryEmbeddings + 1 {
		if _, ok := EmbedQuery(context.Background(), fmt.Sprintf("q%d", i)); !ok {
			t.Fatalf("EmbedQuery %d not ok", i)
		}
	}

	if len(queryEmbeddings.cache) != 1 {
		t.Errorf("cache size = %d, want 1 after eviction", len(queryEmbeddings.cache))
	}
}
