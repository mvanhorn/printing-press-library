// Hand-authored. Own file so `generate --force` keeps it whole.
// Time-series commands: price-history, gone, stale.
//
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type pricePoint struct {
	LandedUSD float64 `json:"landed_usd"`
	At        string  `json:"at"`
	Source    string  `json:"source"` // "api" | "local"
}

// pricePath merges the API's own priceHistory with our local snapshots.
// The API series vanishes when a listing is pulled; the local one survives,
// which is what `gone` depends on.
func pricePath(ctx context.Context, db *sql.DB, v Vehicle) []pricePoint {
	var pts []pricePoint
	fee := int64(0)
	if v.TransportFeeCents != nil {
		fee = *v.TransportFeeCents
	}
	for _, h := range v.PriceHistory {
		if h.Price == nil {
			continue
		}
		pts = append(pts, pricePoint{
			LandedUSD: float64(*h.Price+fee) / 100,
			At:        h.ScrapedAt,
			Source:    "api",
		})
	}
	rows, err := db.QueryContext(ctx,
		`SELECT landed_cents, observed_at FROM price_snapshot WHERE vin = ? ORDER BY observed_at`, v.VIN)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cents sql.NullInt64
			var at sql.NullString
			if rows.Scan(&cents, &at) != nil || !cents.Valid {
				continue
			}
			pts = append(pts, pricePoint{LandedUSD: float64(cents.Int64) / 100, At: at.String, Source: "local"})
		}
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].At < pts[j].At })
	return pts
}

func daysListed(v Vehicle) *float64 {
	if v.FirstSeenAt == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v.FirstSeenAt)
	if err != nil {
		return nil
	}
	d := time.Since(t).Hours() / 24
	d = math.Round(d*10) / 10
	return &d
}

// landedCut is one observed landed-price decrease. Increases are movement,
// not cuts — they must not increment PriceCuts or fail --never-cut.
type landedCut struct {
	FromUSD  float64 `json:"from_usd"`
	ToUSD    float64 `json:"to_usd"`
	DeltaUSD float64 `json:"delta_usd"`
	At       string  `json:"at"`
}

// landedPriceCuts returns observed landed-price decreases in chronological
// order. A cut is a drop; a raise or a flat observation is not a cut.
func landedPriceCuts(pts []pricePoint) []landedCut {
	cuts := []landedCut{}
	for i := 1; i < len(pts); i++ {
		d := pts[i].LandedUSD - pts[i-1].LandedUSD
		if d >= 0 {
			continue
		}
		cuts = append(cuts, landedCut{
			FromUSD:  math.Round(pts[i-1].LandedUSD),
			ToUSD:    math.Round(pts[i].LandedUSD),
			DeltaUSD: math.Round(d),
			At:       pts[i].At,
		})
	}
	return cuts
}

// ── price-history ───────────────────────────────────────────────────────────

func newPriceHistoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "price-history [vin]",
		Short: "See a single car's full observed price path",
		Long: "Reach for this for ONE car's own history — is it moving, how fast, when did it last cut. " +
			"Use `stale` instead to find candidates across the corpus; this command assumes you already " +
			"have a VIN. Depth is bounded by when the source and the local store first saw the car, and " +
			"the command prints its own first-observation date so no one implies more history than exists.",
		Example:     "  teslatracker-pp-cli price-history 5YJ3E1EA7LF745758 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "read a stored price path")
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
			pts := pricePath(ctx, st.DB(), *v)

			cuts := landedPriceCuts(pts)
			view := map[string]any{
				"vin": v.VIN, "year": v.Year, "model": v.Model, "trim": v.Trim,
				"first_seen_at": v.FirstSeenAt, "days_listed": daysListed(*v),
				"observations": len(pts), "changes": cuts,
				"landed_note": "landed = price + transportFee; the transport fee used is the current one",
			}
			if len(pts) > 0 {
				view["first_observed_landed_usd"] = math.Round(pts[0].LandedUSD)
				view["current_landed_usd"] = math.Round(pts[len(pts)-1].LandedUSD)
				view["total_movement_usd"] = math.Round(pts[len(pts)-1].LandedUSD - pts[0].LandedUSD)
			}
			if len(cuts) == 0 {
				view["note"] = "no price cut observed across " + fmt.Sprint(len(pts)) + " observations"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %d %s %s\n", v.VIN, derefInt(v.Year), v.Model, v.Trim)
			if d := daysListed(*v); d != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  listed %.0f days (first seen %s)\n", *d, v.FirstSeenAt[:10])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d observations, %d price cuts\n", len(pts), len(cuts))
			for _, c := range cuts {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s  $%.0f -> $%.0f  (%+.0f)\n", c.At[:10], c.FromUSD, c.ToUSD, c.DeltaUSD)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ── gone ────────────────────────────────────────────────────────────────────

func newGoneCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "gone",
		Short: "See which cars left inventory, at what landed price, and after how many days listed",
		Long: "Reach for this to calibrate a ceiling against reality: what leaves inventory, at what " +
			"landed price, after how many days. NEVER describe a departure as a sale — a listing can be " +
			"pulled, reassigned, or delisted. Returns an explicit message rather than a silent empty " +
			"list when there is no prior sync to compare against.",
		Example:     "  teslatracker-pp-cli gone --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list vehicles that left inventory")
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
			db := st.DB()

			// Currently-listed VINs come from the freshest link walk.
			live, err := vinsFromLinks(ctx, db)
			if err != nil {
				return err
			}
			liveSet := map[string]bool{}
			for _, v := range live {
				liveSet[v] = true
			}
			all, err := loadVehicles(ctx, db)
			if err != nil {
				return err
			}

			type departed struct {
				VIN           string   `json:"vin"`
				Year          *int     `json:"year"`
				Model         string   `json:"model"`
				Trim          string   `json:"trim"`
				Mileage       *int     `json:"mileage"`
				LastLandedUSD *float64 `json:"last_landed_usd"`
				DaysListed    *float64 `json:"days_listed"`
				LastSeenAt    string   `json:"last_seen_at"`
			}
			out := []departed{}
			for _, v := range all {
				stillListed := liveSet[v.VIN]
				if v.IsAvailable != nil && !*v.IsAvailable {
					stillListed = false
				}
				if stillListed {
					continue
				}
				d := departed{VIN: v.VIN, Year: v.Year, Model: v.Model, Trim: v.Trim,
					Mileage: v.Mileage, DaysListed: daysListed(v), LastSeenAt: v.LastSeenAt}
				if pts := pricePath(ctx, db, v); len(pts) > 0 {
					last := math.Round(pts[len(pts)-1].LandedUSD)
					d.LastLandedUSD = &last
				}
				out = append(out, d)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].LastSeenAt > out[j].LastSeenAt })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			view := map[string]any{
				"departed": out, "n": len(out),
				"caveat": "left inventory; a listing can be pulled, reassigned or delisted. Not a confirmed sale.",
			}
			if len(all) == 0 {
				view["note"] = "no hydrated vehicles yet; run sync then hydrate"
			} else if len(out) == 0 {
				view["note"] = "every stored vehicle is still listed; no departures to report yet"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no departures yet — every stored vehicle is still listed")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d cars left inventory (not confirmed sales)\n", len(out))
			for _, d := range out {
				price := "unknown"
				if d.LastLandedUSD != nil {
					price = fmt.Sprintf("$%.0f", *d.LastLandedUSD)
				}
				days := "?"
				if d.DaysListed != nil {
					days = fmt.Sprintf("%.0f", *d.DaysListed)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %d %-8s  last %s after %s days\n",
					d.VIN, derefInt(d.Year), d.Model, price, days)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum departures to return")
	return cmd
}

// ── stale ───────────────────────────────────────────────────────────────────

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var neverCutOnly bool

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Rank the corpus by days listed and flag cars that are long-listed and never cut",
		Long: "Reach for this to FIND candidates worth an offer. It answers \"which cars are getting " +
			"tired\" across the corpus. Once you have a VIN, switch to `price-history` for that car's " +
			"detail and `comps` for whether the price is actually good — a car can be stale simply " +
			"because it is overpriced, and this command deliberately does not judge that.",
		Example:     "  teslatracker-pp-cli stale --limit 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank stored vehicles by days listed")
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
			db := st.DB()

			all, err := loadVehicles(ctx, db)
			if err != nil {
				return err
			}
			type row struct {
				VIN        string   `json:"vin"`
				Year       *int     `json:"year"`
				Model      string   `json:"model"`
				Trim       string   `json:"trim"`
				Mileage    *int     `json:"mileage"`
				LandedUSD  *float64 `json:"landed_usd"`
				DaysListed *float64 `json:"days_listed"`
				PriceCuts  int      `json:"price_cuts"`
				NeverCut   bool     `json:"never_cut"`
			}
			rows := []row{}
			for _, v := range all {
				pts := pricePath(ctx, db, v)
				cuts := len(landedPriceCuts(pts))
				r := row{VIN: v.VIN, Year: v.Year, Model: v.Model, Trim: v.Trim,
					Mileage: v.Mileage, DaysListed: daysListed(v), PriceCuts: cuts, NeverCut: cuts == 0}
				if l := v.LandedCents(); l != nil {
					d := math.Round(float64(*l) / 100)
					r.LandedUSD = &d
				}
				if neverCutOnly && !r.NeverCut {
					continue
				}
				rows = append(rows, r)
			}
			sort.Slice(rows, func(i, j int) bool {
				di, dj := 0.0, 0.0
				if rows[i].DaysListed != nil {
					di = *rows[i].DaysListed
				}
				if rows[j].DaysListed != nil {
					dj = *rows[j].DaysListed
				}
				return di > dj
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			view := map[string]any{
				"cars": rows, "n": len(rows),
				"note": "ranked by days listed. Staleness is not a judgement about price — use comps for that.",
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return emit(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d cars, longest-listed first\n", len(rows))
			for _, r := range rows {
				days, price := "?", "?"
				if r.DaysListed != nil {
					days = fmt.Sprintf("%.0f", *r.DaysListed)
				}
				if r.LandedUSD != nil {
					price = fmt.Sprintf("$%.0f", *r.LandedUSD)
				}
				flag := ""
				if r.NeverCut {
					flag = "  never cut"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %d %-8s %8s  %4s days  %d cuts%s\n",
					r.VIN, derefInt(r.Year), r.Model, price, days, r.PriceCuts, flag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum cars to return")
	cmd.Flags().BoolVar(&neverCutOnly, "never-cut", false, "Only cars with no observed price cut")
	return cmd
}

var _ = context.Background
