package nyt36hours

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureArticleHTML = `<html><head>
<title>36 Hours in Lisbon - The New York Times</title>
<script type="application/ld+json">
{
  "@context":"http://schema.org",
  "@type":"NewsArticle",
  "headline":"36 Hours in Lisbon",
  "description":"How to spend a weekend in Portugal's hilly capital.",
  "datePublished":"2024-04-04T09:00:00.000Z",
  "author":[{"@type":"Person","name":"Ingrid K. Williams"}],
  "url":"https://www.nytimes.com/2024/04/04/travel/36-hours-lisbon.html"
}
</script>
</head>
<body>
<article>
<h1>36 Hours in Lisbon</h1>
<h2>A weekend itinerary across the seven hills.</h2>
<p class="byline">By Ingrid K. Williams</p>
<time datetime="2024-04-04">April 4, 2024</time>
<p>Lisbon's hills, tiles and tram lines reward unhurried wandering.</p>
<p><strong>Time Out Market</strong> — A high-energy food hall with stalls from celebrated chefs. Open daily from noon.</p>
<p><strong>Pastéis de Belém</strong>: The original custard tart, baked on site since 1837. Expect a queue out the door.</p>
<p><strong>Miradouro de Santa Catarina</strong>. A sunset terrace popular with locals.</p>
<p>End the weekend with a fado set in Alfama before catching the morning flight home.</p>
</article>
</body></html>`

func newTestClient() *Client {
	return New(nil, "test-ua")
}

func TestArticle_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureArticleHTML))
	}))
	defer srv.Close()

	c := newTestClient()
	a, err := c.Article(context.Background(), srv.URL+"/2024/04/04/travel/36-hours-lisbon.html")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if a.Title != "36 Hours in Lisbon" {
		t.Errorf("title: %q", a.Title)
	}
	if !strings.Contains(a.Subtitle, "Portugal") {
		t.Errorf("subtitle: %q", a.Subtitle)
	}
	if a.Author != "Ingrid K. Williams" {
		t.Errorf("author: %q", a.Author)
	}
	if a.Date == "" {
		t.Errorf("date missing")
	}

	if len(a.Mentions) < 3 {
		t.Fatalf("expected ≥3 mentions, got %d (%+v)", len(a.Mentions), a.Mentions)
	}
	wantNames := map[string]bool{
		"Time Out Market":              false,
		"Pastéis de Belém":             false,
		"Miradouro de Santa Catarina":  false,
	}
	for _, m := range a.Mentions {
		if _, ok := wantNames[m.Name]; ok {
			wantNames[m.Name] = true
			if m.Description == "" {
				t.Errorf("mention %q has no description", m.Name)
			}
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Errorf("mention not extracted: %q", name)
		}
	}

	if !strings.Contains(a.Body, "hills, tiles and tram lines") {
		t.Errorf("body missing intro: %q", a.Body)
	}
}

func TestArticle_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient()
	if _, err := c.Article(context.Background(), srv.URL+"/2024/04/04/travel/36-hours-x.html"); err == nil {
		t.Fatal("expected error on 403")
	}
}

func TestArticle_EmptyURL(t *testing.T) {
	c := newTestClient()
	if _, err := c.Article(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestArticle_FallsBackToHTMLWhenNoJSONLD(t *testing.T) {
	const minimal = `<html><body>
<h1>36 Hours in Reykjavik</h1>
<h2>Geothermal pools, lava fields, lamb stew.</h2>
<p class="byline">By Pellegrino Riccardi</p>
<time datetime="2023-08-09">August 9, 2023</time>
<p>Iceland's capital is small, eccentric, and worth a long weekend.</p>
<p><strong>Sky Lagoon</strong>: A coastal geothermal spa with an infinity edge.</p>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(minimal))
	}))
	defer srv.Close()

	c := newTestClient()
	a, err := c.Article(context.Background(), srv.URL+"/2023/08/09/travel/36-hours-reykjavik.html")
	if err != nil {
		t.Fatalf("Article: %v", err)
	}
	if a.Title != "36 Hours in Reykjavik" {
		t.Errorf("title (HTML fallback): %q", a.Title)
	}
	if a.Author != "Pellegrino Riccardi" {
		t.Errorf("author (HTML fallback): %q", a.Author)
	}
	if a.Date == "" {
		t.Errorf("date (HTML fallback) missing")
	}
	if len(a.Mentions) != 1 || a.Mentions[0].Name != "Sky Lagoon" {
		t.Errorf("mentions: %+v", a.Mentions)
	}
}
