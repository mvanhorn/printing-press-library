// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/spf13/cobra"
)

const maxTripPlanCombinations = 1_000_000

type planPick struct {
	Leg          string  `json:"leg"`
	PropertyName string  `json:"property_name"`
	Slug         string  `json:"slug"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
}

func newTripCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "trip", Short: "Plan multi-leg Booking.com trips", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newTripPlanCmd(flags))
	return cmd
}

func newTripPlanCmd(flags *rootFlags) *cobra.Command {
	var legs []string
	var budget float64
	var filters string
	cmd := &cobra.Command{
		Use:         "plan",
		Short:       "Pick cheapest hotels across trip legs under a budget",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]planPick, 0))
			}
			if len(legs) == 0 || budget <= 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("trip plan: %w", err)
			}
			options := make([][]planPick, 0, len(legs))
			for _, leg := range legs {
				city, in, out, err := parseLeg(leg)
				if err != nil {
					return fmt.Errorf("trip plan: %w", err)
				}
				data, err := c.Get("/searchresults.html", searchParams(city, in, out, 2, filters))
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: leg %s failed: %v\n", leg, err)
					continue
				}
				cards, err := parseCards(data)
				if err != nil {
					return fmt.Errorf("trip plan: %w", err)
				}
				picks := planPicksForLeg(leg, cards)
				if len(picks) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: leg %s had no parseable prices\n", leg)
					continue
				}
				options = append(options, picks)
			}
			best, err := choosePlan(options, budget)
			if err != nil {
				return fmt.Errorf("trip plan: %w", err)
			}
			return flags.printJSON(cmd, best)
		},
	}
	cmd.Flags().StringSliceVar(&legs, "leg", nil, "Trip leg city:checkin:checkout")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Total budget")
	cmd.Flags().StringVar(&filters, "filters", "", "Extra query filters as key=value,key=value")
	return cmd
}

func planPicksForLeg(leg string, cards []booking.PropertyCard) []planPick {
	picks := make([]planPick, 0)
	for _, card := range cards {
		if card.Price > 0 {
			picks = append(picks, planPick{Leg: leg, PropertyName: card.Name, Slug: card.Slug, Price: card.Price, Currency: card.Currency})
		}
	}
	sort.Slice(picks, func(i, j int) bool { return picks[i].Price < picks[j].Price })
	if len(picks) > 10 {
		picks = picks[:10]
	}
	return picks
}

func parseLeg(s string) (string, time.Time, time.Time, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return "", time.Time{}, time.Time{}, fmt.Errorf("leg must be city:YYYY-MM-DD:YYYY-MM-DD")
	}
	in, err := time.Parse(dateOnly, parts[1])
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	out, err := time.Parse(dateOnly, parts[2])
	return parts[0], in, out, err
}

func choosePlan(options [][]planPick, budget float64) ([]planPick, error) {
	if err := validatePlanSearchSpace(options); err != nil {
		return nil, err
	}
	if len(options) > 0 && len(commonPlanCurrencies(options)) == 0 {
		return nil, fmt.Errorf("no common currency across trip legs; split the plan by currency or adjust --filters")
	}
	best := make([]planPick, 0)
	var bestSum float64
	var dfs func(int, float64, string, []planPick)
	dfs = func(i int, sum float64, currency string, cur []planPick) {
		if sum > budget {
			return
		}
		if i == len(options) {
			if len(cur) == len(options) && (len(best) == 0 || sum < bestSum) {
				best, bestSum = append([]planPick(nil), cur...), sum
			}
			return
		}
		for _, p := range options[i] {
			if p.Currency == "" {
				continue
			}
			nextCurrency := currency
			if nextCurrency == "" {
				nextCurrency = p.Currency
			}
			if p.Currency != nextCurrency {
				continue
			}
			dfs(i+1, sum+p.Price, nextCurrency, append(cur, p))
		}
	}
	dfs(0, 0, "", nil)
	if best == nil {
		return make([]planPick, 0), nil
	}
	return best, nil
}

func commonPlanCurrencies(options [][]planPick) map[string]struct{} {
	common := make(map[string]struct{})
	for i, picks := range options {
		legCurrencies := make(map[string]struct{})
		for _, pick := range picks {
			if pick.Currency != "" {
				legCurrencies[pick.Currency] = struct{}{}
			}
		}
		if i == 0 {
			common = legCurrencies
			continue
		}
		for currency := range common {
			if _, ok := legCurrencies[currency]; !ok {
				delete(common, currency)
			}
		}
	}
	return common
}

func validatePlanSearchSpace(options [][]planPick) error {
	combinations := 1
	for _, picks := range options {
		if len(picks) == 0 {
			continue
		}
		nextCombinations := combinations * len(picks)
		if nextCombinations > maxTripPlanCombinations {
			return fmt.Errorf("too many plan combinations (%d > %d); add filters or reduce --leg count", nextCombinations, maxTripPlanCombinations)
		}
		combinations = nextCombinations
	}
	return nil
}
