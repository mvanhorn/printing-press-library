// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// compareEntry is one column in the side-by-side compare table.
type compareEntry struct {
	Roaster      string  `json:"roaster"`
	Handle       string  `json:"handle"`
	Title        string  `json:"title"`
	Origin       string  `json:"origin,omitempty"`
	Producer     string  `json:"producer,omitempty"`
	Process      string  `json:"process,omitempty"`
	Varietal     string  `json:"varietal,omitempty"`
	Altitude     string  `json:"altitude,omitempty"`
	RoastLevel   string  `json:"roast_level,omitempty"`
	PriceCents   int     `json:"price_cents,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	WeightG      int     `json:"weight_g,omitempty"`
	PricePerOz   string  `json:"price_per_oz,omitempty"`
	InStock      bool    `json:"in_stock"`
	URL          string  `json:"url,omitempty"`
	CoffeeReview *int    `json:"coffee_review_score,omitempty"`
	UserRating   float64 `json:"user_avg_rating,omitempty"`
	UserBrews    int     `json:"user_brews,omitempty"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <bean> [<bean>...]",
		Short: "Side-by-side compare of two or more beans. Identifies each bean by roaster_products.handle or roaster/handle",
		Long: `Joins roaster_products with reviews (Coffee Review score) and brews
(your user rating + brew count) for each named bean. Beans are matched
on handle alone or roaster/handle when the handle is ambiguous across
roasters. The output is a structured row per bean — render side-by-side
in JSON, one row per bean in text mode.`,
		Example: `  coffee-goat-pp-cli compare sey/banko-gotiti onyx/geisha-honey --agent
  coffee-goat-pp-cli compare banko-gotiti geisha-honey la-cabra-yirgacheffe`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 && !flags.asJSON {
				return usageErr(fmt.Errorf("compare needs at least 2 beans"))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			out := make([]compareEntry, 0, len(args))
			for _, raw := range args {
				e, err := lookupCompareEntry(db, raw)
				if err != nil {
					return err
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			renderCompareTable(cmd, out)
			return nil
		},
	}
	return cmd
}

func lookupCompareEntry(db *store.Store, raw string) (compareEntry, error) {
	roaster, handle := splitRoasterHandle(raw)
	q := `SELECT COALESCE(roaster_slug,''), COALESCE(handle,''), COALESCE(title,''),
	             COALESCE(origin,''), COALESCE(producer,''), COALESCE(process,''),
	             COALESCE(varietal,''), COALESCE(altitude,''), COALESCE(roast_level,''),
	             COALESCE(price_cents,0), COALESCE(currency,''), COALESCE(weight_g,0),
	             COALESCE(in_stock,0), COALESCE(url,'')
	      FROM roaster_products
	      WHERE LOWER(handle) = LOWER(?)`
	args := []any{handle}
	if roaster != "" {
		q += ` AND LOWER(roaster_slug) = LOWER(?)`
		args = append(args, roaster)
	}
	q += ` LIMIT 1`
	row := db.DB().QueryRow(q, args...)
	var e compareEntry
	var inStockInt int
	err := row.Scan(
		&e.Roaster, &e.Handle, &e.Title,
		&e.Origin, &e.Producer, &e.Process, &e.Varietal, &e.Altitude, &e.RoastLevel,
		&e.PriceCents, &e.Currency, &e.WeightG,
		&inStockInt, &e.URL,
	)
	if err == sql.ErrNoRows {
		return compareEntry{}, notFoundErr(fmt.Errorf("bean %q not found in roaster_products (run 'sync' first or check the handle)", raw))
	}
	if err != nil {
		return compareEntry{}, err
	}
	e.InStock = inStockInt == 1
	if e.PriceCents > 0 && e.WeightG > 0 {
		ozs := float64(e.WeightG) / 28.3495
		e.PricePerOz = fmt.Sprintf("$%.2f/oz", float64(e.PriceCents)/100.0/ozs)
	}
	if score, ok := lookupCoffeeReviewScore(db, e.Roaster, e.Title); ok {
		e.CoffeeReview = &score
	}
	if avg, n := lookupUserBrewSummary(db, e.Roaster, e.Handle); n > 0 {
		e.UserRating = avg
		e.UserBrews = n
	}
	return e, nil
}

func splitRoasterHandle(raw string) (roaster, handle string) {
	if i := strings.Index(raw, "/"); i > 0 {
		return raw[:i], raw[i+1:]
	}
	return "", raw
}

func lookupCoffeeReviewScore(db *store.Store, roasterSlug, title string) (int, bool) {
	if title == "" {
		return 0, false
	}
	row := db.DB().QueryRow(
		`SELECT score FROM reviews
		 WHERE LOWER(roaster_name) LIKE ?
		   AND LOWER(bean_name) LIKE ?
		 ORDER BY score DESC LIMIT 1`,
		"%"+strings.ToLower(roasterSlug)+"%",
		"%"+strings.ToLower(title)+"%",
	)
	var score int
	if err := row.Scan(&score); err != nil {
		return 0, false
	}
	return score, true
}

func lookupUserBrewSummary(db *store.Store, roasterSlug, handle string) (float64, int) {
	row := db.DB().QueryRow(
		`SELECT COALESCE(AVG(b.rating),0), COUNT(*)
		 FROM brews b
		 JOIN beans bn ON b.bean_id = bn.id
		 WHERE bn.roaster_slug = ? AND bn.product_slug = ? AND b.rating > 0`,
		roasterSlug, handle,
	)
	var avg float64
	var n int
	if err := row.Scan(&avg, &n); err != nil {
		return 0, 0
	}
	return avg, n
}

func renderCompareTable(cmd *cobra.Command, rows []compareEntry) {
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no beans matched")
		return
	}
	for _, r := range rows {
		mark := ""
		if !r.InStock {
			mark = " [out]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s / %s — %s%s\n", r.Roaster, r.Title, r.Origin, mark)
		fmt.Fprintf(cmd.OutOrStdout(), "    producer=%s  process=%s  varietal=%s  altitude=%s\n", r.Producer, r.Process, r.Varietal, r.Altitude)
		fmt.Fprintf(cmd.OutOrStdout(), "    price=%d %s (%dg) %s\n", r.PriceCents, r.Currency, r.WeightG, r.PricePerOz)
		if r.CoffeeReview != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "    coffee_review=%d\n", *r.CoffeeReview)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "    coffee_review=—")
		}
		if r.UserBrews > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "    user_rating=%.1f (%d brews)\n", r.UserRating, r.UserBrews)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "    user_rating=— (no brews)")
		}
	}
}
