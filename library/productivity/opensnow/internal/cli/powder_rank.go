package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/productivity/opensnow/internal/cliutil"

	"github.com/spf13/cobra"
)

func newPowderRankCmd(flags *rootFlags) *cobra.Command {
	var flagSlugs []string

	cmd := &cobra.Command{
		Use:     "powder-rank",
		Short:   "Rank resorts by powder potential on their best upcoming day",
		Long:    "Fetches snow forecast for all favorites (or specified slugs) and ranks them by powder score on their single best upcoming day.",
		Example: "  opensnow-pp-cli powder-rank\n  opensnow-pp-cli powder-rank --slugs vail,aspen,telluride",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			var slugs []string
			if len(flagSlugs) > 0 {
				slugs = flagSlugs
			} else {
				db, favs, err := loadFavorites(cmd.Context())
				if err != nil {
					return err
				}
				db.Close()
				slugs = favs
			}

			if len(slugs) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No favorites configured. Add some with: opensnow-pp-cli favorites add <slug>")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type rankData struct {
				Slug string
				Data map[string]any
			}

			results, errs := cliutil.FanoutRun(
				cmd.Context(),
				slugs,
				func(s string) string { return s },
				func(ctx context.Context, slug string) (rankData, error) {
					path := "/forecast/snow-detail/" + slug
					data, err := c.Get(path, map[string]string{})
					if err != nil {
						return rankData{}, err
					}
					data = extractResponseData(data)
					var obj map[string]any
					if err := json.Unmarshal(data, &obj); err != nil {
						return rankData{}, err
					}
					return rankData{Slug: slug, Data: obj}, nil
				},
			)
			cliutil.FanoutReportErrors(os.Stderr, errs)

			if len(results) == 0 {
				return fmt.Errorf("no forecast data available")
			}

			type rankRow struct {
				Rank       int     `json:"rank"`
				Resort     string  `json:"resort"`
				BestDay    string  `json:"best_day"`
				Score      float64 `json:"score"`
				Snow       string  `json:"expected_snow"`
				Conditions string  `json:"conditions"`
			}

			var ranked []rankRow
			for _, r := range results {
				scores := extractPowderScores(r.Value.Data, 5)
				if len(scores) == 0 {
					continue
				}
				// Find best day
				best := scores[0]
				for _, s := range scores[1:] {
					if s.Score > best.Score {
						best = s
					}
				}
				name := r.Value.Slug
				if v, ok := r.Value.Data["name"].(string); ok {
					name = v
				}
				ranked = append(ranked, rankRow{
					Resort:     name,
					BestDay:    best.Day,
					Score:      best.Score,
					Snow:       best.Snow,
					Conditions: best.Conditions,
				})
			}

			// Sort by score descending
			sort.Slice(ranked, func(i, j int) bool {
				return ranked[i].Score > ranked[j].Score
			})
			for i := range ranked {
				ranked[i].Rank = i + 1
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), ranked, flags)
			}

			headers := []string{"Rank", "Resort", "Best Day", "Score", "Expected Snow", "Conditions"}
			tableRows := make([][]string, 0, len(ranked))
			for _, r := range ranked {
				tableRows = append(tableRows, []string{
					fmt.Sprintf("%d", r.Rank),
					r.Resort,
					r.BestDay,
					fmt.Sprintf("%.1f", r.Score),
					r.Snow,
					r.Conditions,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringSliceVar(&flagSlugs, "slugs", nil, "Comma-separated list of slugs (defaults to favorites)")
	return cmd
}
