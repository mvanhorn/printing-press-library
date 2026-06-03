// Copyright 2026 Jan Medina and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented trending notes stats.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"perfumenz-pp-cli/internal/store"
)

func newNovelStatsNotesCmd(flags *rootFlags) *cobra.Command {
	var gender string
	var limit int

	cmd := &cobra.Command{
		Use:         "stats notes",
		Short:       "Show the most common notes across in-stock items (overall or filtered).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) { fmt.Fprintln(cmd.OutOrStdout(), "would aggregate note frequencies"); return nil }
			dbPath := defaultDBPath("perfumenz-pp-cli")
			if dbPath == "" { dbPath = os.ExpandEnv("$HOME/.local/share/perfumenz-pp-cli/data.db") }
			db, err := store.OpenReadOnly(dbPath)
			if err != nil { return fmt.Errorf("store: %w", err) }
			defer db.Close()

			rows, _ := db.DB().QueryContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type IN ('perfumes','products','products-json')`)
			defer rows.Close()

			freq := map[string]int{}
			for rows.Next() {
				var data []byte
				rows.Scan(&data)
				var p map[string]any
				json.Unmarshal(data, &p)
				pt, _ := p["product_type"].(string)
				if gender != "" && !strings.Contains(strings.ToLower(pt), strings.ToLower(gender)) { continue }
				t, h, b := notesFromAny(p)
				for _, n := range append(append(t, h...), b...) {
					freq[n]++
				}
			}
			type kv struct{ k string; v int }
			var list []kv
			for k, v := range freq { list = append(list, kv{k, v}) }
			sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
			if limit > 0 && len(list) > limit { list = list[:limit] }

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(list)
			}
			for _, e := range list {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d\n", e.k, e.v)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&gender, "gender", "", "Filter by product_type")
	cmd.Flags().IntVar(&limit, "limit", 15, "Top N")
	return cmd
}
