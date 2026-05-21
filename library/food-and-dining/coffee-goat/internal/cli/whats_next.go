// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// nextPick is one candidate brew with the decomposed score.
type nextPick struct {
	BeanID         int64              `json:"bean_id"`
	BeanLabel      string             `json:"bean"`
	Method         string             `json:"method"`
	Status         string             `json:"freshness_status"`
	DaysSinceRoast int                `json:"days_since_roast"`
	UserBrews      int                `json:"user_brews,omitempty"`
	UserAvgRating  float64            `json:"user_avg_rating,omitempty"`
	Score          float64            `json:"score"`
	Decomposition  map[string]float64 `json:"decomposition,omitempty"`
}

func newWhatsNextCmd(flags *rootFlags) *cobra.Command {
	var method string
	var limit int
	cmd := &cobra.Command{
		Use:   "whats-next",
		Short: "Pick one bag from the cellar to brew next, blending freshness + dial-in confidence + your palate fit",
		Long: `Scores each bag in the cellar against three signals for the chosen
method:
  - freshness fit (peak window > approaching > resting > past-peak)
  - dial-in confidence (more brews + high best rating = higher score)
  - palate fit (descriptor overlap with your favorite-rated brews)

Returns the top --limit (default 3) picks ordered by combined score.`,
		Example: `  coffee-goat-pp-cli whats-next --method v60 --agent
  coffee-goat-pp-cli whats-next --method espresso --limit 5`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(method) == "" {
				return usageErr(fmt.Errorf("whats-next requires --method"))
			}
			if limit <= 0 {
				limit = 3
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			picks, err := scoreWhatsNext(db, strings.ToLower(method), limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), picks, flags)
			}
			if len(picks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no candidates (cellar empty)")
				return nil
			}
			for _, p := range picks {
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s  %.2f  [%s, %dd, %d brews]\n",
					p.BeanID, p.BeanLabel, p.Score, p.Status, p.DaysSinceRoast, p.UserBrews)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "Brew method to score against (required)")
	cmd.Flags().IntVar(&limit, "limit", 3, "Number of top picks to return")
	return cmd
}

func scoreWhatsNext(db *store.Store, method string, limit int) ([]nextPick, error) {
	shelf, err := buildShelfRows(db, method)
	if err != nil {
		return nil, err
	}
	if len(shelf) == 0 {
		return nil, nil
	}
	palate := userPalateTokens(db)
	out := make([]nextPick, 0, len(shelf))
	for _, s := range shelf {
		fresh := freshnessFitScore(s.Status)
		dialin := dialInConfidenceScore(s.UserBrews, s.UserAvgRating)
		palateFit := palateMatchScore(db, s.RoasterSlug, s.ProductSlug, palate)
		// Weights: freshness 0.5, dial-in confidence 0.25, palate 0.25.
		total := fresh*0.5 + dialin*0.25 + palateFit*0.25
		label := s.RoasterSlug + "/" + s.ProductSlug
		if s.ProductTitle != "" {
			label += " (" + s.ProductTitle + ")"
		}
		out = append(out, nextPick{
			BeanID:         s.BeanID,
			BeanLabel:      label,
			Method:         method,
			Status:         s.Status,
			DaysSinceRoast: s.DaysSinceRoast,
			UserBrews:      s.UserBrews,
			UserAvgRating:  s.UserAvgRating,
			Score:          round2(total),
			Decomposition: map[string]float64{
				"freshness_fit":      round2(fresh),
				"dial_in_confidence": round2(dialin),
				"palate_fit":         round2(palateFit),
			},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func freshnessFitScore(status string) float64 {
	switch status {
	case "peak":
		return 1.0
	case "approaching-peak":
		return 0.7
	case "past-peak":
		return 0.4
	case "resting":
		return 0.3
	case "stale":
		return 0.1
	default:
		return 0.2
	}
}

func dialInConfidenceScore(n int, avgRating float64) float64 {
	if n == 0 {
		return 0.2 // gentle exploration nudge
	}
	scale := float64(n) / 8.0
	if scale > 1 {
		scale = 1
	}
	ratingBoost := 0.0
	if avgRating > 0 {
		ratingBoost = (avgRating - 5) / 5 // 5 -> 0, 10 -> 1
		if ratingBoost < 0 {
			ratingBoost = 0
		}
	}
	return scale*0.6 + ratingBoost*0.4
}

// userPalateTokens returns the set of descriptor tokens that occur in
// the user's high-rated brews (rating >= 7) and roaster_products
// tag/body fields. Used as a small bag-of-words for palate-fit scoring.
func userPalateTokens(db *store.Store) map[string]int {
	out := map[string]int{}
	rows, err := db.DB().Query(
		`SELECT COALESCE(b.descriptors_json,'') || ' ' || COALESCE(rp.tags_json,'') || ' ' || COALESCE(rp.body_text,'')
		 FROM brews b
		 LEFT JOIN beans bn ON b.bean_id = bn.id
		 LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle
		 WHERE b.rating >= 7`,
	)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var combined string
		if err := rows.Scan(&combined); err != nil {
			continue
		}
		for _, t := range tokenizeSimple(combined) {
			out[t]++
		}
	}
	_ = rows.Err()
	return out
}

func palateMatchScore(db *store.Store, roasterSlug, productSlug string, palate map[string]int) float64 {
	if len(palate) == 0 || roasterSlug == "" || productSlug == "" {
		return 0.3
	}
	var combined string
	_ = db.DB().QueryRow(
		`SELECT COALESCE(tags_json,'') || ' ' || COALESCE(body_text,'')
		 FROM roaster_products WHERE roaster_slug = ? AND handle = ?`,
		roasterSlug, productSlug,
	).Scan(&combined)
	tokens := tokenizeSimple(combined)
	if len(tokens) == 0 {
		return 0.3
	}
	hits := 0
	for _, t := range tokens {
		if palate[t] > 0 {
			hits++
		}
	}
	scale := float64(hits) / float64(len(tokens))
	if scale > 1 {
		scale = 1
	}
	return scale
}

func tokenizeSimple(s string) []string {
	s = strings.ToLower(s)
	out := make([]string, 0, 16)
	cur := ""
	for _, r := range s {
		if r == '-' || (r >= 'a' && r <= 'z') {
			cur += string(r)
			continue
		}
		if len(cur) >= 3 && !commonStopword(cur) {
			out = append(out, cur)
		}
		cur = ""
	}
	if len(cur) >= 3 && !commonStopword(cur) {
		out = append(out, cur)
	}
	return out
}

func commonStopword(t string) bool {
	switch t {
	case "the", "and", "with", "from", "this", "that", "have", "are", "for", "not", "but", "you", "our", "your", "all", "one", "two", "three", "into", "more", "less", "very", "any", "some", "what", "when", "how", "now":
		return true
	}
	return false
}
