// Copyright 2026 Jan Medina and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented recommend (coverage knapsack over notes + budget).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"perfumenz-pp-cli/internal/store"
)

func newNovelRecommendCmd(flags *rootFlags) *cobra.Command {
	var notes string
	var budget float64
	var count int

	cmd := &cobra.Command{
		Use:         "recommend",
		Short:       "Build a small set of perfumes that together cover requested notes under a budget.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  perfumenz-pp-cli recommend --notes \"vanilla,oud,spicy\" --budget 400 --count 5 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) { fmt.Fprintln(cmd.OutOrStdout(), "would solve note-coverage knapsack under budget"); return nil }
			dbPath := defaultDBPath("perfumenz-pp-cli")
			if dbPath == "" { dbPath = os.ExpandEnv("$HOME/.local/share/perfumenz-pp-cli/data.db") }
			db, err := store.OpenReadOnly(dbPath)
			if err != nil { return fmt.Errorf("store: %w", err) }
			defer db.Close()

			q := strings.Split(notes, ",")
			for i := range q { q[i] = strings.ToLower(strings.TrimSpace(q[i])) }

			rows, _ := db.DB().QueryContext(cmd.Context(), `SELECT id, data FROM resources WHERE resource_type IN ('perfumes','products','products-json')`)
			defer rows.Close()

			type cand struct {
				ID string
				Title string
				Price float64
				Covers []string
			}
			var cands []cand
			for rows.Next() {
				var id string; var data []byte
				rows.Scan(&id, &data)
				var p map[string]any
				json.Unmarshal(data, &p)
				t, h, b := notesFromAny(p)
				all := append(append(t, h...), b...)
				var covers []string
				for _, want := range q {
					for _, have := range all {
						if have == want {
							covers = append(covers, want)
							break
						}
					}
				}
				if len(covers) == 0 { continue }
				price := 0.0
				if vs, ok := p["variants"].([]any); ok && len(vs) > 0 {
					if v0, ok := vs[0].(map[string]any); ok {
						if pr, ok := v0["price"].(string); ok {
							price, _ = strconv.ParseFloat(pr, 64)
						}
					}
				}
				if price <= 0 { continue }
				cands = append(cands, cand{ID: id, Title: p["title"].(string), Price: price, Covers: covers})
			}

			// greedy: pick lowest price that adds the most new coverage, repeat until budget or count
			selected := []cand{}
			covered := map[string]bool{}
			remain := budget
			for len(selected) < count && remain > 0 {
				bestIdx := -1
				bestNew := 0
				bestPrice := 0.0
				for i, c := range cands {
					newC := 0
					for _, n := range c.Covers {
						if !covered[n] {
							newC++
						}
					}
					if newC > bestNew && c.Price <= remain {
						bestNew = newC
						bestIdx = i
						bestPrice = c.Price
					}
				}
				if bestIdx < 0 {
					break
				}
				sel := cands[bestIdx]
				selected = append(selected, sel)
				for _, n := range sel.Covers {
					covered[n] = true
				}
				remain -= bestPrice
				// remove to avoid dups
				cands = append(cands[:bestIdx], cands[bestIdx+1:]...)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"selected": selected, "budget": budget, "spent": budget - remain})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Discovery set (budget %.0f, spent %.0f):\n", budget, budget-remain)
			for _, s := range selected {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s ($%.0f) covers %v\n", s.Title, s.Price, s.Covers)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&notes, "notes", "", "Desired notes to cover")
	cmd.Flags().Float64Var(&budget, "budget", 300, "Total budget cap")
	cmd.Flags().IntVar(&count, "count", 5, "Target number of perfumes")
	return cmd
}
