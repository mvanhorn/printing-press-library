// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: live facet breakdown of a SEEK search.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/seekparse"
)

var facetDimensions = map[string]bool{
	"classification": true, "subclassification": true, "region": true,
	"worktype": true, "workarrangement": true, "salary": true, "advertiser": true,
}

type facetRow struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type facetsView struct {
	Keywords        string     `json:"keywords,omitempty"`
	Where           string     `json:"where,omitempty"`
	Site            string     `json:"site"`
	GroupBy         string     `json:"group_by"`
	TotalMatches    int        `json:"total_matches"`
	ScannedListings int        `json:"scanned_listings"`
	MaxScanPages    int        `json:"max_scan_pages"`
	Facets          []facetRow `json:"facets"`
	Note            string     `json:"note,omitempty"`
}

func newNovelListingsFacetsCmd(flags *rootFlags) *cobra.Command {
	var flagKeywords, flagWhere, flagGroupBy, flagSite string
	var flagMaxScanPages, flagLimit int

	cmd := &cobra.Command{
		Use:   "facets",
		Short: "Break a live SEEK query down by classification, work type, pay band, or region",
		Long: "Use 'listings facets' for a live, no-sync breakdown of a query across a chosen\n" +
			"dimension. Counts are tallied from the listings on the scanned result pages, so\n" +
			"they approximate the true totals for large result sets - raise --max-scan-pages\n" +
			"for a bigger sample.\n\n" +
			"For a single total, use 'listings count'. For trends over time or percentile pay\n" +
			"statistics, use 'trends' or 'salary'.",
		Example: strings.Trim(`
  seek-pp-cli listings facets --keywords "data analyst" --where "Brisbane QLD" --group-by classification --agent
  seek-pp-cli listings facets --keywords "nurse" --where "Perth WA" --group-by salary`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings facets")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			flagGroupBy = strings.ToLower(strings.TrimSpace(flagGroupBy))
			if flagGroupBy == "" {
				flagGroupBy = "classification"
			}
			if !facetDimensions[flagGroupBy] {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--group-by must be one of classification, subclassification, region, worktype, workarrangement, salary, advertiser"))
			}
			if strings.TrimSpace(flagKeywords) == "" && strings.TrimSpace(flagWhere) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide --keywords or --where"))
			}

			maxPages := flagMaxScanPages
			if cliutil.IsDogfoodEnv() && maxPages > 2 {
				maxPages = 2
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			opts := seekSearchOpts{Keywords: flagKeywords, Where: flagWhere, SiteKey: flagSite}
			if strings.Contains(flagSite, "NZ") {
				opts.Locale = "en-NZ"
			}
			jobs, total, capHit, err := scanSearch(ctx, c, opts, maxPages)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			counts := map[string]int{}
			for _, j := range jobs {
				v := facetValue(j, flagGroupBy)
				if v == "" {
					v = "(unspecified)"
				}
				counts[v]++
			}
			rows := make([]facetRow, 0, len(counts))
			for k, n := range counts {
				rows = append(rows, facetRow{Value: k, Count: n})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Count != rows[j].Count {
					return rows[i].Count > rows[j].Count
				}
				return rows[i].Value < rows[j].Value
			})
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			view := facetsView{
				Keywords: flagKeywords, Where: flagWhere,
				Site:         firstNonEmpty(flagSite, "AU-Main"),
				GroupBy:      flagGroupBy,
				TotalMatches: total, ScannedListings: len(jobs),
				MaxScanPages: maxPages, Facets: rows,
			}
			if len(jobs) == 0 && capHit {
				view.Note = "scan cap hit with no listings; widen --keywords / --where or raise --max-scan-pages."
			} else if capHit && total > len(jobs) {
				view.Note = fmt.Sprintf("counts are over %d of %d matches; raise --max-scan-pages for the full breakdown.", len(jobs), total)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s breakdown — %s (%d of %d matches sampled)\n", capitalize(flagGroupBy), firstNonEmpty(flagKeywords, flagWhere), len(jobs), total)
			for _, r := range rows {
				fmt.Fprintf(w, "  %5d  %s\n", r.Count, r.Value)
			}
			if view.Note != "" {
				fmt.Fprintf(w, "  note: %s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagKeywords, "keywords", "", "Keyword query")
	cmd.Flags().StringVar(&flagWhere, "where", "", "Location")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "classification", "Facet dimension")
	cmd.Flags().StringVar(&flagSite, "site", "AU-Main", "Marketplace: AU-Main or NZ-Main")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 5, "Maximum search pages to sample (100 listings/page)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum facet rows to return (0 = all)")
	return cmd
}

func facetValue(j seekJob, dim string) string {
	switch dim {
	case "classification":
		_, n := j.topClassification()
		return n
	case "subclassification":
		_, n := j.subClassification()
		return n
	case "region":
		return j.region()
	case "worktype":
		return j.workType()
	case "workarrangement":
		return j.workArrangement()
	case "advertiser":
		return j.Advertiser.Description
	case "salary":
		r := seekparse.ParseSalary(j.SalaryLabel)
		if !r.Ok {
			return "undisclosed"
		}
		return salaryBand(r.Midpoint())
	}
	return ""
}

func salaryBand(v float64) string {
	switch {
	case v < 60000:
		return "< $60k"
	case v < 80000:
		return "$60-80k"
	case v < 100000:
		return "$80-100k"
	case v < 120000:
		return "$100-120k"
	case v < 150000:
		return "$120-150k"
	case v < 200000:
		return "$150-200k"
	default:
		return "$200k+"
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
