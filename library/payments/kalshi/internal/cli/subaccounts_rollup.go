// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/kalshi/internal/store"
)

// newSubaccountsCmd is the parent for novel 'subaccounts' commands. The
// generated CLI does not emit a top-level 'subaccounts' parent (subaccount
// endpoints live under 'portfolio'), so we ship one here for the rollup.
func newSubaccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subaccounts",
		Short: "Subaccount rollup and household-level views (local store)",
	}
	cmd.AddCommand(newSubaccountsRollupCmd(flags))
	return cmd
}

// newSubaccountsRollupCmd aggregates positions, balances, and exposure across
// every subaccount visible in the local store. Read-only and store-backed.
func newSubaccountsRollupCmd(flags *rootFlags) *cobra.Command {
	var byField string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "rollup",
		Short:       "Aggregate positions and balances across subaccounts (local store)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Reads any subaccounts the local store has captured and rolls up positions and
balances into a single household view. The schema may not have a 'subaccounts'
table yet — when it doesn't, this prints a setup hint and exits 0 so agents
get a discoverable command surface today and a populated rollup once the
underlying sync lands.`,
		Example: `  # Aggregate by category (default)
  kalshi-pp-cli subaccounts rollup

  # Aggregate by series
  kalshi-pp-cli subaccounts rollup --by series

  # No grouping; one row per subaccount
  kalshi-pp-cli subaccounts rollup --by none --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch byField {
			case "category", "series", "none":
				// valid
			default:
				return fmt.Errorf("invalid --by %q: must be category, series, or none", byField)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			has, err := hasSubaccountsTable(db.DB())
			if err != nil {
				return fmt.Errorf("checking subaccounts table: %w", err)
			}
			if !has {
				fmt.Fprintln(cmd.OutOrStdout(), "subaccounts not yet synced; run 'sync subaccounts'")
				return nil
			}

			rows, err := querySubaccountsRollup(db.DB(), byField)
			if err != nil {
				return fmt.Errorf("querying subaccounts rollup: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"Group", "Subaccounts", "Positions", "Balance ($)", "Exposure ($)"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					r.Group,
					fmt.Sprintf("%d", r.Subaccounts),
					fmt.Sprintf("%d", r.Positions),
					fmt.Sprintf("%.2f", r.Balance),
					fmt.Sprintf("%.2f", r.Exposure),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&byField, "by", "category", "Group by: category, series, or none")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type subaccountsRollupRow struct {
	Group       string  `json:"group"`
	Subaccounts int     `json:"subaccounts"`
	Positions   int     `json:"positions"`
	Balance     float64 `json:"balance"`
	Exposure    float64 `json:"exposure"`
}

func hasSubaccountsTable(db *sql.DB) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='subaccounts'`).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.EqualFold(name, "subaccounts"), nil
}

func querySubaccountsRollup(db *sql.DB, byField string) ([]subaccountsRollupRow, error) {
	groupExpr := "'all'"
	switch byField {
	case "category":
		groupExpr = "COALESCE(json_extract(s.data, '$.category'), 'unknown')"
	case "series":
		groupExpr = "COALESCE(json_extract(s.data, '$.series_ticker'), 'unknown')"
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS grp,
			COUNT(DISTINCT s.id) AS subaccounts,
			COALESCE(SUM(json_array_length(json_extract(s.data, '$.positions'))), 0) AS positions,
			COALESCE(SUM(json_extract(s.data, '$.balance')), 0) / 100.0 AS balance,
			COALESCE(SUM(json_extract(s.data, '$.exposure')), 0) / 100.0 AS exposure
		FROM subaccounts s
		GROUP BY grp
		ORDER BY exposure DESC
	`, groupExpr)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []subaccountsRollupRow
	for rows.Next() {
		var r subaccountsRollupRow
		if err := rows.Scan(&r.Group, &r.Subaccounts, &r.Positions, &r.Balance, &r.Exposure); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
