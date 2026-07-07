package polymarket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEventForMarket_HappyPath confirms the walk from market slug to
// parent event slug: GET /markets?slug=X yields a markets[0].events[0]
// with the parent slug.
func TestEventForMarket_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/markets", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("slug") != "will-ghana-win-the-2026-fifa-world-cup" {
			http.Error(w, "wrong slug", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"will-ghana-win-the-2026-fifa-world-cup","events":[{"slug":"2026-fifa-world-cup-winner-595","title":"2026 FIFA World Cup Winner","marketCount":48}]}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ev, ok, err := client.EventForMarket(context.Background(), "will-ghana-win-the-2026-fifa-world-cup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.Slug != "2026-fifa-world-cup-winner-595" {
		t.Errorf("event slug = %q, want 2026-fifa-world-cup-winner-595", ev.Slug)
	}
	if ev.MarketCount != 48 {
		t.Errorf("marketCount = %d, want 48", ev.MarketCount)
	}
}

// TestEventForMarket_NotFound returns ok=false with no error when the
// gamma API returns an empty array (market slug doesn't exist).
func TestEventForMarket_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/markets", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, ok, err := client.EventForMarket(context.Background(), "missing")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ok {
		t.Errorf("expected ok=false for missing slug")
	}
}

// TestSiblingsForMarket_HappyPath confirms the two-hop walk surfaces all
// child markets sorted by volume descending and parses outcomePrices
// strings into float yes probabilities.
func TestSiblingsForMarket_HappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/markets", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"will-ghana-win-the-2026-fifa-world-cup","events":[{"slug":"2026-fifa-world-cup-winner-595","title":"2026 FIFA World Cup Winner"}]}]`))
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"2026-fifa-world-cup-winner-595","markets":[
			{"slug":"will-usa-win-the-2026-fifa-world-cup-467","question":"Will USA win the 2026 FIFA World Cup?","outcomePrices":"[\"0.012\",\"0.988\"]","volumeNum":34200000,"closed":false,"endDate":"2026-07-20T00:00:00Z"},
			{"slug":"will-france-win-the-2026-fifa-world-cup","question":"Will France win the 2026 FIFA World Cup?","outcomePrices":"[\"0.179\",\"0.821\"]","volumeNum":42000000,"closed":false},
			{"slug":"will-finland-win-the-2026-fifa-world-cup","question":"Will Finland win the 2026 FIFA World Cup?","outcomePrices":"[\"0.001\",\"0.999\"]","volumeNum":50000,"closed":true}
		]}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	ev, siblings, err := client.SiblingsForMarket(context.Background(), "will-ghana-win-the-2026-fifa-world-cup", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Slug != "2026-fifa-world-cup-winner-595" {
		t.Errorf("event slug = %q, want 2026-fifa-world-cup-winner-595", ev.Slug)
	}
	if len(siblings) != 2 {
		t.Fatalf("siblings = %d, want 2 (closed filtered)", len(siblings))
	}
	if siblings[0].Slug != "will-france-win-the-2026-fifa-world-cup" {
		t.Errorf("first sibling = %q, want France (highest volume)", siblings[0].Slug)
	}
	if siblings[0].YesProbability != 0.179 {
		t.Errorf("France YesProbability = %v, want 0.179", siblings[0].YesProbability)
	}
	if siblings[0].YesPercent != 17.9 {
		t.Errorf("France YesPercent = %v, want 17.9", siblings[0].YesPercent)
	}
	// USA market is force-included regardless of volume rank
	usaFound := false
	for _, s := range siblings {
		if s.Slug == "will-usa-win-the-2026-fifa-world-cup-467" {
			usaFound = true
			if s.YesProbability != 0.012 {
				t.Errorf("USA YesProbability = %v, want 0.012", s.YesProbability)
			}
		}
	}
	if !usaFound {
		t.Errorf("expected USA sibling in result")
	}
}

// TestSiblingsForMarket_IncludeClosed reverses the closed filter.
func TestSiblingsForMarket_IncludeClosed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/markets", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"x","events":[{"slug":"e","title":"E"}]}]`))
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"e","markets":[{"slug":"open","closed":false},{"slug":"closed","closed":true}]}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, siblings, err := client.SiblingsForMarket(context.Background(), "x", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(siblings) != 2 {
		t.Errorf("siblings = %d, want 2 (closed included)", len(siblings))
	}
}

// TestSiblingsForMarket_NoParent surfaces a clear error when the market
// exists but has no parent event (not a multi-outcome family).
func TestSiblingsForMarket_NoParent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/markets", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, _, err := client.SiblingsForMarket(context.Background(), "orphan", false)
	if err == nil {
		t.Errorf("expected error for market with no parent event")
	}
}

// TestParseOutcomePriceYes covers the JSON-string-encoded outcomePrices
// shape Polymarket uses. Invalid inputs return 0 without erroring out.
func TestParseOutcomePriceYes(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{`["0.062","0.938"]`, 0.062},
		{`["1.0","0.0"]`, 1.0},
		{`[]`, 0},
		{``, 0},
		{`not-json`, 0},
	}
	for _, tc := range cases {
		got := parseOutcomePriceYes(tc.in)
		if got != tc.want {
			t.Errorf("parseOutcomePriceYes(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
