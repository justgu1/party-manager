// Package scraper extracts listing metadata (and best-effort availability)
// from rental URLs. Airbnb/Booking are rendered with a headless browser
// (chromedp); everything else is fetched statically and parsed for OpenGraph
// metadata.
package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Source identifies which site a URL belongs to.
type Source string

const (
	SourceAirbnb  Source = "airbnb"
	SourceBooking Source = "booking"
	SourceBR      Source = "br"
	SourceGeneric Source = "generic"
)

// Availability is a best-effort date range extracted from a listing.
type Availability struct {
	Label string     `json:"label"`
	From  *time.Time `json:"from,omitempty"`
	To    *time.Time `json:"to,omitempty"`
}

// Listing is the structured result of scraping a URL.
type Listing struct {
	Source       Source
	Title        string
	Description  string
	Price        string
	Rating       string
	ReviewsCount int
	ImageURL     string
	Availability []Availability
}

// Detect maps a URL to its Source based on the host.
func Detect(rawurl string) Source {
	u, err := url.Parse(rawurl)
	if err != nil {
		return SourceGeneric
	}
	host := strings.ToLower(u.Host)
	switch {
	case strings.Contains(host, "airbnb."):
		return SourceAirbnb
	case strings.Contains(host, "booking."):
		return SourceBooking
	case strings.Contains(host, "temporadalivre.") ||
		strings.Contains(host, "olx.") ||
		strings.Contains(host, "quintoandar.") ||
		strings.Contains(host, "alugatemporada."):
		return SourceBR
	default:
		return SourceGeneric
	}
}

// Scraper resolves a URL into a Listing.
type Scraper struct {
	http *http.Client
	// renderer renders JS-heavy pages; injected so tests can stub it and so we
	// can gracefully fall back to static fetching when Chrome is unavailable.
	renderer func(ctx context.Context, rawurl string) (string, error)
}

// New returns a Scraper using the default HTTP client and chromedp renderer.
func New() *Scraper {
	return &Scraper{
		http:     &http.Client{Timeout: 20 * time.Second},
		renderer: renderWithChrome,
	}
}

// Scrape fetches and parses the listing at rawurl.
func (s *Scraper) Scrape(ctx context.Context, rawurl string) (Listing, error) {
	source := Detect(rawurl)

	var html string
	var err error
	if source == SourceAirbnb || source == SourceBooking {
		html, err = s.renderer(ctx, rawurl)
		if err != nil {
			// Chrome may be unavailable (e.g. local dev). Fall back to static.
			html, err = s.fetchStatic(ctx, rawurl)
		}
	} else {
		html, err = s.fetchStatic(ctx, rawurl)
	}
	if err != nil {
		return Listing{Source: source}, fmt.Errorf("fetch: %w", err)
	}

	listing, err := ParseHTML(html)
	if err != nil {
		return Listing{Source: source}, err
	}
	listing.Source = source
	return listing, nil
}

func (s *Scraper) fetchStatic(ctx context.Context, rawurl string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawurl, nil)
	if err != nil {
		return "", err
	}
	// A realistic UA reduces the chance of being served an empty/blocked page.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "+
		"(KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")

	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // cap at 5MB
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ParseHTML extracts listing metadata from an HTML document using OpenGraph
// tags with sensible fallbacks. It is pure and unit-testable.
func ParseHTML(html string) (Listing, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return Listing{}, err
	}

	meta := func(property string) string {
		if v, ok := doc.Find(`meta[property="` + property + `"]`).Attr("content"); ok {
			return strings.TrimSpace(v)
		}
		if v, ok := doc.Find(`meta[name="` + property + `"]`).Attr("content"); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}

	l := Listing{
		Title:       firstNonEmpty(meta("og:title"), strings.TrimSpace(doc.Find("title").First().Text())),
		Description: firstNonEmpty(meta("og:description"), meta("description")),
		ImageURL:    meta("og:image"),
		Price:       firstNonEmpty(meta("product:price:amount"), meta("og:price:amount")),
	}

	// Structured data (schema.org JSON-LD) fills in price, rating and reviews
	// that OpenGraph rarely carries. Booking and many BR sites embed it.
	if ld := extractJSONLD(doc); ld != nil {
		l.Title = firstNonEmpty(l.Title, ld.name)
		l.Description = firstNonEmpty(l.Description, ld.description)
		l.ImageURL = firstNonEmpty(l.ImageURL, ld.image)
		l.Price = firstNonEmpty(l.Price, ld.price)
		l.Rating = firstNonEmpty(l.Rating, ld.rating)
		if l.ReviewsCount == 0 {
			l.ReviewsCount = ld.reviews
		}
	}
	return l, nil
}

// jsonLD holds the fields we pull out of schema.org structured data.
type jsonLD struct {
	name, description, image, price, rating string
	reviews                                 int
}

// extractJSONLD parses every <script type="application/ld+json"> block and
// merges the first useful values it finds (walking @graph/arrays/nested nodes).
func extractJSONLD(doc *goquery.Document) *jsonLD {
	out := &jsonLD{}
	found := false

	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		var v any
		if err := json.Unmarshal([]byte(strings.TrimSpace(s.Text())), &v); err != nil {
			return
		}
		if walkJSONLD(v, out) {
			found = true
		}
	})
	if !found {
		return nil
	}
	return out
}

// walkJSONLD recursively scans decoded JSON, filling empty fields of out.
func walkJSONLD(v any, out *jsonLD) bool {
	touched := false
	switch node := v.(type) {
	case []any:
		for _, item := range node {
			if walkJSONLD(item, out) {
				touched = true
			}
		}
	case map[string]any:
		if graph, ok := node["@graph"]; ok {
			if walkJSONLD(graph, out) {
				touched = true
			}
		}
		if out.name == "" {
			if s := jsonStr(node["name"]); s != "" {
				out.name = s
				touched = true
			}
		}
		if out.description == "" {
			if s := jsonStr(node["description"]); s != "" {
				out.description = s
				touched = true
			}
		}
		if out.image == "" {
			out.image = firstImage(node["image"])
		}
		if out.price == "" {
			out.price = extractPrice(node["offers"], node["priceRange"])
		}
		if out.rating == "" || out.reviews == 0 {
			extractRating(node["aggregateRating"], out)
		}
	}
	return touched
}

func extractPrice(offers, priceRange any) string {
	if s := jsonStr(priceRange); s != "" {
		return s
	}
	m, ok := offers.(map[string]any)
	if !ok {
		if arr, ok := offers.([]any); ok && len(arr) > 0 {
			m, _ = arr[0].(map[string]any)
		}
	}
	if m == nil {
		return ""
	}
	cur := jsonStr(m["priceCurrency"])
	price := firstNonEmpty(jsonStr(m["price"]), jsonStr(m["lowPrice"]))
	if price == "" {
		return ""
	}
	if cur != "" {
		return cur + " " + price
	}
	return price
}

func extractRating(v any, out *jsonLD) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if out.rating == "" {
		out.rating = jsonStr(m["ratingValue"])
	}
	if out.reviews == 0 {
		if n := jsonStr(firstNonEmptyAny(m["reviewCount"], m["ratingCount"])); n != "" {
			out.reviews = atoiSafe(n)
		}
	}
}

func firstImage(v any) string {
	switch img := v.(type) {
	case string:
		return img
	case []any:
		if len(img) > 0 {
			return firstImage(img[0])
		}
	case map[string]any:
		return jsonStr(img["url"])
	}
	return ""
}

// jsonStr coerces a JSON value (string or number) to a trimmed string.
func jsonStr(v any) string {
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64)
	}
	return ""
}

func firstNonEmptyAny(vals ...any) any {
	for _, v := range vals {
		if jsonStr(v) != "" {
			return v
		}
	}
	return nil
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
