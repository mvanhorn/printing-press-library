// Copyright 2026 Dickie and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Not generated. Rolls up holding value
// by obligor (or currency/cell) across every entity and computes each group's
// share of the consolidated book — the cross-entity denominator the strictly
// entity-scoped API cannot return in one call.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newNovelConcentrationCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagLimit string
	var dbPath string
	var includeRedeemed bool

	cmd := &cobra.Command{
		Use:   "concentration",
		Short: "Consolidated credit/exposure concentration by obligor across all entities",
		Long: strings.Trim(`
Sums live holding value by obligor (default), currency, or cell across every
entity in the local mirror, and reports each group's share of the consolidated
book. With --limit, flags any group whose share exceeds your self-set limit.

Run `+"`ts-pp-cli sync`"+` first to populate the mirror.`, "\n"),
		Example: strings.Trim(`
  ts-pp-cli concentration --by obligor --limit 10% --json
  ts-pp-cli concentration --by currency`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			by := strings.ToLower(strings.TrimSpace(flagBy))
			if by == "" {
				by = "obligor"
			}
			var groupCol string
			switch by {
			case "obligor":
				groupCol = "obligor_exposure_code"
			case "currency":
				groupCol = "currency"
			case "cell":
				groupCol = "cell_code"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by must be one of: obligor, currency, cell"))
			}

			limit, hasLimit := parseShareLimit(flagLimit)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, ok, err := openMirror(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return emitEmpty(cmd, flags)
			}
			defer db.Close()

			where := activeHoldingWhere
			if includeRedeemed {
				where = "1=1"
			}
			// Group by the chosen column; LEFT JOIN obligor_exposure for a human
			// name when grouping by obligor.
			q := `SELECT h.` + groupCol + ` AS k, COALESCE(SUM(h.value), 0) AS exposure, COUNT(*) AS holdings, MAX(oe.description) AS name
				FROM holding h
				LEFT JOIN obligor_exposure oe ON oe.code = h.obligor_exposure_code
				WHERE ` + where + `
				GROUP BY h.` + groupCol

			rows, err := db.DB().QueryContext(ctx, q)
			if err != nil {
				return fmt.Errorf("querying holdings: %w", err)
			}
			defer rows.Close()

			type row struct {
				Key      string  `json:"key"`
				Name     string  `json:"name,omitempty"`
				Exposure float64 `json:"exposure"`
				Share    float64 `json:"share"`
				Holdings int     `json:"holdings"`
				Breach   bool    `json:"breach"`
			}
			var rowsOut []row
			var total float64
			for rows.Next() {
				var k, name sql.NullString
				var exposure sql.NullFloat64
				var holdings sql.NullInt64
				if err := rows.Scan(&k, &exposure, &holdings, &name); err != nil {
					continue
				}
				rowsOut = append(rowsOut, row{
					Key:      k.String,
					Name:     name.String,
					Exposure: exposure.Float64,
					Holdings: int(holdings.Int64),
				})
				total += exposure.Float64
			}

			for i := range rowsOut {
				if total > 0 {
					rowsOut[i].Share = rowsOut[i].Exposure / total
				}
				if hasLimit && rowsOut[i].Share > limit {
					rowsOut[i].Breach = true
				}
			}
			sort.Slice(rowsOut, func(i, j int) bool { return rowsOut[i].Exposure > rowsOut[j].Exposure })

			if jsonMode(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rowsOut, flags)
			}
			if len(rowsOut) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no active holdings found")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			header := strings.ToUpper(by)
			fmt.Fprintf(tw, "%s\tNAME\tEXPOSURE\tSHARE\tHOLDINGS\tBREACH\n", header)
			for _, r := range rowsOut {
				breach := ""
				if r.Breach {
					breach = "!"
				}
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.1f%%\t%d\t%s\n", r.Key, r.Name, r.Exposure, r.Share*100, r.Holdings, breach)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "obligor", "Group exposure by 'obligor', 'currency', or 'cell'")
	cmd.Flags().StringVar(&flagLimit, "limit", "", "Flag groups over this share, e.g. 10% or 0.1")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	cmd.Flags().BoolVar(&includeRedeemed, "include-redeemed", false, "Include redeemed/cancelled holdings")
	return cmd
}
