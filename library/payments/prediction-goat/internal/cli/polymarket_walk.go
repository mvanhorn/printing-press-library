// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/polymarket"
)

type polymarketEventResult struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	EndDate     string `json:"endDate,omitempty"`
	MarketCount int    `json:"marketCount"`
}

type polymarketSiblingsResult struct {
	Event   polymarketEventResult `json:"event"`
	Markets []polymarket.Sibling  `json:"markets"`
}

// newPolymarketCmd registers the Polymarket-specific discovery helpers:
// event-of and siblings. Both walk gamma's markets -> events -> markets
// graph from any known market slug, bypassing the broken upstream
// /public-search that misses hub topics like celebrity markets.
func newPolymarketCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "polymarket",
		Short: "Polymarket-side discovery helpers (read-only)",
	}
	cmd.AddCommand(newPolymarketEventOfCmd(flags))
	cmd.AddCommand(newPolymarketSiblingsCmd(flags))
	return cmd
}

func newPolymarketEventOfCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-of <market-slug>",
		Short: "Look up the parent event for a Polymarket market slug",
		Example: `  prediction-goat-pp-cli polymarket event-of will-ghana-win-the-2026-fifa-world-cup
  prediction-goat-pp-cli polymarket event-of will-kanye-west-visit-israel-by-june-30 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.TrimSpace(args[0])
			client := polymarket.New()
			ev, ok, err := client.EventForMarket(cmd.Context(), slug)
			if err != nil {
				return apiErr(fmt.Errorf("polymarket event-of: %w", err))
			}
			if !ok {
				return notFoundErr(fmt.Errorf("polymarket event-of: market %q has no parent event (single-outcome market)", slug))
			}
			result := polymarketEventResult{Slug: ev.Slug, Title: ev.Title, EndDate: ev.EndDate, MarketCount: ev.MarketCount}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "event:        %s\n", result.Slug)
			fmt.Fprintf(cmd.OutOrStdout(), "title:        %s\n", result.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "marketCount:  %d\n", result.MarketCount)
			if result.EndDate != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "endDate:      %s\n", result.EndDate)
			}
			return nil
		},
	}
	return cmd
}

func newPolymarketSiblingsCmd(flags *rootFlags) *cobra.Command {
	var includeClosed bool
	cmd := &cobra.Command{
		Use:   "siblings <market-slug>",
		Short: "List all sibling markets under the parent event of a market slug",
		Example: `  prediction-goat-pp-cli polymarket siblings will-ghana-win-the-2026-fifa-world-cup
  prediction-goat-pp-cli polymarket siblings 2026-nba-draft-1st-overall-pick --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.TrimSpace(args[0])
			client := polymarket.New()
			ev, siblings, err := client.SiblingsForMarket(cmd.Context(), slug, includeClosed)
			if err != nil {
				return apiErr(fmt.Errorf("polymarket siblings: %w", err))
			}
			result := polymarketSiblingsResult{Event: polymarketEventResult{Slug: ev.Slug, Title: ev.Title, EndDate: ev.EndDate, MarketCount: ev.MarketCount}, Markets: siblings}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "event: %s (%s)\n", result.Event.Title, result.Event.Slug)
			rows := make([][]string, 0, len(siblings))
			for _, s := range siblings {
				probCell := ""
				if s.YesPercent > 0 {
					probCell = fmt.Sprintf("%.1f%%", s.YesPercent)
				}
				closedCell := ""
				if s.Closed {
					closedCell = "closed"
				}
				rows = append(rows, []string{s.Question, probCell, formatNumber(s.Volume), s.EndDate, closedCell})
			}
			return printSimpleTable(cmd.OutOrStdout(), []string{"Question", "%Yes", "Volume", "EndDate", ""}, rows)
		},
	}
	cmd.Flags().BoolVar(&includeClosed, "include-closed", false, "Include closed sibling markets in the result")
	return cmd
}
