// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// palateMapReport surfaces the user's preferred / avoided descriptor
// tokens derived from rating-weighted brews. Distinct from flavor-wheel
// (which is bucketed by the official SCA hierarchy) — palate-map is a
// flat, top-N descriptor signature.
type palateMapReport struct {
	N         int            `json:"brews_scored"`
	Preferred []palateToken  `json:"preferred"`
	Avoided   []palateToken  `json:"avoided"`
	OriginFit []palateOrigin `json:"origin_fit"`
}

type palateToken struct {
	Token  string  `json:"token"`
	Weight float64 `json:"weight"`
	Hits   int     `json:"hits"`
}

type palateOrigin struct {
	Origin string  `json:"origin"`
	Mean   float64 `json:"mean_rating"`
	N      int     `json:"n"`
}

func newPalateMapCmd(flags *rootFlags) *cobra.Command {
	var topN int
	cmd := &cobra.Command{
		Use:   "palate-map",
		Short: "Per-user descriptor signature: which flavor tokens your rated brews tend to land on",
		Long: `Aggregates roaster_products descriptors + brew-time descriptors weighted
by brew rating. Positive weights for ratings >=7 (loved tokens),
negative weights for ratings <=4 (avoided tokens). Also surfaces
per-origin mean rating so you can see which countries you actually
prefer.`,
		Example:     `  coffee-goat-pp-cli palate-map --top 10 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if topN <= 0 {
				topN = 10
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			report, err := buildPalateMapReport(db, topN)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "palate-map (%d brews):\n", report.N)
			fmt.Fprintln(cmd.OutOrStdout(), "  preferred:")
			for _, t := range report.Preferred {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s  (weight %+.1f, %d hits)\n", t.Token, t.Weight, t.Hits)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  avoided:")
			for _, t := range report.Avoided {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s  (weight %+.1f, %d hits)\n", t.Token, t.Weight, t.Hits)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  origin fit:")
			for _, o := range report.OriginFit {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s  mean=%.1f (n=%d)\n", o.Origin, o.Mean, o.N)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&topN, "top", 10, "Top-N preferred / avoided tokens to surface")
	return cmd
}

func buildPalateMapReport(db *store.Store, topN int) (palateMapReport, error) {
	type ratedBag struct {
		Rating int
		Tokens []string
		Origin string
	}
	rows, err := db.DB().Query(
		`SELECT COALESCE(b.rating,0),
		        COALESCE(b.descriptors_json,'') || ' ' || COALESCE(rp.tags_json,'') || ' ' || COALESCE(rp.body_text,''),
		        COALESCE(rp.origin,'')
		 FROM brews b
		 LEFT JOIN beans bn ON b.bean_id = bn.id
		 LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle
		 WHERE b.rating > 0`,
	)
	if err != nil {
		return palateMapReport{}, err
	}
	defer rows.Close()
	var all []ratedBag
	for rows.Next() {
		var r ratedBag
		var combined string
		if err := rows.Scan(&r.Rating, &combined, &r.Origin); err != nil {
			return palateMapReport{}, err
		}
		r.Tokens = tokenizeSimple(combined)
		all = append(all, r)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return palateMapReport{}, err
	}

	weights := map[string]float64{}
	hits := map[string]int{}
	originSum := map[string]float64{}
	originN := map[string]int{}
	for _, b := range all {
		seen := map[string]bool{}
		for _, t := range b.Tokens {
			if seen[t] {
				continue
			}
			seen[t] = true
			hits[t]++
			switch {
			case b.Rating >= 7:
				weights[t] += float64(b.Rating - 6)
			case b.Rating <= 4:
				weights[t] += -float64(5 - b.Rating)
			}
		}
		if b.Origin != "" {
			originSum[b.Origin] += float64(b.Rating)
			originN[b.Origin]++
		}
	}

	tokens := make([]palateToken, 0, len(weights))
	for t, w := range weights {
		tokens = append(tokens, palateToken{Token: t, Weight: round1(w), Hits: hits[t]})
	}
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].Weight > tokens[j].Weight })

	var preferred, avoided []palateToken
	for _, t := range tokens {
		switch {
		case t.Weight > 0:
			if len(preferred) < topN {
				preferred = append(preferred, t)
			}
		case t.Weight < 0:
			avoided = append(avoided, t)
		}
	}
	// Sort avoided by most-negative-first.
	sort.Slice(avoided, func(i, j int) bool { return avoided[i].Weight < avoided[j].Weight })
	if len(avoided) > topN {
		avoided = avoided[:topN]
	}

	originFit := make([]palateOrigin, 0, len(originSum))
	for o, s := range originSum {
		originFit = append(originFit, palateOrigin{Origin: o, Mean: round2(s / float64(originN[o])), N: originN[o]})
	}
	sort.Slice(originFit, func(i, j int) bool { return originFit[i].Mean > originFit[j].Mean })

	return palateMapReport{
		N:         len(all),
		Preferred: preferred,
		Avoided:   avoided,
		OriginFit: originFit,
	}, nil
}
