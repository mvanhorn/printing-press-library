// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type staleEntry struct {
	URL            string `json:"url"`
	MaxRent        int    `json:"max_rent,omitempty"`
	UnchangedDays  int    `json:"unchanged_days"`
	LastChangedAt  string `json:"last_changed_at,omitempty"`
	LastObservedAt string `json:"last_observed_at,omitempty"`
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var limit int

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Flag synced listings whose price and availability haven't changed in N days.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli stale --days 30 --json
  apartments-pp-cli stale --days 60 --limit 50
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().Query(
				`SELECT listing_url, max_rent, available_at, observed_at
				 FROM listing_snapshots
				 ORDER BY listing_url, observed_at DESC`,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			type sample struct {
				rent       int
				avail      string
				observedAt time.Time
			}
			groups := map[string][]sample{}
			urlsOrdered := []string{}
			for rows.Next() {
				var (
					url   string
					rent  int
					avail string
					ts    string
				)
				if err := rows.Scan(&url, &rent, &avail, &ts); err != nil {
					return err
				}
				if _, ok := groups[url]; !ok {
					urlsOrdered = append(urlsOrdered, url)
				}
				groups[url] = append(groups[url], sample{rent, avail, parseSnapshotTime(ts)})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			now := time.Now().UTC()
			threshold := time.Duration(days) * 24 * time.Hour
			var out []staleEntry
			for _, url := range urlsOrdered {
				ss := groups[url]
				if len(ss) == 0 {
					continue
				}
				latest := ss[0]
				lastChanged := latest.observedAt
				for _, s := range ss[1:] {
					if s.rent == latest.rent && s.avail == latest.avail {
						lastChanged = s.observedAt
						continue
					}
					break
				}
				unchanged := now.Sub(lastChanged)
				if unchanged < threshold {
					continue
				}
				out = append(out, staleEntry{
					URL:            url,
					MaxRent:        latest.rent,
					UnchangedDays:  int(unchanged.Hours() / 24),
					LastChangedAt:  lastChanged.Format(time.RFC3339),
					LastObservedAt: latest.observedAt.Format(time.RFC3339),
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].UnchangedDays > out[j].UnchangedDays
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if out == nil {
				out = []staleEntry{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Threshold in days.")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return.")
	return cmd
}
