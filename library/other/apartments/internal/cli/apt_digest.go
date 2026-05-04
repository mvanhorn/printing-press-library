// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

type digestOutput struct {
	SavedSearch     string         `json:"saved_search"`
	Since           string         `json:"since"`
	GeneratedAt     string         `json:"generated_at"`
	NewListings     []watchEntry   `json:"new_listings"`
	RemovedListings []watchEntry   `json:"removed_listings"`
	PriceDrops      []dropEntry    `json:"price_drops"`
	TopBySqft       []rankEntry    `json:"top_by_sqft"`
	StaleListings   []staleEntry   `json:"stale_listings"`
	PhantomListings []phantomEntry `json:"phantom_listings"`
}

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var savedSearch string
	var sinceStr string
	var format string

	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "Single-shot composer: new + removed + drops + top-5 + stale + phantoms for one saved-search.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli digest --saved-search austin-2br --since 7d --json
  apartments-pp-cli digest --saved-search austin-2br --format md
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if savedSearch == "" {
				return usageErr(fmt.Errorf("--saved-search is required"))
			}
			switch format {
			case "", "json", "md":
			default:
				return usageErr(fmt.Errorf("invalid --format %q: must be json|md", format))
			}
			window, err := parseDurationLoose(sinceStr)
			if err != nil {
				return usageErr(err)
			}
			if window <= 0 {
				window = 7 * 24 * time.Hour
			}

			db, derr := openAptStore(cmd.Context())
			if derr != nil {
				return derr
			}
			defer db.Close()

			out := digestOutput{
				SavedSearch:     savedSearch,
				Since:           sinceStr,
				GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
				NewListings:     []watchEntry{},
				RemovedListings: []watchEntry{},
				PriceDrops:      []dropEntry{},
				TopBySqft:       []rankEntry{},
				StaleListings:   []staleEntry{},
				PhantomListings: []phantomEntry{},
			}

			// new + removed via watch logic.
			tsList, err := apt.LatestSyncTimestamps(db.DB(), savedSearch, 2)
			if err != nil {
				return err
			}
			if len(tsList) == 2 {
				latestRows, err := apt.SnapshotsForSearchAt(db.DB(), savedSearch, tsList[0])
				if err != nil {
					return err
				}
				prevRows, err := apt.SnapshotsForSearchAt(db.DB(), savedSearch, tsList[1])
				if err != nil {
					return err
				}
				latest := indexByURL(latestRows)
				prev := indexByURL(prevRows)
				for url, r := range latest {
					if _, ok := prev[url]; !ok {
						out.NewListings = append(out.NewListings, watchEntry{URL: url, MaxRent: r.MaxRent, Beds: r.Beds})
					}
				}
				for url, r := range prev {
					if _, ok := latest[url]; !ok {
						out.RemovedListings = append(out.RemovedListings, watchEntry{URL: url, MaxRent: r.MaxRent, Beds: r.Beds})
					}
				}
			}

			// price drops within saved-search url set + window.
			cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339)
			rows, err := db.DB().Query(
				`SELECT listing_url,
				        MAX(observed_at) AS latest_obs,
				        MIN(observed_at) AS earliest_obs
				 FROM listing_snapshots
				 WHERE saved_search = ? AND observed_at >= ? AND max_rent > 0
				 GROUP BY listing_url
				 HAVING COUNT(*) >= 2`,
				savedSearch, cutoff,
			)
			if err != nil {
				return err
			}
			type pair struct{ url, latestTS, earliestTS string }
			var pairs []pair
			for rows.Next() {
				var p pair
				if err := rows.Scan(&p.url, &p.latestTS, &p.earliestTS); err != nil {
					rows.Close()
					return err
				}
				pairs = append(pairs, p)
			}
			rows.Close()
			for _, p := range pairs {
				latestRent, _ := singleSnapshot(db.DB().QueryRow(
					`SELECT max_rent FROM listing_snapshots
					 WHERE listing_url = ? AND observed_at = ?
					 LIMIT 1`,
					p.url, p.latestTS,
				))
				earliestRent, _ := singleSnapshot(db.DB().QueryRow(
					`SELECT max_rent FROM listing_snapshots
					 WHERE listing_url = ? AND observed_at = ?
					 LIMIT 1`,
					p.url, p.earliestTS,
				))
				if earliestRent <= 0 || latestRent <= 0 {
					continue
				}
				dropPct := float64(earliestRent-latestRent) / float64(earliestRent) * 100.0
				if dropPct < 5 {
					continue
				}
				out.PriceDrops = append(out.PriceDrops, dropEntry{
					URL:          p.url,
					EarliestRent: earliestRent,
					LatestRent:   latestRent,
					DropPct:      dropPct,
					ObservedAt:   p.latestTS,
				})
			}
			sort.SliceStable(out.PriceDrops, func(i, j int) bool {
				return out.PriceDrops[i].DropPct > out.PriceDrops[j].DropPct
			})

			// top by sqft from listings whose URL appears in the
			// saved-search snapshots within window.
			urlSet := map[string]bool{}
			urlsRows, err := db.DB().Query(
				`SELECT DISTINCT listing_url FROM listing_snapshots
				 WHERE saved_search = ? AND observed_at >= ?`,
				savedSearch, cutoff,
			)
			if err == nil {
				for urlsRows.Next() {
					var u string
					if scanErr := urlsRows.Scan(&u); scanErr == nil {
						urlSet[u] = true
					}
				}
				urlsRows.Close()
			}

			cached, err := loadCachedListings(db.DB())
			if err != nil {
				return err
			}
			var ranked []rankEntry
			for _, r := range cached {
				li := r.Data
				if !urlSet[li.URL] {
					continue
				}
				e := rankEntry{URL: li.URL, PropertyID: li.PropertyID, Title: li.Title,
					Beds: li.Beds, MaxRent: li.MaxRent, Sqft: li.Sqft}
				if li.MaxRent > 0 && li.Sqft > 0 {
					e.PricePerSqft = float64(li.MaxRent) / float64(li.Sqft)
				}
				if li.MaxRent > 0 && li.Beds > 0 {
					e.PricePerBed = float64(li.MaxRent) / float64(li.Beds)
				}
				if e.PricePerSqft > 0 {
					ranked = append(ranked, e)
				}
			}
			sort.SliceStable(ranked, func(i, j int) bool {
				return ranked[i].PricePerSqft < ranked[j].PricePerSqft
			})
			if len(ranked) > 5 {
				ranked = ranked[:5]
			}
			for i := range ranked {
				ranked[i].Rank = i + 1
			}
			out.TopBySqft = ranked

			// stale within saved-search snapshots: pick urls observed
			// in this saved-search whose latest observed price hasn't
			// changed for ≥ window.
			staleURLs := map[string]struct {
				rent    int
				avail   string
				changed time.Time
				latest  time.Time
			}{}
			staleRows, err := db.DB().Query(
				`SELECT listing_url, max_rent, available_at, observed_at
				 FROM listing_snapshots
				 WHERE saved_search = ?
				 ORDER BY listing_url, observed_at DESC`,
				savedSearch,
			)
			if err != nil {
				return err
			}
			type ssample struct {
				rent       int
				avail      string
				observedAt time.Time
			}
			groups := map[string][]ssample{}
			ordered := []string{}
			for staleRows.Next() {
				var (
					url   string
					rent  int
					avail string
					ts    string
				)
				if err := staleRows.Scan(&url, &rent, &avail, &ts); err != nil {
					staleRows.Close()
					return err
				}
				if _, ok := groups[url]; !ok {
					ordered = append(ordered, url)
				}
				groups[url] = append(groups[url], ssample{rent, avail, parseSnapshotTime(ts)})
			}
			staleRows.Close()
			now := time.Now().UTC()
			for _, url := range ordered {
				ss := groups[url]
				if len(ss) == 0 {
					continue
				}
				lastChanged := ss[0].observedAt
				for _, s := range ss[1:] {
					if s.rent == ss[0].rent && s.avail == ss[0].avail {
						lastChanged = s.observedAt
						continue
					}
					break
				}
				if now.Sub(lastChanged) >= window {
					staleURLs[url] = struct {
						rent    int
						avail   string
						changed time.Time
						latest  time.Time
					}{ss[0].rent, ss[0].avail, lastChanged, ss[0].observedAt}
				}
			}
			for url, s := range staleURLs {
				out.StaleListings = append(out.StaleListings, staleEntry{
					URL:            url,
					MaxRent:        s.rent,
					UnchangedDays:  int(now.Sub(s.changed).Hours() / 24),
					LastChangedAt:  s.changed.Format(time.RFC3339),
					LastObservedAt: s.latest.Format(time.RFC3339),
				})
			}
			sort.SliceStable(out.StaleListings, func(i, j int) bool {
				return out.StaleListings[i].UnchangedDays > out.StaleListings[j].UnchangedDays
			})

			// phantoms: urls in this saved-search with fetch_error
			// signals or that didn't appear in the latest sync.
			phantoms := map[string][]string{}
			perr := db.DB().QueryRow(
				`SELECT 1 FROM listing_snapshots WHERE saved_search = ? LIMIT 1`,
				savedSearch,
			).Scan(new(int))
			_ = perr
			fetchErrRows, err := db.DB().Query(
				`SELECT DISTINCT listing_url FROM listing_snapshots
				 WHERE saved_search = ? AND fetch_status >= 400`,
				savedSearch,
			)
			if err == nil {
				for fetchErrRows.Next() {
					var u string
					if scanErr := fetchErrRows.Scan(&u); scanErr == nil {
						phantoms[u] = appendUnique(phantoms[u], "fetch_error")
					}
				}
				fetchErrRows.Close()
			}
			if len(tsList) > 0 {
				latestSync := tsList[0]
				droppedRows, err := db.DB().Query(
					`SELECT DISTINCT listing_url FROM listing_snapshots
					 WHERE saved_search = ?`,
					savedSearch,
				)
				if err == nil {
					for droppedRows.Next() {
						var u string
						if scanErr := droppedRows.Scan(&u); scanErr == nil {
							var maxObs string
							if perr := db.DB().QueryRow(
								`SELECT MAX(observed_at) FROM listing_snapshots
								 WHERE listing_url = ? AND saved_search = ?`,
								u, savedSearch,
							).Scan(&maxObs); perr == nil {
								if parseSnapshotTime(maxObs).Before(latestSync.Add(-1 * time.Second)) {
									phantoms[u] = appendUnique(phantoms[u], "dropped_from_search")
								}
							}
						}
					}
					droppedRows.Close()
				}
			}
			for url, st := range staleURLs {
				phantoms[url] = appendUnique(phantoms[url], "stale")
				_ = st
			}
			for url, reasons := range phantoms {
				if len(reasons) == 0 {
					continue
				}
				out.PhantomListings = append(out.PhantomListings, phantomEntry{
					URL:     url,
					Reasons: reasons,
				})
			}
			sort.SliceStable(out.PhantomListings, func(i, j int) bool {
				return len(out.PhantomListings[i].Reasons) > len(out.PhantomListings[j].Reasons)
			})

			if format == "md" {
				return writeDigestMarkdown(cmd, out)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&savedSearch, "saved-search", "", "Saved-search slug (required).")
	cmd.Flags().StringVar(&sinceStr, "since", "7d", "Window: how far back to look.")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json|md.")
	return cmd
}

func writeDigestMarkdown(cmd *cobra.Command, d digestOutput) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "# Digest: %s\n\n", d.SavedSearch)
	fmt.Fprintf(w, "Generated %s, since %s.\n\n", d.GeneratedAt, d.Since)
	mdBlock(w, "New listings", len(d.NewListings))
	for _, e := range d.NewListings {
		fmt.Fprintf(w, "- [%s](%s) — $%d/mo\n", e.URL, e.URL, e.MaxRent)
	}
	mdBlock(w, "Removed listings", len(d.RemovedListings))
	for _, e := range d.RemovedListings {
		fmt.Fprintf(w, "- [%s](%s) — $%d/mo\n", e.URL, e.URL, e.MaxRent)
	}
	mdBlock(w, "Price drops", len(d.PriceDrops))
	for _, e := range d.PriceDrops {
		fmt.Fprintf(w, "- [%s](%s) — %.1f%% drop ($%d → $%d)\n", e.URL, e.URL, e.DropPct, e.EarliestRent, e.LatestRent)
	}
	mdBlock(w, "Top by $/sqft", len(d.TopBySqft))
	for _, e := range d.TopBySqft {
		fmt.Fprintf(w, "- [%s](%s) — $%.2f/sqft (%d sqft, $%d/mo)\n", e.URL, e.URL, e.PricePerSqft, e.Sqft, e.MaxRent)
	}
	mdBlock(w, "Stale", len(d.StaleListings))
	for _, e := range d.StaleListings {
		fmt.Fprintf(w, "- [%s](%s) — unchanged %d days\n", e.URL, e.URL, e.UnchangedDays)
	}
	mdBlock(w, "Phantoms", len(d.PhantomListings))
	for _, e := range d.PhantomListings {
		fmt.Fprintf(w, "- [%s](%s) — %s\n", e.URL, e.URL, strings.Join(e.Reasons, ", "))
	}
	return nil
}

func mdBlock(w io.Writer, title string, n int) {
	fmt.Fprintf(w, "## %s (%d)\n\n", title, n)
}
