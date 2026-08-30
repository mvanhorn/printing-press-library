// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/graphqlclient"
	"github.com/spf13/cobra"
)

var knownMarkets = map[string]int{
	"los angeles": 1,
	"la":          1,
	"atlanta":     2,
	"austin":      3,
	"baltimore":   4,
	"boston":      5,
	"cleveland":   8,
	"denver":      9,
	"houston":     12,
	"milwaukee":   17,
	"minneapolis": 18,
	"nashville":   19,
}

func resolveMarketID(input string) (int, error) {
	inputClean := strings.ToLower(strings.TrimSpace(input))
	inputClean = strings.ReplaceAll(inputClean, "-", " ")
	if id, ok := knownMarkets[inputClean]; ok {
		return id, nil
	}
	// Try parsing as integer
	if id, err := strconv.Atoi(inputClean); err == nil {
		return id, nil
	}
	return 0, fmt.Errorf("market %q not found, try a numeric market ID or one of the known cities", input)
}

func newNovelMarketCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "market",
		Short:       "Look up markets and truck rosters",
		Example:     "  bestfoodtrucks-pp-cli market get los-angeles\n  bestfoodtrucks-pp-cli market list los-angeles\n  bestfoodtrucks-pp-cli market hotlist los-angeles",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelMarketGetCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketListCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMarketHotlistCmd(flags))
	return cmd
}

type GqlMarketGetResult struct {
	ID           GqlID  `json:"id"`
	Name         string `json:"name"`
	CustomerPath string `json:"customerPath"`
}

func newNovelMarketGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <city-or-id>",
		Short: "Get basic information about a market",
		Long:  "Get basic information about a market/city by name or numeric ID.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli market get los-angeles
  bestfoodtrucks-pp-cli market get 1 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market get")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("city-or-id is required"))
			}

			id, err := resolveMarketID(args[0])
			if err != nil {
				return usageErr(err)
			}

			client := graphqlclient.New(flags.timeout)
			query := `
				query GetMarket($id: Int!) {
					market(id: $id) {
						id
						name
						customerPath
					}
				}
			`
			var result struct {
				Market *GqlMarketGetResult `json:"market"`
			}
			err = client.Query(ctx, query, map[string]any{"id": id}, &result)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching market: %w", err), flags)
			}
			if result.Market == nil {
				return notFoundErr(fmt.Errorf("market not found: %d", id))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "%s (Market ID: %s)\n", bold(result.Market.Name), result.Market.ID.String())
				fmt.Fprintf(w, "Customer Path: %s\n", result.Market.CustomerPath)
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result.Market, flags)
		},
	}
	return cmd
}

type GqlMarketListResult struct {
	ID     GqlID            `json:"id"`
	Name   string           `json:"name"`
	Trucks *GqlMarketTrucks `json:"trucks"`
}

type GqlMarketTrucks struct {
	Records []GqlSimpleTruck `json:"records"`
}

type GqlSimpleTruck struct {
	ID   GqlID  `json:"id"`
	Name string `json:"name"`
}

func newNovelMarketListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <city-or-id>",
		Short: "List all registered food trucks in a market",
		Long:  "List all registered food trucks in a market/city.",
		Example: strings.Trim(`
  bestfoodtrucks-pp-cli market list los-angeles
  bestfoodtrucks-pp-cli market list 1 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("city-or-id is required"))
			}

			id, err := resolveMarketID(args[0])
			if err != nil {
				return usageErr(err)
			}

			client := graphqlclient.New(flags.timeout)
			query := `
				query GetMarketTrucks($id: Int!) {
					market(id: $id) {
						id
						name
						trucks {
							records {
								id
								name
							}
						}
					}
				}
			`
			var result struct {
				Market *GqlMarketListResult `json:"market"`
			}
			err = client.Query(ctx, query, map[string]any{"id": id}, &result)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching market: %w", err), flags)
			}
			if result.Market == nil {
				return notFoundErr(fmt.Errorf("market not found: %d", id))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "Registered trucks in %s (Market ID: %s):\n", bold(result.Market.Name), result.Market.ID.String())
				fmt.Fprintln(w, strings.Repeat("-", 50))
				if result.Market.Trucks == nil || len(result.Market.Trucks.Records) == 0 {
					fmt.Fprintln(w, "No registered trucks found.")
					return nil
				}
				for _, tr := range result.Market.Trucks.Records {
					fmt.Fprintf(w, "  - %s (ID: %s)\n", green(tr.Name), tr.ID.String())
				}
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), result.Market, flags)
		},
	}
	return cmd
}
