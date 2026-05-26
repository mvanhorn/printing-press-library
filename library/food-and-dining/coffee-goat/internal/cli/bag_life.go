// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// bagLifeReport is the per-bag rating curve plus fitted slope.
type bagLifeReport struct {
	BeanLabel  string         `json:"bean"`
	BeanID     int64          `json:"bean_id,omitempty"`
	Series     []bagLifePoint `json:"series"`
	Fit        driftSeries    `json:"fit"`
	BestDay    int            `json:"best_day,omitempty"`
	BestRating float64        `json:"best_rating,omitempty"`
}

type bagLifePoint struct {
	DaysSinceRoast int     `json:"days_since_roast"`
	Method         string  `json:"method"`
	Rating         float64 `json:"rating"`
	BrewedAt       string  `json:"brewed_at"`
}

func newBagLifeCmd(flags *rootFlags) *cobra.Command {
	var methodFilter string
	var minBrews int
	cmd := &cobra.Command{
		Use:   "bag-life <bean-id-or-roaster/product>",
		Short: "Per-bag rating curve over days-since-roast. Single-bean view of drift",
		Long: `Plots one bag's rating curve as days since roast advances. Fits the
same OLS line as the global drift diagnostic but scoped to one bean
so you can see whether a specific bag peaked at day 14 vs day 21.`,
		Example: `  coffee-goat-pp-cli bag-life 7 --agent
  coffee-goat-pp-cli bag-life sey/banko-gotiti --method v60
  coffee-goat-pp-cli bag-life 7 --min-brews 4`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			report, err := buildBagLifeReport(db, args[0], methodFilter)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "bag-life for %s (%d brews):\n", report.BeanLabel, report.Fit.N)
			for _, p := range report.Series {
				fmt.Fprintf(cmd.OutOrStdout(), "  day %d  %s  rating=%.1f  (%s)\n", p.DaysSinceRoast, p.Method, p.Rating, p.BrewedAt)
			}
			if report.Fit.N >= 2 && report.Fit.N >= minBrews {
				fmt.Fprintf(cmd.OutOrStdout(), "  fit: slope=%+.3f rating/day  R²=%.2f  %s\n", report.Fit.Slope, report.Fit.R2, report.Fit.Verdict)
				if report.BestDay > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  best so far: day %d rating %.1f\n", report.BestDay, report.BestRating)
				}
			} else if minBrews > 0 && report.Fit.N < minBrews {
				fmt.Fprintf(cmd.OutOrStdout(), "  fit suppressed: %d brews < --min-brews %d\n", report.Fit.N, minBrews)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&methodFilter, "method", "", "Restrict the rating curve to a single brew method like v60 espresso or aeropress")
	cmd.Flags().IntVar(&minBrews, "min-brews", 0, "Suppress the OLS fit until this many brews are present in the per-bag rating curve")
	return cmd
}

func buildBagLifeReport(db *store.Store, target, methodFilter string) (bagLifeReport, error) {
	roasterSlug, productSlug, label, beanID, err := resolveBeanTarget(db, target)
	if err != nil {
		return bagLifeReport{}, err
	}
	rows, err := loadRatingByAge(db, methodFilter, roasterSlug, productSlug)
	if err != nil {
		return bagLifeReport{}, err
	}
	report := bagLifeReport{BeanLabel: label, BeanID: beanID}
	for _, r := range rows {
		report.Series = append(report.Series, bagLifePoint{
			DaysSinceRoast: int(r.Days),
			Method:         r.Method,
			Rating:         r.Rating,
		})
		if r.Rating > report.BestRating {
			report.BestRating = r.Rating
			report.BestDay = int(r.Days)
		}
	}
	sort.Slice(report.Series, func(i, j int) bool { return report.Series[i].DaysSinceRoast < report.Series[j].DaysSinceRoast })
	report.Fit = fitDriftSeries(rows)
	report.Fit.Segment = label
	report.Fit.BeanLabel = label
	return report, nil
}

func resolveBeanTarget(db *store.Store, target string) (roasterSlug, productSlug, label string, beanID int64, err error) {
	if id, perr := strconv.ParseInt(strings.TrimSpace(target), 10, 64); perr == nil && id > 0 {
		var r, p string
		row := db.DB().QueryRow(`SELECT COALESCE(roaster_slug,''), COALESCE(product_slug,'') FROM beans WHERE id = ?`, id)
		if scanErr := row.Scan(&r, &p); scanErr == nil {
			roasterSlug, productSlug, beanID = r, p, id
			label = fmt.Sprintf("bean#%d (%s/%s)", id, r, p)
			return
		}
	}
	r, h := splitRoasterHandle(target)
	q := `SELECT COALESCE(roaster_slug,''), COALESCE(handle,''), COALESCE(title,'')
	      FROM roaster_products WHERE LOWER(handle) = LOWER(?)`
	args := []any{h}
	if r != "" {
		q += ` AND LOWER(roaster_slug) = LOWER(?)`
		args = append(args, r)
	}
	q += ` LIMIT 1`
	var title string
	row := db.DB().QueryRow(q, args...)
	if scanErr := row.Scan(&roasterSlug, &productSlug, &title); scanErr != nil {
		err = notFoundErr(fmt.Errorf("bean %q not found", target))
		return
	}
	label = roasterSlug + "/" + productSlug
	if title != "" {
		label += " (" + title + ")"
	}
	_ = db.DB().QueryRow(`SELECT COALESCE(id,0) FROM beans WHERE roaster_slug = ? AND product_slug = ? ORDER BY added_at DESC LIMIT 1`, roasterSlug, productSlug).Scan(&beanID)
	return
}
