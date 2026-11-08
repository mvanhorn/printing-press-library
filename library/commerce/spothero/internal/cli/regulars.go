// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/store"
)

type regularGarage struct {
	FacilityID    int    `json:"facility_id"`
	Title         string `json:"title,omitempty"`
	Bookings      int    `json:"bookings"`
	LastStarts    string `json:"last_starts,omitempty"`
	LastPaidCents int    `json:"last_paid_cents,omitempty"`
}

// rankRegulars aggregates raw reservation rows. Cancelled bookings are left out so
// "my usual garage" means places actually used.
func rankRegulars(rows []json.RawMessage) []regularGarage {
	byID := map[int]*regularGarage{}
	for _, raw := range rows {
		var r struct {
			FacilityID int    `json:"facility_id"`
			StallName  string `json:"stall_name"`
			// Synced rows nest the name under facility; stall_name is usually empty.
			FacilityTitle string `json:"facility_title"`
			Facility      struct {
				Title string `json:"title"`
				Name  string `json:"name"`
			} `json:"facility"`
			Starts string `json:"starts"`
			Price  int    `json:"price"`
			Status string `json:"reservation_status"`
		}
		if json.Unmarshal(raw, &r) != nil || r.FacilityID == 0 || strings.EqualFold(r.Status, "cancelled") {
			continue
		}
		g := byID[r.FacilityID]
		if g == nil {
			g = &regularGarage{FacilityID: r.FacilityID}
			byID[r.FacilityID] = g
		}
		g.Bookings++
		if r.Starts > g.LastStarts {
			title := r.FacilityTitle
			for _, candidate := range []string{r.Facility.Title, r.Facility.Name, r.StallName} {
				if title == "" {
					title = candidate
				}
			}
			g.LastStarts, g.LastPaidCents, g.Title = r.Starts, r.Price, title
		}
	}
	out := make([]regularGarage, 0, len(byID))
	for _, g := range byID {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bookings != out[j].Bookings {
			return out[i].Bookings > out[j].Bookings
		}
		return out[i].LastStarts > out[j].LastStarts
	})
	return out
}

func newNovelRegularsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "regulars",
		Short: "See which garages you book most, with last booked and last price.",
		Long: "Use this command to see which garages get booked most, from synced reservations. Run sync first.\n" +
			"Do NOT use this command to price a new window; use 'rebook' instead.",
		Example:     "  spothero-pp-cli regulars --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "regulars")
			}
			empty := map[string]any{"count": 0, "garages": []regularGarage{}, "hint": "run: spothero-pp-cli sync"}
			db, err := store.OpenReadOnlyContext(cmd.Context(), defaultDBPath("spothero-pp-cli"))
			if err != nil {
				// No local store yet is an ordinary state, not a failure.
				return flags.printJSON(cmd, empty)
			}
			defer db.Close()
			rows, err := db.List("reservations", 10000)
			if err != nil || len(rows) == 0 {
				return flags.printJSON(cmd, empty)
			}
			garages := rankRegulars(rows)
			if limit > 0 && len(garages) > limit {
				garages = garages[:limit]
			}
			return flags.printJSON(cmd, map[string]any{"count": len(garages), "garages": garages})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Most garages to show")
	return cmd
}
