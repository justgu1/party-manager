package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/guilherme/help-party/internal/config"
	"github.com/guilherme/help-party/internal/db"
	"github.com/guilherme/help-party/internal/queue"
	"github.com/guilherme/help-party/internal/rentals"
	"github.com/guilherme/help-party/internal/scraper"
)

// The worker consumes River jobs — currently rental scraping.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	// Ensure River tables exist (idempotent; also run by the API on boot).
	if err := queue.Migrate(ctx, pool); err != nil {
		log.Fatalf("river migrate: %v", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &rentals.ScrapeWorker{
		Pool:    pool,
		Scraper: scraper.New(),
	})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		Workers: workers,
	})
	if err != nil {
		log.Fatalf("river client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		log.Fatalf("river start: %v", err)
	}
	log.Println("worker up: processing scrape_rental jobs")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("worker shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = client.Stop(shutdownCtx)
}
