// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// budgetSummary is the top-level shape for the default `budget` output.
type budgetSummary struct {
	Currency             string             `json:"currency"`
	PeriodStart          string             `json:"period_start"`
	PeriodEnd            string             `json:"period_end"`
	YTDSpentCents        int                `json:"ytd_spent_cents"`
	ThisMonthSpentCents  int                `json:"this_month_spent_cents"`
	AvgCostPerBrewCents  int                `json:"avg_cost_per_brew_cents"`
	ProjectionLinearC    int                `json:"projection_linear_cents"`
	ProjectionTrailing3C int                `json:"projection_trailing3_cents"`
	ProjectionRange      string             `json:"projection_range"`
	TargetCents          int                `json:"target_cents,omitempty"`
	TargetDeltaCents     int                `json:"target_delta_cents,omitempty"`
	ExcludedBrews        int                `json:"excluded_brews,omitempty"`
	ExcludedBags         int                `json:"excluded_bags,omitempty"`
	TotalBrews           int                `json:"total_brews"`
	TotalBags            int                `json:"total_bags"`
	MonthlyBreakdown     []budgetMonthRow   `json:"monthly_breakdown,omitempty"`
	RoasterBreakdown     []budgetRoasterRow `json:"roaster_breakdown,omitempty"`
	BagBreakdown         []budgetBagRow     `json:"bag_breakdown,omitempty"`
	MethodBreakdown      []budgetMethodRow  `json:"method_breakdown,omitempty"`
	IncludeShipping      bool               `json:"include_shipping,omitempty"`
	TrailingWindow       string             `json:"trailing_window,omitempty"`
}

type budgetMonthRow struct {
	Month       string `json:"month"`
	SpentCents  int    `json:"spent_cents"`
	BrewCount   int    `json:"brew_count"`
	AvgCostCent int    `json:"avg_cost_per_brew_cents"`
}

type budgetRoasterRow struct {
	Roaster      string `json:"roaster"`
	SpentCents   int    `json:"spent_cents"`
	BagCount     int    `json:"bag_count"`
	BrewCount    int    `json:"brew_count"`
	SharePercent int    `json:"share_percent"`
}

type budgetBagRow struct {
	BeanID           int64  `json:"bean_id"`
	Bean             string `json:"bean"`
	BagPriceCents    int    `json:"bag_price_cents"`
	BrewCount        int    `json:"brew_count"`
	TotalDoseG       int    `json:"total_dose_g"`
	BagSizeG         int    `json:"bag_size_g"`
	CostPerBrewCents int    `json:"cost_per_brew_cents"`
	TotalSpentCents  int    `json:"total_spent_cents"`
	PctConsumed      int    `json:"pct_consumed,omitempty"`
}

type budgetMethodRow struct {
	Method              string `json:"method"`
	BrewCount           int    `json:"brew_count"`
	TotalSpentCents     int    `json:"total_spent_cents"`
	AvgCostPerBrewCents int    `json:"avg_cost_per_brew_cents"`
}

// brewAttribution is the per-brew computed cost row used by all
// downstream aggregations. dose_g / bag_size_g × bag_price_cents.
type brewAttribution struct {
	BrewID      int64
	BeanID      int64
	RoasterSlug string
	ProductSlug string
	Method      string
	DoseG       float64
	BagSizeG    int
	PriceCents  int
	CostCents   float64
	BrewedAt    time.Time
}

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	var (
		by              string
		currency        string
		includeShipping bool
		trailing30      bool
		targetAmount    float64
		sinceISO        string
	)
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Monthly coffee spend, cost-per-cup, and annual projection (dose-grams attribution)",
		Long: `Computes spend with dose-grams attribution: each brew is charged
(brew.dose_g / bag_size_g) × bag_price_cents.

Bag size comes from roaster_products.weight_g (joined via beans), falling
back to beans.current_mass_g when the join misses; bag price comes from
beans.price_paid_cents (assumed USD by convention).

Default output: summary line, YTD, this-month, projection range
(linear extrapolation vs trailing-3-month × 12), average cost-per-brew.

Pivot with --by month|roaster|bag|method to drill into a slice. --by bag
ranks by cost-per-cup descending with total-spent as a secondary column.

Brews with missing dose_g and beans with missing price/bag-size are
excluded; counts are reported on stderr (and in JSON) so totals never lie.`,
		Example: `  coffee-goat-pp-cli budget --agent
  coffee-goat-pp-cli budget --by bag --limit 10
  coffee-goat-pp-cli budget --by month --include-shipping
  coffee-goat-pp-cli budget --target 75`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			currency = strings.ToUpper(strings.TrimSpace(currency))
			if currency == "" {
				currency = "USD"
			}
			if _, ok := curatedFXRates[currency]; !ok {
				return usageErr(fmt.Errorf("unsupported currency %q (supported: USD, EUR, GBP, DKK, JPY, AUD, CAD)", currency))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			attrs, excludedBrews, excludedBags, err := loadBrewAttributions(db, includeShipping)
			if err != nil {
				return err
			}
			// Convert USD-stored costs to target currency.
			fxFactor := 1.0
			if currency != "USD" {
				fxFactor = curatedFXRates["USD"] / curatedFXRates[currency]
			}
			summary := summarizeBudget(attrs, currency, fxFactor, trailing30, targetAmount)
			summary.ExcludedBrews = excludedBrews
			summary.ExcludedBags = excludedBags
			summary.IncludeShipping = includeShipping
			if trailing30 {
				summary.TrailingWindow = "30d"
			}
			switch strings.ToLower(strings.TrimSpace(by)) {
			case "month":
				summary.MonthlyBreakdown = aggregateByMonth(attrs, fxFactor)
			case "roaster":
				summary.RoasterBreakdown = aggregateByRoaster(attrs, fxFactor)
			case "bag":
				summary.BagBreakdown = aggregateByBag(attrs, fxFactor)
			case "method":
				summary.MethodBreakdown = aggregateByMethod(attrs, fxFactor)
			case "":
				// default: balanced summary, no detail table
			default:
				return usageErr(fmt.Errorf("--by must be one of: month, roaster, bag, method (got %q)", by))
			}
			emitBudgetExclusionWarnings(excludedBrews, excludedBags)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			renderBudgetHuman(cmd, summary)
			_ = sinceISO // future: --since support
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "Pivot the detail table: month, roaster, bag, or method")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Output currency (USD, EUR, GBP, DKK, JPY, AUD, CAD); rolls up via fx rates")
	cmd.Flags().BoolVar(&includeShipping, "include-shipping", false, "Add per-bag amortized shipping from fx's curated table (1 bag/order worst-case)")
	cmd.Flags().BoolVar(&trailing30, "trailing-30", false, "Use a trailing 30-day window instead of YTD")
	cmd.Flags().Float64Var(&targetAmount, "target", 0, "Monthly spend target in the output currency; output reports over/under for this month")
	cmd.Flags().StringVar(&sinceISO, "since", "", "Future: restrict to brews after this RFC3339 timestamp")
	return cmd
}

// loadBrewAttributions returns the per-brew cost rows for budget
// aggregation. Brews with missing dose_g or beans with missing price /
// bag-size are excluded; the exclusion counts are returned alongside
// the attribution slice so callers can surface them.
func loadBrewAttributions(db *store.Store, includeShipping bool) ([]brewAttribution, int, int, error) {
	q := `SELECT b.id, COALESCE(b.bean_id,0), COALESCE(b.method,''), COALESCE(b.dose_g,0), COALESCE(b.brewed_at,''),
	             COALESCE(bn.roaster_slug,''), COALESCE(bn.product_slug,''),
	             COALESCE(bn.price_paid_cents,0), COALESCE(bn.current_mass_g,0),
	             COALESCE(rp.weight_g,0)
	      FROM brews b
	      LEFT JOIN beans bn ON b.bean_id = bn.id
	      LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle`
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	excludedBrews := 0
	excludedBagSet := map[int64]bool{}
	var out []brewAttribution
	for rows.Next() {
		var a brewAttribution
		var rpWeight int
		var brewedAt string
		if err := rows.Scan(&a.BrewID, &a.BeanID, &a.Method, &a.DoseG, &brewedAt,
			&a.RoasterSlug, &a.ProductSlug, &a.PriceCents, &a.BagSizeG, &rpWeight); err != nil {
			return nil, 0, 0, err
		}
		// Prefer roaster_products.weight_g (true bag size) over beans.current_mass_g.
		if rpWeight > 0 {
			a.BagSizeG = rpWeight
		}
		if a.DoseG <= 0 {
			excludedBrews++
			continue
		}
		if a.PriceCents <= 0 || a.BagSizeG <= 0 {
			if a.BeanID > 0 {
				excludedBagSet[a.BeanID] = true
			}
			excludedBrews++
			continue
		}
		if t, ok := parseBrewedAt(brewedAt); ok {
			a.BrewedAt = t
		} else {
			excludedBrews++
			continue
		}
		bagPrice := a.PriceCents
		if includeShipping {
			bagPrice += lookupRoasterShipping(a.RoasterSlug, "us-domestic")
		}
		a.PriceCents = bagPrice
		a.CostCents = float64(bagPrice) * (a.DoseG / float64(a.BagSizeG))
		out = append(out, a)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, 0, 0, err
	}
	return out, excludedBrews, len(excludedBagSet), nil
}

func parseBrewedAt(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// summarizeBudget computes the top-level summary fields. fxFactor scales
// USD-stored cents into the target currency at quote time so the rest
// of the math stays in integer-friendly cents.
func summarizeBudget(attrs []brewAttribution, currency string, fxFactor float64, trailing30 bool, targetAmount float64) budgetSummary {
	now := time.Now()
	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	windowStart := yearStart
	if trailing30 {
		windowStart = now.AddDate(0, 0, -30)
	}
	var ytdCents, monthCents, allTimeCents float64
	bagSet := map[int64]bool{}
	var ytdBrews int
	for _, a := range attrs {
		allTimeCents += a.CostCents
		if a.BeanID > 0 {
			bagSet[a.BeanID] = true
		}
		if !a.BrewedAt.Before(windowStart) {
			ytdCents += a.CostCents
			ytdBrews++
		}
		if !a.BrewedAt.Before(monthStart) {
			monthCents += a.CostCents
		}
	}
	// Linear projection from YTD: assumes spend continues at YTD pace.
	daysElapsed := math.Max(1, now.Sub(yearStart).Hours()/24)
	yearDays := float64(daysOfYear(now.Year()))
	linearProjection := 0.0
	if !trailing30 {
		linearProjection = ytdCents * (yearDays / daysElapsed)
	}
	// Trailing-3-month × 12.
	trail3Cents := 0.0
	trail3Start := now.AddDate(0, -3, 0)
	for _, a := range attrs {
		if !a.BrewedAt.Before(trail3Start) {
			trail3Cents += a.CostCents
		}
	}
	trail3Projection := (trail3Cents / 3.0) * 12.0
	avgCostPerBrew := 0.0
	if ytdBrews > 0 {
		avgCostPerBrew = ytdCents / float64(ytdBrews)
	}
	summary := budgetSummary{
		Currency:             currency,
		PeriodStart:          windowStart.Format("2006-01-02"),
		PeriodEnd:            now.Format("2006-01-02"),
		YTDSpentCents:        int(math.Round(ytdCents * fxFactor)),
		ThisMonthSpentCents:  int(math.Round(monthCents * fxFactor)),
		AvgCostPerBrewCents:  int(math.Round(avgCostPerBrew * fxFactor)),
		ProjectionLinearC:    int(math.Round(linearProjection * fxFactor)),
		ProjectionTrailing3C: int(math.Round(trail3Projection * fxFactor)),
		TotalBrews:           len(attrs),
		TotalBags:            len(bagSet),
	}
	if linearProjection > 0 || trail3Projection > 0 {
		lo := linearProjection
		hi := trail3Projection
		if hi < lo {
			lo, hi = hi, lo
		}
		summary.ProjectionRange = fmt.Sprintf("%s%.2f – %s%.2f",
			currencySymbol(currency), lo*fxFactor/100.0,
			currencySymbol(currency), hi*fxFactor/100.0)
	}
	if targetAmount > 0 {
		summary.TargetCents = int(math.Round(targetAmount * 100.0))
		summary.TargetDeltaCents = summary.ThisMonthSpentCents - summary.TargetCents
	}
	return summary
}

// aggregateByMonth groups attributions by calendar month (YYYY-MM)
// descending.
func aggregateByMonth(attrs []brewAttribution, fxFactor float64) []budgetMonthRow {
	type acc struct {
		cents float64
		n     int
	}
	bucket := map[string]*acc{}
	for _, a := range attrs {
		key := a.BrewedAt.Format("2006-01")
		if bucket[key] == nil {
			bucket[key] = &acc{}
		}
		bucket[key].cents += a.CostCents
		bucket[key].n++
	}
	out := make([]budgetMonthRow, 0, len(bucket))
	for k, v := range bucket {
		avg := 0
		if v.n > 0 {
			avg = int(math.Round(v.cents * fxFactor / float64(v.n)))
		}
		out = append(out, budgetMonthRow{
			Month:       k,
			SpentCents:  int(math.Round(v.cents * fxFactor)),
			BrewCount:   v.n,
			AvgCostCent: avg,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Month > out[j].Month })
	return out
}

// aggregateByRoaster groups bag spend by roaster slug.
func aggregateByRoaster(attrs []brewAttribution, fxFactor float64) []budgetRoasterRow {
	type acc struct {
		cents float64
		bags  map[int64]bool
		brews int
	}
	bucket := map[string]*acc{}
	var grandTotal float64
	for _, a := range attrs {
		grandTotal += a.CostCents
		key := a.RoasterSlug
		if key == "" {
			key = "(unknown)"
		}
		if bucket[key] == nil {
			bucket[key] = &acc{bags: map[int64]bool{}}
		}
		bucket[key].cents += a.CostCents
		bucket[key].bags[a.BeanID] = true
		bucket[key].brews++
	}
	out := make([]budgetRoasterRow, 0, len(bucket))
	for k, v := range bucket {
		share := 0
		if grandTotal > 0 {
			share = int(math.Round(v.cents / grandTotal * 100))
		}
		out = append(out, budgetRoasterRow{
			Roaster: k, SpentCents: int(math.Round(v.cents * fxFactor)),
			BagCount: len(v.bags), BrewCount: v.brews,
			SharePercent: share,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SpentCents > out[j].SpentCents })
	return out
}

// aggregateByBag ranks bags by cost-per-brew descending; total-spent
// is shown as a secondary column. Bags brewed multiple times converge
// toward the bag's "true" cost-per-cup; bags brewed once or twice
// surface as expensive.
func aggregateByBag(attrs []brewAttribution, fxFactor float64) []budgetBagRow {
	type acc struct {
		bean       int64
		roaster    string
		product    string
		priceCents int
		bagSizeG   int
		totalDoseG float64
		brewCount  int
		totalSpent float64
	}
	bucket := map[int64]*acc{}
	for _, a := range attrs {
		if a.BeanID == 0 {
			continue
		}
		if bucket[a.BeanID] == nil {
			bucket[a.BeanID] = &acc{
				bean: a.BeanID, roaster: a.RoasterSlug, product: a.ProductSlug,
				priceCents: a.PriceCents, bagSizeG: a.BagSizeG,
			}
		}
		bucket[a.BeanID].totalDoseG += a.DoseG
		bucket[a.BeanID].brewCount++
		bucket[a.BeanID].totalSpent += a.CostCents
	}
	out := make([]budgetBagRow, 0, len(bucket))
	for _, v := range bucket {
		costPerBrew := 0
		if v.brewCount > 0 {
			costPerBrew = int(math.Round(v.totalSpent * fxFactor / float64(v.brewCount)))
		}
		pctConsumed := 0
		if v.bagSizeG > 0 {
			pctConsumed = int(math.Round(v.totalDoseG / float64(v.bagSizeG) * 100))
		}
		label := v.roaster + "/" + v.product
		if v.roaster == "" || v.product == "" {
			label = fmt.Sprintf("bean#%d", v.bean)
		}
		out = append(out, budgetBagRow{
			BeanID: v.bean, Bean: label,
			BagPriceCents:    int(math.Round(float64(v.priceCents) * fxFactor)),
			BrewCount:        v.brewCount,
			TotalDoseG:       int(math.Round(v.totalDoseG)),
			BagSizeG:         v.bagSizeG,
			CostPerBrewCents: costPerBrew,
			TotalSpentCents:  int(math.Round(v.totalSpent * fxFactor)),
			PctConsumed:      pctConsumed,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CostPerBrewCents > out[j].CostPerBrewCents })
	return out
}

// aggregateByMethod groups by brew method (v60, espresso, aeropress, ...).
func aggregateByMethod(attrs []brewAttribution, fxFactor float64) []budgetMethodRow {
	type acc struct {
		cents float64
		n     int
	}
	bucket := map[string]*acc{}
	for _, a := range attrs {
		key := a.Method
		if key == "" {
			key = "(unknown)"
		}
		if bucket[key] == nil {
			bucket[key] = &acc{}
		}
		bucket[key].cents += a.CostCents
		bucket[key].n++
	}
	out := make([]budgetMethodRow, 0, len(bucket))
	for k, v := range bucket {
		avg := 0
		if v.n > 0 {
			avg = int(math.Round(v.cents * fxFactor / float64(v.n)))
		}
		out = append(out, budgetMethodRow{
			Method:              k,
			BrewCount:           v.n,
			TotalSpentCents:     int(math.Round(v.cents * fxFactor)),
			AvgCostPerBrewCents: avg,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AvgCostPerBrewCents > out[j].AvgCostPerBrewCents })
	return out
}

// daysOfYear returns 365 or 366 for leap years.
func daysOfYear(year int) int {
	if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
		return 366
	}
	return 365
}

// currencySymbol returns a short prefix for human output. JPY uses ¥;
// EUR uses €; everything else falls back to a 3-letter code prefix.
func currencySymbol(c string) string {
	switch c {
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	default:
		return c + " "
	}
}

// emitBudgetExclusionWarnings prints a stderr summary when any brews or
// bags were excluded. Suppressed when there were none.
func emitBudgetExclusionWarnings(excludedBrews, excludedBags int) {
	if excludedBrews == 0 && excludedBags == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "→ excluded: %d brews (no dose_g), %d bags (no price/size)\n", excludedBrews, excludedBags)
}

// renderBudgetHuman prints the human-readable summary + optional detail.
func renderBudgetHuman(cmd *cobra.Command, s budgetSummary) {
	w := cmd.OutOrStdout()
	sym := currencySymbol(s.Currency)
	fmt.Fprintf(w, "→ Budget summary — %s (%s..%s)\n", s.Currency, s.PeriodStart, s.PeriodEnd)
	fmt.Fprintf(w, "  YTD spent:           %s%.2f\n", sym, float64(s.YTDSpentCents)/100.0)
	fmt.Fprintf(w, "  This month:          %s%.2f\n", sym, float64(s.ThisMonthSpentCents)/100.0)
	if s.TargetCents > 0 {
		delta := float64(s.TargetDeltaCents) / 100.0
		sign := "under"
		if delta > 0 {
			sign = "over"
		}
		fmt.Fprintf(w, "  Target:              %s%.2f  (%s%.2f %s target)\n",
			sym, float64(s.TargetCents)/100.0, sym, math.Abs(delta), sign)
	}
	fmt.Fprintf(w, "  Avg cost-per-brew:   %s%.2f\n", sym, float64(s.AvgCostPerBrewCents)/100.0)
	if s.ProjectionRange != "" {
		fmt.Fprintf(w, "  Projection range:    %s\n", s.ProjectionRange)
		fmt.Fprintf(w, "    linear:            %s%.2f\n", sym, float64(s.ProjectionLinearC)/100.0)
		fmt.Fprintf(w, "    trailing-3mo×12:   %s%.2f\n", sym, float64(s.ProjectionTrailing3C)/100.0)
	}
	fmt.Fprintf(w, "  Brews counted:       %d across %d bags\n", s.TotalBrews, s.TotalBags)
	if len(s.MonthlyBreakdown) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Spend by month:")
		for _, m := range s.MonthlyBreakdown {
			fmt.Fprintf(w, "  %s  %s%-9.2f  %d brews  (%s%.2f/brew)\n",
				m.Month, sym, float64(m.SpentCents)/100.0, m.BrewCount,
				sym, float64(m.AvgCostCent)/100.0)
		}
	}
	if len(s.RoasterBreakdown) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Spend by roaster:")
		for _, r := range s.RoasterBreakdown {
			fmt.Fprintf(w, "  %-22s  %s%-9.2f  %d bags  %d brews  (%d%%)\n",
				r.Roaster, sym, float64(r.SpentCents)/100.0,
				r.BagCount, r.BrewCount, r.SharePercent)
		}
	}
	if len(s.BagBreakdown) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Bags by cost-per-cup (most expensive first):")
		for _, b := range s.BagBreakdown {
			fmt.Fprintf(w, "  #%d  %-30s  %s%.2f/brew  %d brews  %s%.2f total\n",
				b.BeanID, b.Bean, sym, float64(b.CostPerBrewCents)/100.0,
				b.BrewCount, sym, float64(b.TotalSpentCents)/100.0)
		}
	}
	if len(s.MethodBreakdown) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Spend by method:")
		for _, m := range s.MethodBreakdown {
			fmt.Fprintf(w, "  %-14s  %s%.2f/brew  %d brews  %s%.2f total\n",
				m.Method, sym, float64(m.AvgCostPerBrewCents)/100.0,
				m.BrewCount, sym, float64(m.TotalSpentCents)/100.0)
		}
	}
}
