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

func newCompScoreCmd(flags *rootFlags) *cobra.Command {
	var zip string
	var months int
	var beds int
	var baths float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "comp-score <address>",
		Short:       "Rank comparable sold listings by price-per-sqft similarity",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Pull recently sold listings in the same zip code and rank them as comparables
for a subject property. Ranks by price-per-sqft similarity, beds/baths match,
and recency. All data comes from the local synced store.

Run 'redfin-pp-cli sync' first to populate sold listings.`,
		Example: `  # Find comps for a 3/2 in zip 94102
  redfin-pp-cli comp-score "123 Main St" --zip 94102 --beds 3 --baths 2

  # Widen the search to 12 months
  redfin-pp-cli comp-score "456 Oak Ave" --zip 94103 --months 12 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("address argument is required"))
			}
			if zip == "" {
				return usageErr(fmt.Errorf("--zip is required"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'redfin-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `
				SELECT data
				FROM resources
				WHERE resource_type = 'listings'
				  AND json_extract(data, '$.address.zip') = ?
				  AND json_extract(data, '$.status') = 4
				  AND json_extract(data, '$.sqFt.value') IS NOT NULL
				  AND CAST(json_extract(data, '$.sqFt.value') AS REAL) > 0
				  AND synced_at >= datetime('now', ?)
				LIMIT 500`

			monthParam := fmt.Sprintf("-%d months", months)
			rows, err := db.DB().QueryContext(cmd.Context(), query, zip, monthParam)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data — run 'redfin-pp-cli sync --param region_id=%s --param region_type=2 --param al=1 --param num_homes=500' first", zip)
				}
				return fmt.Errorf("querying comps: %w", err)
			}
			defer rows.Close()

			type comp struct {
				ListingID    any     `json:"listing_id,omitempty"`
				Address      string  `json:"address"`
				Price        float64 `json:"price"`
				SqFt         float64 `json:"sqft"`
				PricePerSqFt float64 `json:"price_per_sqft"`
				Beds         float64 `json:"beds"`
				Baths        float64 `json:"baths"`
				Score        float64 `json:"comp_score"`
				ScoreReason  string  `json:"score_reason"`
			}

			var candidates []comp
			for rows.Next() {
				var dataJSON []byte
				if err := rows.Scan(&dataJSON); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(dataJSON, &obj); err != nil {
					continue
				}

				c := comp{ListingID: obj["listingId"]}
				if addr, ok := obj["address"].(map[string]any); ok {
					c.Address = fmt.Sprintf("%v, %v, %v %v",
						addr["street"], addr["city"], addr["state"], addr["zip"])
				}
				c.Price = extractFloat(obj, "price", "value")
				c.SqFt = extractFloat(obj, "sqFt", "value")
				c.Beds = asFloat(obj["beds"])
				c.Baths = asFloat(obj["baths"])

				if c.SqFt > 0 && c.Price > 0 {
					c.PricePerSqFt = c.Price / c.SqFt
				}

				// Apply bed/bath filters when specified
				if beds > 0 && int(c.Beds) != beds {
					continue
				}
				if baths > 0 && c.Baths < baths-0.1 {
					continue
				}

				candidates = append(candidates, c)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			if len(candidates) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No sold comps found in zip %s within %d months matching the criteria.\n", zip, months)
				return nil
			}

			// Score by price-per-sqft similarity (lower delta = higher score)
			// and bed/bath proximity
			medianPPSF := medianFloat(func() []float64 {
				vals := make([]float64, 0, len(candidates))
				for _, c := range candidates {
					if c.PricePerSqFt > 0 {
						vals = append(vals, c.PricePerSqFt)
					}
				}
				return vals
			}())

			for i := range candidates {
				c := &candidates[i]
				ppsfDelta := 0.0
				if medianPPSF > 0 && c.PricePerSqFt > 0 {
					ppsfDelta = math.Abs(c.PricePerSqFt-medianPPSF) / medianPPSF
				}
				ppsfScore := math.Max(0, 10-ppsfDelta*10)
				bedScore := 10.0
				if beds > 0 {
					bedScore = math.Max(0, 10-math.Abs(c.Beds-float64(beds))*3)
				}
				c.Score = math.Round((ppsfScore*0.7+bedScore*0.3)*10) / 10
				c.ScoreReason = fmt.Sprintf("ppsf=$%.0f (median=$%.0f), beds=%.0f", c.PricePerSqFt, medianPPSF, c.Beds)
			}

			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].Score > candidates[j].Score
			})
			if len(candidates) > limit {
				candidates = candidates[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), candidates, flags)
		},
	}

	cmd.Flags().StringVar(&zip, "zip", "", "Zip code to search (required)")
	cmd.Flags().IntVar(&months, "months", 6, "How many months of sold history to include")
	cmd.Flags().IntVar(&beds, "beds", 0, "Filter by number of bedrooms (0 = any)")
	cmd.Flags().Float64Var(&baths, "baths", 0, "Minimum bathrooms (0 = any)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max comps to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func extractFloat(obj map[string]any, keys ...string) float64 {
	current := obj
	for i, key := range keys {
		v, ok := current[key]
		if !ok {
			return 0
		}
		if i == len(keys)-1 {
			return asFloat(v)
		}
		nested, ok := v.(map[string]any)
		if !ok {
			return 0
		}
		current = nested
	}
	return 0
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}

func medianFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sort.Float64s(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 0 {
		return (vals[mid-1] + vals[mid]) / 2
	}
	return vals[mid]
}
