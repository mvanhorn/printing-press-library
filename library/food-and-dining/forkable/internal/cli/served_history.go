// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// served-history: every meal actually served to the user over time. Live
// GraphQL fetch of myDeliveries, flattened to one row per served "piece"
// (chosen meal item), filtered by --since. Hand-authored; preserved across
// generate --force.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// servedHistoryQuery pulls deliveries with their orders and pieces from a
// wide date floor so client-side --since filtering has data to work with.
const servedHistoryQuery = `query { myDeliveries (from: "2000-01-01") { id state forDeliveryAt orders { id state forDeliveryAt total venue { id name displayName } pieces { id name userFullName price dietLevel group state autoOrder } } } }`

type servedMeal struct {
	Date      string  `json:"date"`
	Venue     string  `json:"venue"`
	Name      string  `json:"name"`
	Diner     string  `json:"diner"`
	Price     float64 `json:"price"`
	DietLevel string  `json:"diet_level,omitempty"`
	Group     string  `json:"group,omitempty"`
	AutoOrder bool    `json:"auto_order"`
	OrderID   int64   `json:"order_id"`
}

type servedHistoryView struct {
	Meals []servedMeal `json:"meals"`
	Count int          `json:"count"`
	Since string       `json:"since,omitempty"`
}

// rawDeliveries mirrors the subset of myDeliveries we consume across the
// history/venue/preference commands.
type rawDelivery struct {
	ForDeliveryAt string     `json:"forDeliveryAt"`
	Orders        []rawOrder `json:"orders"`
}

type rawOrder struct {
	ID            int64      `json:"id"`
	ForDeliveryAt string     `json:"forDeliveryAt"`
	Total         float64    `json:"total"`
	Venue         rawVenue   `json:"venue"`
	Pieces        []rawPiece `json:"pieces"`
}

type rawVenue struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type rawPiece struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	UserName  string  `json:"userFullName"`
	Price     float64 `json:"price"`
	DietLevel string  `json:"dietLevel"`
	Group     string  `json:"group"`
	State     string  `json:"state"`
	AutoOrder bool    `json:"autoOrder"`
}

func (v rawVenue) label() string {
	if v.DisplayName != "" {
		return v.DisplayName
	}
	return v.Name
}

func fetchDeliveries(cmd *cobra.Command, flags *rootFlags, query string) ([]rawDelivery, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	data, err := fetchGraphQL(ctx, flags, query)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		MyDeliveries []rawDelivery `json:"myDeliveries"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("parsing deliveries: %w", err)
	}
	return wrap.MyDeliveries, nil
}

func newNovelServedHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "served-history",
		Short:       "See every meal actually served to you over time, with date, venue, price, and dietary level.",
		Long:        "Lists every meal item ('piece') served across your deliveries, newest data pulled live from Forkable and filtered by --since. Forkable's web app shows one delivery at a time and never aggregates history.",
		Example:     "  forkable-pp-cli served-history --since 90d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				emitDryRunShortCircuit(cmd, flags, "fetch served-meal history from Forkable")
				return nil
			}
			deliveries, err := fetchDeliveries(cmd, flags, servedHistoryQuery)
			if err != nil {
				return err
			}
			cutoff := sinceCutoffISO(flagSince)
			meals := make([]servedMeal, 0)
			for _, d := range deliveries {
				for _, o := range d.Orders {
					when := o.ForDeliveryAt
					if when == "" {
						when = d.ForDeliveryAt
					}
					if !dateOnOrAfter(when, cutoff) {
						continue
					}
					for _, p := range o.Pieces {
						meals = append(meals, servedMeal{
							Date:      when,
							Venue:     o.Venue.label(),
							Name:      p.Name,
							Diner:     p.UserName,
							Price:     p.Price,
							DietLevel: p.DietLevel,
							Group:     p.Group,
							AutoOrder: p.AutoOrder,
							OrderID:   o.ID,
						})
					}
				}
			}
			view := servedHistoryView{Meals: meals, Count: len(meals), Since: cutoff}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(meals) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No served meals found for the given window.")
				return nil
			}
			for _, m := range meals {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%-24s\t%-28s\t$%.2f\n", m.Date[:min(10, len(m.Date))], m.Venue, m.Name, m.Price)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only include meals on or after this window (e.g. 90d, 12w, 6mo)")
	return cmd
}
