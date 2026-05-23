// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: novel feature (resolution radar). See research.json novel_features.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/polymarket/internal/store"
)

func newRadarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "radar",
		Short: "Resolution radar — surface markets resolving in the next N days.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRadarResolutionsCmd(flags))
	return cmd
}

func newRadarResolutionsCmd(flags *rootFlags) *cobra.Command {
	var within time.Duration
	var wallet string
	var minValue float64

	cmd := &cobra.Command{
		Use:     "resolutions",
		Short:   "List every market resolving in the next N days, ranked by your position value when a wallet is provided. Stops you missing redemption deadlines.",
		Example: `  polymarket-pp-cli radar resolutions --within 168h --wallet 0xYOUR --min-value 10 --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:novel":      "radar.resolutions",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// Open local store
			dbPath := defaultDBPath("polymarket-pp-cli")
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				// No local store — return a clean hint, not an error.
				fmt.Fprintln(cmd.ErrOrStderr(),
					"hint: local store not initialized. Run 'polymarket-pp-cli sync --resources markets,events' to populate.")
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"window":      within.String(),
					"min_value":   minValue,
					"resolutions": []any{},
					"note":        "local store empty; run sync first",
				}, flags)
			}
			defer s.Close()

			// Pull all markets, filter to those resolving in window.
			rawMarkets, err := s.List("markets", 5000)
			if err != nil {
				return apiErr(fmt.Errorf("listing markets from store: %w", err))
			}

			now := time.Now().UTC()
			cutoff := now.Add(within)

			type marketRow struct {
				ID         string  `json:"id"`
				Slug       string  `json:"slug"`
				Question   string  `json:"question"`
				EndDate    string  `json:"end_date"`
				Closed     bool    `json:"closed"`
				Volume     float64 `json:"volume"`
				PosValue   float64 `json:"position_value,omitempty"`
				Redeemable bool    `json:"redeemable"`
			}
			var rows []marketRow
			for _, raw := range rawMarkets {
				var m map[string]any
				if err := json.Unmarshal(raw, &m); err != nil {
					continue
				}
				endStr, _ := m["endDate"].(string)
				if endStr == "" {
					endStr, _ = m["end_date_iso"].(string)
				}
				if endStr == "" {
					continue
				}
				end, err := time.Parse(time.RFC3339, endStr)
				if err != nil {
					// Try common alternate format
					end, err = time.Parse("2006-01-02T15:04:05Z", endStr)
					if err != nil {
						continue
					}
				}
				if end.Before(now) || end.After(cutoff) {
					// Outside window. Also include already-closed-but-unredeemed
					// when wallet is provided (caller may need to redeem).
					if wallet == "" {
						continue
					}
					if end.After(cutoff) {
						continue
					}
				}
				row := marketRow{
					EndDate: endStr,
				}
				if v, ok := m["id"].(string); ok {
					row.ID = v
				} else if v, ok := m["id"].(float64); ok {
					row.ID = fmt.Sprintf("%.0f", v)
				}
				if v, ok := m["slug"].(string); ok {
					row.Slug = v
				}
				if v, ok := m["question"].(string); ok {
					row.Question = v
				}
				if v, ok := m["closed"].(bool); ok {
					row.Closed = v
				}
				if v, ok := m["volume"].(float64); ok {
					row.Volume = v
				} else if vs, ok := m["volume"].(string); ok {
					_, _ = fmt.Sscanf(vs, "%f", &row.Volume)
				}
				// "Redeemable" heuristic: closed AND end_date in the past.
				row.Redeemable = row.Closed && end.Before(now)
				rows = append(rows, row)
			}

			// If wallet provided, fetch live positions and join.
			if wallet != "" {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				positionsPath := "https://data-api.polymarket.com/positions"
				params := map[string]string{"user": wallet, "limit": "500"}
				posData, perr := c.GetWithHeaders(cmd.Context(), positionsPath, params, nil)
				if perr != nil {
					// Don't fail — print partial result with a hint.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: positions fetch failed (%v); returning markets without wallet filter.\n", perr)
				} else {
					var positions []map[string]any
					if err := json.Unmarshal(posData, &positions); err == nil {
						// Build market -> current value map
						posValue := map[string]float64{}
						for _, p := range positions {
							var marketID string
							if v, ok := p["market"].(string); ok {
								marketID = v
							}
							if v, ok := p["conditionId"].(string); ok && marketID == "" {
								marketID = v
							}
							var val float64
							if v, ok := p["currentValue"].(float64); ok {
								val = v
							} else if v, ok := p["value"].(float64); ok {
								val = v
							}
							if marketID != "" {
								posValue[marketID] = posValue[marketID] + val
							}
						}
						// Filter rows to ones with a position, and stamp value.
						var filtered []marketRow
						for _, r := range rows {
							v := posValue[r.ID]
							if v <= 0 {
								// Also try matching on slug for resilience
								v = posValue[r.Slug]
							}
							if v < minValue {
								continue
							}
							r.PosValue = v
							filtered = append(filtered, r)
						}
						rows = filtered
					}
				}
			}

			// Sort: redeemable first, then by position value desc (if wallet),
			// otherwise by volume desc.
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Redeemable != rows[j].Redeemable {
					return rows[i].Redeemable
				}
				if wallet != "" {
					return rows[i].PosValue > rows[j].PosValue
				}
				return rows[i].Volume > rows[j].Volume
			})

			out := map[string]any{
				"window":      within.String(),
				"wallet":      wallet,
				"min_value":   minValue,
				"now":         now.Format(time.RFC3339),
				"cutoff":      cutoff.Format(time.RFC3339),
				"count":       len(rows),
				"resolutions": rows,
			}
			if len(rows) == 0 && wallet == "" {
				out["note"] = "no markets in window; run 'polymarket-pp-cli sync --resources markets,events' to refresh the local store"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().DurationVar(&within, "within", 7*24*time.Hour, "Resolution window in Go-duration syntax (e.g. 24h, 168h for 7 days)")
	cmd.Flags().StringVar(&wallet, "wallet", "", "Wallet address — when set, ranks by your current position value")
	cmd.Flags().Float64Var(&minValue, "min-value", 10, "Skip positions below this USDC value (only applies when --wallet is set)")
	_ = strings.Contains // keep strings import alive if rewritten
	return cmd
}
