// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: salary distribution over live SEEK listings.
// pp:data-source live

package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/seekparse"
)

type salaryBucket struct {
	From  float64 `json:"from"`
	Count int     `json:"count"`
}

type salaryView struct {
	Role            string         `json:"role,omitempty"`
	Where           string         `json:"where,omitempty"`
	Site            string         `json:"site"`
	Classification  string         `json:"classification,omitempty"`
	TotalMatches    int            `json:"total_matches"`
	ScannedListings int            `json:"scanned_listings"`
	WithSalary      int            `json:"listings_with_salary"`
	DisclosureRate  float64        `json:"disclosure_rate"`
	Currency        string         `json:"currency"`
	P10             float64        `json:"p10"`
	P25             float64        `json:"p25"`
	Median          float64        `json:"p50"`
	P75             float64        `json:"p75"`
	P90             float64        `json:"p90"`
	Histogram       []salaryBucket `json:"histogram,omitempty"`
	JobID           string         `json:"job_id,omitempty"`
	JobSalary       float64        `json:"job_salary,omitempty"`
	JobPercentile   float64        `json:"job_percentile,omitempty"`
	MaxScanPages    int            `json:"max_scan_pages"`
	Note            string         `json:"note,omitempty"`
}

func newNovelSalaryCmd(flags *rootFlags) *cobra.Command {
	var flagWhere, flagSite, flagClassification, flagJob string
	var flagMaxScanPages, flagBuckets int

	cmd := &cobra.Command{
		Use:   "salary [role]",
		Short: "Salary percentiles, histogram, and pay-disclosure rate for a role and location",
		Long: "Use salary for percentile/histogram statistics on disclosed pay from SEEK listings.\n" +
			"It fetches and annualises the salary text of matching listings, then reports p10-p90,\n" +
			"a histogram, and how many listings disclose pay at all. Pass --job <id> to see where\n" +
			"one listing sits in that distribution.\n\n" +
			"Do NOT use it for a live count of jobs in a pay band - that is 'listings facets'.\n" +
			"Do NOT use it for listing-volume trends over time - that is 'trends'.",
		Example: strings.Trim(`
  seek-pp-cli salary "registered nurse" --where "Melbourne VIC" --agent
  seek-pp-cli salary "software engineer" --where "Sydney NSW" --job 94483533`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:no-error-path-probe": "true", "pp:happy-args": "role=registered nurse;--where=Melbourne VIC;--max-scan-pages=2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "salary")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			role := ""
			if len(args) > 0 {
				role = strings.Join(args, " ")
			}
			if role == "" && flagClassification == "" && flagWhere == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a role, --where, or --classification"))
			}

			maxPages := flagMaxScanPages
			if cliutil.IsDogfoodEnv() && maxPages > 2 {
				maxPages = 2
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			opts := seekSearchOpts{
				Keywords:       role,
				Where:          flagWhere,
				SiteKey:        flagSite,
				Classification: flagClassification,
				Salarytype:     "annual",
			}
			if strings.HasSuffix(flagSite, "NZ-Main") {
				opts.Locale = "en-NZ"
			}
			jobs, total, capHit, err := scanSearch(ctx, c, opts, maxPages)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			var salaries []float64
			for _, j := range jobs {
				if r := seekparse.ParseSalary(j.SalaryLabel); r.Ok {
					salaries = append(salaries, r.Midpoint())
				}
			}
			// Clip extreme tails (exec packages, casual-rate parse artifacts)
			// so a handful of outliers can't stretch the histogram or skew p90.
			stats := seekparse.Winsorize(salaries, 2.5, 97.5)

			// Cache what we fetched so trends and offline search benefit.
			if dbPath := defaultDBPath("seek-pp-cli"); dbPath != "" {
				_, _ = cacheJobs(dbPath, jobs)
			}

			buckets := flagBuckets
			if buckets < 2 {
				buckets = 8
			}
			bounds, counts := seekparse.Histogram(stats, buckets)
			hist := make([]salaryBucket, 0, len(bounds))
			for i := range bounds {
				hist = append(hist, salaryBucket{From: bounds[i], Count: counts[i]})
			}

			view := salaryView{
				Role:            role,
				Where:           flagWhere,
				Site:            firstNonEmpty(flagSite, "AU-Main"),
				Classification:  flagClassification,
				TotalMatches:    total,
				ScannedListings: len(jobs),
				WithSalary:      len(salaries),
				Currency:        currencyForSite(flagSite),
				MaxScanPages:    maxPages,
				Histogram:       hist,
			}
			if len(jobs) > 0 {
				view.DisclosureRate = math.Round(float64(len(salaries))/float64(len(jobs))*1000) / 1000
			}
			if len(stats) > 0 {
				view.P10 = seekparse.Percentile(stats, 10)
				view.P25 = seekparse.Percentile(stats, 25)
				view.Median = seekparse.Percentile(stats, 50)
				view.P75 = seekparse.Percentile(stats, 75)
				view.P90 = seekparse.Percentile(stats, 90)
			}

			if flagJob != "" {
				jd, jErr := fetchJobDetails(ctx, c, flagJob, flagSite)
				if jErr == nil {
					if r := seekparse.ParseSalary(jd.SalaryLabel); r.Ok && len(salaries) > 0 {
						view.JobID = flagJob
						view.JobSalary = r.Midpoint()
						below := 0
						for _, s := range salaries {
							if s <= r.Midpoint() {
								below++
							}
						}
						view.JobPercentile = math.Round(float64(below)/float64(len(salaries))*1000) / 10
					}
				}
			}

			if len(salaries) == 0 {
				if capHit {
					view.Note = fmt.Sprintf("scanned %d listings across up to %d pages; none disclosed pay. Widen --where, drop --classification, or raise --max-scan-pages.", len(jobs), maxPages)
				} else {
					view.Note = "no listings in this query disclosed a parseable salary."
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printSalaryHuman(cmd, view)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWhere, "where", "", "Location: suburb, city, state, or 'All Australia' / 'All New Zealand'")
	cmd.Flags().StringVar(&flagSite, "site", "AU-Main", "Marketplace: AU-Main or NZ-Main")
	cmd.Flags().StringVar(&flagClassification, "classification", "", "Classification ID to scope the sample (see 'classifications')")
	cmd.Flags().StringVar(&flagJob, "job", "", "Job ID to rank inside the distribution")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 8, "Maximum search pages to sample (100 listings/page)")
	cmd.Flags().IntVar(&flagBuckets, "buckets", 8, "Number of histogram bins")
	return cmd
}

func currencyForSite(site string) string {
	if strings.Contains(site, "NZ") {
		return "NZD"
	}
	return "AUD"
}

func printSalaryHuman(cmd *cobra.Command, v salaryView) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Salary distribution — %s", firstNonEmpty(v.Role, "(all roles)"))
	if v.Where != "" {
		fmt.Fprintf(w, " in %s", v.Where)
	}
	fmt.Fprintf(w, " (%s)\n", v.Site)
	fmt.Fprintf(w, "  sampled %d of %d listings; %d disclosed pay (%.0f%%)\n",
		v.ScannedListings, v.TotalMatches, v.WithSalary, v.DisclosureRate*100)
	if v.WithSalary == 0 {
		if v.Note != "" {
			fmt.Fprintf(w, "  %s\n", v.Note)
		}
		return
	}
	fmt.Fprintf(w, "  p10 %s   p25 %s   median %s   p75 %s   p90 %s\n",
		money(v.P10, v.Currency), money(v.P25, v.Currency), money(v.Median, v.Currency),
		money(v.P75, v.Currency), money(v.P90, v.Currency))
	if v.JobID != "" {
		fmt.Fprintf(w, "  job %s at %s → %.0fth percentile\n", v.JobID, money(v.JobSalary, v.Currency), v.JobPercentile)
	}
	if len(v.Histogram) > 0 {
		maxc := 1
		for _, b := range v.Histogram {
			if b.Count > maxc {
				maxc = b.Count
			}
		}
		fmt.Fprintln(w)
		for _, b := range v.Histogram {
			bar := strings.Repeat("█", int(math.Round(float64(b.Count)/float64(maxc)*24)))
			fmt.Fprintf(w, "  %-12s %s %d\n", money(b.From, v.Currency), bar, b.Count)
		}
	}
}

func money(v float64, cur string) string {
	if v >= 1000 {
		return fmt.Sprintf("%s $%.0fk", cur, v/1000)
	}
	return fmt.Sprintf("%s $%.0f", cur, v)
}
