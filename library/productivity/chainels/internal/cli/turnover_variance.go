package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

type turnoverVarianceRow struct {
	CommunityID string  `json:"community_id"`
	EntityID    string  `json:"entity_id"`
	Median      float64 `json:"median"`
	Latest      float64 `json:"latest"`
	VariancePct float64 `json:"variance_pct"`
	Samples     int     `json:"samples"`
}

func newTurnoverVarianceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var months int
	cmd := &cobra.Command{
		Use:     "variance",
		Short:   "Per-tenant variance vs trailing-N-month median",
		Long:    "Reads the local store synced by `chainels-pp-cli sync`. Computes the median turnover amount over the last --months periods per tenant, then reports the latest period's deviation from the median in percent. Requires `read.turnover` scope.",
		Example: "  chainels-pp-cli turnover variance --months 12 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if months <= 0 {
				return fmt.Errorf("--months must be positive")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			// Walk all turnover rows; JSON shapes vary per scheme so we scan
			// the raw payload and look for a numeric `amount` plus a string
			// `period`. Rows missing either are skipped.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT companies_id,
				       COALESCE(json_extract(data,'$.entity_id'), '') AS entity_id,
				       COALESCE(json_extract(data,'$.amount'), 0) AS amount,
				       COALESCE(json_extract(data,'$.period'), '') AS period
				FROM companies_turnover
				ORDER BY companies_id, entity_id, period DESC`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type key struct{ community, entity string }
			amounts := map[key][]float64{}
			latest := map[key]float64{}
			for rows.Next() {
				var community, entity, period string
				var amount sql.NullFloat64
				if err := rows.Scan(&community, &entity, &amount, &period); err != nil {
					return err
				}
				if !amount.Valid || period == "" {
					continue
				}
				k := key{community, entity}
				if _, ok := latest[k]; !ok {
					latest[k] = amount.Float64
				}
				amounts[k] = append(amounts[k], amount.Float64)
			}
			out := make([]turnoverVarianceRow, 0, len(amounts))
			for k, samples := range amounts {
				if len(samples) < 2 {
					continue
				}
				if len(samples) > months {
					samples = samples[:months]
				}
				med := median(samples)
				if med == 0 {
					continue
				}
				lat := latest[k]
				v := (lat - med) / med * 100
				out = append(out, turnoverVarianceRow{
					CommunityID: k.community,
					EntityID:    k.entity,
					Median:      med,
					Latest:      lat,
					VariancePct: math.Round(v*100) / 100,
					Samples:     len(samples),
				})
			}
			sort.Slice(out, func(i, j int) bool {
				return math.Abs(out[i].VariancePct) > math.Abs(out[j].VariancePct)
			})
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().IntVar(&months, "months", 12, "Trailing window size in months")
	return cmd
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]float64, len(xs))
	copy(sorted, xs)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
