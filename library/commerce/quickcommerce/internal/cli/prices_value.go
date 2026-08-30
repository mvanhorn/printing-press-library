// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var qcPackRE = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(kg|g|litres?|liters?|l|ml|packs?|pcs?|pieces?|count)`)

type qcValueRow struct {
	Platform  string  `json:"platform"`
	ItemID    string  `json:"item_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  string  `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Unit      string  `json:"unit"`
	Location  string  `json:"location"`
}

func newNovelPricesValueCmd(flags *rootFlags) *cobra.Command {
	var query, location, dbPath string
	cmd := &cobra.Command{Use: "value", Short: "Compare price per unit from explicit pack quantities without guessing missing units.", Example: "  quickcommerce-pp-cli prices value --query milk --location 12.9021,77.6639 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "prices value")
		}
		if strings.TrimSpace(query) == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--query is required; provide a product keyword"))
		}
		if location == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--location is required as latitude,longitude"))
		}
		_, _, canonical, err := parseQCLocation(location)
		if err != nil {
			return usageErr(err)
		}
		path := qcDBPath(dbPath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return qcMissingMirror(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, path, "products")
		}
		if err != nil {
			return apiErr(err)
		}
		db, err := openQCCreatable(cmd.Context(), path)
		if err != nil {
			return err
		}
		defer db.Close()
		rows, err := db.DB().QueryContext(cmd.Context(), `SELECT item_id,platform,location,price,quantity,data FROM quickcommerce_observations WHERE resource IN ('products','items','comparison') AND location=? AND lower(CAST(data AS TEXT)) LIKE lower(?) ORDER BY price ASC`, canonical, "%"+query+"%")
		if err != nil {
			return apiErr(fmt.Errorf("querying value observations: %w", err))
		}
		defer rows.Close()
		out := make([]qcValueRow, 0)
		skipped := 0
		for rows.Next() {
			var itemID, platform, loc, quantity string
			var price sql.NullFloat64
			var data []byte
			if err := rows.Scan(&itemID, &platform, &loc, &price, &quantity, &data); err != nil {
				return apiErr(err)
			}
			if !price.Valid {
				skipped++
				continue
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			name := fmt.Sprint(m["name"])
			_, unit, base, ok := qcPack(quantity)
			if !ok || base <= 0 {
				skipped++
				continue
			}
			out = append(out, qcValueRow{Platform: platform, ItemID: itemID, Name: name, Price: price.Float64, Quantity: quantity, UnitPrice: price.Float64 / base, Unit: unit, Location: loc})
		}
		if err := rows.Err(); err != nil {
			return apiErr(err)
		}
		view := map[string]any{"query": query, "location": canonical, "results": out, "skipped_unpriced_or_ambiguous": skipped}
		if len(out) == 0 {
			view["note"] = "no comparable observations found; only rows with an explicit quantity and price are included"
		}
		return qcPrint(cmd.OutOrStdout(), flags, view, valueTable(out))
	}}
	cmd.Flags().StringVar(&query, "query", "", "Product keyword to compare")
	cmd.Flags().StringVar(&location, "location", "", "Coordinates as latitude,longitude")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path")
	return cmd
}
func qcPack(raw string) (string, string, float64, bool) {
	m := qcPackRE.FindStringSubmatch(raw)
	if len(m) != 3 {
		return "", "", 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return "", "", 0, false
	}
	u := strings.ToLower(m[2])
	switch u {
	case "kg":
		return m[0], "g", n * 1000, true
	case "g":
		return m[0], "g", n, true
	case "l", "liter", "liters", "litre", "litres":
		return m[0], "ml", n * 1000, true
	case "ml":
		return m[0], "ml", n, true
	default:
		return m[0], u, n, true
	}
}
func valueTable(rows []qcValueRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"platform": r.Platform, "item_id": r.ItemID, "name": r.Name, "price": r.Price, "quantity": r.Quantity, "unit_price": r.UnitPrice, "unit": r.Unit})
	}
	return out
}
