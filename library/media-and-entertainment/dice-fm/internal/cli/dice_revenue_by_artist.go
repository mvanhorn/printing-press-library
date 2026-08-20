// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "revenue by-artist" command: aggregate gross / dice_fees
// / net / orders_count grouped by artist name, joining orders -> event ->
// events.artists[].name. This file is NOT generated and survives `generate
// --force`.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// artistRevenueRow is one per-artist revenue aggregate.
type artistRevenueRow struct {
	Artist      string  `json:"artist"`
	EventsCount int     `json:"events_count"`
	Gross       float64 `json:"gross"`
	DiceFees    float64 `json:"dice_fees"`
	Net         float64 `json:"net"`
	OrdersCount int     `json:"orders_count"`
}

// computeRevenueByArtist aggregates order revenue by artist name. By default
// the event's full artist list is used (an event's revenue is attributed to
// each listed artist, producing intentional double-counting when events have
// multiple artists). When headlinerOnly is true only the first artist in the
// list receives attribution. An optional show-date window restricts which
// events are included.
func computeRevenueByArtist(ctx context.Context, db *sql.DB, headlinerOnly bool, fromDate, toDate string) ([]artistRevenueRow, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	events, err := readEvents(ctx, db)
	if err != nil {
		return nil, err
	}
	eligible, dateFiltered, err := eligibleEventsByDate(ctx, db, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	// Build event -> artists map. An event with no artists uses a "(no artist)"
	// bucket so its revenue is not silently dropped.
	type eventArtistInfo struct {
		artists []string // ordered; first is the headliner
	}
	eventArtists := make(map[string]eventArtistInfo, len(events))
	for _, e := range events {
		if len(e.Artists) == 0 {
			eventArtists[e.ID] = eventArtistInfo{artists: []string{"(no artist)"}}
			continue
		}
		names := make([]string, 0, len(e.Artists))
		for _, a := range e.Artists {
			if a.Name != "" {
				names = append(names, a.Name)
			}
		}
		if len(names) == 0 {
			names = []string{"(no artist)"}
		}
		eventArtists[e.ID] = eventArtistInfo{artists: names}
	}

	type agg struct {
		grossCents  int64
		diceCents   int64
		ordersCount int
		eventSet    map[string]bool
	}
	groups := map[string]*agg{} // keyed by artist name

	for _, o := range orders {
		if dateFiltered && !eligible[o.Event.ID] {
			continue
		}
		info, ok := eventArtists[o.Event.ID]
		if !ok {
			// Event row not synced; attribute to an "(unknown event)" bucket.
			info = eventArtistInfo{artists: []string{"(unknown event)"}}
		}

		artists := info.artists
		if headlinerOnly && len(artists) > 0 {
			artists = artists[:1]
		}

		for _, artist := range artists {
			g := groups[artist]
			if g == nil {
				g = &agg{eventSet: map[string]bool{}}
				groups[artist] = g
			}
			g.grossCents += o.Total
			g.diceCents += o.DiceComm
			g.ordersCount++
			if o.Event.ID != "" {
				g.eventSet[o.Event.ID] = true
			}
		}
	}

	rows := make([]artistRevenueRow, 0, len(groups))
	for artist, g := range groups {
		gross := float64(g.grossCents) / 100.0
		fees := float64(g.diceCents) / 100.0
		rows = append(rows, artistRevenueRow{
			Artist:      artist,
			EventsCount: len(g.eventSet),
			Gross:       round2(gross),
			DiceFees:    round2(fees),
			Net:         round2(gross - fees),
			OrdersCount: g.ordersCount,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Gross != rows[j].Gross {
			return rows[i].Gross > rows[j].Gross
		}
		return strings.ToLower(rows[i].Artist) < strings.ToLower(rows[j].Artist)
	})
	return rows, nil
}

func newRevenueByArtistCmd(flags *rootFlags) *cobra.Command {
	var headlinerOnly bool
	var from, to string
	cmd := &cobra.Command{
		Use:   "by-artist",
		Short: "Aggregate gross, Dice fees, and net revenue grouped by artist name",
		Long: "Aggregate order revenue by artist across events from the local store. " +
			"By default every artist listed on an event receives full attribution for " +
			"that event's orders (intentional double-counting for multi-artist events). " +
			"Use --headliner-only to restrict attribution to the first artist per event.",
		Example:     "  dice-fm-pp-cli revenue by-artist --headliner-only --from 2026-01-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if from, err = parseDateFlag("from", from); err != nil {
				return err
			}
			if to, err = parseDateFlag("to", to); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []artistRevenueRow{}, flags)
			}
			defer s.Close()
			rows, err := computeRevenueByArtist(cmd.Context(), s.DB(), headlinerOnly, from, to)
			if err != nil {
				return fmt.Errorf("computing revenue by artist: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().BoolVar(&headlinerOnly, "headliner-only", false, "Attribute revenue only to the first artist per event (no double-counting)")
	cmd.Flags().StringVar(&from, "from", "", "Only include shows on or after this date (YYYY-MM-DD, by show date)")
	cmd.Flags().StringVar(&to, "to", "", "Only include shows on or before this date (YYYY-MM-DD, by show date)")
	return cmd
}
