package main

import (
	"context"
	"log/slog"
	"net/http"
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

func main() {
	coldstart.Ensure("DATABASE_CONNECTION", "SECRET_KEY")

	if err := database.CreateTables(); err != nil {
		slog.Error("startup: create tables failed", "error", err)
		os.Exit(1)
	}

	defer database.CloseDb()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Go(func() { scraper.StartScrapingJob(ctx) })
	wg.Go(func() { digest.StartScheduler(ctx) })

	srv := server.New(serverAddr())

	go func() {
		slog.Info("server listening", "addr", srv.Addr)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	// Wait for the cancelled scrape/digest runs to unwind before the deferred
	// CloseDb above runs, so it's not yanking the pool out from under a write.
	wg.Wait()
}

func serverAddr() string {
	port := utils.Getenv("PORT")

	if port == "" {
		port = "8090"
	}

	return ":" + port
}
