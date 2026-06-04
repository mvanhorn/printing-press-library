// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// championRecipe captures one championship-style brewing routine: bean
// lineage + approximate brew parameters inspired by World Brewers Cup /
// World Barista Championship coverage. Used by `champion-replay shop` to
// suggest which lots in your synced corpus most closely match the
// championship-style bean by origin/process/varietal.
//
// IMPORTANT — these entries are illustrative scaffolds, not verified
// competition records. Competitor names, beans, producers, and brew
// parameters are best-effort reconstructions from public coverage
// (Sprudge editorial, the WBC rule book, post-finals interviews) and
// almost certainly contain errors. Verify against an authoritative
// source before treating any single recipe as ground truth. The data
// structure is the durable contract; the per-row contents are a starting
// point users are expected to correct.
//
// pp:novel-static-reference — curated reference scaffold, not fetched
// from any API.
type championRecipe struct {
	Slug        string   `json:"slug"`
	Year        int      `json:"year"`
	Competition string   `json:"competition"`
	Rank        int      `json:"rank"`
	Competitor  string   `json:"competitor"`
	Country     string   `json:"country"`
	Bean        string   `json:"bean"`
	Producer    string   `json:"producer"`
	Origin      string   `json:"origin"`
	Region      string   `json:"region,omitempty"`
	Process     string   `json:"process"`
	Varietal    string   `json:"varietal"`
	Method      string   `json:"method"`
	DoseG       float64  `json:"dose_g"`
	Ratio       float64  `json:"ratio"`
	TimeS       int      `json:"time_s"`
	TempC       float64  `json:"temp_c"`
	Descriptors []string `json:"descriptors"`
	Notes       string   `json:"notes"`
}

// pp:novel-static-reference
var championLibrary = []championRecipe{
	{
		Slug: "wbrc-2024-style-yirgacheffe-anaerobic", Year: 2024, Competition: "WBrC", Rank: 1,
		Competitor: "Boyuan Fan", Country: "China",
		Bean: "Worka Sakaro", Producer: "Worka Sakaro Cooperative",
		Origin: "Ethiopia", Region: "Yirgacheffe",
		Process: "anaerobic-natural", Varietal: "heirloom",
		Method: "V60", DoseG: 14, Ratio: 16.6, TimeS: 195, TempC: 94,
		Descriptors: []string{"jasmine", "bergamot", "stone-fruit", "honey"},
		Notes:       "Two-pour pulse, 60g bloom @ 0:00, finish @ 2:15, drawdown 3:15.",
	},
	{
		Slug: "wbrc-2023-style-bermudez-gesha", Year: 2023, Competition: "WBrC", Rank: 1,
		Competitor: "Martin Wölfl", Country: "Austria",
		Bean: "Finca El Paraíso Gesha", Producer: "Diego Bermúdez",
		Origin: "Colombia", Region: "Cauca",
		Process: "double-anaerobic-thermal-shock", Varietal: "gesha",
		Method: "V60", DoseG: 15, Ratio: 16, TimeS: 210, TempC: 93,
		Descriptors: []string{"lychee", "rose", "passionfruit", "syrupy"},
		Notes:       "Single continuous pour, 90g bloom, target drawdown 3:30.",
	},
	{
		Slug: "wbc-2022-style-natural-gesha-espresso", Year: 2022, Competition: "WBC", Rank: 1,
		Competitor: "Anthony Douglas", Country: "Australia",
		Bean: "Finca Sophia Gesha", Producer: "Lamastus Family Estates",
		Origin: "Panama", Region: "Volcán",
		Process: "natural", Varietal: "gesha",
		Method: "Espresso", DoseG: 20, Ratio: 2.0, TimeS: 28, TempC: 93,
		Descriptors: []string{"jasmine", "blueberry", "cane-sugar"},
		Notes:       "20g in, 40g out, 28s. Lever profile, declining pressure.",
	},
	{
		Slug: "wbrc-2021-style-sudan-rume", Year: 2021, Competition: "WBrC", Rank: 1,
		Competitor: "Matt Winton", Country: "Switzerland",
		Bean: "Sudan Rume", Producer: "El Mirador",
		Origin: "Colombia", Region: "Huila",
		Process: "washed", Varietal: "sudan-rume",
		Method: "V60", DoseG: 12, Ratio: 16.7, TimeS: 180, TempC: 96,
		Descriptors: []string{"strawberry", "rhubarb", "elderflower"},
		Notes:       "4-pour, Tetsu-style segmented at 0:00 / 0:45 / 1:30 / 2:15.",
	},
	{
		Slug: "wbrc-2020-style-wush-wush", Year: 2020, Competition: "WBrC", Rank: 1,
		Competitor: "Du Jianing", Country: "China",
		Bean: "Wush Wush", Producer: "Aida Batlle",
		Origin: "El Salvador", Region: "Santa Ana",
		Process: "washed", Varietal: "wush-wush",
		Method: "V60", DoseG: 14, Ratio: 17, TimeS: 200, TempC: 95,
		Descriptors: []string{"jasmine", "tropical-fruit", "honeysuckle"},
		Notes:       "Three pours, total 238g water, drawdown 3:20.",
	},
	{
		Slug: "wbc-2023-style-pink-bourbon-espresso", Year: 2023, Competition: "WBC", Rank: 2,
		Competitor: "Diego Campos", Country: "Colombia",
		Bean: "Pink Bourbon", Producer: "Finca Las Margaritas",
		Origin: "Colombia", Region: "Huila",
		Process: "thermal-shock-washed", Varietal: "pink-bourbon",
		Method: "Espresso", DoseG: 19, Ratio: 2.1, TimeS: 30, TempC: 92,
		Descriptors: []string{"raspberry", "cocoa-nib", "rosewater"},
		Notes:       "19g in, 40g out, 30s. Pressure profile peaks at 6 bar.",
	},
}

// championBySlug returns the named recipe, or nil if not found.
func championBySlug(slug string) *championRecipe {
	for i := range championLibrary {
		if championLibrary[i].Slug == slug {
			return &championLibrary[i]
		}
	}
	return nil
}

func newChampionReplayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "champion-replay",
		Short:       "Championship-style brewing recipes inspired by WBC / WBrC coverage — list, show, and shop for matching lots in the synced corpus",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Curated scaffold of championship-style brewing recipes (bean origin /
process / varietal + brew parameters) inspired by World Brewers Cup
and World Barista Championship coverage.

The entries are best-effort reconstructions from public sources, not
verified competition records — competitor names, beans, producers, and
brew parameters almost certainly contain errors. Treat the recipes as
a starting scaffold and verify against an authoritative source before
relying on any single entry.`,
		Example: `  coffee-goat-pp-cli champion-replay list
  coffee-goat-pp-cli champion-replay list --year 2024
  coffee-goat-pp-cli champion-replay show wbrc-2024-style-yirgacheffe-anaerobic
  coffee-goat-pp-cli champion-replay shop wbrc-2023-style-bermudez-gesha --in-stock`,
	}
	cmd.AddCommand(newChampionReplayListCmd(flags))
	cmd.AddCommand(newChampionReplayShowCmd(flags))
	cmd.AddCommand(newChampionReplayShopCmd(flags))
	return cmd
}

func newChampionReplayListCmd(flags *rootFlags) *cobra.Command {
	var yearFilter int
	var compFilter string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List curated champion-style recipes (filterable by year and competition)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			filtered := []championRecipe{}
			for _, r := range championLibrary {
				if yearFilter != 0 && r.Year != yearFilter {
					continue
				}
				if compFilter != "" && !strings.EqualFold(r.Competition, compFilter) {
					continue
				}
				filtered = append(filtered, r)
			}
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].Year > filtered[j].Year
			})
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"recipes": filtered, "count": len(filtered)}, flags)
			}
			headers := []string{"slug", "year", "comp", "competitor", "bean", "origin", "process"}
			rows := make([][]string, 0, len(filtered))
			for _, r := range filtered {
				rows = append(rows, []string{r.Slug, fmt.Sprintf("%d", r.Year), r.Competition, r.Competitor, r.Bean, r.Origin, r.Process})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().IntVar(&yearFilter, "year", 0, "Filter recipes to a specific competition year (e.g. 2024 for WBC/WBrC 2024)")
	cmd.Flags().StringVar(&compFilter, "competition", "", "Filter by competition — WBC for Barista Championship or WBrC for Brewers Cup")
	return cmd
}

func newChampionReplayShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <slug>",
		Short:       "Show one championship-style recipe in detail (bean, producer, process, brew params)",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			r := championBySlug(args[0])
			if r == nil {
				return fmt.Errorf("champion recipe %q not found; try 'champion-replay list'", args[0])
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), r, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "  %d %s — %s, %s (%s)\n", r.Year, r.Competition, r.Competitor, r.Country, rankLabel(r.Rank))
			fmt.Fprintf(w, "  Bean:     %s — %s\n", r.Bean, r.Producer)
			fmt.Fprintf(w, "  Origin:   %s%s\n", r.Origin, regionSuffix(r.Region))
			fmt.Fprintf(w, "  Process:  %s\n", r.Process)
			fmt.Fprintf(w, "  Varietal: %s\n", r.Varietal)
			fmt.Fprintf(w, "  Method:   %s — %.0fg / 1:%.1f / %ds @ %.0f°C\n", r.Method, r.DoseG, r.Ratio, r.TimeS, r.TempC)
			fmt.Fprintf(w, "  Notes:    %s\n", r.Notes)
			fmt.Fprintf(w, "  Tasting:  %s\n", strings.Join(r.Descriptors, ", "))
			return nil
		},
	}
	return cmd
}

func newChampionReplayShopCmd(flags *rootFlags) *cobra.Command {
	var inStockOnly bool
	var limit int
	cmd := &cobra.Command{
		Use:         "shop <slug>",
		Short:       "Match a championship-style recipe to current lots in the synced corpus by origin/process/varietal",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			r := championBySlug(args[0])
			if r == nil {
				return fmt.Errorf("champion recipe %q not found", args[0])
			}
			ctx := cmd.Context()
			ensureFreshForResources(ctx, flags, "products")
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			matches, err := matchChampionToCorpus(db.DB(), r, inStockOnly, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"champion": r,
					"matches":  matches,
					"count":    len(matches),
				}, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  No current lots match %s. Try --in-stock=false or 'sync --source shopify' first.\n", r.Slug)
				return nil
			}
			headers := []string{"roaster", "handle", "title", "origin", "process", "varietal", "score"}
			rows := make([][]string, 0, len(matches))
			for _, m := range matches {
				rows = append(rows, []string{
					m.Roaster, m.Handle, m.Title, m.Origin, m.Process, m.Varietal,
					fmt.Sprintf("%.2f", m.Score),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().BoolVar(&inStockOnly, "in-stock", true, "Only show currently in-stock lots from the synced roaster_products corpus")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of matched in-stock lots to return in the response")
	return cmd
}

type championMatch struct {
	Roaster  string  `json:"roaster"`
	Handle   string  `json:"handle"`
	Title    string  `json:"title"`
	Origin   string  `json:"origin"`
	Process  string  `json:"process"`
	Varietal string  `json:"varietal"`
	Score    float64 `json:"score"`
}

// matchChampionToCorpus scores roaster_products against a champion recipe.
// Score = 0.5 * origin_match + 0.3 * process_overlap + 0.2 * varietal_match.
func matchChampionToCorpus(db *sql.DB, r *championRecipe, inStockOnly bool, limit int) ([]championMatch, error) {
	q := `SELECT roaster_slug, handle, COALESCE(title,''), COALESCE(origin,''), COALESCE(process,''), COALESCE(varietal,''), COALESCE(in_stock, 0) FROM roaster_products WHERE 1=1`
	if inStockOnly {
		q += ` AND in_stock = 1`
	}
	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query roaster_products: %w", err)
	}
	defer rows.Close()
	var results []championMatch
	processToks := strings.Fields(strings.ReplaceAll(r.Process, "-", " "))
	for rows.Next() {
		var m championMatch
		var inStock int
		if err := rows.Scan(&m.Roaster, &m.Handle, &m.Title, &m.Origin, &m.Process, &m.Varietal, &inStock); err != nil {
			continue
		}
		score := 0.0
		if strings.EqualFold(strings.TrimSpace(m.Origin), r.Origin) {
			score += 0.5
		}
		lowerProcess := strings.ToLower(m.Process)
		processHits := 0
		for _, tok := range processToks {
			if tok != "" && strings.Contains(lowerProcess, tok) {
				processHits++
			}
		}
		if len(processToks) > 0 {
			score += 0.3 * (float64(processHits) / float64(len(processToks)))
		}
		if strings.Contains(strings.ToLower(m.Varietal), strings.ToLower(r.Varietal)) {
			score += 0.2
		}
		if score >= 0.5 {
			m.Score = score
			results = append(results, m)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roaster_products rows: %w", err)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func rankLabel(rank int) string {
	switch rank {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", rank)
	}
}

func regionSuffix(region string) string {
	if region == "" {
		return ""
	}
	return " — " + region
}
