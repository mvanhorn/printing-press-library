// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// countryMapRule maps a substring of the free-text origin field to a
// canonical ISO 3166-1 country name + continent bucket. Matching is
// case-insensitive substring against `LOWER(roaster_products.origin)`,
// applied longest-first so "yirgacheffe" wins over "ethiopia".
//
// pp:novel-static-reference — curated reference data not synthesized
// from any API.
type countryMapRule struct {
	Match     string // lowercased substring; longest-first wins
	Country   string // canonical display name
	Continent string // Africa | Americas | Asia | Oceania
	ISO2      string
}

// curatedCountryRules covers the substring keywords most likely to
// appear in roaster_products.origin across the 24 curated roasters.
// Region keywords (Yirgacheffe, Huehuetenango, Nariño) come before
// country names so a fuller country reading wins out over a generic
// "Ethiopian" match.
//
// pp:novel-static-reference
var curatedCountryRules = []countryMapRule{
	// Africa
	{"yirgacheffe", "Ethiopia", "Africa", "ET"},
	{"sidamo", "Ethiopia", "Africa", "ET"},
	{"sidama", "Ethiopia", "Africa", "ET"},
	{"guji", "Ethiopia", "Africa", "ET"},
	{"limu", "Ethiopia", "Africa", "ET"},
	{"harrar", "Ethiopia", "Africa", "ET"},
	{"ethiopia", "Ethiopia", "Africa", "ET"},
	{"nyeri", "Kenya", "Africa", "KE"},
	{"kirinyaga", "Kenya", "Africa", "KE"},
	{"kiambu", "Kenya", "Africa", "KE"},
	{"kenya", "Kenya", "Africa", "KE"},
	{"rwanda", "Rwanda", "Africa", "RW"},
	{"burundi", "Burundi", "Africa", "BI"},
	{"tanzania", "Tanzania", "Africa", "TZ"},
	{"uganda", "Uganda", "Africa", "UG"},
	{"congo", "DR Congo", "Africa", "CD"},
	{"drc", "DR Congo", "Africa", "CD"},

	// Americas
	{"huila", "Colombia", "Americas", "CO"},
	{"narino", "Colombia", "Americas", "CO"},
	{"nariño", "Colombia", "Americas", "CO"},
	{"tolima", "Colombia", "Americas", "CO"},
	{"antioquia", "Colombia", "Americas", "CO"},
	{"colombia", "Colombia", "Americas", "CO"},
	{"brazil", "Brazil", "Americas", "BR"},
	{"brasil", "Brazil", "Americas", "BR"},
	{"minas gerais", "Brazil", "Americas", "BR"},
	{"sul de minas", "Brazil", "Americas", "BR"},
	{"cerrado", "Brazil", "Americas", "BR"},
	{"huehuetenango", "Guatemala", "Americas", "GT"},
	{"antigua", "Guatemala", "Americas", "GT"},
	{"guatemala", "Guatemala", "Americas", "GT"},
	{"costa rica", "Costa Rica", "Americas", "CR"},
	{"el salvador", "El Salvador", "Americas", "SV"},
	{"honduras", "Honduras", "Americas", "HN"},
	{"nicaragua", "Nicaragua", "Americas", "NI"},
	{"panama", "Panama", "Americas", "PA"},
	{"mexico", "Mexico", "Americas", "MX"},
	{"chiapas", "Mexico", "Americas", "MX"},
	{"oaxaca", "Mexico", "Americas", "MX"},
	{"peru", "Peru", "Americas", "PE"},
	{"perú", "Peru", "Americas", "PE"},
	{"bolivia", "Bolivia", "Americas", "BO"},
	{"ecuador", "Ecuador", "Americas", "EC"},
	{"jamaica", "Jamaica", "Americas", "JM"},
	{"dominican", "Dominican Republic", "Americas", "DO"},
	{"haiti", "Haiti", "Americas", "HT"},

	// Asia / Pacific
	{"sumatra", "Indonesia", "Asia", "ID"},
	{"java", "Indonesia", "Asia", "ID"},
	{"sulawesi", "Indonesia", "Asia", "ID"},
	{"bali", "Indonesia", "Asia", "ID"},
	{"flores", "Indonesia", "Asia", "ID"},
	{"aceh", "Indonesia", "Asia", "ID"},
	{"indonesia", "Indonesia", "Asia", "ID"},
	{"vietnam", "Vietnam", "Asia", "VN"},
	{"papua", "Papua New Guinea", "Oceania", "PG"},
	{"png", "Papua New Guinea", "Oceania", "PG"},
	{"yemen", "Yemen", "Asia", "YE"},
	{"india", "India", "Asia", "IN"},
	{"china", "China", "Asia", "CN"},
	{"yunnan", "China", "Asia", "CN"},
	{"thailand", "Thailand", "Asia", "TH"},
	{"laos", "Laos", "Asia", "LA"},
	{"philippines", "Philippines", "Asia", "PH"},
	{"hawaii", "Hawaii (US)", "Oceania", "US"},
	{"kona", "Hawaii (US)", "Oceania", "US"},
	{"taiwan", "Taiwan", "Asia", "TW"},
}

// curatedWorldOrigins is the static list used when --world is passed —
// every specialty-coffee producing country we curate for, even those
// not currently in any synced roaster's catalog.
//
// pp:novel-static-reference
var curatedWorldOrigins = []countryMapRule{
	// Africa
	{"", "Ethiopia", "Africa", "ET"},
	{"", "Kenya", "Africa", "KE"},
	{"", "Rwanda", "Africa", "RW"},
	{"", "Burundi", "Africa", "BI"},
	{"", "Tanzania", "Africa", "TZ"},
	{"", "Uganda", "Africa", "UG"},
	{"", "DR Congo", "Africa", "CD"},
	{"", "Malawi", "Africa", "MW"},
	{"", "Zambia", "Africa", "ZM"},
	{"", "Cameroon", "Africa", "CM"},
	// Americas
	{"", "Colombia", "Americas", "CO"},
	{"", "Brazil", "Americas", "BR"},
	{"", "Guatemala", "Americas", "GT"},
	{"", "Costa Rica", "Americas", "CR"},
	{"", "El Salvador", "Americas", "SV"},
	{"", "Honduras", "Americas", "HN"},
	{"", "Nicaragua", "Americas", "NI"},
	{"", "Panama", "Americas", "PA"},
	{"", "Mexico", "Americas", "MX"},
	{"", "Peru", "Americas", "PE"},
	{"", "Bolivia", "Americas", "BO"},
	{"", "Ecuador", "Americas", "EC"},
	{"", "Jamaica", "Americas", "JM"},
	{"", "Dominican Republic", "Americas", "DO"},
	{"", "Haiti", "Americas", "HT"},
	{"", "Venezuela", "Americas", "VE"},
	// Asia / Pacific
	{"", "Indonesia", "Asia", "ID"},
	{"", "Vietnam", "Asia", "VN"},
	{"", "Yemen", "Asia", "YE"},
	{"", "India", "Asia", "IN"},
	{"", "China", "Asia", "CN"},
	{"", "Thailand", "Asia", "TH"},
	{"", "Laos", "Asia", "LA"},
	{"", "Philippines", "Asia", "PH"},
	{"", "Papua New Guinea", "Oceania", "PG"},
	{"", "Hawaii (US)", "Oceania", "US"},
	{"", "Taiwan", "Asia", "TW"},
}

// canonicalCountry maps a free-text origin string to a curated country.
// Returns "Unknown" when no rule matches. Longest-first match so
// region keywords win over country-name keywords. Case-insensitive.
func canonicalCountry(rawOrigin string) (string, string) {
	lo := strings.ToLower(strings.TrimSpace(rawOrigin))
	if lo == "" {
		return "Unknown", "Unknown"
	}
	// Build longest-first ordering once per invocation (cheap; small slice).
	rules := append([]countryMapRule{}, curatedCountryRules...)
	sort.SliceStable(rules, func(i, j int) bool { return len(rules[i].Match) > len(rules[j].Match) })
	for _, r := range rules {
		if strings.Contains(lo, r.Match) {
			return r.Country, r.Continent
		}
	}
	return "Unknown", "Unknown"
}

// coffeeMapCountry is one row in the coverage table.
type coffeeMapCountry struct {
	Country       string             `json:"country"`
	Continent     string             `json:"continent"`
	ISO2          string             `json:"iso2,omitempty"`
	Producers     int                `json:"producers"`
	Bags          int                `json:"bags"`
	BrewsLogged   int                `json:"brews_logged"`
	BestRating    int                `json:"best_rating,omitempty"`
	Depth         string             `json:"depth"`
	InCorpusNow   int                `json:"in_corpus_now"`
	FillSuggested []coffeeMapFillRow `json:"fill_suggested,omitempty"`
}

type coffeeMapFillRow struct {
	Roaster string `json:"roaster"`
	Handle  string `json:"handle"`
	Title   string `json:"title"`
	CRScore int    `json:"cr_score,omitempty"`
	Process string `json:"process,omitempty"`
	URL     string `json:"url,omitempty"`
}

type coffeeMapOutput struct {
	Universe       string                      `json:"universe"`
	Summary        coffeeMapSummary            `json:"summary"`
	Continents     []coffeeMapContinentSection `json:"continents"`
	UnmatchedCount int                         `json:"unmatched_origin_count,omitempty"`
}

type coffeeMapSummary struct {
	TotalCountries  int    `json:"total_countries"`
	Tasted          int    `json:"tasted"`
	Explored        int    `json:"explored"`
	Deep            int    `json:"deep"`
	Unexplored      int    `json:"unexplored"`
	CoveragePercent int    `json:"coverage_percent"`
	Sentence        string `json:"sentence"`
}

type coffeeMapContinentSection struct {
	Continent string             `json:"continent"`
	Tasted    int                `json:"tasted"`
	Total     int                `json:"total"`
	Countries []coffeeMapCountry `json:"countries"`
}

func newCoffeeMapCmd(flags *rootFlags) *cobra.Command {
	var (
		useWorld bool
		withFill bool
	)
	cmd := &cobra.Command{
		Use:   "coffee-map",
		Short: "Origin coverage tracker: countries tasted vs gaps in the synced corpus (or curated world map)",
		Long: `Country-primary coverage with per-country producer/brew/best-rating
depth signal. Default universe is the synced corpus (countries appearing
in roaster_products); --world swaps to the curated specialty-origin
list (Ethiopia, Kenya, ... Burundi, Yemen, PNG).

Each country gets a depth label:
  unexplored = 0 brews
  tasted     = ≥1 brew
  explored   = ≥3 producers OR ≥5 brews
  deep       = ≥5 producers OR ≥15 brews

With --fill, each gap row gains a suggested currently-stocked bag (CR
score primary, user-process-preference tiebreak). See also
'coffee-map fill --limit N' for a ranked list across all gaps.`,
		Example: `  coffee-goat-pp-cli coffee-map
  coffee-goat-pp-cli coffee-map --world
  coffee-goat-pp-cli coffee-map --fill
  coffee-goat-pp-cli coffee-map fill --limit 5`,
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
			out, err := buildCoffeeMap(db, useWorld, withFill)
			if err != nil {
				return err
			}
			if out.UnmatchedCount > 0 {
				fmt.Fprintf(os.Stderr, "→ %d origin strings unmatched (consider adding country rules)\n", out.UnmatchedCount)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			renderCoffeeMap(cmd, out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&useWorld, "world", false, "Use the curated world-map list instead of the synced corpus")
	cmd.Flags().BoolVar(&withFill, "fill", false, "Add 1 suggested currently-stocked bag per gap row")
	cmd.AddCommand(newCoffeeMapFillCmd(flags))
	return cmd
}

func newCoffeeMapFillCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "fill",
		Short:       "Rank currently-stocked bags that would close unexplored-country gaps (CR score, process tiebreak)",
		Example:     `  coffee-goat-pp-cli coffee-map fill --limit 5`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit <= 0 {
				limit = 5
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			picks, err := rankGapFillers(db, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), picks, flags)
			}
			if len(picks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no gap-filler candidates (every synced country is tasted, or no in-stock bags)")
				return nil
			}
			for _, p := range picks {
				score := ""
				if p.CRScore > 0 {
					score = fmt.Sprintf(" CR=%d", p.CRScore)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s/%s — %s  [country=%s%s]\n",
					p.Roaster, p.Handle, p.Title, p.Country, score)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Max ranked gap-fillers returned")
	return cmd
}

// buildCoffeeMap aggregates origin coverage. Two passes:
//  1. Walk roaster_products to count producers and in-stock listings
//     per canonical country.
//  2. Walk brews → beans → roaster_products to count user brews/rating
//     per canonical country.
func buildCoffeeMap(db *store.Store, useWorld, withFill bool) (coffeeMapOutput, error) {
	corpusByCountry, unmatched, err := corpusCountryStats(db)
	if err != nil {
		return coffeeMapOutput{}, err
	}
	userByCountry, err := userCountryStats(db)
	if err != nil {
		return coffeeMapOutput{}, err
	}
	universe := "synced-corpus"
	countries := map[string]*coffeeMapCountry{}
	if useWorld {
		universe = "world-curated"
		for _, w := range curatedWorldOrigins {
			countries[w.Country] = &coffeeMapCountry{
				Country: w.Country, Continent: w.Continent, ISO2: w.ISO2,
			}
		}
	}
	for country, stats := range corpusByCountry {
		if _, ok := countries[country]; !ok {
			if useWorld {
				continue
			}
			countries[country] = &coffeeMapCountry{
				Country: country, Continent: stats.continent, ISO2: stats.iso2,
			}
		}
		countries[country].InCorpusNow = stats.inStock
		if countries[country].Continent == "" {
			countries[country].Continent = stats.continent
		}
		if countries[country].ISO2 == "" {
			countries[country].ISO2 = stats.iso2
		}
	}
	for country, ustats := range userByCountry {
		if _, ok := countries[country]; !ok {
			// User has brewed a country not in --world set; include anyway.
			countries[country] = &coffeeMapCountry{Country: country, Continent: ustats.continent}
		}
		c := countries[country]
		c.Producers = ustats.producers
		c.Bags = ustats.bags
		c.BrewsLogged = ustats.brews
		c.BestRating = ustats.bestRating
		c.Depth = depthLabel(ustats.producers, ustats.brews)
	}
	for _, c := range countries {
		if c.Depth == "" {
			c.Depth = "unexplored"
		}
	}
	if withFill {
		fills, _ := rankGapFillers(db, 100)
		fillByCountry := map[string][]coffeeMapFillRow{}
		for _, f := range fills {
			fillByCountry[f.Country] = append(fillByCountry[f.Country], f.coffeeMapFillRow)
		}
		for country, c := range countries {
			if c.Depth != "unexplored" {
				continue
			}
			if pick := fillByCountry[country]; len(pick) > 0 {
				c.FillSuggested = []coffeeMapFillRow{pick[0]}
			}
		}
	}
	out := groupByContinent(countries, universe, unmatched)
	return out, nil
}

type corpusCountryStat struct {
	continent string
	iso2      string
	inStock   int
}

func corpusCountryStats(db *store.Store) (map[string]corpusCountryStat, int, error) {
	rows, err := db.DB().Query(
		`SELECT COALESCE(origin,''), COALESCE(in_stock,0) FROM roaster_products`,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[string]corpusCountryStat{}
	unmatched := 0
	for rows.Next() {
		var origin string
		var inStock int
		if err := rows.Scan(&origin, &inStock); err != nil {
			return nil, 0, err
		}
		country, continent := canonicalCountry(origin)
		if country == "Unknown" {
			unmatched++
			continue
		}
		stat := out[country]
		stat.continent = continent
		if inStock == 1 {
			stat.inStock++
		}
		// iso2 lookup from rules cache.
		for _, r := range curatedCountryRules {
			if r.Country == country {
				stat.iso2 = r.ISO2
				break
			}
		}
		out[country] = stat
	}
	return out, unmatched, rows.Err()
}

type userCountryStat struct {
	continent  string
	producers  int
	bags       int
	brews      int
	bestRating int
}

func userCountryStats(db *store.Store) (map[string]userCountryStat, error) {
	q := `SELECT COALESCE(rp.origin,''), COALESCE(rp.producer,''),
	             b.bean_id, COALESCE(b.rating,0)
	      FROM brews b
	      JOIN beans bn ON b.bean_id = bn.id
	      LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle`
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type acc struct {
		continent  string
		producers  map[string]bool
		bags       map[int64]bool
		brews      int
		bestRating int
	}
	bucket := map[string]*acc{}
	for rows.Next() {
		var origin, producer string
		var beanID int64
		var rating int
		if err := rows.Scan(&origin, &producer, &beanID, &rating); err != nil {
			return nil, err
		}
		country, continent := canonicalCountry(origin)
		if country == "Unknown" {
			continue
		}
		if bucket[country] == nil {
			bucket[country] = &acc{continent: continent, producers: map[string]bool{}, bags: map[int64]bool{}}
		}
		bucket[country].brews++
		if producer != "" {
			bucket[country].producers[strings.ToLower(producer)] = true
		}
		if beanID > 0 {
			bucket[country].bags[beanID] = true
		}
		if rating > bucket[country].bestRating {
			bucket[country].bestRating = rating
		}
	}
	out := map[string]userCountryStat{}
	for k, v := range bucket {
		out[k] = userCountryStat{
			continent: v.continent, producers: len(v.producers), bags: len(v.bags),
			brews: v.brews, bestRating: v.bestRating,
		}
	}
	return out, rows.Err()
}

// depthLabel applies the spec's tiered labels (unexplored / tasted /
// explored / deep) to a (producers, brews) pair.
func depthLabel(producers, brews int) string {
	switch {
	case brews >= 15 || producers >= 5:
		return "deep"
	case producers >= 3 || brews >= 5:
		return "explored"
	case brews >= 1:
		return "tasted"
	default:
		return "unexplored"
	}
}

// groupByContinent assembles the final coffeeMapOutput, computing
// summary counts and ordering continents Africa→Americas→Asia→Oceania.
func groupByContinent(countries map[string]*coffeeMapCountry, universe string, unmatched int) coffeeMapOutput {
	byContinent := map[string][]coffeeMapCountry{}
	total, tasted, explored, deep, unexplored := 0, 0, 0, 0, 0
	for _, c := range countries {
		byContinent[c.Continent] = append(byContinent[c.Continent], *c)
		total++
		switch c.Depth {
		case "deep":
			deep++
			tasted++
		case "explored":
			explored++
			tasted++
		case "tasted":
			tasted++
		default:
			unexplored++
		}
	}
	cov := 0
	if total > 0 {
		cov = int(float64(tasted) / float64(total) * 100)
	}
	order := []string{"Africa", "Americas", "Asia", "Oceania", "Unknown"}
	var continents []coffeeMapContinentSection
	for _, name := range order {
		if list, ok := byContinent[name]; ok {
			sort.Slice(list, func(i, j int) bool {
				di, dj := depthOrder(list[i].Depth), depthOrder(list[j].Depth)
				if di != dj {
					return di < dj
				}
				return list[i].Country < list[j].Country
			})
			t := 0
			for _, c := range list {
				if c.Depth != "unexplored" {
					t++
				}
			}
			continents = append(continents, coffeeMapContinentSection{
				Continent: name, Tasted: t, Total: len(list), Countries: list,
			})
		}
	}
	return coffeeMapOutput{
		Universe: universe,
		Summary: coffeeMapSummary{
			TotalCountries:  total,
			Tasted:          tasted,
			Explored:        explored,
			Deep:            deep,
			Unexplored:      unexplored,
			CoveragePercent: cov,
			Sentence: fmt.Sprintf("%d of %d countries tasted (%d%%); %d deep, %d explored, %d tasted, %d unexplored",
				tasted, total, cov, deep, explored, tasted-explored-deep, unexplored),
		},
		Continents:     continents,
		UnmatchedCount: unmatched,
	}
}

func depthOrder(d string) int {
	switch d {
	case "deep":
		return 0
	case "explored":
		return 1
	case "tasted":
		return 2
	case "unexplored":
		return 3
	default:
		return 4
	}
}

// gapFillerRow is the cross-gap ranking row for `coffee-map fill`.
type gapFillerRow struct {
	Country string `json:"country"`
	coffeeMapFillRow
}

// rankGapFillers walks roaster_products for in-stock bags, identifies
// unexplored countries (no brews logged), and ranks candidates by CR
// score with a user-process-preference tiebreak.
func rankGapFillers(db *store.Store, limit int) ([]gapFillerRow, error) {
	userStats, err := userCountryStats(db)
	if err != nil {
		return nil, err
	}
	processPref := userProcessRatings(db)
	q := `SELECT COALESCE(roaster_slug,''), COALESCE(handle,''), COALESCE(title,''),
	             COALESCE(origin,''), COALESCE(process,''), COALESCE(url,'')
	      FROM roaster_products
	      WHERE in_stock = 1`
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type cand struct {
		row     gapFillerRow
		process string
	}
	candidates := map[string][]cand{}
	for rows.Next() {
		var r, h, title, origin, process, url string
		if err := rows.Scan(&r, &h, &title, &origin, &process, &url); err != nil {
			return nil, err
		}
		country, _ := canonicalCountry(origin)
		if country == "Unknown" {
			continue
		}
		ustats := userStats[country]
		if ustats.brews > 0 {
			continue
		}
		row := gapFillerRow{
			Country: country,
			coffeeMapFillRow: coffeeMapFillRow{
				Roaster: r, Handle: h, Title: title, Process: process, URL: url,
			},
		}
		if score, ok := lookupCoffeeReviewScore(db, r, title); ok {
			row.CRScore = score
		}
		candidates[country] = append(candidates[country], cand{row: row, process: strings.ToLower(process)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []gapFillerRow
	for _, list := range candidates {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].row.CRScore != list[j].row.CRScore {
				return list[i].row.CRScore > list[j].row.CRScore
			}
			pi := processPref[list[i].process]
			pj := processPref[list[j].process]
			return pi > pj
		})
		if len(list) > 0 {
			out = append(out, list[0].row)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CRScore > out[j].CRScore })
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

// userProcessRatings returns the mean user rating per process (washed,
// natural, honey, anaerobic). Used as a tiebreak in fill ranking.
func userProcessRatings(db *store.Store) map[string]float64 {
	out := map[string]float64{}
	rows, err := db.DB().Query(
		`SELECT LOWER(COALESCE(rp.process,'')), AVG(b.rating)
		 FROM brews b
		 JOIN beans bn ON b.bean_id = bn.id
		 LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle
		 WHERE b.rating > 0 AND rp.process IS NOT NULL AND rp.process <> ''
		 GROUP BY LOWER(rp.process)`,
	)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var process string
		var avg float64
		if err := rows.Scan(&process, &avg); err == nil {
			out[process] = avg
		}
	}
	_ = rows.Err()
	return out
}

// renderCoffeeMap prints the human-readable summary + continent-grouped table.
func renderCoffeeMap(cmd *cobra.Command, out coffeeMapOutput) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "→ %s\n", out.Summary.Sentence)
	fmt.Fprintf(w, "  universe: %s\n", out.Universe)
	for _, sec := range out.Continents {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s (%d of %d)\n", sec.Continent, sec.Tasted, sec.Total)
		for _, c := range sec.Countries {
			tag := c.Depth
			if c.InCorpusNow > 0 && c.Depth == "unexplored" {
				tag = fmt.Sprintf("unexplored (%d in stock)", c.InCorpusNow)
			}
			rating := ""
			if c.BestRating > 0 {
				rating = fmt.Sprintf("  best=%d", c.BestRating)
			}
			fmt.Fprintf(w, "  %-22s  %-10s  prod=%d bags=%d brews=%d%s\n",
				c.Country, tag, c.Producers, c.Bags, c.BrewsLogged, rating)
			for _, f := range c.FillSuggested {
				fmt.Fprintf(w, "      → %s/%s — %s\n", f.Roaster, f.Handle, f.Title)
			}
		}
	}
}

// Allow embedding coffeeMapFillRow into gapFillerRow without ambiguity.
var _ sql.Result = (sql.Result)(nil)
