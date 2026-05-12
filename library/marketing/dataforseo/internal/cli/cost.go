// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// pricePer1K maps endpoint paths (slash form) to dollars per 1000 operations.
// Sourced from DataForSEO public price list (subject to plan tier). The map
// is intentionally small — the goal is correctness for the high-traffic
// endpoints, not full coverage. Unknown endpoints fall back to defaultUnit.
var pricePer1K = map[string]float64{
	// SERP
	"serp/google/organic/live/regular":  2.00,
	"serp/google/organic/live/advanced": 3.00,
	"serp/google/organic/task_post":     0.60,
	// Keywords Data (Google Ads)
	"keywords_data/google_ads/search_volume/live":          25.00,
	"keywords_data/google_ads/keywords_for_keywords/live":  30.00,
	"keywords_data/google_ads/keywords_for_site/live":      30.00,
	// Backlinks
	"backlinks/summary/live":            50.00,
	"backlinks/referring_domains/live":  100.00,
	"backlinks/anchors/live":            50.00,
	// On-page
	"on_page/instant_pages":             25.00,
	// DataForSEO Labs
	"dataforseo_labs/google/keyword_ideas/live": 100.00,
	// AI Optimization
	"ai_optimization/chat_gpt/llm_responses/live":   25.00,
	"ai_optimization/claude/llm_responses/live":     25.00,
	"ai_optimization/gemini/llm_responses/live":     25.00,
	"ai_optimization/perplexity/llm_responses/live": 25.00,
}

const defaultUnitCostPer1K = 2.00 // conservative fallback

type costEstimate struct {
	Endpoint   string  `json:"endpoint"`
	Operations int     `json:"operations"`
	UnitCost   float64 `json:"unit_cost"`
	TotalCost  float64 `json:"total_cost"`
	Currency   string  `json:"currency"`
}

func newCostCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Estimate API spend before making live calls",
	}
	cmd.AddCommand(newCostEstimateCmd(flags))
	return cmd
}

// pp:novel-static-reference
//
// cost estimate is intentionally a pure-local computation: it multiplies a
// curated price table (Live vs Standard per endpoint family, sourced from
// dataforseo.com pricing pages) by the planned input size parsed from --keywords
// or --in. No API call is made — making one would defeat the purpose of a
// pre-call cost estimator. This directive tells the reimplementation_check
// it is correct that there is no client call here.
func newCostEstimateCmd(flags *rootFlags) *cobra.Command {
	var keywordsFile string
	var inFile string
	var confirmOver float64

	cmd := &cobra.Command{
		Use:   "estimate <endpoint>",
		Short: "Predict spend for an endpoint × input size, optionally gating large jobs",
		Long: `Pure-local cost estimation. No API call made.

The endpoint is the slash-form path (e.g. keywords_data/google_ads/search_volume/live).
Kebab-form is also accepted; hyphens are normalized to underscores.

Operation count is derived from --keywords (line count) or --in (JSON array length).
With neither flag, assumes a single call.

--confirm-over $N exits with code 7 when the estimated total exceeds N dollars
so scripts can gate spend without parsing output.`,
		Example: strings.Trim(`
  dataforseo-pp-cli cost estimate keywords_data/google_ads/search_volume/live --keywords /tmp/raw-keywords.txt
  dataforseo-pp-cli cost estimate serp/google/organic/live/advanced --in batch.json --confirm-over 5.00 --json
  dataforseo-pp-cli cost estimate backlinks/summary/live
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":        "true",
			"pp:typed-exit-codes": "0,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			endpoint := normalizeCostEndpoint(args[0])
			ops, err := countOperations(keywordsFile, inFile)
			if err != nil {
				return usageErr(err)
			}

			unit, ok := pricePer1K[endpoint]
			if !ok {
				unit = defaultUnitCostPer1K
			}
			perCall := unit / 1000.0
			total := perCall * float64(ops)

			result := costEstimate{
				Endpoint:   endpoint,
				Operations: ops,
				UnitCost:   perCall,
				TotalCost:  round2(total),
				Currency:   "USD",
			}

			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Estimated: $%.2f (%d operation(s) × $%.6f each on %s)\n",
					total, ops, perCall, endpoint,
				)
			}

			if confirmOver > 0 && total > confirmOver {
				return rateLimitErr(fmt.Errorf("estimated cost $%.2f exceeds --confirm-over $%.2f", total, confirmOver))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&keywordsFile, "keywords", "", "File of newline-separated keywords; operation count = line count")
	cmd.Flags().StringVar(&inFile, "in", "", "JSON file containing a top-level array; operation count = array length")
	cmd.Flags().Float64Var(&confirmOver, "confirm-over", 0, "Exit non-zero if estimated total exceeds this dollar amount")
	return cmd
}

func normalizeCostEndpoint(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimPrefix(s, "v3/")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func countOperations(keywordsFile, inFile string) (int, error) {
	if keywordsFile == "" && inFile == "" {
		return 1, nil
	}
	if keywordsFile != "" {
		f, err := os.Open(keywordsFile)
		if err != nil {
			return 0, fmt.Errorf("open %s: %w", keywordsFile, err)
		}
		defer f.Close()
		n := 0
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) != "" {
				n++
			}
		}
		if err := scanner.Err(); err != nil {
			return 0, err
		}
		return n, nil
	}
	data, err := os.ReadFile(inFile)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", inFile, err)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return 0, fmt.Errorf("parse %s as JSON array: %w", inFile, err)
	}
	return len(arr), nil
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100.0
}
