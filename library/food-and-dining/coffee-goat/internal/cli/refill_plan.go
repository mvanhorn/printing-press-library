// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// refillCandidate is one row of the plan: a bag that's running out
// soon (with projected days remaining) and a list of twin replacement
// candidates.
type refillCandidate struct {
	BeanID        int64        `json:"bean_id"`
	BeanLabel     string       `json:"bean"`
	CurrentMassG  int          `json:"current_mass_g"`
	BurnGPerDay   float64      `json:"burn_g_per_day"`
	DaysRemaining int          `json:"days_remaining"`
	RecentBrews   int          `json:"recent_brews"`
	Twins         []refillTwin `json:"twin_suggestions,omitempty"`
}

type refillTwin struct {
	RoasterSlug string  `json:"roaster_slug"`
	Handle      string  `json:"handle"`
	Title       string  `json:"title"`
	InStock     bool    `json:"in_stock"`
	Similarity  float64 `json:"similarity"`
}

func newRefillPlanCmd(flags *rootFlags) *cobra.Command {
	var lookbackDays int
	var horizonDays int
	var twinsPerBag int
	cmd := &cobra.Command{
		Use:   "refill-plan",
		Short: "Project depletion across the cellar and suggest twin replacements for soon-empty bags",
		Long: `For each bag with a brew log over the lookback window, computes per-day
gram burn (sum(dose_g) / lookback_days) and projects days remaining at
current mass. Bags due to run out within --horizon-days are flagged
with twin suggestions sourced from the cross-roaster corpus.`,
		Example: `  coffee-goat-pp-cli refill-plan --horizon-days 14 --agent
  coffee-goat-pp-cli refill-plan --lookback-days 21 --twins-per-bag 5`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if lookbackDays <= 0 {
				lookbackDays = 14
			}
			if horizonDays <= 0 {
				horizonDays = 7
			}
			if twinsPerBag <= 0 {
				twinsPerBag = 3
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			plan, err := buildRefillPlan(db, lookbackDays, horizonDays, twinsPerBag)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			if len(plan) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no bags projected to run out within the horizon (or no recent brew data)")
				return nil
			}
			for _, p := range plan {
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s  mass=%dg  burn=%.1fg/day  ~%dd left\n",
					p.BeanID, p.BeanLabel, p.CurrentMassG, p.BurnGPerDay, p.DaysRemaining)
				for _, t := range p.Twins {
					mark := ""
					if !t.InStock {
						mark = " [out]"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    twin %.2f  %s / %s%s\n", t.Similarity, t.RoasterSlug, t.Title, mark)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&lookbackDays, "lookback-days", 14, "Days of brew history to use for the burn-rate estimate")
	cmd.Flags().IntVar(&horizonDays, "horizon-days", 7, "Flag bags projected to deplete within this many days")
	cmd.Flags().IntVar(&twinsPerBag, "twins-per-bag", 3, "Twin candidates returned per soon-empty bag")
	return cmd
}

func buildRefillPlan(db *store.Store, lookbackDays, horizonDays, twinsPerBag int) ([]refillCandidate, error) {
	beans, err := queryBeans(db, 0, "")
	if err != nil {
		return nil, err
	}
	since := time.Now().AddDate(0, 0, -lookbackDays).UTC().Format(time.RFC3339)
	var out []refillCandidate
	for _, b := range beans {
		if b.CurrentMassG <= 0 {
			continue
		}
		burn, n, err := burnRatePerDay(db, b.ID, lookbackDays, since)
		if err != nil {
			return nil, err
		}
		if burn <= 0 {
			continue
		}
		daysLeft := int(math.Floor(float64(b.CurrentMassG) / burn))
		if daysLeft > horizonDays {
			continue
		}
		label := b.RoasterSlug + "/" + b.ProductSlug
		if b.ProductTitle != "" {
			label += " (" + b.ProductTitle + ")"
		}
		c := refillCandidate{
			BeanID:        b.ID,
			BeanLabel:     label,
			CurrentMassG:  b.CurrentMassG,
			BurnGPerDay:   round1(burn),
			DaysRemaining: daysLeft,
			RecentBrews:   n,
		}
		c.Twins = lookupRefillTwins(db, b.RoasterSlug, b.ProductSlug, twinsPerBag)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DaysRemaining < out[j].DaysRemaining })
	return out, nil
}

func burnRatePerDay(db *store.Store, beanID int64, lookbackDays int, since string) (float64, int, error) {
	row := db.DB().QueryRow(
		`SELECT COALESCE(SUM(dose_g),0), COUNT(*) FROM brews
		 WHERE bean_id = ? AND brewed_at >= ? AND dose_g > 0`,
		beanID, since,
	)
	var sumDose float64
	var n int
	if err := row.Scan(&sumDose, &n); err != nil && err != sql.ErrNoRows {
		return 0, 0, err
	}
	if n == 0 {
		return 0, 0, nil
	}
	return sumDose / float64(lookbackDays), n, nil
}

func lookupRefillTwins(db *store.Store, roasterSlug, productSlug string, top int) []refillTwin {
	if roasterSlug == "" || productSlug == "" {
		return nil
	}
	ref, others, err := loadTwinCorpus(db, productSlug)
	if err != nil || ref.Handle == "" {
		return nil
	}
	ranked := rankTwins(ref, others, top)
	out := make([]refillTwin, 0, len(ranked))
	for _, r := range ranked {
		// Look up in-stock flag for the candidate.
		var inStockInt int
		_ = db.DB().QueryRow(
			`SELECT COALESCE(in_stock,0) FROM roaster_products WHERE roaster_slug = ? AND handle = ?`,
			r.Roaster, r.Handle,
		).Scan(&inStockInt)
		out = append(out, refillTwin{
			RoasterSlug: r.Roaster,
			Handle:      r.Handle,
			Title:       r.Title,
			Similarity:  r.Similarity,
			InStock:     inStockInt == 1,
		})
	}
	return out
}
