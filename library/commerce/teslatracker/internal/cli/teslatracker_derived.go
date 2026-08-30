// Hand-authored transcendence commands. Own file so `generate --force` keeps it whole.
//
// pp:data-source local
//
// Shared contract for everything here:
//   - reads the local mirror only; run `sync` then `hydrate` first
//   - money is landed cost (price + real per-car transport fee), never sticker
//   - every derived number ships with its arithmetic and its n
//   - a missing input field stays missing; it is never coerced to zero

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// minCohort is the floor below which a percentile is not meaningful. Under it the
// commands say "insufficient" rather than returning a confident wrong number.
const minCohort = 5

// mirrorGuard resolves the db path and reports the sync hint when no mirror exists.
// Returns ok=false when the caller should return nil immediately.
func mirrorGuard(cmd *cobra.Command, flags *rootFlags, dbPath *string) bool {
	if *dbPath == "" {
		*dbPath = defaultDBPath("teslatracker-pp-cli")
	}
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no local mirror at %s\nrun: teslatracker-pp-cli sync && teslatracker-pp-cli hydrate\n", *dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return false
	}
	return true
}

func emit(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

func pctRank(vals []float64, x float64) float64 {
	if len(vals) == 0 {
		return math.NaN()
	}
	below := 0
	for _, v := range vals {
		if v < x {
			below++
		}
	}
	return float64(below) / float64(len(vals)) * 100
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return math.NaN()
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// ── warranty ────────────────────────────────────────────────────────────────

type warrantyLimit struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // "time" | "mileage"
	Remaining *float64 `json:"remaining"`
	Unit      string   `json:"unit"`
	Basis     string   `json:"basis"`
}

func newWarrantyCmd(flags *rootFlags) *cobra.Command {
	var dbPath, delivery string
	var annualMiles int

	cmd := &cobra.Command{
		Use:   "warranty [vin]",
		Short: "See exactly how many months and miles of Tesla warranty are left on the day you would take delivery",
		Long: "Reach for this when the question is coverage or risk on ONE vehicle. Not for pricing — " +
			"use `comps`. Not for battery condition — use `degradation`; warranty months remaining and " +
			"range delta are unrelated measures. Expiry values are as published by the source; warranty " +
			"transferability is not in the data and is not asserted here.",
		Example:     "  teslatracker-pp-cli warranty 5YJ3E1EA7LF745758 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compute remaining warranty for a stored vehicle")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a VIN is required"))
			}
			vin := strings.ToUpper(args[0])
			if !mirrorGuard(cmd, flags, &dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openMirrorRO(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()

			v, err := loadVehicle(ctx, st.DB(), vin)
			if err != nil {
				return err
			}
			if v == nil {
				all, _ := loadVehicles(ctx, st.DB())
				if len(all) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"local mirror has no hydrated vehicles\nrun: teslatracker-pp-cli sync && teslatracker-pp-cli hydrate\n")
					return emit(cmd, flags, map[string]any{"vin": vin,
						"note": "no hydrated vehicles in the local mirror; run sync then hydrate"})
				}
				return notFoundErr(fmt.Errorf("VIN %s is not in the local mirror; run hydrate", vin))
			}

			at := time.Now().UTC()
			if delivery != "" {
				parsed, perr := time.Parse("2006-01-02", delivery)
				if perr != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--delivery must be YYYY-MM-DD"))
				}
				at = parsed
			}

			limits := []warrantyLimit{}
			addTime := func(name, iso string) {
				if iso == "" {
					limits = append(limits, warrantyLimit{Name: name, Kind: "time", Remaining: nil,
						Unit: "months", Basis: "not published by the source"})
					return
				}
				exp, perr := time.Parse(time.RFC3339, iso)
				if perr != nil {
					limits = append(limits, warrantyLimit{Name: name, Kind: "time", Remaining: nil,
						Unit: "months", Basis: "unparseable expiry: " + iso})
					return
				}
				// One decimal. The inputs are a calendar date and a 30.44-day
				// average month, so any further digits are false precision.
				months := math.Round(exp.Sub(at).Hours()/24/30.44*10) / 10
				limits = append(limits, warrantyLimit{Name: name, Kind: "time", Remaining: &months,
					Unit: "months", Basis: fmt.Sprintf("%s minus %s", exp.Format("2006-01-02"), at.Format("2006-01-02"))})
			}
			addMiles := func(name string, cap *int) {
				if cap == nil || v.Mileage == nil {
					why := "warranty mileage cap not published"
					if v.Mileage == nil {
						why = "odometer not published"
					}
					limits = append(limits, warrantyLimit{Name: name, Kind: "mileage", Remaining: nil,
						Unit: "miles", Basis: why})
					return
				}
				// project the odometer forward to the delivery date
				days := at.Sub(time.Now().UTC()).Hours() / 24
				if days < 0 {
					days = 0
				}
				projected := math.Round(float64(*v.Mileage) + (days/365.25)*float64(annualMiles))
				rem := float64(*cap) - projected
				limits = append(limits, warrantyLimit{Name: name, Kind: "mileage", Remaining: &rem,
					Unit: "miles", Basis: fmt.Sprintf("%d cap minus %.0f projected odometer", *cap, projected)})
			}
			addTime("battery & drive unit (time)", v.WarrantyBatteryExpDate)
			addMiles("battery & drive unit (mileage)", v.WarrantyBatteryMile)
			addTime("vehicle (time)", v.WarrantyVehicleExpDate)
			addMiles("vehicle (mileage)", v.WarrantyVehicleMile)

			// The binding limit is the soonest limit that has NOT already lapsed.
			// An expired limit is reported separately: on an older car the basic
			// vehicle warranty is routinely long gone, and letting it win "binding"
			// buries the live constraint (e.g. battery cover expiring in weeks).
			binding := ""
			bindingYears := math.Inf(1)
			expired := []string{}
			for _, l := range limits {
				if l.Remaining == nil {
					continue
				}
				if *l.Remaining <= 0 {
					expired = append(expired, l.Name)
					continue
				}
				var years float64
				if l.Kind == "time" {
					years = *l.Remaining / 12
				} else if annualMiles > 0 {
					years = *l.Remaining / float64(annualMiles)
				} else {
					continue
				}
				if years < bindingYears {
					bindingYears, binding = years, l.Name
				}
			}
			view := map[string]any{
				"vin": v.VIN, "year": v.Year, "model": v.Model, "trim": v.Trim,
				"odometer": v.Mileage, "delivery_date": at.Format("2006-01-02"),
				"annual_miles_assumed": annualMiles, "limits": limits,
			}
			if binding != "" {
				view["binding_limit"] = binding
				view["binding_years_remaining"] = math.Round(bindingYears*100) / 100
			} else {
				view["note"] = "no unexpired warranty limit remains"
			}
			if len(expired) > 0 {
				view["already_expired"] = expired
			}
			view["caveat"] = "expiry values are as published by the source; transferability is not in the data"

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d %s %s  VIN %s\n", derefInt(v.Year), v.Model, v.Trim, v.VIN)
			for _, l := range limits {
				if l.Remaining == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-32s  —      (%s)\n", l.Name, l.Basis)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-32s  %8.0f %-7s (%s)\n", l.Name, *l.Remaining, l.Unit, l.Basis)
			}
			if len(expired) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nalready expired: %s\n", strings.Join(expired, ", "))
			}
			if binding != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "binding limit: %s — about %.1f years at %d mi/yr\n",
					binding, bindingYears, annualMiles)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "no unexpired warranty limit remains")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&delivery, "delivery", "", "Delivery date, YYYY-MM-DD (default today)")
	cmd.Flags().IntVar(&annualMiles, "annual-miles", 12000, "Your expected annual mileage")
	return cmd
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// ── degradation ─────────────────────────────────────────────────────────────

func newDegradationCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "degradation [vin]",
		Short: "Compare a car's actual range against its rated range, and place that gap in its cohort",
		Long: "Reach for this when the question is battery or range condition. With a VIN it returns that " +
			"car's delta and its percentile within the cohort; without one it returns the cohort curve. " +
			"CRITICAL: this is the source's published rated-vs-actual spread, NOT a measured battery " +
			"health test. Do not report it to a user as verified degradation. Sibling: `warranty` covers " +
			"contractual battery coverage; this covers observed range.",
		Example:     "  teslatracker-pp-cli degradation 5YJ3E1EA7LF745758 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compute rated-vs-actual range placement")
			}
			if !mirrorGuard(cmd, flags, &dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openMirrorRO(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()

			all, err := loadVehicles(ctx, st.DB())
			if err != nil {
				return err
			}
			type row struct {
				VIN     string   `json:"vin"`
				Year    *int     `json:"year"`
				Model   string   `json:"model"`
				Rated   *int     `json:"rated_range"`
				Actual  *int     `json:"actual_range"`
				Mileage *int     `json:"mileage"`
				Pct     *float64 `json:"retained_pct"`
			}
			mk := func(v Vehicle) row {
				r := row{VIN: v.VIN, Year: v.Year, Model: v.Model,
					Rated: v.Range, Actual: v.ActualRange, Mileage: v.Mileage}
				if v.Range != nil && v.ActualRange != nil && *v.Range > 0 {
					p := float64(*v.ActualRange) / float64(*v.Range) * 100
					r.Pct = &p
				}
				return r
			}

			if len(args) == 0 {
				rows := make([]row, 0, len(all))
				vals := []float64{}
				for _, v := range all {
					r := mk(v)
					rows = append(rows, r)
					if r.Pct != nil {
						vals = append(vals, *r.Pct)
					}
				}
				sort.Float64s(vals)
				sort.Slice(rows, func(i, j int) bool {
					if rows[i].Pct == nil {
						return false
					}
					if rows[j].Pct == nil {
						return true
					}
					return *rows[i].Pct < *rows[j].Pct
				})
				view := map[string]any{
					"n": len(vals), "cars": rows,
					"caveat": "source-published rated-vs-actual spread, not a measured battery test",
				}
				if len(vals) >= minCohort {
					view["median_retained_pct"] = math.Round(median(vals)*10) / 10
					view["worst_retained_pct"] = math.Round(vals[0]*10) / 10
					view["best_retained_pct"] = math.Round(vals[len(vals)-1]*10) / 10
				} else {
					view["cohort_note"] = fmt.Sprintf(
						"cohort has %d cars, below the floor of %d — no aggregate statistics reported", len(vals), minCohort)
				}
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return emit(cmd, flags, view)
				}
				if len(vals) >= minCohort {
					fmt.Fprintf(cmd.OutOrStdout(), "cohort of %d cars — median %.1f%% of rated range retained\n",
						len(vals), median(vals))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(),
						"cohort of %d cars, below the floor of %d — no aggregate statistics reported\n",
						len(vals), minCohort)
				}
				for _, r := range rows {
					if r.Pct == nil {
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  %d %-8s %3d/%3d mi  %5.1f%%  %6d mi odo\n",
						r.VIN, derefInt(r.Year), r.Model, derefInt(r.Actual), derefInt(r.Rated), *r.Pct, derefInt(r.Mileage))
				}
				return nil
			}

			vin := strings.ToUpper(args[0])
			var target *Vehicle
			for i := range all {
				if all[i].VIN == vin {
					target = &all[i]
					break
				}
			}
			if target == nil {
				if len(all) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"local mirror has no hydrated vehicles\nrun: teslatracker-pp-cli sync && teslatracker-pp-cli hydrate\n")
					return emit(cmd, flags, map[string]any{"vin": vin,
						"note": "no hydrated vehicles in the local mirror; run sync then hydrate"})
				}
				return notFoundErr(fmt.Errorf("VIN %s is not in the local mirror; run hydrate", vin))
			}
			tr := mk(*target)
			if tr.Pct == nil {
				return emit(cmd, flags, map[string]any{
					"vin": vin, "note": "range or actualRange not published for this vehicle; no delta computable",
				})
			}
			// cohort: same model, year within +/-1
			vals := []float64{}
			for _, v := range all {
				if v.VIN == vin || !strings.EqualFold(v.Model, target.Model) {
					continue
				}
				if v.Year != nil && target.Year != nil && math.Abs(float64(*v.Year-*target.Year)) > 1 {
					continue
				}
				r := mk(v)
				if r.Pct != nil {
					vals = append(vals, *r.Pct)
				}
			}
			sort.Float64s(vals)
			view := map[string]any{
				"vin": vin, "year": target.Year, "model": target.Model, "trim": target.Trim,
				"rated_range": target.Range, "actual_range": target.ActualRange,
				"retained_pct": math.Round(*tr.Pct*10) / 10,
				"odometer":     target.Mileage,
				"cohort":       map[string]any{"definition": "same model, year +/-1", "n": len(vals)},
				"caveat":       "source-published rated-vs-actual spread, not a measured battery test",
			}
			if len(vals) >= minCohort {
				view["cohort_median_retained_pct"] = math.Round(median(vals)*10) / 10
				view["percentile_in_cohort"] = math.Round(pctRank(vals, *tr.Pct))
			} else {
				view["cohort_note"] = fmt.Sprintf(
					"cohort has %d cars, below the floor of %d — no percentile reported", len(vals), minCohort)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %d %s %s\n", vin, derefInt(target.Year), target.Model, target.Trim)
			fmt.Fprintf(cmd.OutOrStdout(), "  %d of %d rated miles = %.1f%% retained (%d mi odometer)\n",
				derefInt(target.ActualRange), derefInt(target.Range), *tr.Pct, derefInt(target.Mileage))
			if len(vals) >= minCohort {
				fmt.Fprintf(cmd.OutOrStdout(), "  cohort n=%d, median %.1f%% — this car is at the %.0fth percentile\n",
					len(vals), median(vals), pctRank(vals, *tr.Pct))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  cohort n=%d, below the floor of %d — no percentile\n", len(vals), minCohort)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  note: source-published spread, not a measured battery test")
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ── comps ───────────────────────────────────────────────────────────────────

func newCompsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var mileageBand int

	cmd := &cobra.Command{
		Use:   "comps [vin]",
		Short: "See where one car's landed cost sits in the distribution of comparable cars",
		Long: "Reach for this to answer \"is this price good\". Returns placement and arithmetic, never a " +
			"verdict. Use `premium` instead when the question is what a configuration attribute costs " +
			"across the market; use `price-history` for one car's own trajectory. When the cohort is " +
			"below the sample floor the command says so explicitly — treat that as unknown, never as average.",
		Example:     "  teslatracker-pp-cli comps 5YJ3E1EA7LF745758 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compute cohort price placement")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a VIN is required"))
			}
			vin := strings.ToUpper(args[0])
			if !mirrorGuard(cmd, flags, &dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openMirrorRO(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()

			all, err := loadVehicles(ctx, st.DB())
			if err != nil {
				return err
			}
			var target *Vehicle
			for i := range all {
				if all[i].VIN == vin {
					target = &all[i]
					break
				}
			}
			if target == nil {
				if len(all) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"local mirror has no hydrated vehicles\nrun: teslatracker-pp-cli sync && teslatracker-pp-cli hydrate\n")
					return emit(cmd, flags, map[string]any{"vin": vin,
						"note": "no hydrated vehicles in the local mirror; run sync then hydrate"})
				}
				return notFoundErr(fmt.Errorf("VIN %s is not in the local mirror; run hydrate", vin))
			}
			tl := target.LandedCents()
			if tl == nil {
				return emit(cmd, flags, map[string]any{
					"vin": vin, "note": "no price published for this vehicle; no placement computable"})
			}

			peers := []float64{}
			for _, v := range all {
				if v.VIN == vin || !strings.EqualFold(v.Model, target.Model) {
					continue
				}
				if v.Year != nil && target.Year != nil && math.Abs(float64(*v.Year-*target.Year)) > 1 {
					continue
				}
				if mileageBand > 0 && v.Mileage != nil && target.Mileage != nil &&
					math.Abs(float64(*v.Mileage-*target.Mileage)) > float64(mileageBand) {
					continue
				}
				if l := v.LandedCents(); l != nil {
					peers = append(peers, float64(*l)/100)
				}
			}
			sort.Float64s(peers)
			targetUSD := float64(*tl) / 100

			view := map[string]any{
				"vin": vin, "year": target.Year, "model": target.Model, "trim": target.Trim,
				"odometer":       target.Mileage,
				"sticker_usd":    dollars(target.PurchasePriceCents),
				"transport_usd":  dollars(target.TransportFeeCents),
				"landed_usd":     math.Round(targetUSD),
				"cohort":         map[string]any{"definition": fmt.Sprintf("same model, year +/-1, mileage band +/-%d", mileageBand), "n": len(peers)},
				"landed_formula": "landed = totalPrice (or purchasePrice) + transportFee, converted from cents",
			}
			if len(peers) >= minCohort {
				med := median(peers)
				view["cohort_median_landed_usd"] = math.Round(med)
				view["gap_to_median_usd"] = math.Round(targetUSD - med)
				view["percentile_in_cohort"] = math.Round(pctRank(peers, targetUSD))
			} else {
				view["cohort_note"] = fmt.Sprintf(
					"cohort has %d cars, below the floor of %d — no percentile reported. Widen with --mileage-band or sync more pages.",
					len(peers), minCohort)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %d %s %s\n", vin, derefInt(target.Year), target.Model, target.Trim)
			fmt.Fprintf(cmd.OutOrStdout(), "  sticker $%.0f + transport $%.0f = landed $%.0f\n",
				derefF(dollars(target.PurchasePriceCents)), derefF(dollars(target.TransportFeeCents)), targetUSD)
			if len(peers) >= minCohort {
				med := median(peers)
				fmt.Fprintf(cmd.OutOrStdout(), "  cohort n=%d, median landed $%.0f — this car is $%.0f %s median, %.0fth percentile\n",
					len(peers), med, math.Abs(targetUSD-med),
					map[bool]string{true: "above", false: "below"}[targetUSD > med], pctRank(peers, targetUSD))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  cohort n=%d, below the floor of %d — no percentile reported\n", len(peers), minCohort)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&mileageBand, "mileage-band", 25000, "Cohort mileage window, +/- miles (0 = ignore)")
	return cmd
}

func derefF(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

var _ = sql.ErrNoRows
