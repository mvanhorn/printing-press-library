// Package history reads JSONL dumps of Amazon order history and writes them
// into the store.
//
// JSONL shape (one order per line):
//
//	{
//	  "order_id":   "112-1234567-1234567",
//	  "placed_at":  "2026-04-15T00:00:00Z",  // also accepts "April 15, 2026"
//	  "total":      "$42.31",                 // optional, parsed to cents
//	  "items": [
//	    {"asin": "B07X1Y2Z3", "title": "Charmin Ultra Strong 24 Mega Rolls",
//	     "quantity": 1, "unit_price": "$28.49"}
//	  ]
//	}
//
// Anything the parser does not recognize is stored on the raw_json column so
// `history list --json` still surfaces it. The companion DevTools dumper at
// docs/dumper.js produces this exact shape.
package history

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/store"
)

// RawOrder is the JSONL line shape — kept lenient: timestamps and prices are
// strings because Amazon's HTML uses human-formatted versions.
type RawOrder struct {
	OrderID  string    `json:"order_id"`
	PlacedAt string    `json:"placed_at"`
	Total    string    `json:"total"`
	Items    []RawItem `json:"items"`
}

// RawItem is one line in a RawOrder.
type RawItem struct {
	ASIN      string `json:"asin"`
	Title     string `json:"title"`
	Quantity  int    `json:"quantity"`
	UnitPrice string `json:"unit_price"`
}

// ImportResult is returned to the caller.
type ImportResult struct {
	OrdersRead    int `json:"orders_read"`
	OrdersWritten int `json:"orders_written"`
	ItemsWritten  int `json:"items_written"`
	Skipped       int `json:"skipped"`
}

// ImportFile reads path and upserts orders/order_items into s.
// Returns the count of rows touched.
func ImportFile(ctx context.Context, s *store.Store, path string) (ImportResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImportResult{}, err
	}
	defer f.Close()
	return ImportReader(ctx, s, f)
}

// ImportReader is the same as ImportFile but takes a Reader directly so tests
// can pass a strings.Reader.
func ImportReader(ctx context.Context, s *store.Store, r io.Reader) (ImportResult, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var res ImportResult
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var raw RawOrder
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			res.Skipped++
			continue
		}
		res.OrdersRead++
		if raw.OrderID == "" {
			res.Skipped++
			continue
		}
		placedAt, err := parseTimestamp(raw.PlacedAt)
		if err != nil {
			res.Skipped++
			continue
		}
		o := store.Order{
			OrderID:    raw.OrderID,
			PlacedAt:   placedAt,
			TotalCents: parsePriceCents(raw.Total),
		}
		if err := s.UpsertOrder(ctx, o, line); err != nil {
			return res, fmt.Errorf("upsert order %s: %w", raw.OrderID, err)
		}
		res.OrdersWritten++
		for _, ri := range raw.Items {
			if ri.ASIN == "" {
				continue
			}
			qty := ri.Quantity
			if qty <= 0 {
				qty = 1
			}
			oi := store.OrderItem{
				OrderID:        raw.OrderID,
				ASIN:           ri.ASIN,
				Title:          ri.Title,
				Quantity:       qty,
				UnitPriceCents: parsePriceCents(ri.UnitPrice),
			}
			itemJSON, _ := json.Marshal(ri)
			if err := s.UpsertOrderItem(ctx, oi, string(itemJSON)); err != nil {
				return res, fmt.Errorf("upsert item %s on %s: %w", ri.ASIN, raw.OrderID, err)
			}
			res.ItemsWritten++
		}
	}
	if err := scanner.Err(); err != nil {
		return res, err
	}
	if err := s.RebuildPurchasedItems(ctx); err != nil {
		return res, fmt.Errorf("rebuild purchased_items: %w", err)
	}
	return res, nil
}

// parseTimestamp accepts RFC3339, RFC3339 without timezone, and Amazon's
// human format ("April 15, 2026" / "Apr 15, 2026").
func parseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty placed_at")
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse timestamp %q", s)
}

var priceRe = regexp.MustCompile(`-?(\d{1,3}(?:,\d{3})*|\d+)(?:\.(\d{1,2}))?`)

// parsePriceCents handles "$42.31", "USD 42.31", "$1,234.56", "42.31".
// Returns 0 on failure (caller can treat 0 as unknown).
func parsePriceCents(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	m := priceRe.FindStringSubmatch(s)
	if len(m) == 0 {
		return 0
	}
	dollars := strings.ReplaceAll(m[1], ",", "")
	cents := m[2]
	if len(cents) == 1 {
		cents += "0"
	}
	if cents == "" {
		cents = "00"
	}
	d, err := strconv.ParseInt(dollars, 10, 64)
	if err != nil {
		return 0
	}
	c, err := strconv.ParseInt(cents, 10, 64)
	if err != nil {
		return 0
	}
	return d*100 + c
}
