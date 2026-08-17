// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// upcoming-digest: one agent-shaped line per upcoming day. Live GraphQL
// fetch of myDeliveries (from today) + myInProgressDeliveryIds, joined into
// a compact briefing. Hand-authored; preserved across generate --force.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// upcomingQuery pulls today-forward deliveries with the selected meal and
// allowance headroom fields.
var upcomingQuery = fmt.Sprintf(`query { myDeliveries (from: "%s") { id state forDeliveryAt weeklyAllowance weeklyAllowanceAvailable orders { id total venue { id name displayName } pieces { name userFullName price autoOrder } } } myInProgressDeliveryIds }`, time.Now().Format("2006-01-02"))

type upcomingItem struct {
	Date               string  `json:"date"`
	State              string  `json:"state"`
	Venue              string  `json:"venue"`
	Meal               string  `json:"meal,omitempty"`
	Price              float64 `json:"price"`
	AutoSelected       bool    `json:"auto_selected"`
	InProgress         bool    `json:"in_progress"`
	AllowanceAvailable float64 `json:"allowance_available"`
	DeliveryID         int64   `json:"delivery_id"`
}

type upcomingDigestView struct {
	Days  []upcomingItem `json:"days"`
	Count int            `json:"count"`
}

func newNovelUpcomingDigestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "upcoming-digest",
		Short:       "One agent-shaped line per upcoming day: date, venue, auto-selected item, price, allowance headroom.",
		Long:        "Summarizes your upcoming and in-progress Forkable deliveries as one compact line per day, joining the future delivery schedule with in-progress delivery IDs. Built for agent consumption with --agent.",
		Example:     "  forkable-pp-cli upcoming-digest --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				emitDryRunShortCircuit(cmd, flags, "summarize upcoming deliveries")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, err := fetchGraphQL(ctx, flags, upcomingQuery)
			if err != nil {
				return err
			}
			var wrap struct {
				MyDeliveries []struct {
					ID                 int64   `json:"id"`
					State              string  `json:"state"`
					ForDeliveryAt      string  `json:"forDeliveryAt"`
					WeeklyAllowanceAvl float64 `json:"weeklyAllowanceAvailable"`
					Orders             []struct {
						Total  float64  `json:"total"`
						Venue  rawVenue `json:"venue"`
						Pieces []struct {
							Name      string  `json:"name"`
							UserName  string  `json:"userFullName"`
							Price     float64 `json:"price"`
							AutoOrder bool    `json:"autoOrder"`
						} `json:"pieces"`
					} `json:"orders"`
				} `json:"myDeliveries"`
				InProgress []int64 `json:"myInProgressDeliveryIds"`
			}
			if err := json.Unmarshal(data, &wrap); err != nil {
				return fmt.Errorf("parsing upcoming deliveries: %w", err)
			}
			inProgress := map[int64]bool{}
			for _, id := range wrap.InProgress {
				inProgress[id] = true
			}
			days := make([]upcomingItem, 0)
			for _, d := range wrap.MyDeliveries {
				item := upcomingItem{
					Date:               d.ForDeliveryAt,
					State:              d.State,
					InProgress:         inProgress[d.ID],
					AllowanceAvailable: d.WeeklyAllowanceAvl,
					DeliveryID:         d.ID,
				}
				// Take the first venue/piece as the headline (individual meal).
				for _, o := range d.Orders {
					item.Venue = o.Venue.label()
					if len(o.Pieces) > 0 {
						item.Meal = o.Pieces[0].Name
						item.Price = o.Pieces[0].Price
						item.AutoSelected = o.Pieces[0].AutoOrder
					}
					break
				}
				days = append(days, item)
			}
			sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })
			view := upcomingDigestView{Days: days, Count: len(days)}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(days) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No upcoming deliveries.")
				return nil
			}
			for _, d := range days {
				marker := " "
				if d.InProgress {
					marker = "*"
				}
				meal := d.Meal
				if meal == "" {
					meal = "(no meal selected)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s  %-22s  %-28s  $%.2f  allowance:$%.2f\n", marker, d.Date[:min(10, len(d.Date))], d.Venue, meal, d.Price, d.AllowanceAvailable)
			}
			return nil
		},
	}
	return cmd
}
