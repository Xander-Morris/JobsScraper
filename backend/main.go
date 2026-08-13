package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"main/database"
	"main/scraper"
	"main/server"
)

func main() {
	defer database.CloseDb()

	go scraper.StartScrapingJob()

	srv := server.New(serverAddr())

	go func() {
		log.Printf("server listening on %s", srv.Addr)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

func serverAddr() string {
	port := os.Getenv("PORT")

	if port == "" {
		port = "8090"
	}

	return ":" + port
}
