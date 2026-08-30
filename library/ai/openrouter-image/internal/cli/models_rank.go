// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: capability+budget model ranker. Joins the synced image model
// catalog (supported_parameters) against per-endpoint pricing in the local
// store to rank (model, provider) combos cheapest-first under capability and
// budget constraints. No live API call.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"net/url"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/store"
)

// pp:data-source local

type rankEntry struct {
	ModelID     string   `json:"model_id"`
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	SupportsI2I bool     `json:"supports_image_to_image"`
	Resolutions []string `json:"resolutions,omitempty"`
	Streaming   bool     `json:"supports_streaming"`
	CostUSD     float64  `json:"cost_usd"`
	Unit        string   `json:"unit,omitempty"`
}

type modelsRankView struct {
	Items         []rankEntry `json:"items"`
	ScannedModels int         `json:"scanned_models"`
	Limit         int         `json:"limit"`
	MaxCost       float64     `json:"max_cost,omitempty"`
	Note          string      `json:"note,omitempty"`
}

func newNovelModelsRankCmd(flags *rootFlags) *cobra.Command {
	var (
		flagImageToImage bool
		flagResolution   string
		flagMaxCost      string
		flagLimit        string
		dbPath           string
	)

	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Rank every image model+provider combo cheapest-first under your capability and budget constraints.",
		Long: `Rank image model+provider combinations cheapest-first under capability and
budget constraints, using the locally synced catalog.

Filters:
  --image-to-image   only models that accept reference images
  --resolution 4K    only models whose supported_parameters allow that tier
  --max-cost 0.10    only combos whose cheapest per-image price is at or below

The ranking joins the synced image model catalog against cached per-endpoint
pricing, so it works offline after sync.

Use this command to choose a model+provider combo from capability and budget
constraints.
Do NOT use it to inspect a specific model's providers; use 'models endpoints <model>' instead.
Do NOT use it to estimate cost for a model you already picked; use 'cost-estimate' instead.`,
		Example: strings.Trim(`
  openrouter-image-pp-cli models rank --image-to-image --resolution 4K --max-cost 0.10 --limit 5
  openrouter-image-pp-cli models rank --max-cost 0.05 --limit 3 --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank image models by capability and budget")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			limit := 10
			if flagLimit != "" {
				v, err := strconv.Atoi(flagLimit)
				if err != nil || v < 1 {
					return usageErr(fmt.Errorf("--limit must be a positive integer, got %q", flagLimit))
				}
				limit = v
			}
			maxCost := 0.0
			if flagMaxCost != "" {
				v, err := strconv.ParseFloat(flagMaxCost, 64)
				if err != nil || v < 0 {
					return usageErr(fmt.Errorf("--max-cost must be a non-negative number, got %q", flagMaxCost))
				}
				maxCost = v
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openrouter-image-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: openrouter-image-pp-cli sync --resources images --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
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

			rows, err := db.DB().QueryContext(ctx,
				`SELECT id, COALESCE(name,''), COALESCE(data,'{}') FROM images ORDER BY id`)
			if err != nil {
				return fmt.Errorf("querying image catalog: %w", err)
			}
			type rawRow struct {
				id   string
				name string
				data json.RawMessage
			}
			rawRows := make([]rawRow, 0)
			for rows.Next() {
				var r rawRow
				var data string
				if err := rows.Scan(&r.id, &r.name, &data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan catalog row: %w", err)
				}
				r.data = json.RawMessage(data)
				rawRows = append(rawRows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate catalog rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close catalog rows: %w", err)
			}

			view := modelsRankView{Items: make([]rankEntry, 0), ScannedModels: len(rawRows), Limit: limit, MaxCost: maxCost}

			for _, raw := range rawRows {
				var meta struct {
					ID           string `json:"id"`
					Name         string `json:"name"`
					Architecture struct {
						InputModalities []string `json:"input_modalities"`
					} `json:"architecture"`
					SupportedParameters map[string]any `json:"supported_parameters"`
					SupportsStreaming   bool           `json:"supports_streaming"`
				}
				_ = json.Unmarshal(raw.data, &meta)
				id := raw.id
				if meta.ID != "" {
					id = meta.ID
				}
				name := raw.name
				if meta.Name != "" {
					name = meta.Name
				}

				supportsI2I := false
				for _, m := range meta.Architecture.InputModalities {
					if m == "image" {
						supportsI2I = true
					}
				}
				if flagImageToImage && !supportsI2I {
					continue
				}
				resolutions := resolutionValues(meta.SupportedParameters)
				if flagResolution != "" && !containsStr(resolutions, flagResolution) {
					continue
				}

				entry := rankEntry{
					ModelID:     id,
					Name:        name,
					SupportsI2I: supportsI2I,
					Resolutions: resolutions,
					Streaming:   meta.SupportsStreaming,
				}

				// Pricing from the endpoint cache if present.
				if cached, err := db.GetEndpointCache(ctx, id); err == nil && cached != nil {
					if unit, prov, unitType := cheapestOutputUnit(cached.Data); unit > 0 {
						_ = unitType
						entry.CostUSD = unit
						entry.Unit = "image"
						entry.Provider = prov
						if maxCost > 0 && unit > maxCost {
							continue
						}
					}
				} else {
					// No pricing cached: fetch live (public endpoint) and
					// cache, so subsequent runs are offline.
					if c, err := flags.newClient(); err == nil {
						if data, err := c.Get(ctx, "/images/models/"+urlPathEscape(id)+"/endpoints", nil); err == nil {
							_ = db.PutEndpointCache(ctx, id, data)
							if unit, prov, unitType := cheapestOutputUnit(data); unit > 0 {
								_ = unitType
								entry.CostUSD = unit
								entry.Unit = "image"
								entry.Provider = prov
								if maxCost > 0 && unit > maxCost {
									continue
								}
							}
						}
					}
					// Still no pricing: keep the entry only when no budget
					// filter is active, tagged with empty cost so it sorts last.
					if maxCost > 0 && entry.CostUSD == 0 {
						continue
					}
				}
				view.Items = append(view.Items, entry)
				if len(view.Items) >= limit {
					break
				}
			}

			if len(view.Items) == 0 && len(rawRows) > 0 {
				view.Note = "no model matched the capability and budget filters; widen --max-cost or drop --resolution/--image-to-image"
			}
			pricingSort(view.Items)

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, e := range view.Items {
				cost := "no pricing"
				if e.CostUSD > 0 {
					cost = fmt.Sprintf("$%.4f/image", e.CostUSD)
				}
				i2i := ""
				if e.SupportsI2I {
					i2i = " i2i"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-45s %-24s %s%s\n", e.ModelID, cost, strings.Join(e.Resolutions, "/"), i2i)
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagImageToImage, "image-to-image", false, "Only models that accept reference images")
	cmd.Flags().StringVar(&flagResolution, "resolution", "", "Only models supporting this resolution tier (512, 1K, 2K, 4K)")
	cmd.Flags().StringVar(&flagMaxCost, "max-cost", "", "Only combos at or below this USD per-image price")
	cmd.Flags().StringVar(&flagLimit, "limit", "10", "Maximum number of ranked combos to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: platform data dir)")
	return cmd
}

func resolutionValues(sp map[string]any) []string {
	raw, ok := sp["resolution"]
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	valuesRaw, ok := m["values"]
	if !ok {
		return nil
	}
	arr, ok := valuesRaw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func containsStr(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}

// urlPathEscape escapes a model id for use in a URL path segment while
// preserving the slash between author and slug.
func urlPathEscape(id string) string {
	parts := strings.Split(id, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}
