// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// dialInRecommendation suggests next-brew params combining user priors
// and cross-cluster averages.
type dialInRecommendation struct {
	BeanLabel      string             `json:"bean"`
	Method         string             `json:"method"`
	N              int                `json:"brew_count"`
	UserMean       dialInPoint        `json:"user_mean,omitempty"`
	UserBest       dialInPoint        `json:"user_best,omitempty"`
	ClusterMean    dialInPoint        `json:"cluster_mean,omitempty"`
	Suggestion     dialInPoint        `json:"suggestion"`
	BlendWeights   map[string]float64 `json:"blend_weights"`
	ConfidenceNote string             `json:"confidence_note,omitempty"`
}

type dialInPoint struct {
	DoseG        float64 `json:"dose_g,omitempty"`
	YieldG       float64 `json:"yield_g,omitempty"`
	TimeS        float64 `json:"time_s,omitempty"`
	TemperatureC float64 `json:"temperature_c,omitempty"`
	Ratio        float64 `json:"ratio_yield_over_dose,omitempty"`
	Rating       float64 `json:"rating,omitempty"`
}

func newDialInCmd(flags *rootFlags) *cobra.Command {
	var method string
	cmd := &cobra.Command{
		Use:   "dial-in <bean-id-or-roaster/product>",
		Short: "Bayesian-flavored dial-in: blends your brew priors with cross-cluster averages for the same origin/process to suggest next-brew params",
		Long: `Computes a suggested next-brew dose / yield / time / temperature for
one bean by combining your own brew log with the corpus average for
the bean's origin+process cluster. When you have <3 brews on the bean,
the suggestion leans heavily on the cluster (75/25). With 5+ brews
the user mean dominates (75/25 the other way). At 8+ brews and a
top-rated brew exists, the suggestion is your best brew itself.`,
		Example: `  coffee-goat-pp-cli dial-in 3 --method espresso --agent
  coffee-goat-pp-cli dial-in sey/banko-gotiti --method v60`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(method) == "" {
				return usageErr(fmt.Errorf("dial-in requires --method"))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rec, err := buildDialInRecommendation(db, args[0], strings.ToLower(method))
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "dial-in for %s (%s, %d brews):\n", rec.BeanLabel, rec.Method, rec.N)
			fmt.Fprintf(cmd.OutOrStdout(), "  suggest dose=%.1fg yield=%.1fg time=%.0fs temp=%.0fC (ratio %.2f)\n",
				rec.Suggestion.DoseG, rec.Suggestion.YieldG, rec.Suggestion.TimeS, rec.Suggestion.TemperatureC, rec.Suggestion.Ratio)
			if rec.ConfidenceNote != "" {
				fmt.Fprintln(cmd.OutOrStdout(), "  "+rec.ConfidenceNote)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "Brew method (required)")
	return cmd
}

func buildDialInRecommendation(db *store.Store, target, method string) (dialInRecommendation, error) {
	roasterSlug, productSlug, beanID, label, origin, process, err := resolveTargetBean(db, target)
	if err != nil {
		return dialInRecommendation{}, err
	}
	rec := dialInRecommendation{BeanLabel: label, Method: method}

	userMean, userBest, n := userBrewStats(db, beanID, roasterSlug, productSlug, method)
	rec.UserMean = userMean
	rec.UserBest = userBest
	rec.N = n

	clusterMean := clusterBrewStats(db, origin, process, method)
	rec.ClusterMean = clusterMean

	w := blendWeights(n, userBest.Rating)
	rec.BlendWeights = map[string]float64{
		"user_mean":    w.userMean,
		"user_best":    w.userBest,
		"cluster_mean": w.clusterMean,
	}
	rec.Suggestion = blendPoints(userMean, userBest, clusterMean, w)
	rec.ConfidenceNote = dialInConfidenceNote(n, userBest.Rating, clusterMean)
	return rec, nil
}

func resolveTargetBean(db *store.Store, target string) (roasterSlug, productSlug string, beanID int64, label, origin, process string, err error) {
	// Try as bean id first.
	if id, perr := strconv.ParseInt(strings.TrimSpace(target), 10, 64); perr == nil && id > 0 {
		var r, p string
		row := db.DB().QueryRow(`SELECT COALESCE(roaster_slug,''), COALESCE(product_slug,'') FROM beans WHERE id = ?`, id)
		if scanErr := row.Scan(&r, &p); scanErr == nil {
			roasterSlug, productSlug, beanID = r, p, id
			label = fmt.Sprintf("bean#%d (%s/%s)", id, r, p)
			origin, process = lookupProductOriginProcess(db, r, p)
			return
		}
	}
	// Try as roaster/handle.
	r, h := splitRoasterHandle(target)
	q := `SELECT COALESCE(roaster_slug,''), COALESCE(handle,''), COALESCE(title,''), COALESCE(origin,''), COALESCE(process,'')
	      FROM roaster_products WHERE LOWER(handle) = LOWER(?)`
	args := []any{h}
	if r != "" {
		q += ` AND LOWER(roaster_slug) = LOWER(?)`
		args = append(args, r)
	}
	q += ` LIMIT 1`
	var title string
	row := db.DB().QueryRow(q, args...)
	if scanErr := row.Scan(&roasterSlug, &productSlug, &title, &origin, &process); scanErr != nil {
		err = notFoundErr(fmt.Errorf("bean %q not found in cellar or roaster_products", target))
		return
	}
	label = fmt.Sprintf("%s/%s", roasterSlug, productSlug)
	if title != "" {
		label += " (" + title + ")"
	}
	// Best-effort bean id resolution; can be 0 if the bag isn't in the cellar.
	_ = db.DB().QueryRow(`SELECT COALESCE(id,0) FROM beans WHERE roaster_slug = ? AND product_slug = ? ORDER BY added_at DESC LIMIT 1`, roasterSlug, productSlug).Scan(&beanID)
	return
}

func lookupProductOriginProcess(db *store.Store, roasterSlug, productSlug string) (origin, process string) {
	_ = db.DB().QueryRow(
		`SELECT COALESCE(origin,''), COALESCE(process,'') FROM roaster_products WHERE roaster_slug = ? AND handle = ?`,
		roasterSlug, productSlug,
	).Scan(&origin, &process)
	return
}

func userBrewStats(db *store.Store, beanID int64, roasterSlug, productSlug, method string) (dialInPoint, dialInPoint, int) {
	q := `SELECT COALESCE(b.dose_g,0), COALESCE(b.yield_g,0), COALESCE(b.time_s,0),
	             COALESCE(b.temperature_c,0), COALESCE(b.rating,0)
	      FROM brews b
	      LEFT JOIN beans bn ON b.bean_id = bn.id
	      WHERE LOWER(b.method) = ?`
	args := []any{method}
	switch {
	case beanID > 0:
		q += ` AND b.bean_id = ?`
		args = append(args, beanID)
	case roasterSlug != "" && productSlug != "":
		q += ` AND bn.roaster_slug = ? AND bn.product_slug = ?`
		args = append(args, roasterSlug, productSlug)
	default:
		return dialInPoint{}, dialInPoint{}, 0
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return dialInPoint{}, dialInPoint{}, 0
	}
	defer rows.Close()

	var sumDose, sumYield, sumTime, sumTemp float64
	var n int
	var best dialInPoint
	for rows.Next() {
		var p dialInPoint
		if err := rows.Scan(&p.DoseG, &p.YieldG, &p.TimeS, &p.TemperatureC, &p.Rating); err != nil {
			continue
		}
		if p.DoseG > 0 {
			sumDose += p.DoseG
		}
		if p.YieldG > 0 {
			sumYield += p.YieldG
		}
		if p.TimeS > 0 {
			sumTime += p.TimeS
		}
		if p.TemperatureC > 0 {
			sumTemp += p.TemperatureC
		}
		if p.Rating > best.Rating {
			best = p
			if best.DoseG > 0 {
				best.Ratio = round2(best.YieldG / best.DoseG)
			}
		}
		n++
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: userBrewStats iteration error: %v\n", err)
	}
	if n == 0 {
		return dialInPoint{}, dialInPoint{}, 0
	}
	mean := dialInPoint{
		DoseG:        round1(sumDose / float64(n)),
		YieldG:       round1(sumYield / float64(n)),
		TimeS:        round1(sumTime / float64(n)),
		TemperatureC: round1(sumTemp / float64(n)),
	}
	if mean.DoseG > 0 {
		mean.Ratio = round2(mean.YieldG / mean.DoseG)
	}
	return mean, best, n
}

func clusterBrewStats(db *store.Store, origin, process, method string) dialInPoint {
	if origin == "" && process == "" {
		return dialInPoint{}
	}
	q := `SELECT COALESCE(AVG(b.dose_g),0), COALESCE(AVG(b.yield_g),0),
	             COALESCE(AVG(b.time_s),0), COALESCE(AVG(b.temperature_c),0)
	      FROM brews b
	      JOIN beans bn ON b.bean_id = bn.id
	      JOIN roaster_products rp ON rp.roaster_slug = bn.roaster_slug AND rp.handle = bn.product_slug
	      WHERE LOWER(b.method) = ?`
	args := []any{method}
	if origin != "" {
		q += ` AND LOWER(rp.origin) = LOWER(?)`
		args = append(args, origin)
	}
	if process != "" {
		q += ` AND LOWER(rp.process) = LOWER(?)`
		args = append(args, process)
	}
	var p dialInPoint
	row := db.DB().QueryRow(q, args...)
	if err := row.Scan(&p.DoseG, &p.YieldG, &p.TimeS, &p.TemperatureC); err != nil && err != sql.ErrNoRows {
		return dialInPoint{}
	}
	p.DoseG = round1(p.DoseG)
	p.YieldG = round1(p.YieldG)
	p.TimeS = round1(p.TimeS)
	p.TemperatureC = round1(p.TemperatureC)
	if p.DoseG > 0 {
		p.Ratio = round2(p.YieldG / p.DoseG)
	}
	return p
}

type dialInWeights struct {
	userMean    float64
	userBest    float64
	clusterMean float64
}

func blendWeights(n int, bestRating float64) dialInWeights {
	switch {
	case n == 0:
		return dialInWeights{0, 0, 1.0}
	case n < 3:
		return dialInWeights{0.25, 0, 0.75}
	case n < 5:
		return dialInWeights{0.5, 0, 0.5}
	case n < 8:
		return dialInWeights{0.75, 0, 0.25}
	default:
		if bestRating >= 8 {
			return dialInWeights{0.25, 0.65, 0.10}
		}
		return dialInWeights{0.75, 0.10, 0.15}
	}
}

func blendPoints(userMean, userBest, cluster dialInPoint, w dialInWeights) dialInPoint {
	out := dialInPoint{
		DoseG:        blend(userMean.DoseG, userBest.DoseG, cluster.DoseG, w),
		YieldG:       blend(userMean.YieldG, userBest.YieldG, cluster.YieldG, w),
		TimeS:        blend(userMean.TimeS, userBest.TimeS, cluster.TimeS, w),
		TemperatureC: blend(userMean.TemperatureC, userBest.TemperatureC, cluster.TemperatureC, w),
	}
	out.DoseG = round1(out.DoseG)
	out.YieldG = round1(out.YieldG)
	out.TimeS = round1(out.TimeS)
	out.TemperatureC = round1(out.TemperatureC)
	if out.DoseG > 0 {
		out.Ratio = round2(out.YieldG / out.DoseG)
	}
	return out
}

func blend(userMean, userBest, cluster float64, w dialInWeights) float64 {
	// Zero values shouldn't drag the average down; redistribute their
	// weight proportionally to the other terms that are non-zero.
	contribs := []struct {
		v, w float64
	}{{userMean, w.userMean}, {userBest, w.userBest}, {cluster, w.clusterMean}}
	var totalW, val float64
	for _, c := range contribs {
		if c.v > 0 {
			val += c.v * c.w
			totalW += c.w
		}
	}
	if totalW == 0 {
		return 0
	}
	return val / totalW
}

func dialInConfidenceNote(n int, bestRating float64, cluster dialInPoint) string {
	switch {
	case n == 0:
		if cluster.DoseG == 0 {
			return "no priors — no brews on this bean and no cluster data. Treat suggestion as zero; log a brew first."
		}
		return "no priors on this bean; using cluster mean only."
	case n < 3:
		return "low confidence (<3 brews) — suggestion leans on the cluster mean."
	case n < 8:
		return "moderate confidence — blending user history with cluster."
	default:
		if bestRating >= 8 {
			return "high confidence — replaying your best-rated brew."
		}
		return "high confidence — user mean dominates."
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }
