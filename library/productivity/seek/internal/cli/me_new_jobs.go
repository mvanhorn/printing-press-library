// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: new listings across every SEEK saved search.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/seekparse"
)

type newJobHit struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	SalaryLabel string `json:"salary_label,omitempty"`
	ListingDate string `json:"listing_date"`
	SavedSearch string `json:"saved_search"`
	URL         string `json:"url"`
}

type newJobsView struct {
	SavedSearches int         `json:"saved_searches_run"`
	Since         string      `json:"since,omitempty"`
	NewCount      int         `json:"new_count"`
	CachedBefore  int         `json:"already_seen"`
	Jobs          []newJobHit `json:"jobs"`
	Skipped       []string    `json:"skipped_searches,omitempty"`
	Note          string      `json:"note,omitempty"`
}

func newNovelMeNewJobsCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagSearch string
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "new-jobs",
		Short: "Run every SEEK saved search and return only postings not already in the local store",
		Long: "Use 'me new-jobs' to run every one of your SEEK-native saved searches and return\n" +
			"only the postings that aren't already cached locally, tagged by which search matched.\n" +
			"New postings are cached so the next run only shows what changed.\n\n" +
			"To list the saved-search definitions themselves, use 'me saved-searches'.\n" +
			"Requires a SEEK session: run 'seek-pp-cli auth login --chrome' once.",
		Example: strings.Trim(`
  seek-pp-cli me new-jobs --since 7d --agent
  seek-pp-cli me new-jobs --search "Graduate Software Engineer"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:typed-exit-codes": "0,4", "pp:happy-args": "--since=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "me new-jobs")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var since time.Time
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = time.Now().Add(-d)
			}
			maxPages := flagMaxScanPages
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			searches, err := fetchSavedSearches(ctx, c)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			dbPath := defaultDBPath("seek-pp-cli")
			known, _ := knownJobIDs(dbPath)
			cachedBefore := len(known)

			wantName := strings.ToLower(strings.TrimSpace(flagSearch))
			view := newJobsView{Since: flagSince, CachedBefore: cachedBefore, Jobs: make([]newJobHit, 0)}
			seenThisRun := map[string]bool{}
			var fresh []seekJob

			for _, ss := range searches {
				if wantName != "" && !strings.Contains(strings.ToLower(ss.Name), wantName) {
					continue
				}
				p := seekparse.ParseSavedSearchQuery(ss.Query.SearchQueryString)
				if p.Keywords == "" && p.Where == "" && p.Classification == "" {
					view.Skipped = append(view.Skipped, ss.Name)
					continue
				}
				view.SavedSearches++
				opts := seekSearchOpts{
					Keywords: p.Keywords, Where: p.Where, Classification: p.Classification,
					Subclassification: p.Subclassification, Worktype: p.Worktype,
					Salaryrange: p.Salaryrange, Salarytype: p.Salarytype,
					Workarrangement: p.Workarrangement, SiteKey: firstNonEmpty(p.SiteKey, siteForCountry(ss.CountryCode)),
				}
				jobs, _, _, sErr := scanSearch(ctx, c, opts, maxPages)
				if sErr != nil {
					view.Skipped = append(view.Skipped, ss.Name)
					continue
				}
				for _, j := range jobs {
					if j.ID == "" || known[j.ID] || seenThisRun[j.ID] {
						continue
					}
					if !since.IsZero() {
						if listed, ok := parseSeekDate(j.ListingDate); ok && listed.Before(since) {
							continue
						}
					}
					seenThisRun[j.ID] = true
					fresh = append(fresh, j)
					view.Jobs = append(view.Jobs, newJobHit{
						ID: j.ID, Title: j.Title, Company: j.Advertiser.Description,
						Location: locationLabel(j), SalaryLabel: j.SalaryLabel,
						ListingDate: j.ListingDate, SavedSearch: ss.Name,
						URL: "https://au.seek.com/job/" + j.ID,
					})
				}
			}

			if dbPath != "" && len(fresh) > 0 {
				_, _ = cacheJobs(dbPath, fresh)
			}
			sort.Slice(view.Jobs, func(i, j int) bool { return view.Jobs[i].ListingDate > view.Jobs[j].ListingDate })
			view.NewCount = len(view.Jobs)
			if view.SavedSearches == 0 {
				view.Note = "no runnable saved searches found for your account (or none matched --search)."
			} else if view.NewCount == 0 {
				view.Note = fmt.Sprintf("ran %d saved search(es); nothing new since last run.", view.SavedSearches)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d new listing(s) across %d saved search(es)\n", view.NewCount, view.SavedSearches)
			for _, jb := range view.Jobs {
				fmt.Fprintf(w, "  [%s] %s — %s (%s)\n    %s\n", jb.SavedSearch, jb.Title, jb.Company, jb.Location, jb.URL)
			}
			if view.Note != "" {
				fmt.Fprintf(w, "  %s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only postings listed within this window (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&flagSearch, "search", "", "Only run saved searches whose name contains this text")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 3, "Maximum search pages to check per saved search")
	return cmd
}

func siteForCountry(cc string) string {
	if strings.EqualFold(cc, "NZ") {
		return "NZ-Main"
	}
	return "AU-Main"
}
