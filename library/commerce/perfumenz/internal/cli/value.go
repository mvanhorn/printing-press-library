// Copyright 2026 Jan Medina and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented value / price-per-ml ranking.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"perfumenz-pp-cli/internal/store"
)

func newNovelValueCmd(flags *rootFlags) *cobra.Command {
	var notes string
	var gender string
	var sortBy string // ppm
	var limit int

	cmd := &cobra.Command{
		Use:         "value",
		Short:       "Rank current stock by price per ml (or per 100ml) with filters.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  perfumenz-pp-cli value --notes \"citrus,woody\" --gender unisex --sort ppm --limit 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank by price-per-ml over local store")
				return nil
			}
			dbPath := defaultDBPath("perfumenz-pp-cli")
			if dbPath == "" { dbPath = os.ExpandEnv("$HOME/.local/share/perfumenz-pp-cli/data.db") }
			db, err := store.OpenReadOnly(dbPath)
			if err != nil { return fmt.Errorf("store: %w", err) }
			defer db.Close()

			qTop, qHeart, qBase := ParseNotes("Top Notes: " + notes)

			rows, _ := db.DB().QueryContext(cmd.Context(), `SELECT id, data FROM resources WHERE resource_type IN ('perfumes','products','products-json')`)
			defer rows.Close()

			type row struct {
				Title string
				Vendor string
				Price float64
				SizeML float64
				PPM   float64
			}
			var all []row
			for rows.Next() {
				var id string; var data []byte
				rows.Scan(&id, &data)
				var p map[string]any
				json.Unmarshal(data, &p)

				title, _ := p["title"].(string)
				vendor, _ := p["vendor"].(string)
				pt, _ := p["product_type"].(string)
				if gender != "" && !strings.Contains(strings.ToLower(pt), strings.ToLower(gender)) { continue }

				// price + size guess from variant title or price
				price := 0.0
				size := 100.0 // default assume 100ml if unknown
				if vs, ok := p["variants"].([]any); ok && len(vs) > 0 {
					if v0, ok := vs[0].(map[string]any); ok {
						if pr, ok := v0["price"].(string); ok {
							price, _ = strconv.ParseFloat(pr, 64)
						}
						if t, ok := v0["title"].(string); ok {
							// crude "100ml" parse
							for _, tok := range strings.Fields(t) {
								if strings.HasSuffix(strings.ToLower(tok), "ml") {
									if n, err := strconv.ParseFloat(strings.TrimSuffix(strings.ToLower(tok), "ml"), 64); err == nil && n > 0 {
										size = n
									}
								}
							}
						}
					}
				}
				if price <= 0 { continue }
				ppm := price / size * 100.0 // per 100ml equiv

				t, h, b := notesFromAny(p)
				if len(qTop)+len(qHeart)+len(qBase) > 0 {
					if NotesOverlap(qTop, qHeart, qBase, t, h, b) == 0 { continue }
				}

				all = append(all, row{Title: title, Vendor: vendor, Price: price, SizeML: size, PPM: ppm})
			}

			sort.Slice(all, func(i, j int) bool { return all[i].PPM < all[j].PPM })
			if limit > 0 && len(all) > limit { all = all[:limit] }

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(all)
			}
			for _, r := range all {
				fmt.Fprintf(cmd.OutOrStdout(), "%s — %s $%.0f / %.0fml (%.1f per 100ml)\n", r.Title, r.Vendor, r.Price, r.SizeML, r.PPM)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Filter to perfumes with these notes")
	cmd.Flags().StringVar(&gender, "gender", "", "Filter by product_type (mens/unisex/etc)")
	cmd.Flags().StringVar(&sortBy, "sort", "ppm", "Sort key (ppm)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max rows")
	return cmd
}
