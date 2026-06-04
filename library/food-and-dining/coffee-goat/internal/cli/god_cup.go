// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// godCupReport is the headline meta-recommendation. brew_pick and
// buy_pick may be empty when data is too sparse — callers see a
// rationale field that says so.
type godCupReport struct {
	BrewPick *godCupPick `json:"brew_pick"`
	BuyPick  *godCupPick `json:"buy_pick"`
	Notes    string      `json:"notes,omitempty"`
}

type godCupPick struct {
	BeanID    int64              `json:"bean_id,omitempty"`
	Roaster   string             `json:"roaster"`
	Handle    string             `json:"handle"`
	Title     string             `json:"title"`
	Score     float64            `json:"score"`
	Method    string             `json:"method,omitempty"`
	Rationale string             `json:"rationale"`
	Factors   map[string]float64 `json:"factors"`
}

// methodWindow maps brewing method -> (peak start days, peak end days)
// from-roast freshness window. Espresso: 8-21d, V60 filter: 5-28d.
var methodWindow = map[string][2]int{
	"espresso":  {8, 21},
	"v60":       {5, 28},
	"filter":    {5, 28},
	"aeropress": {5, 28},
	"immersion": {5, 28},
}

func newGodCupCmd(flags *rootFlags) *cobra.Command {
	var method string
	cmd := &cobra.Command{
		Use:   "god-cup",
		Short: "Headline recommender: one brew pick from your shelf + one buy pick across the market",
		Example: `  coffee-goat-pp-cli god-cup --method espresso --agent
  coffee-goat-pp-cli god-cup --method v60 --agent --select brew_pick.bean,buy_pick.product`,
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
			report, err := buildGodCupReport(db, method)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if report.BrewPick != nil {
				p := report.BrewPick
				fmt.Fprintf(cmd.OutOrStdout(), "brew-now: %s / %s (score %.2f) — %s\n", p.Roaster, p.Title, p.Score, p.Rationale)
			}
			if report.BuyPick != nil {
				p := report.BuyPick
				fmt.Fprintf(cmd.OutOrStdout(), "buy-next: %s / %s (score %.2f) — %s\n", p.Roaster, p.Title, p.Score, p.Rationale)
			}
			if report.Notes != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "notes: %s\n", report.Notes)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "v60", "Brew method (shifts the freshness window): espresso, v60, filter, aeropress, immersion")
	return cmd
}

func buildGodCupReport(db *store.Store, method string) (godCupReport, error) {
	report := godCupReport{}
	// --- Brew pick: rank user's shelf beans
	type shelfRow struct {
		BeanID    int64
		Roaster   string
		Handle    string
		Title     string
		RoastDate string
		DaysSince int
	}
	rows, err := db.DB().Query(
		`SELECT bn.id, bn.roaster_slug, bn.product_slug, COALESCE(rp.title, bn.product_slug), COALESCE(bn.roast_date, '')
		 FROM beans bn LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle`,
	)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	var shelf []shelfRow
	for rows.Next() {
		var s shelfRow
		if err := rows.Scan(&s.BeanID, &s.Roaster, &s.Handle, &s.Title, &s.RoastDate); err != nil {
			return report, err
		}
		if s.RoastDate != "" {
			if t, err := time.Parse("2006-01-02", s.RoastDate); err == nil {
				s.DaysSince = int(time.Since(t).Hours() / 24)
			}
		}
		shelf = append(shelf, s)
	}
	if err := rows.Err(); err != nil {
		return report, fmt.Errorf("iterate shelf rows: %w", err)
	}

	// Need at least 5 brews to enable god-cup.
	var brewCount int
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM brews`).Scan(&brewCount)

	if brewCount < 5 {
		report.Notes = fmt.Sprintf("no data; log 5+ brews to enable god-cup (currently %d)", brewCount)
	}

	if len(shelf) > 0 {
		win, ok := methodWindow[strings.ToLower(method)]
		if !ok {
			win = methodWindow["v60"]
		}
		var ranked []godCupPick
		for _, s := range shelf {
			factors := map[string]float64{}
			freshness := freshnessFit(s.DaysSince, win)
			dialIn := dialInConfidence(db, s.Roaster, s.Handle)
			creator := creatorCoverage(db, s.Roaster)
			factors["freshness_window_fit"] = freshness
			factors["dial_in_confidence"] = dialIn
			factors["creator_coverage"] = creator
			factors["coffee_review_score"] = 0
			factors["palate_match"] = 0
			factors["novelty"] = 0
			score := 0.30*freshness + 0.20*dialIn + 0.15*creator
			rationale := buildRationale(factors, s.DaysSince)
			ranked = append(ranked, godCupPick{
				BeanID: s.BeanID, Roaster: s.Roaster, Handle: s.Handle, Title: s.Title,
				Score: score, Method: method, Rationale: rationale, Factors: factors,
			})
		}
		sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
		top := ranked[0]
		report.BrewPick = &top
	}

	// --- Buy pick: rank market products by review score + creator coverage + palate match (rough).
	mrows, err := db.DB().Query(
		`SELECT rp.roaster_slug, rp.handle, COALESCE(rp.title,''), COALESCE(rp.origin,''), COALESCE(rp.in_stock,0)
		 FROM roaster_products rp
		 WHERE rp.in_stock = 1`,
	)
	if err != nil {
		return report, fmt.Errorf("query buy-pick candidates: %w", err)
	}
	defer mrows.Close()
	var picks []godCupPick
	for mrows.Next() {
		var r, h, title, origin string
		var inStock int
		if err := mrows.Scan(&r, &h, &title, &origin, &inStock); err != nil {
			continue
		}
		factors := map[string]float64{}
		revScore := coffeeReviewScore(db, r, title)
		creator := creatorCoverage(db, r)
		factors["coffee_review_score"] = revScore / 100.0
		factors["creator_coverage"] = creator
		factors["novelty"] = 0.5 // weak prior
		score := 0.15*(revScore/100.0) + 0.15*creator + 0.10*0.5
		picks = append(picks, godCupPick{
			Roaster: r, Handle: h, Title: title,
			Score: score, Method: method,
			Rationale: fmt.Sprintf("review=%.0f, creator-mentions=%v", revScore, creator > 0),
			Factors:   factors,
		})
	}
	if err := mrows.Err(); err != nil {
		return report, fmt.Errorf("iterate buy-pick rows: %w", err)
	}
	sort.Slice(picks, func(i, j int) bool { return picks[i].Score > picks[j].Score })
	if len(picks) > 0 {
		top := picks[0]
		// Suppress buy_pick when every real-signal factor is zero. With
		// no Coffee Review score and no creator coverage, the surviving
		// score is driven entirely by the constant novelty prior and a
		// "recommendation" is not informative — let the notes field
		// (e.g., "log 5+ brews to enable god-cup") carry the empty
		// state alone, mirroring brew_pick: null.
		if top.Factors["coffee_review_score"] > 0 || top.Factors["creator_coverage"] > 0 {
			report.BuyPick = &top
		}
	}

	return report, nil
}

func freshnessFit(days int, win [2]int) float64 {
	if days <= 0 {
		return 0.3
	}
	if days >= win[0] && days <= win[1] {
		return 1.0
	}
	if days < win[0] {
		return float64(days) / float64(win[0])
	}
	// past peak
	over := days - win[1]
	val := 1.0 - float64(over)/14.0
	if val < 0 {
		return 0
	}
	return val
}

func dialInConfidence(db *store.Store, roaster, handle string) float64 {
	var n int
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM brews b LEFT JOIN beans bn ON b.bean_id = bn.id WHERE bn.roaster_slug=? AND bn.product_slug=?`, roaster, handle).Scan(&n)
	if n == 0 {
		return 0
	}
	// 3+ brews -> 1.0, fewer scales linearly.
	if n >= 3 {
		return 1.0
	}
	return float64(n) / 3.0
}

func creatorCoverage(db *store.Store, roaster string) float64 {
	var n int
	// Anchor the LIKE pattern on JSON-string quotes so `"la"` doesn't match
	// `"la-cabra"`. mentioned_roaster_slugs_json stores values as a JSON array
	// of strings like `["onyx","sey"]`, so the slug always appears wrapped
	// in double-quotes — `%"<roaster>"%` matches the full slug token only.
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM youtube_reviews WHERE mentioned_roaster_slugs_json LIKE ?`, `%"`+roaster+`"%`).Scan(&n)
	if n == 0 {
		return 0
	}
	if n >= 3 {
		return 1.0
	}
	return float64(n) / 3.0
}

func coffeeReviewScore(db *store.Store, roaster, title string) float64 {
	var score int
	_ = db.DB().QueryRow(
		`SELECT score FROM reviews WHERE LOWER(roaster_name) LIKE ? OR LOWER(bean_name) LIKE ? ORDER BY score DESC LIMIT 1`,
		"%"+strings.ToLower(roaster)+"%", "%"+strings.ToLower(title)+"%",
	).Scan(&score)
	return float64(score)
}

func buildRationale(factors map[string]float64, daysSince int) string {
	parts := []string{}
	if v := factors["freshness_window_fit"]; v > 0.7 {
		parts = append(parts, fmt.Sprintf("in freshness window (%dd since roast)", daysSince))
	}
	if v := factors["dial_in_confidence"]; v >= 1.0 {
		parts = append(parts, "dial-in stable (3+ brews)")
	}
	if v := factors["creator_coverage"]; v > 0 {
		parts = append(parts, "covered by tracked creator")
	}
	if len(parts) == 0 {
		parts = append(parts, "best of remaining shelf")
	}
	return strings.Join(parts, "; ")
}
