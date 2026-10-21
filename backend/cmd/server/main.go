// Command server runs the API, the scrape loop and the digest scheduler.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"main/coldstart"
	"main/database"
	"main/digest"
	"main/scraper"
	"main/server"
	"main/utils"
)

// Vars so tests can run the binary without exiting, scraping, or emailing.
var (
	exit           = os.Exit
	ensure         = coldstart.Ensure
	createTables   = database.CreateTables
	startScraping  = scraper.StartScrapingJob
	startScheduler = digest.StartScheduler
	// shutdownTimeout bounds how long in-flight requests get to finish.
	shutdownTimeout = 5 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx)
	stop()
	exit(code)
}

// run serves until ctx is cancelled or the server fails, returning the exit code.
func run(ctx context.Context) int {
	if err := ensure("DATABASE_CONNECTION", "SECRET_KEY"); err != nil {
		return 1
	}

	if err := createTables(); err != nil {
		slog.Error("startup: create tables failed", "error", err)
		return 1
	}

	defer database.CloseDb()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Go(func() { startScraping(ctx) })
	wg.Go(func() { startScheduler(ctx) })

	srv := server.New(serverAddr())
	serveErr := make(chan error, 1)

	go func() {
		slog.Info("server listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	code := 0

	select {
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancelShutdown()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	case err := <-serveErr:
		// ListenAndServe only returns early on failure, e.g. the port is taken.
		slog.Error("server error", "error", err)
		code = 1
	}

	// Wait for the cancelled scrape/digest runs to unwind before the deferred
	// CloseDb runs, so it's not yanking the pool out from under a write.
	cancel()
	wg.Wait()

	return code
}

func serverAddr() string {
	port := utils.Getenv("PORT")

	if port == "" {
		port = "8090"
	}

	return ":" + port
}
