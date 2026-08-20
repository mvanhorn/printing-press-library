package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

func cmdSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	closingBefore := fs.String("closing-before", "", "only those open until this deadline (YYYY-MM-DD)")
	agency := fs.String("agency", "", "agency code, e.g. HHS-NIH11, NSF")
	rows := fs.Int("rows", 15, "number of results")
	details := fs.Bool("details", false, "fetch award ceiling + eligibility")
	minAward := fs.Int64("min-award", 0, "min. award amount USD (implies detail fetching)")
	eligibility := fs.String("eligibility", "", "eligibility filter, substring")
	asJSON := fs.Bool("json", false, "JSON output")
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	keyword := strings.Join(pos, " ")
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "a keyword is required: grants-pp-cli search <keyword>")
		return 2
	}

	var cutoff time.Time
	if *closingBefore != "" {
		cutoff, err = time.Parse("2006-01-02", *closingBefore)
		if err != nil {
			return fail(fmt.Errorf("--closing-before format: YYYY-MM-DD (got: %q)", *closingBefore))
		}
	}

	opps, total, err := sources.SearchOpportunities(keyword, *agency, *rows)
	if err != nil {
		return fail(err)
	}

	// Capture initial fetch count before any filtering; used to detect
	// truncation for client-side filters that only see the first page.
	initialFetched := len(opps)

	if !cutoff.IsZero() {
		opps = ClosingBefore(opps, cutoff)
		if initialFetched == *rows {
			fmt.Fprintf(os.Stderr, "warning: results may be truncated (deadline filter applied to first %d rows only)\n", *rows)
		}
	}

	needDetails := *details || *minAward > 0 || *eligibility != ""
	if needDetails {
		var kept []sources.Opportunity
		var eligibilitySkipped int
		for _, o := range opps {
			d, derr := sources.FetchDetails(o.ID)
			if derr != nil {
				fmt.Fprintf(os.Stderr, "  (warn: %v — skipped)\n", derr)
				continue
			}
			o.Details = d
			if *minAward > 0 && d.AwardCap() < *minAward {
				continue
			}
			if !EligibilityMatches(d.ApplicantTypes, *eligibility) {
				if *eligibility != "" && len(d.ApplicantTypes) == 0 {
					eligibilitySkipped++
				}
				continue
			}
			kept = append(kept, o)
		}
		opps = kept

		// Warn when the initial page was full: higher-award grants on subsequent
		// pages are silently missed because Grants.gov has no amount sort.
		if *minAward > 0 && initialFetched == *rows {
			fmt.Fprintf(os.Stderr, "  (warning: --min-award filter applies only to the first %d fetched results; increase --rows to fetch more)\n", *rows)
		}

		// Same page-full truncation risk for the eligibility filter: it is applied
		// client-side over the first page only, so matching opportunities on later
		// pages are dropped silently. Kept independent of the --min-award guard so
		// both warnings surface when both filters are active.
		if *eligibility != "" && initialFetched == *rows && total > initialFetched {
			fmt.Fprintf(os.Stderr, "  (warning: --eligibility filter applies only to the first %d fetched results; increase --rows to fetch more)\n", *rows)
		}

		// Warn when opportunities were dropped solely because applicant-type
		// metadata was absent, not because they genuinely do not match.
		if eligibilitySkipped > 0 {
			fmt.Fprintf(os.Stderr, "  (warning: %d opportunity/opportunities excluded because applicant-type metadata was absent; they may still be eligible — verify manually)\n", eligibilitySkipped)
		}
	}

	if *asJSON {
		return printJSON(map[string]any{"keyword": keyword, "totalHits": total, "shown": len(opps), "opportunities": opps})
	}
	fmt.Printf("🔎 %q — %d open opportunities total, %d shown, newest posting first", keyword, total, len(opps))
	if !cutoff.IsZero() {
		fmt.Printf(" (deadline ≤ %s)", cutoff.Format("2006-01-02"))
	}
	fmt.Println()
	for _, o := range opps {
		deadline := o.CloseDate
		if !o.HasDeadline() {
			deadline = "not stated"
		}
		marker := ""
		if o.Stale() {
			marker = "  [posted " + o.OpenDate + "]"
		}
		fmt.Printf("  %-14s %-9s closes: %-10s  %s%s\n", o.Number, o.AgencyCode, deadline, truncate(o.Title, 70), marker)
		if o.Details != nil {
			label := "ceiling"
			if o.Details.AwardCeiling == 0 && o.Details.EstimatedFunding > 0 {
				label = "estimated" // estimatedFunding, not a hard ceiling
			}
			fmt.Printf("  %14s %s: %s", "", label, FormatMoney(o.Details.AwardCap()))
			if len(o.Details.ApplicantTypes) > 0 {
				fmt.Printf("  · eligible: %s", truncate(strings.Join(o.Details.ApplicantTypes, "; "), 80))
			}
			fmt.Println()
		}
	}
	if len(opps) == 0 {
		fmt.Println("  (no results with these filters)")
	}

	// "posted" means published and never closed, not that applications are
	// still accepted; Grants.gov leaves stale records in that state for years.
	if stale := sources.CountStale(opps); stale > 0 {
		fmt.Fprintf(os.Stderr,
			"  (warning: %d of %d shown were posted over two years ago; \"posted\" does not guarantee they still accept applications — verify at the agency)\n",
			stale, len(opps))
	}

	return 0
}
func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fail(err)
	}
	return 0
}
