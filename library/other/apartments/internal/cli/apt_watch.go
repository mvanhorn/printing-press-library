// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

// watchEntry is what we emit for each diff bucket.
type watchEntry struct {
	URL      string `json:"url"`
	MaxRent  int    `json:"max_rent,omitempty"`
	PrevRent int    `json:"prev_max_rent,omitempty"`
	Beds     int    `json:"beds,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string

	cmd := &cobra.Command{
		Use:         "watch <saved-search>",
		Short:       "Diff the latest two syncs of a saved-search: NEW, REMOVED, PRICE_CHANGED.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli watch austin-2br
  apartments-pp-cli watch austin-2br --since 7d --json
  apartments-pp-cli watch downtown-pet-friendly --since 24h
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			out := map[string]any{
				"saved_search":     slug,
				"new_listings":     []watchEntry{},
				"removed_listings": []watchEntry{},
				"price_changed":    []watchEntry{},
			}

			tsList, err := apt.LatestSyncTimestamps(db.DB(), slug, 2)
			if err != nil {
				return err
			}
			since, perr := parseDurationLoose(sinceStr)
			if perr != nil {
				return usageErr(perr)
			}
			// Optional --since gate: drop the older timestamp if it
			// falls outside the window.
			if since > 0 && len(tsList) >= 2 {
				cutoff := time.Now().Add(-since)
				if tsList[1].Before(cutoff) {
					tsList = tsList[:1]
				}
			}
			if len(tsList) < 2 {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			latestRows, err := apt.SnapshotsForSearchAt(db.DB(), slug, tsList[0])
			if err != nil {
				return err
			}
			prevRows, err := apt.SnapshotsForSearchAt(db.DB(), slug, tsList[1])
			if err != nil {
				return err
			}
			latest := indexByURL(latestRows)
			prev := indexByURL(prevRows)

			var newOnes, removed, priced []watchEntry
			for url, r := range latest {
				if _, ok := prev[url]; !ok {
					newOnes = append(newOnes, watchEntry{URL: url, MaxRent: r.MaxRent, Beds: r.Beds})
				}
			}
			for url, r := range prev {
				if _, ok := latest[url]; !ok {
					removed = append(removed, watchEntry{URL: url, MaxRent: r.MaxRent, Beds: r.Beds})
				}
			}
			for url, r := range latest {
				p, ok := prev[url]
				if !ok {
					continue
				}
				if r.MaxRent != p.MaxRent && r.MaxRent > 0 && p.MaxRent > 0 {
					priced = append(priced, watchEntry{
						URL:      url,
						MaxRent:  r.MaxRent,
						PrevRent: p.MaxRent,
						Beds:     r.Beds,
					})
				}
			}
			if newOnes == nil {
				newOnes = []watchEntry{}
			}
			if removed == nil {
				removed = []watchEntry{}
			}
			if priced == nil {
				priced = []watchEntry{}
			}
			out["new_listings"] = newOnes
			out["removed_listings"] = removed
			out["price_changed"] = priced
			out["latest_synced_at"] = tsList[0].Format(time.RFC3339)
			out["prev_synced_at"] = tsList[1].Format(time.RFC3339)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", "Only diff if the previous sync is newer than this (e.g. 24h, 7d).")
	return cmd
}

func indexByURL(rows []apt.SnapshotRow) map[string]apt.SnapshotRow {
	m := map[string]apt.SnapshotRow{}
	for _, r := range rows {
		m[r.ListingURL] = r
	}
	return m
}
