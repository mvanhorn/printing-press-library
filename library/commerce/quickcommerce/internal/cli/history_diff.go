// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newNovelHistoryDiffCmd(flags *rootFlags) *cobra.Command {
	var item, latest, dbPath string
	cmd := &cobra.Command{
		Use: "diff", Short: "Show the field-level changes between the latest saved observations.",
		Example:     "  quickcommerce-pp-cli history diff --item 501346 --latest 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "history diff")
			}
			if item == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--item is required; use a platform item ID"))
			}
			n := 2
			if latest != "" {
				parsed, err := strconv.Atoi(latest)
				if err != nil || parsed < 2 || parsed > 20 {
					return usageErr(fmt.Errorf("--latest must be an integer from 2 through 20"))
				}
				n = parsed
			}
			path := qcDBPath(dbPath)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return qcMissingMirror(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, path, "products")
			} else if err != nil {
				return apiErr(err)
			}
			db, err := openQCCreatable(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT captured_at, data, price, mrp, inventory, available, quantity, eta, open, platform FROM quickcommerce_observations WHERE item_id=? ORDER BY captured_at DESC LIMIT ?`, item, n)
			if err != nil {
				return apiErr(fmt.Errorf("querying observations: %w", err))
			}
			type obs struct {
				At                      time.Time
				Data                    map[string]any
				Price, MRP              sql.NullFloat64
				Inventory               sql.NullInt64
				Available, Open         sql.NullBool
				Quantity, ETA, Platform string
			}
			found := make([]obs, 0, n)
			for rows.Next() {
				var at any
				var data []byte
				var p, m sql.NullFloat64
				var inv sql.NullInt64
				var av, op sql.NullBool
				var q, e, pl string
				if err := rows.Scan(&at, &data, &p, &m, &inv, &av, &q, &e, &op, &pl); err != nil {
					_ = rows.Close()
					return apiErr(err)
				}
				var dm map[string]any
				_ = json.Unmarshal(data, &dm)
				found = append(found, obs{qcDBTime(at), dm, p, m, inv, av, op, q, e, pl})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			_ = rows.Close()
			result := map[string]any{"item": item, "observations": make([]map[string]any, 0), "changes": map[string]any{}}
			for _, o := range found {
				result["observations"] = append(result["observations"].([]map[string]any), map[string]any{"captured_at": o.At.Format(time.RFC3339), "platform": o.Platform, "price": nullableFloat(o.Price), "mrp": nullableFloat(o.MRP), "inventory": nullableInt(o.Inventory), "available": nullableBool(o.Available), "quantity": o.Quantity, "eta": o.ETA, "open": nullableBool(o.Open)})
			}
			if len(found) < 2 {
				result["note"] = fmt.Sprintf("only %d observation(s) found; ingest at least two real responses before diffing", len(found))
				return qcPrint(cmd.OutOrStdout(), flags, result, []map[string]any{result})
			}
			changes := result["changes"].(map[string]any)
			before, after := found[1], found[0]
			compareQCField(changes, "price", nullableFloat(before.Price), nullableFloat(after.Price))
			compareQCField(changes, "mrp", nullableFloat(before.MRP), nullableFloat(after.MRP))
			compareQCField(changes, "inventory", nullableInt(before.Inventory), nullableInt(after.Inventory))
			compareQCField(changes, "available", nullableBool(before.Available), nullableBool(after.Available))
			compareQCField(changes, "quantity", before.Quantity, after.Quantity)
			compareQCField(changes, "eta", before.ETA, after.ETA)
			compareQCField(changes, "open", nullableBool(before.Open), nullableBool(after.Open))
			return qcPrint(cmd.OutOrStdout(), flags, result, []map[string]any{result})
		},
	}
	cmd.Flags().StringVar(&item, "item", "", "Platform item ID to diff")
	cmd.Flags().StringVar(&latest, "latest", "2", "Number of recent observations to inspect (2-20)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path")
	return cmd
}
func nullableFloat(v sql.NullFloat64) any {
	if v.Valid {
		return v.Float64
	}
	return nil
}
func nullableInt(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
func nullableBool(v sql.NullBool) any {
	if v.Valid {
		return v.Bool
	}
	return nil
}
func compareQCField(out map[string]any, key string, before, after any) {
	if fmt.Sprint(before) != fmt.Sprint(after) {
		out[key] = map[string]any{"before": before, "after": after}
	}
}
