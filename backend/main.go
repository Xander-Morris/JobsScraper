package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"main/database"
	"main/scraper"
	"main/server"
	"main/utils"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := utils.RequireEnv("DATABASE_CONNECTION", "SECRET_KEY"); err != nil {
		slog.Error("startup: env validation failed", "error", err)
		os.Exit(1)
	}

	err := database.CreateTables()

	if err != nil {
		slog.Error("startup: create tables failed", "error", err)
		os.Exit(1)
	}

	defer database.CloseDb()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		scraper.StartScrapingJob(ctx)
		return nil
	})

	g.Go(func() error {
		server.StartDigestScheduler(ctx)
		return nil
	})

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

	// wait for any scrape/digest run in flight to finish before the deferred
	// CloseDb above runs, so it's not yanking the pool out from under a write
	if err := g.Wait(); err != nil {
		log.Printf("background service error: %v", err)
	}
}

func serverAddr() string {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8090"
	}

	return ":" + port
}
