package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newAccommodationsDriftCmd(flags *rootFlags) *cobra.Command {
	var since string
	var threshold float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare current availability prices against stored history to detect price drift",
		Long: `Drift reads historical availability data from the local store, fetches current
prices from the API, and flags properties where pricing moved beyond the
specified threshold.

Requires synced availability data: run 'booking-com-demand-pp-cli sync --full' first.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli accommodations drift
  booking-com-demand-pp-cli accommodations drift --threshold 15
  booking-com-demand-pp-cli accommodations drift --since 7d
  booking-com-demand-pp-cli accommodations drift --since 30d --threshold 5`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nhint: run 'booking-com-demand-pp-cli sync --full' first", err)
			}
			defer s.Close()

			sinceDur, err := parseDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}

			cutoff := time.Now().Add(-sinceDur)
			rows, err := s.DB().Query(
				"SELECT id, data, synced_at FROM availability WHERE synced_at >= ?",
				cutoff.UTC(),
			)
			if err != nil {
				return fmt.Errorf("querying stored availability: %w", err)
			}
			defer rows.Close()

			type storedPrice struct {
				ID       string
				OldPrice float64
				SyncedAt time.Time
			}
			var stored []storedPrice
			var ids []string

			for rows.Next() {
				var id, data string
				var syncedAt time.Time
				if err := rows.Scan(&id, &data, &syncedAt); err != nil {
					continue
				}
				var obj map[string]any
				if json.Unmarshal([]byte(data), &obj) != nil {
					continue
				}
				price := extractPrice(obj)
				if price > 0 {
					stored = append(stored, storedPrice{ID: id, OldPrice: price, SyncedAt: syncedAt})
					ids = append(ids, id)
				}
			}

			if len(stored) == 0 {
				return writeNoop(flags, "no_data", "no availability data in store; run 'sync --full' first")
			}

			// Fetch current prices
			body := map[string]any{"accommodations": ids}
			currentData, _, err := c.Post("/accommodations/bulk-availability", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			currentPrices := map[string]float64{}
			var items []map[string]any
			if json.Unmarshal(currentData, &items) == nil {
				for _, item := range items {
					aid := fmt.Sprintf("%v", item["id"])
					p := extractPrice(item)
					if p > 0 {
						currentPrices[aid] = p
					}
				}
			}

			type driftResult struct {
				ID            string  `json:"id"`
				OldPrice      float64 `json:"old_price"`
				CurrentPrice  float64 `json:"current_price"`
				ChangePercent float64 `json:"change_percent"`
				Direction     string  `json:"direction"`
				SyncedAt      string  `json:"synced_at"`
			}

			var drifts []driftResult
			for _, sp := range stored {
				cp, ok := currentPrices[sp.ID]
				if !ok {
					continue
				}
				pctChange := ((cp - sp.OldPrice) / sp.OldPrice) * 100
				if math.Abs(pctChange) >= threshold {
					dir := "up"
					if pctChange < 0 {
						dir = "down"
					}
					drifts = append(drifts, driftResult{
						ID:            sp.ID,
						OldPrice:      sp.OldPrice,
						CurrentPrice:  cp,
						ChangePercent: math.Round(pctChange*100) / 100,
						Direction:     dir,
						SyncedAt:      sp.SyncedAt.Format(time.RFC3339),
					})
				}
			}

			if len(drifts) == 0 {
				return writeNoop(flags, "no_drift", fmt.Sprintf("no properties exceeded %.0f%% threshold", threshold))
			}

			return flags.printJSON(cmd, drifts)
		},
	}

	cmd.Flags().StringVar(&since, "since", "7d", "Look back duration (e.g. 7d, 24h, 30d)")
	cmd.Flags().Float64Var(&threshold, "threshold", 10, "Minimum percentage change to report")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")

	return cmd
}

// extractPrice attempts to find a price value in an API result object.
func extractPrice(obj map[string]any) float64 {
	for _, key := range []string{"price", "min_price", "total_price", "amount"} {
		if v, ok := obj[key]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case string:
				var f float64
				if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
					return f
				}
			}
		}
	}
	// Check nested price objects
	if priceObj, ok := obj["price"].(map[string]any); ok {
		for _, key := range []string{"total", "amount", "value"} {
			if v, ok := priceObj[key]; ok {
				if f, ok := v.(float64); ok {
					return f
				}
			}
		}
	}
	return 0
}

// parseDuration parses durations like "7d", "24h", "30d", "2w".
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 7 * 24 * time.Hour, nil
	}
	// Try standard Go duration first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Handle day/week shorthand
	var n int
	var unit string
	if _, err := fmt.Sscanf(s, "%d%s", &n, &unit); err != nil {
		return 0, fmt.Errorf("expected format like 7d, 24h, 2w")
	}
	switch unit {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit %q; use h, d, or w", unit)
	}
}
