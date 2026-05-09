// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ChangeEvent describes a detected catalog change.
type ChangeEvent struct {
	Slug        string `json:"slug"`
	Country     string `json:"country"`
	Type        string `json:"type"` // "added", "removed", "price_change"
	OldPrice    string `json:"old_price,omitempty"`
	NewPrice    string `json:"new_price,omitempty"`
	Category    string `json:"category,omitempty"`
	URL         string `json:"url,omitempty"`
	DetectedAt  string `json:"detected_at"`
}

func newChangesCmd(flags *rootFlags) *cobra.Command {
	var since string
	var country string
	var changeType string

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Detect products added, removed, or changed in price since your last sync",
		Long: `Compare the current catalog snapshot against a previous one to find what changed.
Requires at least two sync runs for the target country.

Run 'iqos-pp-cli sync --country <code>' to record a new snapshot.`,
		Example: strings.Trim(`
  iqos-pp-cli changes --since 7d --country gb --json
  iqos-pp-cli changes --since 30d --country us --type added
  iqos-pp-cli changes --since 7d --country gb --type price`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if country == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Usage: iqos-pp-cli changes --since <duration> --country <code>")
				return cmd.Help()
			}

			db, err := iqosOpenDB(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff, err := parseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: use e.g. 7d, 30d, 90d", since)
			}
			cutoffStr := cutoff.UTC().Format("2006-01-02T15:04:05Z")

			// Find the oldest snapshot within the window for this country
			var oldestAt string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT MIN(snapshot_at) FROM iqos_snapshots WHERE country=? AND snapshot_at >= ?`,
				country, cutoffStr).Scan(&oldestAt)
			if err != nil || oldestAt == "" {
				return fmt.Errorf("no snapshots found for %s since %s — run 'iqos-pp-cli sync --country %s' first",
					country, since, country)
			}

			// Find the most recent snapshot
			var latestAt string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT MAX(snapshot_at) FROM iqos_snapshots WHERE country=?`,
				country).Scan(&latestAt)
			if err != nil || latestAt == "" || latestAt == oldestAt {
				return fmt.Errorf("only one snapshot found for %s — run 'iqos-pp-cli sync --country %s' again to compare", country, country)
			}

			var events []ChangeEvent
			rawDB := db.DB()

			// Products in latest but not in oldest → added
			if changeType == "" || changeType == "added" {
				rows, err := rawDB.QueryContext(cmd.Context(), `
					SELECT s2.slug, s2.country, COALESCE(p.url,''), COALESCE(p.category,''), s2.snapshot_at
					FROM iqos_snapshots s2
					LEFT JOIN iqos_products p ON p.slug=s2.slug AND p.country=s2.country
					WHERE s2.country=? AND s2.snapshot_at=?
					  AND s2.slug NOT IN (
					    SELECT slug FROM iqos_snapshots WHERE country=? AND snapshot_at=?
					  )
				`, country, latestAt, country, oldestAt)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var e ChangeEvent
						var det string
						if sErr := rows.Scan(&e.Slug, &e.Country, &e.URL, &e.Category, &det); sErr == nil {
							e.Type = "added"
							e.DetectedAt = det
							events = append(events, e)
						}
					}
				}
			}

			// Products in oldest but not in latest → removed
			if changeType == "" || changeType == "removed" {
				rows, err := rawDB.QueryContext(cmd.Context(), `
					SELECT s1.slug, s1.country, COALESCE(p.url,''), COALESCE(p.category,''), ?
					FROM iqos_snapshots s1
					LEFT JOIN iqos_products p ON p.slug=s1.slug AND p.country=s1.country
					WHERE s1.country=? AND s1.snapshot_at=?
					  AND s1.slug NOT IN (
					    SELECT slug FROM iqos_snapshots WHERE country=? AND snapshot_at=?
					  )
				`, latestAt, country, oldestAt, country, latestAt)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var e ChangeEvent
						var det string
						if sErr := rows.Scan(&e.Slug, &e.Country, &e.URL, &e.Category, &det); sErr == nil {
							e.Type = "removed"
							e.DetectedAt = det
							events = append(events, e)
						}
					}
				}
			}

			// Products in both with different prices → price_change
			if changeType == "" || changeType == "price" || changeType == "price_change" {
				rows, err := rawDB.QueryContext(cmd.Context(), `
					SELECT s2.slug, s2.country, s1.price, s2.price, COALESCE(p.url,''), COALESCE(p.category,''), s2.snapshot_at
					FROM iqos_snapshots s1
					JOIN iqos_snapshots s2 ON s1.slug=s2.slug AND s1.country=s2.country
					LEFT JOIN iqos_products p ON p.slug=s2.slug AND p.country=s2.country
					WHERE s1.country=? AND s1.snapshot_at=? AND s2.snapshot_at=?
					  AND s1.price != s2.price
					  AND s1.price != '' AND s2.price != ''
				`, country, oldestAt, latestAt)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var e ChangeEvent
						if sErr := rows.Scan(&e.Slug, &e.Country, &e.OldPrice, &e.NewPrice,
							&e.URL, &e.Category, &e.DetectedAt); sErr == nil {
							e.Type = "price_change"
							events = append(events, e)
						}
					}
				}
			}

			if events == nil {
				events = []ChangeEvent{}
			}

			data, err := json.Marshal(events)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Look back window (e.g. 7d, 30d, 90d)")
	cmd.Flags().StringVar(&country, "country", "", "Country code to check (e.g. gb, us, de)")
	cmd.Flags().StringVar(&changeType, "type", "", "Filter by change type: added, removed, price")
	return cmd
}

// parseDuration parses simple duration strings like "7d", "30d" into a past time.Time.
func parseDuration(s string) (time.Time, error) {
	if s == "" {
		s = "7d"
	}
	s = strings.ToLower(strings.TrimSpace(s))
	now := time.Now().UTC()
	switch {
	case strings.HasSuffix(s, "d"):
		n := 0
		if _, err := fmt.Sscanf(s, "%dd", &n); err != nil || n <= 0 {
			return time.Time{}, fmt.Errorf("invalid day count in %q", s)
		}
		return now.AddDate(0, 0, -n), nil
	case strings.HasSuffix(s, "h"):
		n := 0
		if _, err := fmt.Sscanf(s, "%dh", &n); err != nil || n <= 0 {
			return time.Time{}, fmt.Errorf("invalid hour count in %q", s)
		}
		return now.Add(-time.Duration(n) * time.Hour), nil
	case strings.HasSuffix(s, "w"):
		n := 0
		if _, err := fmt.Sscanf(s, "%dw", &n); err != nil || n <= 0 {
			return time.Time{}, fmt.Errorf("invalid week count in %q", s)
		}
		return now.AddDate(0, 0, -n*7), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported duration %q — use Nd, Nw, or Nh", s)
	}
}
