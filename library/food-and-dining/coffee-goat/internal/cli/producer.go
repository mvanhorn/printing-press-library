// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// producerSummary is one row in the producer index.
type producerSummary struct {
	Producer     string            `json:"producer"`
	Origin       string            `json:"origin,omitempty"`
	Roasters     []producerRoaster `json:"roasters"`
	RoasterCount int               `json:"roaster_count"`
	YearsSeen    []string          `json:"years_seen,omitempty"`
	InStockNow   int               `json:"in_stock_now"`
}

type producerRoaster struct {
	RoasterSlug string `json:"roaster_slug"`
	Handle      string `json:"handle"`
	Title       string `json:"title"`
	Origin      string `json:"origin,omitempty"`
	Process     string `json:"process,omitempty"`
	Varietal    string `json:"varietal,omitempty"`
	InStock     bool   `json:"in_stock"`
}

func newProducerCmd(flags *rootFlags) *cobra.Command {
	var (
		nameFilter  string
		minRoasters int
		inStockOnly bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "producer [name]",
		Short: "Cross-roaster producer index. Lists which roasters currently carry each producer's lots over time",
		Long: `Aggregates roaster_products.producer across the 24 synced roasters and
returns each producer with the set of roasters carrying their lots,
the years seen, and current in-stock count. Use to track a specific
producer's distribution (--name "Diego Bermudez") or surface
multi-roaster producers (--min-roasters 3).`,
		Example: `  coffee-goat-pp-cli producer "Diego Bermudez" --agent
  coffee-goat-pp-cli producer --min-roasters 3 --in-stock --limit 20`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) > 0 && nameFilter == "" {
				nameFilter = strings.Join(args, " ")
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			summaries, err := buildProducerSummaries(db, nameFilter, inStockOnly)
			if err != nil {
				return err
			}
			if minRoasters > 0 {
				summaries = filterProducerMinRoasters(summaries, minRoasters)
			}
			sort.Slice(summaries, func(i, j int) bool {
				if summaries[i].RoasterCount != summaries[j].RoasterCount {
					return summaries[i].RoasterCount > summaries[j].RoasterCount
				}
				return summaries[i].Producer < summaries[j].Producer
			})
			if limit > 0 && limit < len(summaries) {
				summaries = summaries[:limit]
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), summaries, flags)
			}
			if len(summaries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no producers matched (try 'sync' first)")
				return nil
			}
			for _, p := range summaries {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — %d roaster(s), %d in stock now\n", p.Producer, p.RoasterCount, p.InStockNow)
				for _, r := range p.Roasters {
					mark := ""
					if !r.InStock {
						mark = " [out]"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    %s / %s (%s, %s, %s)%s\n", r.RoasterSlug, r.Title, r.Origin, r.Process, r.Varietal, mark)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&nameFilter, "name", "", "Restrict to producers matching this case-insensitive substring")
	cmd.Flags().IntVar(&minRoasters, "min-roasters", 0, "Only producers with at least N roasters carrying their lots")
	cmd.Flags().BoolVar(&inStockOnly, "in-stock", false, "Only count rows currently in stock")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max producers to return (0 = no limit)")
	return cmd
}

func buildProducerSummaries(db *store.Store, nameFilter string, inStockOnly bool) ([]producerSummary, error) {
	q := `SELECT COALESCE(producer,''), COALESCE(roaster_slug,''), COALESCE(handle,''),
	             COALESCE(title,''), COALESCE(origin,''), COALESCE(process,''),
	             COALESCE(varietal,''), COALESCE(in_stock,0), COALESCE(published_at,'')
	      FROM roaster_products
	      WHERE producer IS NOT NULL AND producer != ''`
	args := []any{}
	if nameFilter != "" {
		q += ` AND LOWER(producer) LIKE ?`
		args = append(args, "%"+strings.ToLower(nameFilter)+"%")
	}
	if inStockOnly {
		q += ` AND in_stock = 1`
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byName := map[string]*producerSummary{}
	yearSet := map[string]map[string]bool{}
	for rows.Next() {
		var producer, roasterSlug, handle, title, origin, process, varietal, publishedAt string
		var inStockInt int
		if err := rows.Scan(&producer, &roasterSlug, &handle, &title, &origin, &process, &varietal, &inStockInt, &publishedAt); err != nil {
			return nil, err
		}
		key := strings.ToLower(strings.TrimSpace(producer))
		if key == "" {
			continue
		}
		summary, ok := byName[key]
		if !ok {
			summary = &producerSummary{Producer: producer, Origin: origin}
			byName[key] = summary
			yearSet[key] = map[string]bool{}
		}
		summary.Roasters = append(summary.Roasters, producerRoaster{
			RoasterSlug: roasterSlug,
			Handle:      handle,
			Title:       title,
			Origin:      origin,
			Process:     process,
			Varietal:    varietal,
			InStock:     inStockInt == 1,
		})
		if inStockInt == 1 {
			summary.InStockNow++
		}
		if len(publishedAt) >= 4 {
			yearSet[key][publishedAt[:4]] = true
		}
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	out := make([]producerSummary, 0, len(byName))
	for key, s := range byName {
		seen := map[string]bool{}
		uniqRoasters := 0
		for _, r := range s.Roasters {
			if !seen[r.RoasterSlug] {
				seen[r.RoasterSlug] = true
				uniqRoasters++
			}
		}
		s.RoasterCount = uniqRoasters
		years := make([]string, 0, len(yearSet[key]))
		for y := range yearSet[key] {
			years = append(years, y)
		}
		sort.Strings(years)
		s.YearsSeen = years
		out = append(out, *s)
	}
	return out, nil
}

func filterProducerMinRoasters(rows []producerSummary, minN int) []producerSummary {
	out := make([]producerSummary, 0, len(rows))
	for _, r := range rows {
		if r.RoasterCount >= minN {
			out = append(out, r)
		}
	}
	return out
}
