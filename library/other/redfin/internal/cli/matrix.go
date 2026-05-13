// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

// redfinPropTypeLabel maps Redfin propertyType integer codes to readable labels.
func redfinPropTypeLabel(code int) string {
	switch code {
	case 1:
		return "single_family"
	case 2:
		return "condo"
	case 3:
		return "townhouse"
	case 4:
		return "multi_family"
	case 6:
		return "land"
	default:
		return fmt.Sprintf("type_%d", code)
	}
}

func newMatrixCmd(flags *rootFlags) *cobra.Command {
	var zips string
	var propTypes string
	var field string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "matrix",
		Short:       "Compare median price, DOM, and inventory across zip codes and property types",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Build a pivot table comparing key metrics across a grid of zip codes (rows)
and property types (columns). Think of it as the ITA Matrix for real estate —
one query that shows you how markets stack up against each other.

Field options:
  avg_price   Average list price per cell
  avg_dom     Average days-on-market per cell (default)
  inventory   Active listing count per cell

Run 'redfin-pp-cli sync' for each zip you want in the matrix.`,
		Example: `  # Compare DOM across zips and property types
  redfin-pp-cli matrix --zips 94102,94103,94110

  # Price matrix, specific property types
  redfin-pp-cli matrix --zips 94102,94103 --types single_family,condo --field avg_price

  # JSON pivot for agent analysis
  redfin-pp-cli matrix --zips 94102,94103,94110,94117 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if zips == "" {
				return usageErr(fmt.Errorf("--zips is required (comma-separated zip codes)"))
			}

			validFields := map[string]bool{"avg_price": true, "avg_dom": true, "inventory": true}
			if field != "" && !validFields[field] {
				return usageErr(fmt.Errorf("--field must be one of: avg_price, avg_dom, inventory"))
			}
			if field == "" {
				field = "avg_dom"
			}

			zipList := splitTrim(zips)
			if len(zipList) == 0 {
				return usageErr(fmt.Errorf("--zips must contain at least one zip code"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'redfin-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Build parameterized IN clause for zip codes
			zipPlaceholders := make([]string, len(zipList))
			queryArgs := make([]any, len(zipList))
			for i, z := range zipList {
				zipPlaceholders[i] = "?"
				queryArgs[i] = z
			}

			query := `
				SELECT
					json_extract(data, '$.address.zip') AS zip,
					CAST(json_extract(data, '$.propertyType') AS INTEGER) AS prop_type,
					COUNT(*) AS cnt,
					AVG(CAST(json_extract(data, '$.price.value') AS REAL)) AS avg_price,
					AVG(CAST(json_extract(data, '$.dom.value') AS REAL)) AS avg_dom
				FROM resources
				WHERE resource_type = 'listings'
				  AND json_extract(data, '$.status') = 1
				  AND json_extract(data, '$.address.zip') IN (` + strings.Join(zipPlaceholders, ",") + `)
				GROUP BY zip, prop_type
				ORDER BY zip, prop_type`

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data — run 'redfin-pp-cli sync' for each zip first")
				}
				return fmt.Errorf("querying matrix: %w", err)
			}
			defer rows.Close()

			type Cell struct {
				Count    int
				AvgPrice float64
				AvgDOM   float64
			}

			// matrix[zip][propType] = Cell
			matrix := map[string]map[string]Cell{}
			allTypes := map[string]bool{}

			for rows.Next() {
				var zip string
				var propTypeCode int
				var cnt int
				var avgPrice, avgDOM *float64
				if err := rows.Scan(&zip, &propTypeCode, &cnt, &avgPrice, &avgDOM); err != nil {
					continue
				}

				propLabel := redfinPropTypeLabel(propTypeCode)

				// Filter by --types if specified
				if propTypes != "" {
					typeFilter := splitTrim(propTypes)
					found := false
					for _, t := range typeFilter {
						if t == propLabel {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				allTypes[propLabel] = true

				if matrix[zip] == nil {
					matrix[zip] = map[string]Cell{}
				}
				cell := Cell{Count: cnt}
				if avgPrice != nil {
					cell.AvgPrice = roundTo(*avgPrice, 0)
				}
				if avgDOM != nil {
					cell.AvgDOM = roundTo(*avgDOM, 1)
				}
				matrix[zip][propLabel] = cell
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			if len(matrix) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No data found for the specified zips — run 'redfin-pp-cli sync' for each zip first.")
				return nil
			}

			// Build sorted column list
			cols := make([]string, 0, len(allTypes))
			for t := range allTypes {
				cols = append(cols, t)
			}
			sort.Strings(cols)

			// Build output rows
			type MatrixRow struct {
				Zip    string             `json:"zip"`
				Cells  map[string]float64 `json:"cells"`
				Totals map[string]float64 `json:"totals,omitempty"`
			}

			var output []MatrixRow
			for _, zip := range zipList {
				row := MatrixRow{
					Zip:   zip,
					Cells: map[string]float64{},
				}
				for _, col := range cols {
					cell, ok := matrix[zip][col]
					if !ok {
						row.Cells[col] = -1 // sentinel for "no data"
						continue
					}
					switch field {
					case "avg_price":
						row.Cells[col] = cell.AvgPrice
					case "avg_dom":
						row.Cells[col] = cell.AvgDOM
					case "inventory":
						row.Cells[col] = float64(cell.Count)
					}
				}
				output = append(output, row)
			}

			// Wrap with metadata
			result := map[string]any{
				"field":   field,
				"columns": cols,
				"rows":    output,
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&zips, "zips", "", "Comma-separated zip codes for matrix rows (required)")
	cmd.Flags().StringVar(&propTypes, "types", "", "Comma-separated property types for columns (default: all found)")
	cmd.Flags().StringVar(&field, "field", "", "Metric to show in cells: avg_price, avg_dom, inventory (default: avg_dom)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
