// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// rawGSCRow tolerates both the Search Console API row shape
// ({"keys":["term"],"position":..}) and a flattened export
// ({"query":"term","position":..}).
type rawGSCRow struct {
	Keys        []string `json:"keys"`
	Query       string   `json:"query"`
	Position    float64  `json:"position"`
	Clicks      float64  `json:"clicks"`
	Impressions float64  `json:"impressions"`
}

// gscQueryRow is a normalized organic-search row from a GSC export.
type gscQueryRow struct {
	Query       string
	Position    float64
	Clicks      float64
	Impressions float64
}

// overlapResult is a query you pay for that also ranks organically.
type overlapResult struct {
	Query           string  `json:"query"`
	PaidCost        float64 `json:"paid_cost"`
	PaidClicks      int64   `json:"paid_clicks"`
	PaidConversions float64 `json:"paid_conversions"`
	OrganicPosition float64 `json:"organic_position"`
	OrganicClicks   float64 `json:"organic_clicks"`
}

func newPaidOrganicOverlapCmd(flags *rootFlags) *cobra.Command {
	var flagCustomerID string
	var flagGSCFile string
	var flagDays int
	var flagMaxPosition float64
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "paid-organic-overlap",
		Short:       "Find queries you pay for that already rank organically",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  google-ads-pp-cli paid-organic-overlap --customer-id 1234567890 --gsc-file gsc.json
  google-ads-pp-cli paid-organic-overlap --customer-id 1234567890 --gsc-file gsc.json --max-position 5 --days 30`,
		Long: `Joins paid search terms (live from Google Ads) against an organic-query export
from Google Search Console, surfacing terms you pay for that you already rank
for organically — candidates to stop paying for.

--gsc-file accepts JSON in any of these shapes (read-only, no GSC call is made):
  - a top-level array of rows
  - {"rows": [...]}   (the Search Console API response shape)
  - {"data": [...]}
Each row is {"keys":["<query>"], ...} or {"query":"<query>", ...} with a
numeric "position" (and optional "clicks"/"impressions"). Rows without a query
are dropped and counted.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagCustomerID == "" {
				return fmt.Errorf("required flag %q not set", "customer-id")
			}
			if flagGSCFile == "" {
				return fmt.Errorf("required flag %q not set", "gsc-file")
			}
			if flagDays <= 0 {
				return fmt.Errorf("--days must be greater than 0")
			}

			query := buildWastedSpendQuery(flagDays, time.Now().UTC())

			// Dry-run: emit nothing (composite read-only contract).
			if flags.dryRun {
				return nil
			}

			gscData, err := os.ReadFile(flagGSCFile)
			if err != nil {
				return fmt.Errorf("reading --gsc-file: %w", err)
			}
			gscRows, dropped, err := parseGSCExport(gscData)
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			paidRows, err := fetchGAQLRows[gaqlSearchTermRow](c, flagCustomerID, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			results := computeOverlap(paidRows, gscRows, flagMaxPosition, flagLimit)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"QUERY", "PAID COST", "ORGANIC POS"}
				tableRows := make([][]string, 0, len(results))
				for _, r := range results {
					tableRows = append(tableRows, []string{
						r.Query,
						fmt.Sprintf("%.2f", r.PaidCost),
						fmt.Sprintf("%.1f", r.OrganicPosition),
					})
				}
				return flags.printTable(cmd, headers, tableRows)
			}

			payload := map[string]any{
				"customer_id":      flagCustomerID,
				"window_days":      flagDays,
				"max_position":     flagMaxPosition,
				"gsc_rows_dropped": dropped,
				"count":            len(results),
				"results":          results,
			}
			return flags.printJSON(cmd, payload)
		},
	}

	cmd.Flags().StringVar(&flagCustomerID, "customer-id", "", "Google Ads customer ID (required).")
	cmd.Flags().StringVar(&flagGSCFile, "gsc-file", "", "Path to a Google Search Console query export JSON (required).")
	cmd.Flags().IntVar(&flagDays, "days", 7, "Paid look-back window in days.")
	cmd.Flags().Float64Var(&flagMaxPosition, "max-position", 10, "Only count organic queries ranking at or above this position.")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows to return after ranking.")
	return cmd
}

// parseGSCExport decodes a GSC query export in array, {rows:[...]}, or
// {data:[...]} form. It returns normalized rows plus a count of rows dropped
// for having no query key.
func parseGSCExport(data []byte) ([]gscQueryRow, int, error) {
	var rows []rawGSCRow
	if err := json.Unmarshal(data, &rows); err != nil {
		var wrapped struct {
			Rows []rawGSCRow `json:"rows"`
			Data []rawGSCRow `json:"data"`
		}
		if err2 := json.Unmarshal(data, &wrapped); err2 != nil {
			return nil, 0, fmt.Errorf("parsing --gsc-file: expected a JSON array or {\"rows\"|\"data\":[...]}: %w", err)
		}
		if len(wrapped.Rows) > 0 {
			rows = wrapped.Rows
		} else {
			rows = wrapped.Data
		}
	}
	out := make([]gscQueryRow, 0, len(rows))
	dropped := 0
	for _, r := range rows {
		q := r.Query
		if q == "" && len(r.Keys) > 0 {
			q = r.Keys[0]
		}
		if strings.TrimSpace(q) == "" {
			dropped++
			continue
		}
		out = append(out, gscQueryRow{
			Query:       q,
			Position:    r.Position,
			Clicks:      r.Clicks,
			Impressions: r.Impressions,
		})
	}
	return out, dropped, nil
}

// computeOverlap joins paid search terms against organic GSC rows on a
// normalized query key. Organic duplicates collapse to their best (lowest)
// position. A query is included when it has paid spend and ranks organically
// at or above maxPosition. Ranked by paid cost descending, capped at limit.
func computeOverlap(paidRows []gaqlSearchTermRow, gscRows []gscQueryRow, maxPosition float64, limit int) []overlapResult {
	organic := make(map[string]gscQueryRow, len(gscRows))
	for _, g := range gscRows {
		key := normalizeQueryKey(g.Query)
		if key == "" {
			continue
		}
		if existing, ok := organic[key]; !ok || g.Position < existing.Position {
			organic[key] = g
		}
	}

	results := make([]overlapResult, 0)
	seen := make(map[string]bool)
	for _, p := range paidRows {
		key := normalizeQueryKey(p.SearchTermView.SearchTerm)
		if key == "" || seen[key] {
			continue
		}
		g, ok := organic[key]
		if !ok || g.Position > maxPosition {
			continue
		}
		seen[key] = true
		results = append(results, overlapResult{
			Query:           p.SearchTermView.SearchTerm,
			PaidCost:        float64(p.Metrics.CostMicros) / 1e6,
			PaidClicks:      int64(p.Metrics.Clicks),
			PaidConversions: p.Metrics.Conversions,
			OrganicPosition: g.Position,
			OrganicClicks:   g.Clicks,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].PaidCost > results[j].PaidCost
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}
