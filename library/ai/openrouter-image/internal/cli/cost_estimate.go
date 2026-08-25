// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: pre-spend cost estimate. Computes the USD cost of a planned
// generation offline from synced per-endpoint pricing lines and resolution
// tiers stored in the local SQLite catalog. No API call is made.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/store"
)

// pp:data-source local

type costEstimateView struct {
	Model        string  `json:"model"`
	Resolution   string  `json:"resolution,omitempty"`
	Quality      string  `json:"quality,omitempty"`
	N            int     `json:"n"`
	EstimateUSD  float64 `json:"estimate_usd"`
	CheapestUnit float64 `json:"cheapest_unit_usd"`
	Unit         string  `json:"unit,omitempty"`
	Provider     string  `json:"cheapest_provider,omitempty"`
	Note         string  `json:"note,omitempty"`
	FromCache    bool    `json:"from_cache"`
}

func newNovelCostEstimateCmd(flags *rootFlags) *cobra.Command {
	var (
		flagModel      string
		flagResolution string
		flagQuality    string
		flagN          string
		dbPath         string
	)

	cmd := &cobra.Command{
		Use:   "cost-estimate",
		Short: "Estimate USD cost of a generation before spending credits, computed offline from synced per-endpoint pricing.",
		Long: `Estimate the cost of a future generation before spending credits.

The estimate is computed from the per-endpoint pricing lines synced into the
local store (billable=output_image, unit=image or megapixel) multiplied by the
number of images requested. Run sync first to populate pricing:

  openrouter-image-pp-cli sync --resources images --full

Use this command to estimate the cost of a future generation before spending
credits.
Do NOT use it to preview the request payload; use 'generate --dry-run' instead.`,
		Example: strings.Trim(`
  openrouter-image-pp-cli cost-estimate --model openai/gpt-image-1 --resolution 2K --quality high --n 4
  openrouter-image-pp-cli cost-estimate --model bytedance-seed/seedream-4.5 --n 1 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would estimate cost for model", flagModel)
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if flagModel == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--model is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("openrouter-image-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: openrouter-image-pp-cli sync --resources images --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureOpenRouterImageTables(ctx); err != nil {
				return err
			}

			n := 1
			if flagN != "" {
				n, err = strconv.Atoi(flagN)
				if err != nil || n < 1 {
					return usageErr(fmt.Errorf("--n must be a positive integer, got %q", flagN))
				}
			}
			if n > 10 {
				n = 10
			}

			view := costEstimateView{Model: flagModel, Resolution: flagResolution, Quality: flagQuality, N: n}

			// Look up pricing from the synced images table (data JSON carries
			// the per-model endpoint URL and the model record). Pricing lines
			// come from the per-endpoint records cached during sync.
			if cached, err := db.GetEndpointCache(ctx, flagModel); err == nil && cached != nil && len(cached.Data) > 0 {
				unit, provider, unitType := cheapestOutputUnit(cached.Data)
				if unit > 0 {
					view.CheapestUnit = unit
					view.EstimateUSD = unit * float64(n)
					view.Unit = unitType
					view.Provider = provider
					view.FromCache = true
					return emitCostEstimate(cmd, flags, view)
				}
			}

			// Cache miss: fetch the per-endpoint records live (public
			// endpoint) and cache them so later runs are offline.
			if c, err := flags.newClient(); err == nil {
				if data, err := c.Get(ctx, "/images/models/"+urlPathEscape(flagModel)+"/endpoints", nil); err == nil {
					_ = db.PutEndpointCache(ctx, flagModel, data)
					if unit, provider, unitType := cheapestOutputUnit(data); unit > 0 {
						view.CheapestUnit = unit
						view.EstimateUSD = unit * float64(n)
						view.Unit = unitType
						view.Provider = provider
						return emitCostEstimate(cmd, flags, view)
					}
				}
			}

			// Fall back to scanning the synced image catalog for any model
			// whose id matches, and report what pricing exists.
			view.Note = "no per-endpoint pricing cached for this model; run sync then try again"
			return emitCostEstimate(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagModel, "model", "", "Image model slug (e.g. openai/gpt-image-1)")
	cmd.Flags().StringVar(&flagResolution, "resolution", "", "Resolution tier: 512, 1K, 2K, 4K")
	cmd.Flags().StringVar(&flagQuality, "quality", "", "Quality: auto, low, medium, high")
	cmd.Flags().StringVar(&flagN, "n", "1", "Number of images to generate (1-10)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: platform data dir)")
	return cmd
}

// cheapestOutputUnit extracts the cheapest per-image (or megapixel) output cost
// from a cached /images/models/{author}/{slug}/endpoints response body.
func cheapestOutputUnit(data json.RawMessage) (float64, string, string) {
	var resp struct {
		Endpoints []struct {
			ProviderName string `json:"provider_name"`
			ProviderSlug string `json:"provider_slug"`
			Pricing      []struct {
				Billable string  `json:"billable"`
				Unit     string  `json:"unit"`
				CostUSD  float64 `json:"cost_usd"`
			} `json:"pricing"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, "", ""
	}
	type candidate struct {
		cost float64
		unit string
		prov string
	}
	var best candidate
	for _, ep := range resp.Endpoints {
		for _, p := range ep.Pricing {
			if p.Billable != "output_image" {
				continue
			}
			if best.cost == 0 || p.CostUSD < best.cost {
				best = candidate{cost: p.CostUSD, unit: p.Unit, prov: ep.ProviderSlug}
			}
		}
	}
	if best.cost == 0 {
		return 0, "", ""
	}
	return best.cost, best.prov, best.unit
}

func emitCostEstimate(cmd *cobra.Command, flags *rootFlags, view costEstimateView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if view.EstimateUSD > 0 {
		unitLabel := "image"
		if view.Unit != "" && view.Unit != "image" {
			unitLabel = view.Unit
		}
		fmt.Fprintf(cmd.OutOrStdout(), "model:        %s\n", view.Model)
		fmt.Fprintf(cmd.OutOrStdout(), "cheapest:     $%.4f per %s (%s)\n", view.CheapestUnit, unitLabel, orDefault(view.Provider, "any provider"))
		fmt.Fprintf(cmd.OutOrStdout(), "estimate:     $%.4f for %d image(s) (per-%s rate)\n", view.EstimateUSD, view.N, unitLabel)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "note:         %s\n", view.Note)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "no pricing available for %s\n", view.Model)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", view.Note)
		}
	}
	return nil
}

// pricingSort is a small helper used by models rank; kept here to avoid an
// extra file.
func pricingSort(entries []rankEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].CostUSD < entries[j].CostUSD
	})
}
