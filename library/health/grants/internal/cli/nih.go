package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

func cmdNIH(args []string) int {
	fs := flag.NewFlagSet("nih", flag.ContinueOnError)
	minAmount := fs.Int64("min-amount", 0, "min. award USD")
	year := fs.Int("year", 0, "fiscal year, e.g. 2025")
	rows := fs.Int("rows", 15, "number of results")
	includeCenters := fs.Bool("include-centers", false,
		"also include center, program and consortium awards (P30, U01, UM1, ...)")
	asJSON := fs.Bool("json", false, "JSON output")
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	keyword := strings.Join(pos, " ")
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "a keyword is required: grants-pp-cli nih <keyword>")
		return 2
	}

	query := sources.NIHQuery{
		Keyword:       keyword,
		FiscalYear:    *year,
		MinAmount:     *minAmount,
		Limit:         *rows,
		IncludeCenter: *includeCenters,
	}

	projects, total, err := sources.SearchNIH(query)
	if err != nil {
		return fail(err)
	}

	// The listing is sorted by amount descending, which shows the largest
	// award records rather than a representative one. The estimate below is what
	// answers "how much is granted for this topic", so it is fetched
	// alongside. A failure here must not sink the listing.
	typical, typicalErr := sources.TypicalNIHAward(query)

	if *asJSON {
		out := map[string]any{
			"keyword": keyword,
			// RePORTER returns one record per support year, so this is a count
			// of award records, not of distinct awarded projects.
			"total":           total,
			"total_unit":      "matching award records (one per support year), not distinct projects",
			"shown":           len(projects),
			"include_centers": *includeCenters,
			"projects":        projects,
		}
		if typicalErr == nil && typical.Population > 0 {
			// Reported as an estimate with its bounds, not as an exact order
			// statistic: it is located by counting award records above a series of
			// amounts, so the true middle sits somewhere in [low, high].
			out["typical_award"] = map[string]any{
				"population":      typical.Population,
				"population_unit": "matching award records (one per support year), not distinct projects",
				"estimate":        typical.Estimate,
				"bracket_low":     typical.Low,
				"bracket_high":    typical.High,
				"method":          "count-based bisection on award amount over award records; estimate is the midpoint of the bracket",
			}
		}
		return printJSON(out)
	}

	scope := "research grants"
	if *includeCenters {
		scope = "research grants + centers/consortia"
	}
	fmt.Printf("🏥 NIH RePORTER %q — %d matching award records total (one per support year, so fewer distinct projects), %d shown (%s, descending by award)\n",
		keyword, total, len(projects), scope)

	for _, p := range projects {
		fmt.Printf("  %-16s %12s  FY%-5d %-5s %-28s %s\n",
			p.ProjectNum, FormatMoney(int64(p.AwardAmount)), p.FiscalYear,
			p.ActivityCode, truncate(p.Org.Name, 28), truncate(p.Title, 48))
	}
	if len(projects) == 0 {
		fmt.Println("  (no results)")
	}

	if typicalErr == nil && typical.Population > 0 {
		// The bracket is how far the bisection narrowed the median's LOCATION,
		// not how the awards themselves are spread. Award sizes are heavily
		// right-skewed: the same result set carries multi-million dollar rows,
		// so claiming half of them sit inside a few-thousand-dollar band would
		// be plainly false. Word it as uncertainty about the median.
		if typical.High > 0 {
			fmt.Printf("\n  Typical award: about %s — the median across %d award records lies between %s and %s\n",
				FormatMoney(typical.Estimate), typical.Population,
				FormatMoney(typical.Low), FormatMoney(typical.High))
		} else {
			fmt.Printf("\n  Typical award: above %s (%d matching award records)\n",
				FormatMoney(typical.Low), typical.Population)
		}
		fmt.Println("  The list above is sorted by size, so it shows the largest award records, not typical ones.")
		fmt.Println("  Each record is one support year of a project, and the estimate above is computed over those records.")
	}
	if !*includeCenters {
		fmt.Println("  Center, program and consortium awards are excluded. Add --include-centers to see them.")
	}

	return 0
}
