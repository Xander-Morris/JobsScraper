package main

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"
)

// stubStartup skips real startup, scraping and digests; ensureErr and tablesErr fail those steps.
func stubStartup(t *testing.T, ensureErr, tablesErr error) {
	t.Helper()
	prevEnsure, prevTables, prevScrape, prevDigest := ensure, createTables, startScraping, startScheduler
	t.Cleanup(func() {
		ensure, createTables, startScraping, startScheduler = prevEnsure, prevTables, prevScrape, prevDigest
	})

	ensure = func(...string) error { return ensureErr }
	createTables = func() error { return tablesErr }
	startScraping = func(ctx context.Context) { <-ctx.Done() }
	startScheduler = func(ctx context.Context) { <-ctx.Done() }
}

// listen binds every interface, as the server does, so the port is truly taken on every OS.
func listen(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func portOf(l net.Listener) string { return strconv.Itoa(l.Addr().(*net.TCPAddr).Port) }

// freePort returns a port nothing is listening on right now.
func freePort(t *testing.T) string {
	t.Helper()
	l := listen(t)
	defer l.Close()
	return portOf(l)
}

func TestServerAddr(t *testing.T) {
	t.Setenv("PORT", "")
	if got := serverAddr(); got != ":8090" {
		t.Errorf("default = %q", got)
	}
	t.Setenv("PORT", "9000")
	if got := serverAddr(); got != ":9000" {
		t.Errorf("custom = %q", got)
	}
}

func TestRunStartupFailures(t *testing.T) {
	stubStartup(t, errors.New("missing env"), nil)
	if code := run(t.Context()); code != 1 {
		t.Errorf("bad config = %d, want 1", code)
	}

	stubStartup(t, nil, errors.New("bad migration"))
	if code := run(t.Context()); code != 1 {
		t.Errorf("bad migration = %d, want 1", code)
	}
}

func TestRunStopsCleanlyWhenCancelled(t *testing.T) {
	stubStartup(t, nil, nil)
	t.Setenv("PORT", freePort(t))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if code := run(ctx); code != 0 {
		t.Errorf("run = %d, want 0", code)
	}
}

func TestRunFailsWhenPortTaken(t *testing.T) {
	stubStartup(t, nil, nil)
	taken := listen(t)
	defer taken.Close()
	t.Setenv("PORT", portOf(taken))

	if code := run(t.Context()); code != 1 {
		t.Errorf("run = %d, want 1", code)
	}
}

func TestRunLogsSlowShutdown(t *testing.T) {
	stubStartup(t, nil, nil)
	prevTimeout := shutdownTimeout
	shutdownTimeout = 10 * time.Millisecond
	t.Cleanup(func() { shutdownTimeout = prevTimeout })

	port := freePort(t)
	t.Setenv("PORT", port)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan int)
	go func() { done <- run(ctx) }()

	var conn net.Conn
	for deadline := time.Now().Add(5 * time.Second); conn == nil && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		conn, _ = net.Dial("tcp", "127.0.0.1:"+port)
	}
	if conn == nil {
		t.Fatal("server never started listening")
	}
	defer conn.Close()

	// A half-sent request keeps its connection active, so shutdown times out.
	_, _ = conn.Write([]byte("GET /api/health HTTP/1.1\r\n"))
	time.Sleep(50 * time.Millisecond)

	cancel()
	if code := <-done; code != 0 {
		t.Errorf("run = %d, want 0", code)
	}
}

func TestMainExitsWithRunCode(t *testing.T) {
	stubStartup(t, errors.New("missing env"), nil)
	prevExit := exit
	t.Cleanup(func() { exit = prevExit })

	code := -1
	exit = func(c int) { code = c }
	main()

	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}
