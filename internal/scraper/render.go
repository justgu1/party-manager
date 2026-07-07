package scraper

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

// renderWithChrome loads a URL in a headless Chrome instance and returns the
// fully-rendered outer HTML. Requires a Chromium binary on the host (provided
// in the worker Docker image). On any failure the caller falls back to static
// fetching, so JS-heavy sites still yield OpenGraph metadata.
func renderWithChrome(ctx context.Context, rawurl string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.NoSandbox,
			chromedp.Flag("headless", true),
			chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "+
				"(KHTML, like Gecko) Chrome/124.0 Safari/537.36"),
		)...,
	)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var html string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(rawurl),
		// Give client-side rendering a moment to populate the DOM.
		chromedp.Sleep(3*time.Second),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		return "", err
	}
	return html, nil
}
