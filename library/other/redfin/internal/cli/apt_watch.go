// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

// watchDiff is the per-URL diff record emitted by the watch command.
type watchDiff struct {
	URL       string `json:"url"`
	OldStatus string `json:"old_status,omitempty"`
	NewStatus string `json:"new_status,omitempty"`
	OldPrice  int    `json:"old_price,omitempty"`
	NewPrice  int    `json:"new_price,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "watch <slug>",
		Short: "Diff the two latest sync runs for a saved search.",
		Long: `Reads the two most recent observed_at timestamps from listing_snapshots
for the given saved-search slug and surfaces the four kinds of change:
  • new_listings       — present in latest, missing from previous
  • removed_listings   — present in previous, missing from latest
  • price_changed      — present in both, price differs
  • status_changed     — present in both, status differs

Returns an empty-diff envelope when fewer than two syncs exist.`,
		Example: `  redfin-pp-cli watch austin-3br --json
  redfin-pp-cli watch austin-3br --since 7d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			slug := args[0]
			out := map[string]any{
				"saved_search":     slug,
				"new_listings":     []watchDiff{},
				"removed_listings": []watchDiff{},
				"price_changed":    []watchDiff{},
				"status_changed":   []watchDiff{},
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			ts, err := redfin.LatestSyncTimestamps(s.DB(), slug, 2)
			if err != nil {
				return err
			}
			if len(ts) == 0 {
				// No snapshots ever for this slug — likely a typo or
				// missing sync. Surface to stderr and exit non-zero so
				// the user can recover.
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: saved-search %q has no snapshots in the local store. Run `redfin-pp-cli sync-search %s --region-id <N> --region-type <N> ...` first.\n", slug, slug)
				out["needs_sync"] = true
				out["snapshot_count"] = 0
				_ = printJSONFiltered(cmd.OutOrStdout(), out, flags)
				return notFoundErr(fmt.Errorf("no snapshots for saved-search %q", slug))
			}
			if len(ts) < 2 {
				// One sync only — diff isn't possible yet but the slug exists.
				out["needs_sync"] = false
				out["snapshot_count"] = 1
				out["note"] = "only one sync exists; run sync-search again to enable diff"
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			latest, err := redfin.SnapshotsForSearchAt(s.DB(), slug, ts[0])
			if err != nil {
				return err
			}
			prev, err := redfin.SnapshotsForSearchAt(s.DB(), slug, ts[1])
			if err != nil {
				return err
			}
			latestByURL := map[string]redfin.Snapshot{}
			for _, sn := range latest {
				latestByURL[sn.ListingURL] = sn
			}
			prevByURL := map[string]redfin.Snapshot{}
			for _, sn := range prev {
				prevByURL[sn.ListingURL] = sn
			}
			var newL, remL, priceL, statusL []watchDiff
			for url, l := range latestByURL {
				p, ok := prevByURL[url]
				if !ok {
					newL = append(newL, watchDiff{URL: url, NewStatus: l.Status, NewPrice: l.Price})
					continue
				}
				if l.Price != p.Price {
					priceL = append(priceL, watchDiff{URL: url, OldPrice: p.Price, NewPrice: l.Price})
				}
				if l.Status != p.Status {
					statusL = append(statusL, watchDiff{URL: url, OldStatus: p.Status, NewStatus: l.Status})
				}
			}
			for url, p := range prevByURL {
				if _, ok := latestByURL[url]; !ok {
					remL = append(remL, watchDiff{URL: url, OldStatus: p.Status, OldPrice: p.Price})
				}
			}
			if newL == nil {
				newL = []watchDiff{}
			}
			if remL == nil {
				remL = []watchDiff{}
			}
			if priceL == nil {
				priceL = []watchDiff{}
			}
			if statusL == nil {
				statusL = []watchDiff{}
			}
			out["new_listings"] = newL
			out["removed_listings"] = remL
			out["price_changed"] = priceL
			out["status_changed"] = statusL
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Window for sync lookbacks (accepts 7d, 2w, 24h)")
	return cmd
}
