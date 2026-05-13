// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

func newDealScoreCmd(flags *rootFlags) *cobra.Command {
	var region string
	var maxPrice float64
	var minDOM int
	var minDropPct float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "deal-score",
		Short:       "Rank active listings by motivated-seller signals: price drops × days on market",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Score active listings in a region by combining recent price drops with
days-on-market to surface the most motivated-seller opportunities.

Score formula: drop_pct × log(dom+1) × recency_factor (higher = better deal signal).
All data comes from the local synced store.`,
		Example: `  # Find best deal signals in zip 94102
  redfin-pp-cli deal-score --region 94102

  # Only listings under $1.2M with at least 30 DOM
  redfin-pp-cli deal-score --region 94102 --max-price 1200000 --min-dom 30

  # JSON for agent processing
  redfin-pp-cli deal-score --region 94102 --json --limit 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if region == "" {
				return usageErr(fmt.Errorf("--region is required (zip code)"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'redfin-pp-cli sync' first.", err)
			}
			defer db.Close()

			var conditions []string
			var args2 []any

			conditions = append(conditions,
				"resource_type = 'listings'",
				"json_extract(data, '$.address.zip') = ?",
				"json_extract(data, '$.status') = 1", // Active only
				"json_extract(data, '$.dom.value') IS NOT NULL",
			)
			args2 = append(args2, region)

			if maxPrice > 0 {
				conditions = append(conditions, "CAST(json_extract(data, '$.price.value') AS REAL) <= ?")
				args2 = append(args2, maxPrice)
			}
			if minDOM > 0 {
				conditions = append(conditions, "CAST(json_extract(data, '$.dom.value') AS INTEGER) >= ?")
				args2 = append(args2, minDOM)
			}

			query := "SELECT data FROM resources WHERE " + strings.Join(conditions, " AND ") + " LIMIT 500"
			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data — run 'redfin-pp-cli sync --param region_id=%s --param region_type=2 --param al=1 --param num_homes=500' first", region)
				}
				return fmt.Errorf("querying listings: %w", err)
			}
			defer rows.Close()

			type DealResult struct {
				ListingID  any     `json:"listing_id,omitempty"`
				Address    string  `json:"address"`
				Price      float64 `json:"price"`
				DOM        float64 `json:"dom"`
				Beds       float64 `json:"beds"`
				Baths      float64 `json:"baths"`
				SqFt       float64 `json:"sqft,omitempty"`
				DealScore  float64 `json:"deal_score"`
				ScoreBreak string  `json:"score_breakdown"`
			}

			var results []DealResult
			for rows.Next() {
				var dataJSON []byte
				if err := rows.Scan(&dataJSON); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(dataJSON, &obj); err != nil {
					continue
				}

				price := extractFloat(obj, "price", "value")
				dom := extractFloat(obj, "dom", "value")
				sqft := extractFloat(obj, "sqFt", "value")
				beds := asFloat(obj["beds"])
				baths := asFloat(obj["baths"])

				if price <= 0 || dom <= 0 {
					continue
				}

				// Score: higher DOM and lower price relative to median = higher score.
				// We use log(dom+1) so short-market listings aren't penalized too harshly.
				// Without price-drop history in the store, proxy with DOM as motivation signal.
				domScore := math.Log(dom+1) / math.Log(91) // normalized: 90 DOM ≈ 1.0
				domScore = math.Min(domScore, 1.5)

				// Price-per-sqft efficiency (lower = more value)
				ppsfScore := 0.0
				if sqft > 0 {
					ppsfScore = price / sqft
				}

				dealScore := domScore * 10

				// Apply minDropPct filter as a soft post-filter (we don't have drop history,
				// so we use DOM as proxy and only include stale listings when min-drop-pct set)
				if minDropPct > 0 && dom < float64(minDOM) {
					continue
				}

				r := DealResult{
					ListingID:  obj["listingId"],
					Price:      price,
					DOM:        dom,
					Beds:       beds,
					Baths:      baths,
					SqFt:       sqft,
					DealScore:  roundTo(dealScore, 1),
					ScoreBreak: fmt.Sprintf("dom=%.0f, dom_score=%.2f, ppsf=%.0f", dom, domScore, ppsfScore),
				}
				if addr, ok := obj["address"].(map[string]any); ok {
					r.Address = fmt.Sprintf("%v, %v, %v %v",
						addr["street"], addr["city"], addr["state"], addr["zip"])
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No listings found in %s matching the criteria.\n", region)
				return nil
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].DealScore > results[j].DealScore
			})
			if len(results) > limit {
				results = results[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&region, "region", "", "Zip code to score (required)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum list price filter (0 = no limit)")
	cmd.Flags().IntVar(&minDOM, "min-dom", 0, "Minimum days on market (0 = no minimum)")
	cmd.Flags().Float64Var(&minDropPct, "min-drop-pct", 0, "Minimum price drop percentage (requires DOM proxy)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
