package scraper

import "testing"

func TestDetect(t *testing.T) {
	cases := map[string]Source{
		"https://www.airbnb.com.br/rooms/123":     SourceAirbnb,
		"https://www.booking.com/hotel/br/x.html":  SourceBooking,
		"https://www.temporadalivre.com/anuncio/1": SourceBR,
		"https://olx.com.br/imovel/2":              SourceBR,
		"https://example.com/casa":                 SourceGeneric,
	}
	for url, want := range cases {
		if got := Detect(url); got != want {
			t.Errorf("Detect(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestParseHTML(t *testing.T) {
	html := `<!doctype html><html><head>
		<title>Fallback Title</title>
		<meta property="og:title" content="Casa na Praia" />
		<meta property="og:description" content="4 quartos, piscina" />
		<meta property="og:image" content="https://img/cover.jpg" />
	</head><body></body></html>`

	l, err := ParseHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "Casa na Praia" {
		t.Errorf("title = %q", l.Title)
	}
	if l.Description != "4 quartos, piscina" {
		t.Errorf("desc = %q", l.Description)
	}
	if l.ImageURL != "https://img/cover.jpg" {
		t.Errorf("image = %q", l.ImageURL)
	}
}

func TestParseHTMLJSONLD(t *testing.T) {
	html := `<html><head>
		<meta property="og:title" content="Pousada" />
		<script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Hotel","name":"Pousada do Sol",
		 "description":"Vista pro mar","image":["https://img/1.jpg"],
		 "aggregateRating":{"ratingValue":"9.2","reviewCount":"148"},
		 "offers":{"@type":"Offer","price":"850","priceCurrency":"BRL"}}
		</script></head><body></body></html>`

	l, err := ParseHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if l.Rating != "9.2" {
		t.Errorf("rating = %q, want 9.2", l.Rating)
	}
	if l.ReviewsCount != 148 {
		t.Errorf("reviews = %d, want 148", l.ReviewsCount)
	}
	if l.Price != "BRL 850" {
		t.Errorf("price = %q, want 'BRL 850'", l.Price)
	}
	if l.Description != "Vista pro mar" {
		t.Errorf("desc = %q", l.Description)
	}
}

func TestParseHTMLTitleFallback(t *testing.T) {
	l, err := ParseHTML(`<html><head><title>Só o title</title></head></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "Só o title" {
		t.Errorf("expected title fallback, got %q", l.Title)
	}
}
