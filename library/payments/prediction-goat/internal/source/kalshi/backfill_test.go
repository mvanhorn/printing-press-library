package kalshi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

// fakeReader returns a fixed candidate list for the backfill pass.
type fakeReader struct {
	candidates []BackfillCandidate
	err        error
	gotMin     float64
}

func (f *fakeReader) BackfillCandidates(_ context.Context, minVolume float64) ([]BackfillCandidate, error) {
	f.gotMin = minVolume
	if f.err != nil {
		return nil, f.err
	}
	return f.candidates, nil
}

// fakeStore captures upserts so tests can assert what got written.
type fakeStore struct {
	mu       sync.Mutex
	upserted map[string]json.RawMessage
	failOn   string // ticker that should error on upsert
}

func (s *fakeStore) Upsert(resourceType, id string, data json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn != "" && id == s.failOn {
		return errors.New("upsert failed")
	}
	if s.upserted == nil {
		s.upserted = map[string]json.RawMessage{}
	}
	s.upserted[resourceType+"|"+id] = append(json.RawMessage{}, data...)
	return nil
}

func newFakeKalshiServer(t *testing.T, payloads map[string]string, fail404 map[string]bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/markets/", func(w http.ResponseWriter, r *http.Request) {
		ticker := strings.TrimPrefix(r.URL.Path, "/markets/")
		if fail404[ticker] {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		body, ok := payloads[ticker]
		if !ok {
			http.Error(w, "no fixture", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	return httptest.NewServer(mux)
}

// TestBackfill_HappyPath verifies that BackfillMarketPrices calls GetMarket
// for every candidate ticker and upserts the unwrapped market payload.
func TestBackfill_HappyPath(t *testing.T) {
	srv := newFakeKalshiServer(t, map[string]string{
		"KXNBAWEST-26-OKC": `{"market":{"ticker":"KXNBAWEST-26-OKC","status":"active","yes_ask_dollars":0.78,"no_ask_dollars":0.23,"last_price_dollars":0.78,"volume_24h_fp":371000}}`,
		"KXNBAWEST-26-SAS": `{"market":{"ticker":"KXNBAWEST-26-SAS","status":"active","yes_ask_dollars":0.23,"no_ask_dollars":0.78,"last_price_dollars":0.22,"volume_24h_fp":280000}}`,
	}, nil)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	reader := &fakeReader{candidates: []BackfillCandidate{{Ticker: "KXNBAWEST-26-OKC"}, {Ticker: "KXNBAWEST-26-SAS"}}}
	store := &fakeStore{}
	stats, err := BackfillMarketPrices(context.Background(), client, store, reader, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Considered != 2 {
		t.Errorf("considered = %d, want 2", stats.Considered)
	}
	if stats.Updated != 2 {
		t.Errorf("updated = %d, want 2", stats.Updated)
	}
	if stats.Errors != 0 {
		t.Errorf("errors = %d, want 0", stats.Errors)
	}
	if reader.gotMin != 1000 {
		t.Errorf("reader.gotMin = %v, want 1000", reader.gotMin)
	}
	// Verify the stored body is the unwrapped market object, not the
	// {"market": ...} envelope.
	raw, ok := store.upserted["kalshi_markets|KXNBAWEST-26-OKC"]
	if !ok {
		t.Fatalf("expected upsert for KXNBAWEST-26-OKC")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj["ticker"] != "KXNBAWEST-26-OKC" {
		t.Errorf("stored ticker = %v, want KXNBAWEST-26-OKC", obj["ticker"])
	}
	if obj["yes_ask_dollars"] != 0.78 {
		t.Errorf("stored yes_ask_dollars = %v, want 0.78", obj["yes_ask_dollars"])
	}
}

// TestBackfill_404Continues verifies a per-market 404 is logged + counted
// as an error but doesn't fail the whole backfill.
func TestBackfill_404Continues(t *testing.T) {
	srv := newFakeKalshiServer(t, map[string]string{
		"GOOD": `{"market":{"ticker":"GOOD","status":"active","last_price_dollars":0.5}}`,
	}, map[string]bool{"BAD": true})
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	reader := &fakeReader{candidates: []BackfillCandidate{{Ticker: "BAD"}, {Ticker: "GOOD"}}}
	store := &fakeStore{}
	stats, err := BackfillMarketPrices(context.Background(), client, store, reader, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Updated != 1 {
		t.Errorf("updated = %d, want 1", stats.Updated)
	}
	if stats.Errors != 1 {
		t.Errorf("errors = %d, want 1", stats.Errors)
	}
}

// TestBackfill_UpsertErrorContinues verifies a store upsert failure is
// counted but doesn't abort the run.
func TestBackfill_UpsertErrorContinues(t *testing.T) {
	srv := newFakeKalshiServer(t, map[string]string{
		"A": `{"market":{"ticker":"A","status":"active"}}`,
		"B": `{"market":{"ticker":"B","status":"active"}}`,
	}, nil)
	defer srv.Close()
	client := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	reader := &fakeReader{candidates: []BackfillCandidate{{Ticker: "A"}, {Ticker: "B"}}}
	store := &fakeStore{failOn: "A"}
	stats, err := BackfillMarketPrices(context.Background(), client, store, reader, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Updated != 1 {
		t.Errorf("updated = %d, want 1", stats.Updated)
	}
	if stats.Errors != 1 {
		t.Errorf("errors = %d, want 1", stats.Errors)
	}
}

// TestBackfill_NilReader returns a structured error, not a panic.
func TestBackfill_NilReader(t *testing.T) {
	_, err := BackfillMarketPrices(context.Background(), nil, &fakeStore{}, nil, 0)
	if err == nil {
		t.Errorf("expected error on nil reader")
	}
}

// TestResolveBackfillMinVolume covers the env override semantics: invalid
// input falls back to default; absent uses default; valid parses as float.
func TestResolveBackfillMinVolume(t *testing.T) {
	prev := os.Getenv("PREDICTION_GOAT_KALSHI_BACKFILL_MIN_VOLUME")
	defer os.Setenv("PREDICTION_GOAT_KALSHI_BACKFILL_MIN_VOLUME", prev)

	cases := []struct {
		name string
		env  string
		want float64
	}{
		{"unset", "", DefaultBackfillMinVolume},
		{"valid 5000", "5000", 5000},
		{"valid 0", "0", 0},
		{"invalid string", "not-a-number", DefaultBackfillMinVolume},
		{"negative falls back", "-1", DefaultBackfillMinVolume},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("PREDICTION_GOAT_KALSHI_BACKFILL_MIN_VOLUME", tc.env)
			got := ResolveBackfillMinVolume()
			if got != tc.want {
				t.Errorf("env=%q -> %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}
