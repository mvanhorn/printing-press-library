// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

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

// recipe is one row of the curated V60 recipe library. The structured
// 5-tuple (DoseG/Ratio/TimeS/TempC/Grind) drives matching, scaling, and
// the brew row written by `apply --log`. The freeform Technique markdown
// is what a human reads at the kettle — pour patterns, bloom timings,
// and agitation cues that don't reduce to a schema.
//
// RecommendedFor is the curated tag set match scoring intersects against
// bean features (roast level, origin token, process, descriptor tokens).
//
// pp:novel-static-reference — curated reference data, not synthesized
// from an API.
type recipe struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Author         string   `json:"author"`
	Year           int      `json:"year,omitempty"`
	Method         string   `json:"method"`
	DoseG          float64  `json:"dose_g"`
	Ratio          float64  `json:"ratio"`
	TimeS          int      `json:"time_s"`
	TempC          float64  `json:"temp_c"`
	Grind          string   `json:"grind_label"`
	RecommendedFor []string `json:"recommended_for"`
	Technique      string   `json:"technique"`
	Source         string   `json:"source,omitempty"`
}

// recipeLibrary is the curated V60 library shipped with v1. Adding a
// recipe is a matter of appending one row plus a `recommended_for` tag
// set the match scorer can intersect against bean features.
//
// pp:novel-static-reference
var recipeLibrary = []recipe{
	{
		Slug: "hoffmann-ultimate-2018", Name: "Hoffmann Ultimate V60 (2018)",
		Author: "James Hoffmann", Year: 2018, Method: "v60",
		DoseG: 30, Ratio: 16.7, TimeS: 240, TempC: 95, Grind: "medium-fine",
		RecommendedFor: []string{"light-roast", "medium-roast", "ethiopian", "kenyan", "washed", "floral", "citric"},
		Technique: `1. Rinse paper filter with boiling water; warm V60 and server.
2. Add 30g coffee, level the bed.
3. Bloom: pour 60g water (2x dose), swirl gently, wait 45s.
4. First pour: smoothly add water to 300g by 1:15 (spiraling outward).
5. Second pour: top up to 500g by 1:45, finishing center.
6. Stir once each direction, gentle swirl to flatten the bed.
7. Drawdown should complete by 3:30-4:00 total.`,
		Source: "youtube: James Hoffmann V60 technique (2018)",
	},
	{
		Slug: "hoffmann-ultimate-2023", Name: "Hoffmann Ultimate V60 (2023 update)",
		Author: "James Hoffmann", Year: 2023, Method: "v60",
		DoseG: 15, Ratio: 16.6, TimeS: 210, TempC: 96, Grind: "medium-fine",
		RecommendedFor: []string{"light-roast", "medium-roast", "ethiopian", "colombian", "washed", "natural", "fruity", "clarity"},
		Technique: `1. Rinse paper, preheat dripper and server.
2. Add 15g coffee, flatten the bed.
3. Bloom: pour 45g (3x dose) by 0:10, swirl until uniformly wet, wait until 0:45.
4. Pour 1: bring up to 125g by 1:15, spiral outward to inward.
5. Pour 2: bring up to 250g by 1:45, gentle center pour.
6. Final swirl. Aim for total drawdown 3:00-3:30.`,
		Source: "youtube: James Hoffmann V60 update (2023)",
	},
	{
		Slug: "tetsu-4-6-light", Name: "Tetsu Kasuya 4:6 (light roast variant)",
		Author: "Tetsu Kasuya", Year: 2016, Method: "v60",
		DoseG: 20, Ratio: 15, TimeS: 210, TempC: 93, Grind: "medium-coarse",
		RecommendedFor: []string{"light-roast", "medium-roast", "ethiopian", "kenyan", "washed", "bright", "acidity"},
		Technique: `40/60 split: first 40% of water controls sweetness/acidity, last 60% controls strength.

For sweeter & brighter (light roast):
1. Pour 1: 60g (3x dose) by 0:00, wait until 0:45.
2. Pour 2: to 120g by 0:45, wait until 1:30.
3. Pour 3: to 180g by 1:30, wait until 2:10.
4. Pour 4: to 240g by 2:10, wait until 2:40.
5. Pour 5: to 300g by 2:40. Drawdown by ~3:30.

For brighter cup: smaller first pour (50g). For sweeter: larger (70g).`,
		Source: "World Brewers Cup 2016 (Tetsu Kasuya)",
	},
	{
		Slug: "tetsu-4-6-dark", Name: "Tetsu Kasuya 4:6 (dark roast variant)",
		Author: "Tetsu Kasuya", Year: 2016, Method: "v60",
		DoseG: 20, Ratio: 15, TimeS: 210, TempC: 88, Grind: "medium",
		RecommendedFor: []string{"medium-dark", "dark-roast", "brazilian", "indonesian", "natural", "chocolate", "body", "syrupy"},
		Technique: `40/60 split tuned for darker roasts — lower temp, coarser grind, fewer late pours to limit bitter extraction.

1. Pour 1: 70g (3.5x dose) by 0:00, wait until 0:45 (larger bloom for solubility).
2. Pour 2: to 120g by 0:45, wait until 1:30.
3. Pour 3: to 200g by 1:30, wait until 2:30.
4. Pour 4: to 300g by 2:30. Drawdown by ~3:30.

3 late pours instead of 4 reduces final strength and bitterness.`,
		Source: "World Brewers Cup 2016 (Tetsu Kasuya), dark variant",
	},
	{
		Slug: "lance-hedrick-pulse", Name: "Lance Hedrick Pulse V60",
		Author: "Lance Hedrick", Year: 2022, Method: "v60",
		DoseG: 18, Ratio: 16.7, TimeS: 195, TempC: 96, Grind: "medium-fine",
		RecommendedFor: []string{"light-roast", "medium-roast", "ethiopian", "kenyan", "colombian", "washed", "natural", "complexity"},
		Technique: `Continuous pulse strategy for higher extraction with stable bed.

1. Rinse paper, preheat fully.
2. Bloom: 50g (~3x dose) by 0:10, vigorous swirl, wait until 0:40.
3. Pulse pours of 30-40g each, spaced ~10-15s apart, keeping the slurry level high.
   Target intervals: 0:40, 0:55, 1:10, 1:25, 1:40, 1:55, 2:10.
4. Final pour to 300g by 2:15. No final stir.
5. Drawdown 3:00-3:15.

Optionally Rao spin at 0:10 to deepen bloom.`,
		Source: "youtube: Lance Hedrick V60 pulse",
	},
	{
		Slug: "wbrc-2020-jia-ning-du", Name: "WBrC 2020 — Jia Ning Du V60",
		Author: "Jia Ning Du", Year: 2020, Method: "v60",
		DoseG: 14, Ratio: 16, TimeS: 195, TempC: 93, Grind: "medium-fine",
		RecommendedFor: []string{"light-roast", "ethiopian", "natural", "anaerobic", "floral", "fruity"},
		Technique: `Brewers Cup routine optimized for clarity on fruity anaerobic naturals.

1. Pre-wet paper, preheat dripper.
2. Bloom: 40g by 0:10, gentle stir with chopstick, wait until 0:45.
3. Main pour 1: to 130g by 1:15, spiraling center-to-edge.
4. Main pour 2: to 224g by 1:45, centered.
5. Light swirl. Drawdown by ~3:00.`,
		Source: "WBrC 2020 China qualifiers (representative routine)",
	},
	{
		Slug: "wbrc-2021-matt-winton", Name: "WBrC 2021 — Matt Winton V60",
		Author: "Matt Winton", Year: 2021, Method: "v60",
		DoseG: 18, Ratio: 14.4, TimeS: 165, TempC: 95, Grind: "medium",
		RecommendedFor: []string{"light-roast", "medium-roast", "geisha", "panama", "ethiopian", "washed", "tea-like", "delicate"},
		Technique: `Lower ratio (1:14.4) for higher strength, faster drawdown to preserve top notes.

1. Bloom: 50g by 0:10, swirl, wait until 0:40.
2. Single continuous pour to 260g by 1:30, gentle spiral.
3. No final stir.
4. Drawdown by ~2:45.`,
		Source: "WBrC 2021 (representative routine)",
	},
	{
		Slug: "wbrc-2022-sheng-tien-cheng", Name: "WBrC 2022 — Sheng-Tien Cheng V60",
		Author: "Sheng-Tien Cheng", Year: 2022, Method: "v60",
		DoseG: 15, Ratio: 16, TimeS: 180, TempC: 92, Grind: "medium-fine",
		RecommendedFor: []string{"light-roast", "anaerobic", "natural", "ethiopian", "wine-like", "berry"},
		Technique: `Three-pour routine tuned for anaerobic naturals.

1. Bloom: 45g by 0:10, gentle stir, wait until 0:50.
2. Pour 1: to 150g by 1:15, centered.
3. Pour 2: to 240g by 1:50, gentle spiral.
4. Drawdown by ~3:00.`,
		Source: "WBrC 2022 (representative routine)",
	},
	{
		Slug: "wbrc-2023-mikael-jasin", Name: "WBrC 2023 — Mikael Jasin V60",
		Author: "Mikael Jasin", Year: 2023, Method: "v60",
		DoseG: 16, Ratio: 16, TimeS: 210, TempC: 96, Grind: "medium-fine",
		RecommendedFor: []string{"light-roast", "medium-roast", "indonesian", "washed", "natural", "complex", "balanced"},
		Technique: `4-pour routine with extended bloom for Indonesian washed lots.

1. Bloom: 50g (~3x dose) by 0:10, swirl until even, wait until 0:50.
2. Pour 1: to 110g by 1:10, spiraling.
3. Pour 2: to 180g by 1:35.
4. Pour 3: to 256g by 2:00, centered.
5. Light swirl. Drawdown ~3:20-3:30.`,
		Source: "WBrC 2023 (Mikael Jasin, Indonesia)",
	},
	{
		Slug: "wbrc-2024-carlos-de-toro", Name: "WBrC 2024 — Carlos de Toro V60",
		Author: "Carlos de Toro", Year: 2024, Method: "v60",
		DoseG: 14, Ratio: 16.4, TimeS: 180, TempC: 94, Grind: "medium-fine",
		RecommendedFor: []string{"light-roast", "geisha", "panama", "ethiopian", "natural", "anaerobic", "floral", "jasmine"},
		Technique: `Designed for high-end naturals and Geishas.

1. Bloom: 42g by 0:10, gentle stir, wait until 0:45.
2. Pour 1: to 140g by 1:10, spiraling outward.
3. Pour 2: to 230g by 1:40, center pour.
4. No final stir.
5. Drawdown by ~3:00.`,
		Source: "WBrC 2024 (representative routine)",
	},
}

func newRecipeReplayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe-replay",
		Short: "Curated V60 recipe library (Hoffmann, Tetsu 4:6, Lance Pulse, WBrC champions) — list, show, match-to-bean, and apply",
		Long: `Curated V60 recipe library shipped with the binary as static reference
data. Each recipe carries a structured 5-tuple (dose, ratio, time, temp,
grind) for matching/scaling and a freeform technique markdown for what
the human reads at the kettle.

Subcommands:
  list     List all recipes (optionally filter by --for <token>)
  show     Show one recipe by slug (with full technique markdown)
  match    Given a bean, rank recipes by tag overlap
  apply    Combine a recipe + bean, output customized 5-tuple + technique;
           with --log, also write a pre-filled brews row`,
		Example: `  coffee-goat-pp-cli recipe-replay list
  coffee-goat-pp-cli recipe-replay show hoffmann-ultimate-2023
  coffee-goat-pp-cli recipe-replay match sey/banko-gotiti
  coffee-goat-pp-cli recipe-replay apply tetsu-4-6-light sey/banko-gotiti --dose 18 --log`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRecipeReplayListCmd(flags))
	cmd.AddCommand(newRecipeReplayShowCmd(flags))
	cmd.AddCommand(newRecipeReplayMatchCmd(flags))
	cmd.AddCommand(newRecipeReplayApplyCmd(flags))
	return cmd
}

func newRecipeReplayListCmd(flags *rootFlags) *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all recipes (optionally filter --for <token> against recommended_for tags)",
		Example:     `  coffee-goat-pp-cli recipe-replay list --for light-roast`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			out := make([]recipe, 0, len(recipeLibrary))
			filt := strings.ToLower(strings.TrimSpace(filter))
			for _, r := range recipeLibrary {
				if filt == "" || tagsContain(r.RecommendedFor, filt) {
					out = append(out, r)
				}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no recipes matched")
				return nil
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-32s  %s (%d)  %.0fg @ 1:%.1f  %ds  %.0f°C\n",
					r.Slug, r.Author, r.Year, r.DoseG, r.Ratio, r.TimeS, r.TempC)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&filter, "for", "", "Filter recipes whose recommended_for tags include this token (e.g. light-roast, ethiopian, anaerobic)")
	return cmd
}

func newRecipeReplayShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <slug>",
		Short:       "Show full recipe detail: structured 5-tuple + technique markdown",
		Example:     `  coffee-goat-pp-cli recipe-replay show hoffmann-ultimate-2023`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			r, ok := lookupRecipe(args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("recipe %q not in library (run 'recipe-replay list' to see slugs)", args[0]))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), r, flags)
			}
			renderRecipe(cmd, r)
			return nil
		},
	}
	return cmd
}

func newRecipeReplayMatchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "match <bean>",
		Short:       "Rank recipes for a bean by recommended_for tag overlap with bean features",
		Example:     `  coffee-goat-pp-cli recipe-replay match sey/banko-gotiti --limit 3`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if limit <= 0 {
				limit = 3
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			feats, err := loadBeanFeatures(db, args[0])
			if err != nil {
				return err
			}
			matches := rankRecipesForBean(feats, limit)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no recipes matched")
				return nil
			}
			for _, m := range matches {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  fit=%d/%d  [%s]\n",
					m.Slug, m.Hits, m.TagsTotal, strings.Join(m.MatchedTags, ", "))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 3, "Max ranked recipes returned")
	return cmd
}

func newRecipeReplayApplyCmd(flags *rootFlags) *cobra.Command {
	var doseG float64
	var doLog bool
	cmd := &cobra.Command{
		Use:   "apply <recipe-slug> <bean>",
		Short: "Apply a recipe to a bean: output customized 5-tuple + technique; --log writes a brews row",
		Long: `Scales dose + water linearly while preserving ratio (if --dose is set);
time, temp, and grind stay unchanged. Technique markdown is shown verbatim.

If the recipe's recommended_for tags don't intersect with the bean's
roast level, a warning is appended (no auto-adjust — recipe author
parameters are kept as published).

With --log, a brews row is inserted with method='v60', the customized
dose/yield/time/temperature, rating=0 (fill in after tasting), and
notes='recipe:<slug>' so 'brews list' shows which recipe drove the brew.`,
		Example: `  coffee-goat-pp-cli recipe-replay apply hoffmann-ultimate-2023 sey/banko-gotiti --dose 18
  coffee-goat-pp-cli recipe-replay apply tetsu-4-6-light sey/banko-gotiti --dose 20 --log`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			r, ok := lookupRecipe(args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("recipe %q not in library", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			feats, err := loadBeanFeatures(db, args[1])
			if err != nil {
				return err
			}
			result := applyRecipe(r, feats, doseG)
			if doLog {
				id, err := logRecipeBrew(db, r, feats, result)
				if err != nil {
					return err
				}
				result.LoggedBrewID = id
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			renderApplyResult(cmd, result)
			return nil
		},
	}
	cmd.Flags().Float64Var(&doseG, "dose", 0, "Override the recipe's default dose (g); water scales linearly to preserve ratio")
	cmd.Flags().BoolVar(&doLog, "log", false, "Also write a brews row with these parameters (rating=0, notes='recipe:<slug>')")
	return cmd
}

// lookupRecipe returns the recipe with the given slug, case-insensitive.
func lookupRecipe(slug string) (recipe, bool) {
	s := strings.ToLower(strings.TrimSpace(slug))
	for _, r := range recipeLibrary {
		if r.Slug == s {
			return r, true
		}
	}
	return recipe{}, false
}

// tagsContain reports whether tags include token (case-insensitive substring).
func tagsContain(tags []string, token string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), token) {
			return true
		}
	}
	return false
}

// beanFeatures is the projection of a bean used for recipe matching.
// Source columns come from roaster_products via beans JOIN; roast level,
// origin, process, and descriptor tokens are all curated tags the match
// scorer can intersect against recipe.RecommendedFor.
type beanFeatures struct {
	BeanRef      string   `json:"bean"`
	RoasterSlug  string   `json:"roaster_slug,omitempty"`
	ProductSlug  string   `json:"product_slug,omitempty"`
	Title        string   `json:"title,omitempty"`
	RoastLevel   string   `json:"roast_level,omitempty"`
	Origin       string   `json:"origin,omitempty"`
	Process      string   `json:"process,omitempty"`
	Descriptors  []string `json:"descriptors,omitempty"`
	OriginTokens []string `json:"origin_tokens,omitempty"`
	AllTokens    []string `json:"-"`
}

// loadBeanFeatures resolves a bean reference ("roaster/handle" or local
// bean#id) into the curated tag set the match scorer uses. The lookup
// reads roaster_products primarily, since that's where roast_level
// origin/process live; bean#id resolves through beans → roaster_products.
func loadBeanFeatures(db *store.Store, raw string) (beanFeatures, error) {
	roaster, handle := splitRoasterHandle(raw)
	q := `SELECT COALESCE(rp.roaster_slug,''), COALESCE(rp.handle,''), COALESCE(rp.title,''),
	             COALESCE(rp.roast_level,''), COALESCE(rp.origin,''), COALESCE(rp.process,''),
	             COALESCE(rp.tags_json,''), COALESCE(rp.body_text,'')
	      FROM roaster_products rp
	      WHERE LOWER(rp.handle) = LOWER(?)`
	args := []any{handle}
	if roaster != "" {
		q += ` AND LOWER(rp.roaster_slug) = LOWER(?)`
		args = append(args, roaster)
	}
	q += ` LIMIT 1`
	var rSlug, hdl, title, roast, origin, process, tags, body string
	row := db.DB().QueryRow(q, args...)
	err := row.Scan(&rSlug, &hdl, &title, &roast, &origin, &process, &tags, &body)
	if err == sql.ErrNoRows {
		return beanFeatures{}, notFoundErr(fmt.Errorf("bean %q not found in roaster_products (run 'sync' or check the handle)", raw))
	}
	if err != nil {
		return beanFeatures{}, err
	}
	feats := beanFeatures{
		BeanRef:     rSlug + "/" + hdl,
		RoasterSlug: rSlug, ProductSlug: hdl, Title: title,
		RoastLevel: strings.ToLower(roast),
		Origin:     strings.ToLower(origin),
		Process:    strings.ToLower(process),
	}
	feats.OriginTokens = tokenizeSimple(origin)
	feats.Descriptors = tokenizeSimple(tags + " " + body)
	feats.AllTokens = append([]string{feats.RoastLevel, feats.Origin, feats.Process}, feats.OriginTokens...)
	feats.AllTokens = append(feats.AllTokens, feats.Descriptors...)
	return feats, nil
}

// recipeMatch is one ranked match output for `match`.
type recipeMatch struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Author      string   `json:"author"`
	Year        int      `json:"year,omitempty"`
	Hits        int      `json:"hits"`
	TagsTotal   int      `json:"tags_total"`
	MatchedTags []string `json:"matched_tags"`
}

// rankRecipesForBean intersects each recipe's RecommendedFor with the
// bean's feature tokens and returns the top `limit` by intersection
// count. Ties break by recipe year descending (newer first).
func rankRecipesForBean(feats beanFeatures, limit int) []recipeMatch {
	tokenSet := map[string]bool{}
	for _, t := range feats.AllTokens {
		if t != "" {
			tokenSet[strings.ToLower(t)] = true
		}
	}
	out := make([]recipeMatch, 0, len(recipeLibrary))
	for _, r := range recipeLibrary {
		var matched []string
		for _, tag := range r.RecommendedFor {
			if tokenSet[strings.ToLower(tag)] {
				matched = append(matched, tag)
			}
		}
		out = append(out, recipeMatch{
			Slug: r.Slug, Name: r.Name, Author: r.Author, Year: r.Year,
			Hits: len(matched), TagsTotal: len(r.RecommendedFor),
			MatchedTags: matched,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return out[i].Year > out[j].Year
	})
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out
}

// applyResult is the customized recipe output for `apply`.
type applyResult struct {
	RecipeSlug    string       `json:"recipe_slug"`
	RecipeName    string       `json:"recipe_name"`
	Bean          string       `json:"bean"`
	BeanTitle     string       `json:"bean_title,omitempty"`
	Method        string       `json:"method"`
	DoseG         float64      `json:"dose_g"`
	YieldG        float64      `json:"yield_g"`
	TimeS         int          `json:"time_s"`
	TempC         float64      `json:"temp_c"`
	Grind         string       `json:"grind_label"`
	Technique     string       `json:"technique"`
	Warnings      []string     `json:"warnings,omitempty"`
	LoggedBrewID  int64        `json:"logged_brew_id,omitempty"`
	RecipeOrig    recipe       `json:"recipe,omitempty"`
	MatchedTags   []string     `json:"matched_tags,omitempty"`
	BeanFeatures  beanFeatures `json:"bean_features,omitempty"`
	ScalingFactor float64      `json:"scaling_factor,omitempty"`
}

// applyRecipe customizes a recipe for a bean. Scaling rules per spec:
//
//   - If overrideDose > 0 and differs from r.DoseG by ≥5%, scale dose and
//     water linearly (preserve ratio). Time, temp, and grind unchanged.
//   - Technique markdown reproduced verbatim; a scaling-warning footer is
//     appended when scaling was applied so the user knows pour weights in
//     the text don't auto-scale.
//   - Roast-level mismatch (recipe tagged light/medium/dark vs bean's
//     roast_level) appends a warning; no auto-adjust.
func applyRecipe(r recipe, feats beanFeatures, overrideDose float64) applyResult {
	dose := r.DoseG
	scaling := 1.0
	if overrideDose > 0 {
		if math.Abs(overrideDose-r.DoseG)/r.DoseG >= 0.05 {
			scaling = overrideDose / r.DoseG
		}
		dose = overrideDose
	}
	yield := dose * r.Ratio
	result := applyResult{
		RecipeSlug: r.Slug, RecipeName: r.Name,
		Bean: feats.BeanRef, BeanTitle: feats.Title,
		Method: r.Method,
		DoseG:  round2(dose), YieldG: round2(yield),
		TimeS: r.TimeS, TempC: r.TempC, Grind: r.Grind,
		Technique:     r.Technique,
		ScalingFactor: round2(scaling),
		RecipeOrig:    r,
		BeanFeatures:  feats,
	}
	// Compute matched tags so the apply output is self-explanatory.
	tokenSet := map[string]bool{}
	for _, t := range feats.AllTokens {
		if t != "" {
			tokenSet[strings.ToLower(t)] = true
		}
	}
	for _, tag := range r.RecommendedFor {
		if tokenSet[strings.ToLower(tag)] {
			result.MatchedTags = append(result.MatchedTags, tag)
		}
	}
	if scaling != 1.0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"technique pour timings are calibrated to %.1fg dose; adjust pour weights proportionally (~%.2fx here)",
			r.DoseG, scaling))
	}
	if w := roastMismatchWarning(r, feats); w != "" {
		result.Warnings = append(result.Warnings, w)
	}
	return result
}

// roastMismatchWarning returns a warning string when the recipe's roast
// tag set (light-roast / medium-roast / dark-roast / medium-dark) does
// not intersect with the bean's roast level. Empty when the recipe
// doesn't carry a roast tag (e.g., method-agnostic recipes) or when the
// bean has no roast_level.
func roastMismatchWarning(r recipe, feats beanFeatures) string {
	if feats.RoastLevel == "" {
		return ""
	}
	roastTags := map[string]bool{}
	for _, tag := range r.RecommendedFor {
		lt := strings.ToLower(tag)
		if strings.Contains(lt, "roast") || lt == "medium-dark" {
			roastTags[lt] = true
		}
	}
	if len(roastTags) == 0 {
		return ""
	}
	beanRoast := strings.ToLower(feats.RoastLevel)
	// Match: "light", "medium", "medium-dark", "dark" → recipe tag prefixes.
	matched := false
	for tag := range roastTags {
		switch {
		case strings.Contains(beanRoast, "light") && strings.Contains(tag, "light"):
			matched = true
		case strings.Contains(beanRoast, "medium-dark") && (tag == "medium-dark" || strings.Contains(tag, "dark")):
			matched = true
		case strings.Contains(beanRoast, "medium") && strings.Contains(tag, "medium"):
			matched = true
		case strings.Contains(beanRoast, "dark") && (strings.Contains(tag, "dark") || tag == "medium-dark"):
			matched = true
		}
	}
	if matched {
		return ""
	}
	tagList := make([]string, 0, len(roastTags))
	for t := range roastTags {
		tagList = append(tagList, t)
	}
	sort.Strings(tagList)
	return fmt.Sprintf("recipe targets %s; bean roast_level=%q — consider adjusting temp/grind",
		strings.Join(tagList, "/"), feats.RoastLevel)
}

// logRecipeBrew inserts a brews row with the customized parameters and
// returns the new brew ID. notes carries `recipe:<slug>` so `brews list`
// shows which recipe drove the brew. bean_id is left NULL when the bean
// reference cannot be resolved against the local cellar.
func logRecipeBrew(db *store.Store, r recipe, feats beanFeatures, result applyResult) (int64, error) {
	var beanID sql.NullInt64
	if feats.RoasterSlug != "" && feats.ProductSlug != "" {
		var id int64
		err := db.DB().QueryRow(
			`SELECT id FROM beans WHERE roaster_slug=? AND product_slug=? ORDER BY added_at DESC LIMIT 1`,
			feats.RoasterSlug, feats.ProductSlug,
		).Scan(&id)
		if err == nil {
			beanID = sql.NullInt64{Int64: id, Valid: true}
		}
	}
	notes := "recipe:" + r.Slug
	res, err := db.DB().Exec(
		`INSERT INTO brews (bean_id, method, grind, dose_g, yield_g, time_s, temperature_c, rating, notes, brewed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		beanID, r.Method, r.Grind,
		result.DoseG, result.YieldG, result.TimeS, result.TempC,
		notes,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("insert brew: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// renderRecipe is the human-readable single-recipe view.
func renderRecipe(cmd *cobra.Command, r recipe) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — %s", r.Slug, r.Name)
	if r.Year > 0 {
		fmt.Fprintf(w, " (%d)", r.Year)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  author: %s\n", r.Author)
	fmt.Fprintf(w, "  method: %s\n", r.Method)
	fmt.Fprintf(w, "  dose: %.1fg @ 1:%.1f → %.0fg yield\n", r.DoseG, r.Ratio, r.DoseG*r.Ratio)
	fmt.Fprintf(w, "  time: %ds  temp: %.0f°C  grind: %s\n", r.TimeS, r.TempC, r.Grind)
	if len(r.RecommendedFor) > 0 {
		fmt.Fprintf(w, "  recommended_for: %s\n", strings.Join(r.RecommendedFor, ", "))
	}
	if r.Source != "" {
		fmt.Fprintf(w, "  source: %s\n", r.Source)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, r.Technique)
}

// renderApplyResult is the human-readable apply output.
func renderApplyResult(cmd *cobra.Command, ar applyResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "→ %s applied to %s", ar.RecipeName, ar.Bean)
	if ar.BeanTitle != "" {
		fmt.Fprintf(w, " (%s)", ar.BeanTitle)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  dose: %.1fg  yield: %.0fg  time: %ds  temp: %.0f°C  grind: %s\n",
		ar.DoseG, ar.YieldG, ar.TimeS, ar.TempC, ar.Grind)
	if ar.ScalingFactor != 1.0 && ar.ScalingFactor != 0 {
		fmt.Fprintf(w, "  scaled %.2fx from recipe default\n", ar.ScalingFactor)
	}
	if len(ar.MatchedTags) > 0 {
		fmt.Fprintf(w, "  matched tags: %s\n", strings.Join(ar.MatchedTags, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, ar.Technique)
	for _, warn := range ar.Warnings {
		fmt.Fprintf(w, "\n⚠ %s\n", warn)
	}
	if ar.LoggedBrewID > 0 {
		fmt.Fprintf(w, "\n✓ logged brew #%d (rating=0; update with 'brews show %d' after tasting)\n",
			ar.LoggedBrewID, ar.LoggedBrewID)
	}
}
