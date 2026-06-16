// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package kdpsource

import (
	"encoding/json"
	"fmt"
	"html"
	"testing"
)

// buildDataPage wraps a books JSON array in an Inertia envelope, marshals it,
// HTML-escapes it the way the server does, and embeds it in a data-page div.
func buildDataPage(t *testing.T, currentPage, lastPage int, books []Book) []byte {
	t.Helper()
	env := map[string]any{
		"component": "app/book/index",
		"props": map[string]any{
			"books": map[string]any{
				"current_page": currentPage,
				"last_page":    lastPage,
				"per_page":     50,
				"total":        len(books),
				"data":         books,
			},
			"savedBookIds": []int{},
		},
		"url":     "/app/category/hidden_gems",
		"version": "abc123",
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	escaped := html.EscapeString(string(raw))
	return []byte(fmt.Sprintf(`<!doctype html><html><body><div id="app" data-page="%s"></div></body></html>`, escaped))
}

func TestParseDataPage(t *testing.T) {
	sample := Book{
		ID:                      2584,
		Title:                   "TAI CHI FOR BELLY FAT AFTER 50",
		AmazonURL:               "https://www.amazon.com/dp/B0GTDWD9QL",
		ImageURL:                "https://images-na.ssl-images-amazon.com/images/I/61+Fhl-UsRL.jpg",
		Price:                   "19.99",
		Publisher:               "Independently published",
		EstimatedMonthlySales:   271,
		EstimatedMonthlyRevenue: 5422.71,
	}
	second := Book{
		ID:                      99,
		Title:                   "Another Niche & Title",
		AmazonURL:               "https://www.amazon.com/dp/B0ABCDEFGH",
		Price:                   "9.99",
		Publisher:               "Whale Press",
		EstimatedMonthlySales:   10,
		EstimatedMonthlyRevenue: 99.90,
	}

	tests := []struct {
		name        string
		htmlBytes   []byte
		wantErr     bool
		wantBooks   int
		wantCurrent int
		wantLast    int
	}{
		{
			name:        "two books",
			htmlBytes:   buildDataPage(t, 1, 3, []Book{sample, second}),
			wantBooks:   2,
			wantCurrent: 1,
			wantLast:    3,
		},
		{
			name:      "no data-page",
			htmlBytes: []byte(`<html><body>no inertia here</body></html>`),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			books, current, last, err := ParseDataPage(tc.htmlBytes)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(books) != tc.wantBooks {
				t.Fatalf("got %d books, want %d", len(books), tc.wantBooks)
			}
			if current != tc.wantCurrent || last != tc.wantLast {
				t.Fatalf("got current=%d last=%d, want current=%d last=%d", current, last, tc.wantCurrent, tc.wantLast)
			}
			b := books[0]
			if b.ID != sample.ID || b.Title != sample.Title || b.Price != sample.Price ||
				b.Publisher != sample.Publisher || b.EstimatedMonthlySales != sample.EstimatedMonthlySales ||
				b.EstimatedMonthlyRevenue != sample.EstimatedMonthlyRevenue {
				t.Fatalf("book fields did not round-trip: %+v", b)
			}
		})
	}
}

func TestASIN(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.amazon.com/dp/B0GTDWD9QL", "B0GTDWD9QL"},
		{"https://www.amazon.com/dp/B0ABCDEFGH?ref=x", "B0ABCDEFGH"},
		{"https://www.amazon.com/some/path", ""},
		{"", ""},
		{"https://www.amazon.com/dp/short", ""},
	}
	for _, tc := range tests {
		if got := ASIN(tc.url); got != tc.want {
			t.Errorf("ASIN(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}
