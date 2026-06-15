// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored transcendence command (Phase 3). Safe to edit.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type cheapestMatch struct {
	Shop        string `json:"shop"`
	ServiceName string `json:"service_name"`
	CostCents   int    `json:"cost_cents"`
	Variable    bool   `json:"variable"`
}

type cheapestResult struct {
	ServiceTerm   string          `json:"service_term"`
	ScannedShops  int             `json:"scanned_shops"`
	Matches       []cheapestMatch `json:"matches"`
	FetchFailures []fetchFailure  `json:"fetch_failures"`
	Note          string          `json:"note,omitempty"`
}

func newNovelCheapestCmd(flags *rootFlags) *cobra.Command {
	var flagNear string
	var flagCityID string
	var flagLat string
	var flagLon string
	var flagLimit int
	var flagMaxScan int

	cmd := &cobra.Command{
		Use:   "cheapest <service-term>",
		Short: "Rank shops by the lowest price for one service category (e.g. Haircut) across a city or near a shop.",
		Example: strings.Trim(`
  squire-pp-cli cheapest Haircut --near barber-theory-toronto,another-shop
  squire-pp-cli cheapest "Haircut & Beard" --city-id <id> --lat 43.65 --lon -79.38 --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank shops by cheapest matching service")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("service term is required (e.g. \"Haircut\")"))
			}
			term := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Build the shop set: explicit --near list, or a city discovery page.
			shops := parseShopList(flagNear, nil)
			if len(shops) == 0 {
				if flagCityID == "" || flagLat == "" || flagLon == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("provide --near <shops> or --city-id with --lat and --lon"))
				}
				ents, err := fetchCityShops(ctx, c, flagCityID, flagLat, flagLon, 1)
				if err != nil {
					return apiErr(fmt.Errorf("city discovery: %w", err))
				}
				for _, e := range ents {
					if r := sqString(e, "route"); r != "" {
						shops = append(shops, r)
					} else if id := sqString(e, "id"); id != "" {
						shops = append(shops, id)
					}
				}
			}
			if flagMaxScan > 0 && len(shops) > flagMaxScan {
				shops = shops[:flagMaxScan]
			}
			matches := make([]cheapestMatch, 0)
			failures := make([]fetchFailure, 0)
			for _, shop := range shops {
				uuid, _, _, _, _, _, err := resolveShop(ctx, c, shop)
				if err != nil {
					failures = append(failures, fetchFailure{Shop: shop, Error: err.Error()})
					continue
				}
				svcs, err := fetchServices(ctx, c, uuid)
				if err != nil {
					failures = append(failures, fetchFailure{Shop: shop, Error: err.Error()})
					continue
				}
				best := cheapestMatch{Shop: shop, CostCents: -1}
				for _, s := range svcs {
					cats, _ := s["categories"].([]any)
					if !serviceMatches(sqString(s, "name"), cats, term) {
						continue
					}
					cost := sqInt(s, "cost")
					variable := false
					if cr := sqMap(s, "costRange"); cr != nil {
						if low := sqInt(cr, "low"); low > 0 {
							cost = low
							variable = true
						}
					}
					if best.CostCents == -1 || cost < best.CostCents {
						best.CostCents = cost
						best.ServiceName = strings.TrimSpace(sqString(s, "name"))
						best.Variable = variable
					}
				}
				if best.CostCents >= 0 {
					matches = append(matches, best)
				}
			}
			sort.SliceStable(matches, func(i, j int) bool { return matches[i].CostCents < matches[j].CostCents })
			if flagLimit > 0 && len(matches) > flagLimit {
				matches = matches[:flagLimit]
			}
			res := cheapestResult{ServiceTerm: term, ScannedShops: len(shops), Matches: matches, FetchFailures: failures}
			if len(matches) == 0 {
				res.Note = fmt.Sprintf("scanned %d shop(s); no service matched %q — widen the term or shop set", len(shops), term)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagNear, "near", "", "Comma-separated shop slugs or UUIDs to scan")
	cmd.Flags().StringVar(&flagCityID, "city-id", "", "City UUID for discovery (with --lat/--lon)")
	cmd.Flags().StringVar(&flagLat, "lat", "", "Latitude for city discovery")
	cmd.Flags().StringVar(&flagLon, "lon", "", "Longitude for city discovery")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum matching shops to return")
	cmd.Flags().IntVar(&flagMaxScan, "max-scan-shops", 25, "Maximum shops to scan before ranking")
	return cmd
}
