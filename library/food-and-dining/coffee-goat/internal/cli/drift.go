// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// driftSeries summarizes the rating-vs-days-since-roast slope for a
// segment (one bean or one method). Slope is the per-day change in
// rating estimated by ordinary least squares.
type driftSeries struct {
	Segment    string  `json:"segment"`
	Method     string  `json:"method,omitempty"`
	BeanLabel  string  `json:"bean_label,omitempty"`
	N          int     `json:"n"`
	Slope      float64 `json:"slope_per_day"`
	Intercept  float64 `json:"intercept"`
	MeanRating float64 `json:"mean_rating"`
	R2         float64 `json:"r_squared"`
	Verdict    string  `json:"verdict"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var method string
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Rating-drift diagnostic. OLS on rating vs days-since-roast per method to detect per-method staling rates",
		Long: `Pools all brews logged for the given method (or every method when --method
is empty) and fits a simple OLS line of rating against days since
roast. A negative slope means the user perceives quality dropping with
age on that method; a near-zero slope means age isn't moving rating.
Per-bean detail is available via 'bag-life <bean>'.`,
		Example: `  coffee-goat-pp-cli drift --method espresso --agent
  coffee-goat-pp-cli drift`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			series, err := computeDriftByMethod(db, strings.ToLower(method))
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), series, flags)
			}
			if len(series) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no brews with roast_date + rating yet")
				return nil
			}
			for _, s := range series {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s (n=%d)  slope=%+.3f rating/day  mean=%.2f  R²=%.2f  %s\n",
					s.Segment, s.N, s.Slope, s.MeanRating, s.R2, s.Verdict)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "Restrict to one method (empty = one row per method)")
	return cmd
}

type ratingByAge struct {
	Days   float64
	Rating float64
	Method string
	Bean   string
}

func loadRatingByAge(db *store.Store, restrictMethod, restrictBeanRoaster, restrictBeanProduct string) ([]ratingByAge, error) {
	q := `SELECT b.rating, COALESCE(bn.roast_date,''), COALESCE(b.brewed_at,''),
	             COALESCE(b.method,''), COALESCE(bn.roaster_slug,''), COALESCE(bn.product_slug,'')
	      FROM brews b
	      JOIN beans bn ON b.bean_id = bn.id
	      WHERE b.rating > 0 AND bn.roast_date IS NOT NULL AND bn.roast_date != ''`
	args := []any{}
	if restrictMethod != "" {
		q += ` AND LOWER(b.method) = ?`
		args = append(args, restrictMethod)
	}
	if restrictBeanRoaster != "" && restrictBeanProduct != "" {
		q += ` AND bn.roaster_slug = ? AND bn.product_slug = ?`
		args = append(args, restrictBeanRoaster, restrictBeanProduct)
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ratingByAge
	for rows.Next() {
		var rating int
		var roastDate, brewedAt, method, roasterSlug, productSlug string
		if err := rows.Scan(&rating, &roastDate, &brewedAt, &method, &roasterSlug, &productSlug); err != nil {
			return nil, err
		}
		brewedTime, err := parseLooseTime(brewedAt)
		if err != nil {
			continue
		}
		days := daysSince(roastDate, brewedTime)
		if days <= 0 {
			continue
		}
		out = append(out, ratingByAge{
			Days:   float64(days),
			Rating: float64(rating),
			Method: method,
			Bean:   roasterSlug + "/" + productSlug,
		})
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}

func parseLooseTime(s string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q", s)
}

func computeDriftByMethod(db *store.Store, restrictMethod string) ([]driftSeries, error) {
	rows, err := loadRatingByAge(db, restrictMethod, "", "")
	if err != nil {
		return nil, err
	}
	byMethod := map[string][]ratingByAge{}
	for _, r := range rows {
		byMethod[r.Method] = append(byMethod[r.Method], r)
	}
	out := make([]driftSeries, 0, len(byMethod))
	for method, segment := range byMethod {
		s := fitDriftSeries(segment)
		s.Segment = method
		s.Method = method
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Segment < out[j].Segment })
	return out, nil
}

func fitDriftSeries(rows []ratingByAge) driftSeries {
	n := len(rows)
	if n < 2 {
		s := driftSeries{N: n, Verdict: "insufficient data"}
		if n == 1 {
			s.MeanRating = rows[0].Rating
		}
		return s
	}
	var sumX, sumY, sumXY, sumXX, sumYY float64
	for _, r := range rows {
		sumX += r.Days
		sumY += r.Rating
		sumXY += r.Days * r.Rating
		sumXX += r.Days * r.Days
		sumYY += r.Rating * r.Rating
	}
	N := float64(n)
	denom := N*sumXX - sumX*sumX
	if denom == 0 {
		mean := sumY / N
		return driftSeries{N: n, MeanRating: round2(mean), Verdict: "no variation in age"}
	}
	slope := (N*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / N
	mean := sumY / N
	// R² = 1 - SS_res/SS_tot
	var ssRes, ssTot float64
	for _, r := range rows {
		pred := intercept + slope*r.Days
		ssRes += (r.Rating - pred) * (r.Rating - pred)
		ssTot += (r.Rating - mean) * (r.Rating - mean)
	}
	r2 := 0.0
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}
	verdict := driftVerdict(slope, n)
	return driftSeries{
		N:          n,
		Slope:      round3(slope),
		Intercept:  round2(intercept),
		MeanRating: round2(mean),
		R2:         round2(r2),
		Verdict:    verdict,
	}
}

func driftVerdict(slope float64, n int) string {
	switch {
	case n < 4:
		return "low confidence (need more brews)"
	case math.Abs(slope) < 0.02:
		return "stable (no meaningful drift)"
	case slope < -0.05:
		return "fast decay — rating drops noticeably day over day"
	case slope < 0:
		return "slow decay — small negative trend"
	case slope > 0.05:
		return "improves with age (unusual; check sample)"
	default:
		return "slight positive trend"
	}
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
