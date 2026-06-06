// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelCategoryTrendCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "trend [category]",
		Short:       "Per-round percent-in-optimal score for one of the ~13 categories, across every round",
		Long:        "Aggregates biomarkers in a category by round, computing the percent of measurements inside Function's optimal range. Returns a per-round time series — the category's overall health trajectory.",
		Example:     "  function-health-pp-cli category trend cardiovascular\n  function-health-pp-cli category trend Heart --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.ToLower(strings.Join(args, " "))
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			s, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer safeCloseStore(s)
			rows, err := loadAllResults(ctx, s)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return noStoreData("category trend")
			}

			points, distinct := aggregateCategoryTrend(rows, query)
			if len(points) == 0 {
				return notFoundErr(fmt.Errorf("no synced biomarkers matched category %q", query))
			}

			result := map[string]any{
				"category":        query,
				"rounds":          len(points),
				"history":         points,
				"biomarkers_used": distinct,
			}
			if flags != nil && flags.asJSON {
				return flags.printJSON(cmd, result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Category trend for %q (%d biomarkers across %d rounds):\n", query, distinct, len(points))
			for _, p := range points {
				bar := percentBar(p.PercentOptimal, 24)
				fmt.Fprintf(w, "  %-10s  %3d/%-3d in-optimal  %5.1f%%  %s\n",
					p.DrawDate, p.InOptimal, p.Total, p.PercentOptimal, bar)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	return cmd
}

// categoryRoundPoint is one requisition round's percent-in-optimal score for a
// category.
type categoryRoundPoint struct {
	RequisitionID  string  `json:"requisition_id"`
	DrawDate       string  `json:"draw_date"`
	Total          int     `json:"total_biomarkers"`
	InOptimal      int     `json:"in_optimal"`
	PercentOptimal float64 `json:"percent_optimal"`
}

// aggregateCategoryTrend buckets every draw whose category substring-matches
// query into per-requisition rounds (sorted oldest→newest) and returns them
// alongside the count of DISTINCT biomarkers that contributed. The distinct
// count — not the draw-row total — is what `biomarkers_used` reports: a
// biomarker measured across N rounds must count once, not N times, or the
// number balloons to (biomarker_count × round_count).
func aggregateCategoryTrend(rows []resultRow, query string) (points []categoryRoundPoint, distinctBiomarkers int) {
	byRound := map[string]*categoryRoundPoint{}
	biomarkers := map[string]bool{}
	for _, r := range rows {
		if !strings.Contains(strings.ToLower(r.Category), query) {
			continue
		}
		if k := biomarkerKey(r); k != "" {
			biomarkers[k] = true
		}
		key := r.RequisitionID
		a, ok := byRound[key]
		if !ok {
			a = &categoryRoundPoint{RequisitionID: key, DrawDate: formatDrawDate(r.DrawDate)}
			byRound[key] = a
		}
		if a.DrawDate == "" {
			a.DrawDate = formatDrawDate(r.DrawDate)
		}
		a.Total++
		if optimalSign(r) == 0 && hasOptimal(r) {
			a.InOptimal++
		}
	}
	for _, a := range byRound {
		if a.Total > 0 {
			a.PercentOptimal = float64(a.InOptimal) / float64(a.Total) * 100
		}
		points = append(points, *a)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].DrawDate < points[j].DrawDate })
	return points, len(biomarkers)
}

func percentBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
