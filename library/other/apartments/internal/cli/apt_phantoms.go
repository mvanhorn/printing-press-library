// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type phantomEntry struct {
	URL            string   `json:"url"`
	Reasons        []string `json:"reasons"`
	LastObservedAt string   `json:"last_observed_at,omitempty"`
	LastMaxRent    int      `json:"last_max_rent,omitempty"`
}

func newPhantomsCmd(flags *rootFlags) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:         "phantoms",
		Short:       "Surface listings flagged by 404, dropped-from-search, or stale.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli phantoms --days 45 --json
  apartments-pp-cli phantoms --days 30
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

			latest, err := latestObservationPerURL(db.DB())
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			threshold := time.Duration(days) * 24 * time.Hour
			reasonsByURL := map[string][]string{}

			// Signal A: any snapshot with fetch_status >= 400.
			rows, err := db.DB().Query(
				`SELECT DISTINCT listing_url
				 FROM listing_snapshots
				 WHERE fetch_status >= 400`,
			)
			if err == nil {
				for rows.Next() {
					var url string
					if scanErr := rows.Scan(&url); scanErr == nil {
						reasonsByURL[url] = append(reasonsByURL[url], "fetch_error")
					}
				}
				rows.Close()
			}

			// Signal B: dropped from saved-search. For each url, find
			// the latest sync of any saved-search it ever appeared in,
			// and compare to its own latest observation.
			savedRows, err := db.DB().Query(
				`SELECT s.listing_url, s.saved_search,
				        (SELECT MAX(observed_at) FROM listing_snapshots ls
				         WHERE ls.saved_search = s.saved_search) AS search_latest,
				        s.observed_at AS url_latest_in_search
				 FROM (
				   SELECT listing_url, saved_search, MAX(observed_at) AS observed_at
				   FROM listing_snapshots
				   WHERE saved_search IS NOT NULL AND saved_search <> ''
				   GROUP BY listing_url, saved_search
				 ) s`,
			)
			if err == nil {
				for savedRows.Next() {
					var (
						url          string
						search       string
						searchLatest string
						urlLatest    string
					)
					if scanErr := savedRows.Scan(&url, &search, &searchLatest, &urlLatest); scanErr == nil {
						if parseSnapshotTime(urlLatest).Before(parseSnapshotTime(searchLatest)) {
							reasonsByURL[url] = appendUnique(reasonsByURL[url], "dropped_from_search")
						}
					}
				}
				savedRows.Close()
			}

			// Signal C: stale ≥ days.
			latestByURL := map[string]int{}
			latestObsByURL := map[string]time.Time{}
			for _, r := range latest {
				latestByURL[r.ListingURL] = r.MaxRent
				latestObsByURL[r.ListingURL] = r.ObservedAt
				if now.Sub(r.ObservedAt) >= threshold {
					reasonsByURL[r.ListingURL] = appendUnique(reasonsByURL[r.ListingURL], "stale")
				}
			}

			var out []phantomEntry
			for url, rs := range reasonsByURL {
				if len(rs) == 0 {
					continue
				}
				obs := latestObsByURL[url]
				out = append(out, phantomEntry{
					URL:            url,
					Reasons:        rs,
					LastObservedAt: obs.Format(time.RFC3339),
					LastMaxRent:    latestByURL[url],
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				if len(out[i].Reasons) != len(out[j].Reasons) {
					return len(out[i].Reasons) > len(out[j].Reasons)
				}
				return out[i].URL < out[j].URL
			})
			if out == nil {
				out = []phantomEntry{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 45, "Stale threshold in days.")
	return cmd
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
