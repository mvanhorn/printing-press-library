// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// venue-rotation: rank venues by served-frequency and recency. Live GraphQL
// fetch of myDeliveries, aggregated over the --since window. Distinct from
// the single-venue `venue-usage get` endpoint read. Hand-authored; preserved
// across generate --force.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type venueStat struct {
	VenueID       int64  `json:"venue_id"`
	Venue         string `json:"venue"`
	Times         int    `json:"times_served"`
	LastSeen      string `json:"last_seen"`
	DaysSinceSeen int    `json:"days_since_seen"`
}

type venueRotationView struct {
	Venues []venueStat `json:"venues"`
	Count  int         `json:"count"`
	Since  string      `json:"since,omitempty"`
}

func newNovelVenueRotationCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "venue-rotation",
		Short:       "Rank venues by how often they've served you and how recently.",
		Long:        "Ranks the restaurants that have served your meals by frequency and recency across the --since window. Distinct from 'venue-usage get', which reads one venue's raw usage stats from the API.",
		Example:     "  forkable-pp-cli venue-rotation --since 120d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				emitDryRunShortCircuit(cmd, flags, "rank venues by frequency and recency")
				return nil
			}
			deliveries, err := fetchDeliveries(cmd, flags, servedHistoryQuery)
			if err != nil {
				return err
			}
			cutoff := sinceCutoffISO(flagSince)
			type acc struct {
				name     string
				times    int
				lastSeen string
			}
			byVenue := map[int64]*acc{}
			for _, d := range deliveries {
				for _, o := range d.Orders {
					when := o.ForDeliveryAt
					if when == "" {
						when = d.ForDeliveryAt
					}
					if !dateOnOrAfter(when, cutoff) {
						continue
					}
					if len(o.Pieces) == 0 {
						continue
					}
					vid := o.Venue.ID
					a := byVenue[vid]
					if a == nil {
						a = &acc{name: o.Venue.label()}
						byVenue[vid] = a
					}
					a.times += len(o.Pieces)
					if len(when) >= 10 && when[:10] > a.lastSeen {
						a.lastSeen = when[:10]
					}
				}
			}
			now := time.Now()
			stats := make([]venueStat, 0, len(byVenue))
			for vid, a := range byVenue {
				days := daysSinceUTC(a.lastSeen, now)
				stats = append(stats, venueStat{VenueID: vid, Venue: a.name, Times: a.times, LastSeen: a.lastSeen, DaysSinceSeen: days})
			}
			// Rank by frequency desc, then recency (fewer days since) asc.
			sort.Slice(stats, func(i, j int) bool {
				if stats[i].Times != stats[j].Times {
					return stats[i].Times > stats[j].Times
				}
				return stats[i].LastSeen > stats[j].LastSeen
			})
			view := venueRotationView{Venues: stats, Count: len(stats), Since: cutoff}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(stats) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No venues found for the given window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %8s %12s %10s\n", "venue", "times", "last_seen", "days_ago")
			for _, s := range stats {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %8d %12s %10d\n", s.Venue, s.Times, s.LastSeen, s.DaysSinceSeen)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only include deliveries on or after this window (e.g. 120d, 16w)")
	return cmd
}
