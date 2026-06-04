// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// twinResult is one ranked candidate.
type twinResult struct {
	Roaster       string             `json:"roaster"`
	Handle        string             `json:"handle"`
	Title         string             `json:"title"`
	Origin        string             `json:"origin,omitempty"`
	Process       string             `json:"process,omitempty"`
	Varietal      string             `json:"varietal,omitempty"`
	Similarity    float64            `json:"similarity_score"`
	Decomposition map[string]float64 `json:"decomposition,omitempty"`
}

// twinFeatures projects a row from roaster_products into a feature
// vector for cosine similarity.
type twinFeatures struct {
	Roaster      string
	Handle       string
	Title        string
	Origin       string
	Process      string
	Varietal     string
	AltitudeBand int // 0=unknown, 1=<1500, 2=1500-1800, 3=1800-2100, 4=>2100
	Tags         []string
}

func newTwinCmd(flags *rootFlags) *cobra.Command {
	var top int
	cmd := &cobra.Command{
		Use:         "twin <roaster-or-product-slug>",
		Short:       "Find the closest match to a bean across all 24 roasters via attribute + descriptor similarity",
		Example:     `  coffee-goat-pp-cli twin sey-banko-gotiti --top 5 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			input := args[0]
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			ref, others, err := loadTwinCorpus(db, input)
			if err != nil {
				return err
			}
			if ref.Handle == "" {
				return notFoundErr(fmt.Errorf("twin: bean or product %q not found in local store; run 'sync' first", input))
			}
			results := rankTwins(ref, others, top)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no candidates")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "twins of %s / %s:\n", ref.Roaster, ref.Title)
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %.2f  %s / %s (%s, %s)\n", r.Similarity, r.Roaster, r.Title, r.Origin, r.Process)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&top, "top", 5, "Number of twins to return")
	return cmd
}

func loadTwinCorpus(db *store.Store, input string) (twinFeatures, []twinFeatures, error) {
	rows, err := db.DB().Query(
		`SELECT roaster_slug, handle, COALESCE(title,''), COALESCE(origin,''), COALESCE(process,''),
		        COALESCE(varietal,''), COALESCE(altitude,''), COALESCE(tags_json,'')
		 FROM roaster_products`,
	)
	if err != nil {
		return twinFeatures{}, nil, err
	}
	defer rows.Close()
	var ref twinFeatures
	var all []twinFeatures
	for rows.Next() {
		var f twinFeatures
		var altRaw string
		var tagsJSON string
		if err := rows.Scan(&f.Roaster, &f.Handle, &f.Title, &f.Origin, &f.Process, &f.Varietal, &altRaw, &tagsJSON); err != nil {
			return twinFeatures{}, nil, err
		}
		f.AltitudeBand = altitudeBand(altRaw)
		f.Tags = parseTagsLoose(tagsJSON)
		all = append(all, f)
		if ref.Handle == "" && (f.Handle == input || strings.EqualFold(f.Title, input) || strings.EqualFold(f.Roaster, input)) {
			ref = f
		}
	}
	if err := rows.Err(); err != nil {
		return twinFeatures{}, nil, fmt.Errorf("iterate roaster_products rows: %w", err)
	}
	if ref.Handle == "" {
		return twinFeatures{}, nil, nil
	}
	var others []twinFeatures
	for _, f := range all {
		if f.Roaster == ref.Roaster && f.Handle == ref.Handle {
			continue
		}
		others = append(others, f)
	}
	return ref, others, nil
}

func altitudeBand(alt string) int {
	if alt == "" {
		return 0
	}
	// crude: find first 4-digit number.
	for i := 0; i+4 <= len(alt); i++ {
		if isDigits(alt[i : i+4]) {
			n := atoiSimple(alt[i : i+4])
			switch {
			case n < 1500:
				return 1
			case n < 1800:
				return 2
			case n < 2100:
				return 3
			default:
				return 4
			}
		}
	}
	return 0
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func atoiSimple(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func parseTagsLoose(tagsJSON string) []string {
	if tagsJSON == "" {
		return nil
	}
	// Simple substring extract — strip brackets and quotes.
	clean := strings.NewReplacer(`[`, "", `]`, "", `"`, "", `\n`, "").Replace(tagsJSON)
	parts := strings.Split(clean, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// rankTwins runs cosine similarity over all candidates. Each
// attribute contributes a one-hot dimension; tags contribute a small
// tf-idf-style vector via shared-tag count over union.
func rankTwins(ref twinFeatures, others []twinFeatures, top int) []twinResult {
	if top <= 0 {
		top = 5
	}
	scored := make([]twinResult, 0, len(others))
	for _, c := range others {
		s, decomp := twinSimilarity(ref, c)
		scored = append(scored, twinResult{
			Roaster: c.Roaster, Handle: c.Handle, Title: c.Title,
			Origin: c.Origin, Process: c.Process, Varietal: c.Varietal,
			Similarity:    s,
			Decomposition: decomp,
		})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Similarity > scored[j].Similarity })
	if top < len(scored) {
		scored = scored[:top]
	}
	return scored
}

// twinSimilarity returns a 0..1 score combining attribute matches +
// tag Jaccard. Weights: origin 0.30, process 0.20, varietal 0.20,
// altitude 0.15, tags 0.15.
func twinSimilarity(a, b twinFeatures) (float64, map[string]float64) {
	d := map[string]float64{}
	d["origin"] = match(a.Origin, b.Origin) * 0.30
	d["process"] = match(a.Process, b.Process) * 0.20
	d["varietal"] = match(a.Varietal, b.Varietal) * 0.20
	d["altitude"] = altitudeMatch(a.AltitudeBand, b.AltitudeBand) * 0.15
	d["tags"] = jaccard(a.Tags, b.Tags) * 0.15
	total := 0.0
	for _, v := range d {
		total += v
	}
	// Clamp into [0,1] just in case rounding tweaks push above 1.
	if total > 1 {
		total = 1
	}
	return math.Round(total*100) / 100, d
}

func match(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) {
		return 1
	}
	return 0
}

func altitudeMatch(a, b int) float64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a == b {
		return 1
	}
	if abs(a-b) == 1 {
		return 0.5
	}
	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	set := map[string]int{}
	for _, x := range a {
		set[x] |= 1
	}
	for _, x := range b {
		set[x] |= 2
	}
	inter, union := 0, 0
	for _, v := range set {
		union++
		if v == 3 {
			inter++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
