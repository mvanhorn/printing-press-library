// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// blindCupReport summarises the calibration between user ratings and
// Coffee Review scores across overlapping beans.
type blindCupReport struct {
	N              int            `json:"n"`
	SpearmanRho    float64        `json:"spearman_rho"`
	MeanUserRating float64        `json:"mean_user_rating"`
	MeanCRScore    float64        `json:"mean_coffee_review_score"`
	Verdict        string         `json:"verdict"`
	Pairs          []blindCupPair `json:"pairs"`
}

type blindCupPair struct {
	BeanLabel    string  `json:"bean"`
	UserRating   float64 `json:"user_rating"`
	CoffeeReview int     `json:"coffee_review_score"`
	UserRank     int     `json:"user_rank"`
	CRRank       int     `json:"cr_rank"`
}

func newBlindCupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blind-cup",
		Short: "Palate calibration: Spearman rank correlation of your brew ratings vs Coffee Review scores for overlapping beans",
		Long: `For every bean where you have at least one rated brew AND a Coffee
Review score exists in the synced reviews table, computes the average
user rating and pairs it with the CR score. Spearman's ρ on the two
rank vectors quantifies palate alignment with the CR panel (1.0 =
perfect agreement, 0 = no relationship, -1 = inverted preferences).`,
		Example:     `  coffee-goat-pp-cli blind-cup --agent`,
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
			report, err := buildBlindCupReport(db)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "blind-cup (%d overlapping beans):\n", report.N)
			if report.N < 2 {
				fmt.Fprintln(cmd.OutOrStdout(), "  need at least 2 paired beans to compute Spearman")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Spearman ρ = %+.3f  (%s)\n", report.SpearmanRho, report.Verdict)
			fmt.Fprintf(cmd.OutOrStdout(), "  mean user rating=%.1f  mean CR=%.1f\n", report.MeanUserRating, report.MeanCRScore)
			for _, p := range report.Pairs {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s  you=%.1f (rank %d)  CR=%d (rank %d)\n", p.BeanLabel, p.UserRating, p.UserRank, p.CoffeeReview, p.CRRank)
			}
			return nil
		},
	}
	return cmd
}

func buildBlindCupReport(db *store.Store) (blindCupReport, error) {
	rows, err := db.DB().Query(
		`SELECT bn.roaster_slug, bn.product_slug, COALESCE(rp.title,''),
		        COALESCE(AVG(b.rating),0)
		 FROM brews b
		 JOIN beans bn ON b.bean_id = bn.id
		 LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle
		 WHERE b.rating > 0
		 GROUP BY bn.roaster_slug, bn.product_slug
		 HAVING COUNT(*) >= 1`,
	)
	if err != nil {
		return blindCupReport{}, err
	}
	defer rows.Close()
	var pairs []blindCupPair
	for rows.Next() {
		var roaster, slug, title string
		var avg float64
		if err := rows.Scan(&roaster, &slug, &title, &avg); err != nil {
			return blindCupReport{}, err
		}
		score, ok := lookupCoffeeReviewScore(db, roaster, title)
		if !ok {
			continue
		}
		label := roaster + "/" + slug
		if title != "" {
			label += " (" + title + ")"
		}
		pairs = append(pairs, blindCupPair{
			BeanLabel:    label,
			UserRating:   round2(avg),
			CoffeeReview: score,
		})
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return blindCupReport{}, err
	}
	report := blindCupReport{N: len(pairs), Pairs: pairs}
	if len(pairs) == 0 {
		report.Verdict = "no overlap (need rated brews on beans that also appear in reviews table)"
		return report, nil
	}
	var sumU, sumC float64
	for _, p := range pairs {
		sumU += p.UserRating
		sumC += float64(p.CoffeeReview)
	}
	report.MeanUserRating = round2(sumU / float64(len(pairs)))
	report.MeanCRScore = round2(sumC / float64(len(pairs)))
	if len(pairs) < 2 {
		report.Verdict = "n<2, cannot compute ρ"
		return report, nil
	}
	rankPairs(pairs)
	report.Pairs = pairs
	report.SpearmanRho = round3(spearmanRho(pairs))
	report.Verdict = spearmanVerdict(report.SpearmanRho, len(pairs))
	return report, nil
}

func rankPairs(pairs []blindCupPair) {
	// User ranks
	byUser := make([]int, len(pairs))
	for i := range pairs {
		byUser[i] = i
	}
	sort.Slice(byUser, func(i, j int) bool { return pairs[byUser[i]].UserRating > pairs[byUser[j]].UserRating })
	for rank, idx := range byUser {
		pairs[idx].UserRank = rank + 1
	}
	// CR ranks
	byCR := make([]int, len(pairs))
	for i := range pairs {
		byCR[i] = i
	}
	sort.Slice(byCR, func(i, j int) bool { return pairs[byCR[i]].CoffeeReview > pairs[byCR[j]].CoffeeReview })
	for rank, idx := range byCR {
		pairs[idx].CRRank = rank + 1
	}
}

func spearmanRho(pairs []blindCupPair) float64 {
	n := len(pairs)
	if n < 2 {
		return 0
	}
	var sumDsq float64
	for _, p := range pairs {
		d := float64(p.UserRank - p.CRRank)
		sumDsq += d * d
	}
	N := float64(n)
	return 1 - (6*sumDsq)/(N*(N*N-1))
}

func spearmanVerdict(rho float64, n int) string {
	switch {
	case n < 4:
		return "low n — ρ is noisy"
	case math.Abs(rho) < 0.2:
		return "weak / no agreement with CR panel"
	case rho > 0.7:
		return "strong agreement with the CR panel"
	case rho > 0.4:
		return "moderate agreement with the CR panel"
	case rho > 0:
		return "weak positive agreement"
	case rho < -0.4:
		return "your palate is inverted relative to CR (interesting!)"
	default:
		return "slight disagreement with CR"
	}
}
