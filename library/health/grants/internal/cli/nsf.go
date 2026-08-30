package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

func cmdNSF(args []string) int {
	fs := flag.NewFlagSet("nsf", flag.ContinueOnError)
	minAmount := fs.Int64("min-amount", 0, "min. amount USD")
	rows := fs.Int("rows", 15, "number of results (max 25)")
	asJSON := fs.Bool("json", false, "JSON output")
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	keyword := strings.Join(pos, " ")
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "a keyword is required: grants-pp-cli nsf <keyword>")
		return 2
	}

	awards, stats, err := sources.SearchNSF(keyword, *rows)
	// A partial pool is not fatal — the awards already fetched are worth
	// showing — but it is still an error: the command keeps printing results
	// and warnings below and then exits non-zero, so a script checking $?
	// never mistakes an incomplete search for a completed one.
	partial := errors.Is(err, sources.ErrPartialPool)
	if err != nil && !partial {
		return fail(err)
	}

	// Paging stopped early on an upstream error, so the results below were
	// ranked from part of the intended pool. Reported before anything else:
	// the relevance counts and the "few awards match" advice further down are
	// both unreliable in this state, and a silent partial search would read as
	// a genuine shortage of matching awards.
	if stats.Partial() {
		fmt.Fprintf(os.Stderr,
			"  (warn: NSF returned an error after %d of %d pages; results are ranked from a partial pool and may be incomplete: %s)\n",
			stats.PagesFetched, stats.PagesPlanned, stats.PoolError)
	}

	shownBeforeAmount := len(awards)
	if *minAmount > 0 {
		var kept []sources.NSFAward
		for _, a := range awards {
			if sources.ParseMoney(a.FundsObligated) >= *minAmount {
				kept = append(kept, a)
			}
		}
		awards = kept
		// The amount filter runs on the rows we are about to show, not on the
		// whole candidate pool. When those rows filled the requested page,
		// matching awards below the cut almost certainly exist.
		if shownBeforeAmount == *rows && stats.Matched > *rows {
			fmt.Fprintf(os.Stderr,
				"  (warn: --min-amount applies only to the top %d ranked NSF results of %d matches; increase --rows to fetch more)\n",
				*rows, stats.Matched)
		}
	}

	if *asJSON {
		// incomplete/pages_* are top level on purpose: a JSON consumer must be
		// able to see that the search was cut short without reading stderr.
		if code := printJSON(map[string]any{
			"keyword":         keyword,
			"shown":           len(awards),
			"awards":          awards,
			"relevance":       stats,
			"incomplete":      partial,
			"pages_requested": stats.PagesPlanned,
			"pages_succeeded": stats.PagesFetched,
		}); code != 0 {
			return code
		}
		if partial {
			return 1
		}
		return 0
	}

	fmt.Printf("🔬 NSF %q — %d awarded grants shown\n", keyword, len(awards))
	for _, a := range awards {
		fmt.Printf("  %-9s %12s  %-10s→%-10s %-26s %s\n",
			a.ID, FormatMoney(sources.ParseMoney(a.FundsObligated)),
			a.StartDate, a.ExpDate, truncate(a.Awardee, 26), truncate(a.Title, 48))
	}
	if len(awards) == 0 {
		fmt.Println("  (no results)")
	}

	if stats.Partial() {
		fmt.Printf("  searched %d recent NSF awards from %d of %d pages; %d mention every word of your query, %d in the title\n",
			stats.Examined, stats.PagesFetched, stats.PagesPlanned, stats.Matched, stats.TitleMatched)
		fmt.Println("  the search was cut short by an upstream error, so these counts are a floor — retry for the full picture")
	} else {
		fmt.Printf("  searched %d recent NSF awards; %d mention every word of your query, %d in the title\n",
			stats.Examined, stats.Matched, stats.TitleMatched)
		// Only worth advising when the pool was complete. After a partial
		// fetch a low count says more about the failure than about the query.
		if stats.Matched < 10 {
			fmt.Println("  few awards use all of these words together — try fewer or broader words (e.g. one topic word instead of two)")
		}
	}
	if partial {
		return 1
	}
	return 0
}
