// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: price drift over local snapshots.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/store"

	"github.com/spf13/cobra"
)

type driftPoint struct {
	TakenAt string  `json:"taken_at"`
	Total   float64 `json:"cheapest_total"`
	Count   int     `json:"offer_count"`
}

type driftView struct {
	Search    string       `json:"search"`
	Points    []driftPoint `json:"points"`
	First     float64      `json:"first_total"`
	Last      float64      `json:"last_total"`
	Min       float64      `json:"min_total"`
	Max       float64      `json:"max_total"`
	Change    float64      `json:"change"`    // last - first
	Direction string       `json:"direction"` // up | down | flat
	Currency  string       `json:"currency"`
}

// summarizeDrift computes the trend aggregates (first, last, min, max, change,
// direction) over an ordered series of snapshot points. Direction is "flat"
// unless the net change exceeds a one-cent deadband, so floating-point noise
// never reads as a real move. The returned view carries the points plus the
// aggregates; callers fill Search and Currency.
func summarizeDrift(points []driftPoint) driftView {
	view := driftView{Points: points, Direction: "flat"}
	n := len(points)
	if n == 0 {
		return view
	}
	view.First = points[0].Total
	view.Last = points[n-1].Total
	view.Min, view.Max = points[0].Total, points[0].Total
	for _, p := range points {
		if p.Total < view.Min {
			view.Min = p.Total
		}
		if p.Total > view.Max {
			view.Max = p.Total
		}
	}
	view.Change = view.Last - view.First
	switch {
	case view.Change > 0.01:
		view.Direction = "up"
	case view.Change < -0.01:
		view.Direction = "down"
	}
	return view
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "drift <saved-name>",
		Short:       "Show how a saved search's cheapest full-insurance price has moved over time",
		Long:        "Read the local price snapshots recorded for a saved search and show the trend — first, last, min, max and direction. Requires prior snapshots from 'search', 'suppliers' or 'watch'.",
		Example:     "  rentalcarspain-pp-cli drift agp-august --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("drift needs a <saved-name>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("rentalcarspain-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local store at %s\nrun a search first: rentalcarspain-pp-cli search <code> <pickup> <dropoff>\n", dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			ss, err := db.GetSavedSearch(ctx, args[0])
			if err != nil {
				return apiErr(err)
			}
			if ss == nil {
				return notFoundErr(fmt.Errorf("no saved search named %q — add it with 'saved add'", args[0]))
			}
			key := searchKey(ss.LocationCode, ss.DropoffCode, ss.Pickup, ss.Dropoff, ss.DriverAge)
			snaps, err := db.ListSnapshots(ctx, key, 0)
			if err != nil {
				return apiErr(err)
			}
			points := make([]driftPoint, 0, len(snaps))
			for _, s := range snaps {
				points = append(points, driftPoint{TakenAt: s.TakenAt, Total: s.CheapestFITotal, Count: s.OfferCount})
			}
			view := summarizeDrift(points)
			view.Search = args[0]
			view.Currency = "EUR"
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(view)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			if len(view.Points) == 0 {
				fmt.Fprintf(w, "No snapshots for %q yet. Run 'rentalcarspain-pp-cli watch %s' or a search to record one.\n", args[0], args[0])
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "TAKEN AT\tCHEAPEST TOTAL\tOFFERS")
			for _, p := range view.Points {
				fmt.Fprintf(tw, "%s\t%.2f %s\t%d\n", p.TakenAt, p.Total, view.Currency, p.Count)
			}
			tw.Flush()
			fmt.Fprintf(w, "\nfirst %.2f → last %.2f (%+.2f %s, %s); min %.2f, max %.2f\n",
				view.First, view.Last, view.Change, view.Currency, view.Direction, view.Min, view.Max)
			return nil
		},
	}
	return cmd
}
