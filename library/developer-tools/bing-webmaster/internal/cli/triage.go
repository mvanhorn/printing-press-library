// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// triage: crawl-error triage. Categorizes GetCrawlIssues by decoded severity,
// snapshots them, and diffs against the previous run (new vs resolved).
// Hand-authored transcendence command.
package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type bTriageIssue struct {
	URL        string   `json:"url"`
	Categories []string `json:"categories"`
}

type bTriageResult struct {
	Site    string `json:"site"`
	Summary struct {
		Total             int            `json:"total"`
		ByCategory        map[string]int `json:"by_category"`
		NewSinceLast      int            `json:"new_since_last"`
		ResolvedSinceLast int            `json:"resolved_since_last"`
	} `json:"summary"`
	Issues []bTriageIssue `json:"issues"`
}

// parseTriageIssues normalizes a GetCrawlIssues array into URL + decoded
// categories. Pure for testability.
func parseTriageIssues(data []byte) []bTriageIssue {
	out := []bTriageIssue{}
	for _, it := range bArray(data) {
		m := bCIMap(it)
		if m == nil {
			continue
		}
		url := bStr(m, "Url")
		if url == "" {
			url = bStr(m, "Url ")
		}
		flagsVal, ok := bNum(m, "Issues")
		if !ok {
			flagsVal, _ = bNum(m, "CrawlIssues")
		}
		out = append(out, bTriageIssue{URL: url, Categories: bDecodeCrawlIssues(int64(flagsVal))})
	}
	return out
}

func urlSet(issues []bTriageIssue) map[string]bool {
	s := make(map[string]bool, len(issues))
	for _, i := range issues {
		s[i.URL] = true
	}
	return s
}

func newTriageCmd(flags *rootFlags) *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:         "triage",
		Short:       "Prioritized crawl-error worklist: categorized, deduped, and diffed against the last run",
		Long:        "Read crawl issues and crawl stats, decode each issue into a human category, and diff against the previous snapshot to show what is new and what resolved — a worklist instead of a raw dump.",
		Example:     "  bing-webmaster-pp-cli triage --site https://example.com",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			proceed, err := siteOrDryRun(cmd, flags, site, fmt.Sprintf("would read and categorize crawl issues for %q", site))
			if err != nil || !proceed {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/json/GetCrawlIssues", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			issues := parseTriageIssues(data)

			var result bTriageResult
			result.Site = site
			result.Issues = issues
			result.Summary.Total = len(issues)
			result.Summary.ByCategory = map[string]int{}
			for _, is := range issues {
				for _, cat := range is.Categories {
					result.Summary.ByCategory[cat]++
				}
			}

			db, err := openSnapshots()
			if err != nil {
				return err
			}
			defer db.Close()
			now := time.Now()
			prev, hadPrev, err := db.Latest(site, "crawl_issues")
			if err != nil {
				return err
			}
			if err := db.Capture(site, "crawl_issues", data, now); err != nil {
				return err
			}
			if hadPrev {
				prevSet := urlSet(parseTriageIssues(prev.Data))
				curSet := urlSet(issues)
				for u := range curSet {
					if !prevSet[u] {
						result.Summary.NewSinceLast++
					}
				}
				for u := range prevSet {
					if !curSet[u] {
						result.Summary.ResolvedSinceLast++
					}
				}
			}
			return emitIntel(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "Crawl triage for %s: %d issues\n", site, result.Summary.Total)
				cats := make([]string, 0, len(result.Summary.ByCategory))
				for k := range result.Summary.ByCategory {
					cats = append(cats, k)
				}
				sort.Strings(cats)
				for _, k := range cats {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-28s %d\n", k, result.Summary.ByCategory[k])
				}
				if hadPrev {
					fmt.Fprintf(cmd.OutOrStdout(), "  new since last: %d   resolved: %d\n", result.Summary.NewSinceLast, result.Summary.ResolvedSinceLast)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "  (first run — baseline captured for next-time diff)")
				}
			})
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	return cmd
}
