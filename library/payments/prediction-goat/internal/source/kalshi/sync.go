package kalshi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cliutil"
)

type KalshiStore interface {
	Upsert(resourceType, id string, data json.RawMessage) error
}

func SyncMarkets(ctx context.Context, c *Client, st KalshiStore, maxPages int) (count int, err error) {
	params := url.Values{}
	limit := "1000"
	if cliutil.IsDogfoodEnv() {
		limit = "50"
	}
	params.Set("limit", limit)
	params.Set("status", marketStatus(maxPages))

	count, err = syncPages(ctx, c, st, "/markets", params, maxPages, "kalshi_markets", "ticker", decodeMarkets)
	if err != nil {
		return count, fmt.Errorf("kalshi sync markets: %w", err)
	}
	return count, nil
}

func SyncEvents(ctx context.Context, c *Client, st KalshiStore, maxPages int) (count int, err error) {
	params := url.Values{}
	params.Set("limit", "200")

	count, err = syncPages(ctx, c, st, "/events", params, maxPages, "kalshi_events", "event_ticker", decodeEvents)
	if err != nil {
		return count, fmt.Errorf("kalshi sync events: %w", err)
	}
	return count, nil
}

func SyncSeries(ctx context.Context, c *Client, st KalshiStore, maxPages int) (count int, err error) {
	params := url.Values{}
	params.Set("limit", "200")

	count, err = syncPages(ctx, c, st, "/series", params, maxPages, "kalshi_series", "ticker", decodeSeries)
	if err != nil {
		return count, fmt.Errorf("kalshi sync series: %w", err)
	}
	return count, nil
}

type pageDecoder func([]byte) ([]json.RawMessage, string, error)

func syncPages(
	ctx context.Context,
	c *Client,
	st KalshiStore,
	path string,
	params url.Values,
	maxPages int,
	resourceType string,
	idField string,
	decode pageDecoder,
) (int, error) {
	if c == nil {
		c = New()
	}
	if st == nil {
		return 0, fmt.Errorf("nil store")
	}

	total := 0
	cursor := ""
	for page := 0; ; page++ {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		if maxPages > 0 && page >= maxPages {
			return total, nil
		}

		pageParams := cloneValues(params)
		if cursor != "" {
			pageParams.Set("cursor", cursor)
		}

		body, err := c.Get(ctx, path, pageParams)
		if err != nil {
			return total, err
		}
		items, nextCursor, err := decode(body)
		if err != nil {
			return total, err
		}
		for _, raw := range items {
			id, err := extractTicker(raw, idField)
			if err != nil {
				return total, err
			}
			if id == "" {
				fmt.Fprintf(os.Stderr, "kalshi: skipping %s item with empty %s\n", resourceType, idField)
				continue
			}
			if err := st.Upsert(resourceType, id, raw); err != nil {
				return total, err
			}
			total++
		}
		if nextCursor == "" {
			return total, nil
		}
		cursor = nextCursor
	}
}

func decodeMarkets(body []byte) ([]json.RawMessage, string, error) {
	var resp MarketsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("decode markets: %w", err)
	}
	if resp.Markets == nil {
		resp.Markets = make([]json.RawMessage, 0)
	}
	return resp.Markets, resp.Cursor, nil
}

func decodeEvents(body []byte) ([]json.RawMessage, string, error) {
	var resp EventsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("decode events: %w", err)
	}
	if resp.Events == nil {
		resp.Events = make([]json.RawMessage, 0)
	}
	return resp.Events, resp.Cursor, nil
}

func decodeSeries(body []byte) ([]json.RawMessage, string, error) {
	var resp SeriesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("decode series: %w", err)
	}
	if resp.Series == nil {
		resp.Series = make([]json.RawMessage, 0)
	}
	return resp.Series, resp.Cursor, nil
}

func extractTicker(raw json.RawMessage, idField string) (string, error) {
	var item map[string]json.RawMessage
	if err := json.Unmarshal(raw, &item); err != nil {
		return "", fmt.Errorf("extract %s: %w", idField, err)
	}
	var id string
	if value, ok := item[idField]; ok {
		if err := json.Unmarshal(value, &id); err != nil {
			return "", fmt.Errorf("extract %s: %w", idField, err)
		}
	}
	return id, nil
}

func cloneValues(values url.Values) url.Values {
	clone := url.Values{}
	for key, vals := range values {
		for _, value := range vals {
			clone.Add(key, value)
		}
	}
	return clone
}

func marketStatus(maxPages int) string {
	if maxPages > 5 {
		return "open,settled"
	}
	return "open"
}
