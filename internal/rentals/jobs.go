package rentals

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/guilherme/help-party/internal/scraper"
)

// ScrapeRentalArgs is the River job payload for scraping one rental.
type ScrapeRentalArgs struct {
	RentalID string `json:"rental_id"`
	URL      string `json:"url"`
}

func (ScrapeRentalArgs) Kind() string { return "scrape_rental" }

// ScrapeWorker processes ScrapeRentalArgs jobs: it scrapes the URL and updates
// the rental row (and its availability) with the result.
type ScrapeWorker struct {
	river.WorkerDefaults[ScrapeRentalArgs]
	Pool    *pgxpool.Pool
	Scraper *scraper.Scraper
}

func (w *ScrapeWorker) Work(ctx context.Context, job *river.Job[ScrapeRentalArgs]) error {
	id := job.Args.RentalID
	log.Printf("scraping rental %s (%s)", id, job.Args.URL)

	listing, scrapeErr := w.Scraper.Scrape(ctx, job.Args.URL)
	if scrapeErr != nil {
		// Persist the failure so the UI can show it; don't retry endlessly on
		// permanently-broken links.
		_, _ = w.Pool.Exec(ctx,
			`UPDATE rentals SET status='failed', error=$2, updated_at=now() WHERE id=$1`,
			id, scrapeErr.Error())
		log.Printf("rental %s failed: %v", id, scrapeErr)
		return nil
	}

	_, err := w.Pool.Exec(ctx,
		`UPDATE rentals
		 SET source=$2, title=$3, description=$4, price=$5, rating=$6,
		     reviews_count=$7, image_url=$8, status='scraped', error='', updated_at=now()
		 WHERE id=$1`,
		id, string(listing.Source), listing.Title, listing.Description,
		listing.Price, listing.Rating, listing.ReviewsCount, listing.ImageURL,
	)
	if err != nil {
		return err
	}

	// Replace availability rows with whatever was found (best-effort).
	_, _ = w.Pool.Exec(ctx, `DELETE FROM rental_availability WHERE rental_id=$1`, id)
	for _, a := range listing.Availability {
		raw, _ := json.Marshal(a)
		_, _ = w.Pool.Exec(ctx,
			`INSERT INTO rental_availability (rental_id, label, date_from, date_to, raw)
			 VALUES ($1, $2, $3, $4, $5)`,
			id, a.Label, a.From, a.To, raw)
	}
	log.Printf("rental %s scraped: %q", id, listing.Title)
	return nil
}
