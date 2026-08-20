// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. Preserved across `generate --force`.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/cliutil"
)

type findView struct {
	Hits         int    `json:"hits"`
	Returned     int    `json:"returned"`
	ScannedPages int    `json:"scanned_pages"`
	ScannedJobs  int    `json:"scanned_jobs"`
	Note         string `json:"note,omitempty"`
	Results      []Job  `json:"results"`
}

func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var country, state, city, sort string
	var category, schedule string
	var intern, manager, university bool
	var limit, page, maxScanPages int
	var postedWithin, descContains, descNotContains string

	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "Search Amazon jobs live, with location, recency, and text filters",
		Long: strings.Trim(`
Search amazon.jobs live by keyword and location, then apply client-side filters
(--category, --schedule, --intern, --manager, --university, --posted-within,
--description-contains) for the facets the API cannot filter server-side.

RECENCY: --posted-within filters on posted_date, the true posting date. Note
that the API's updated_time field is NOT a posting time -- it tracks the last
edit or re-index of any kind, and amazon.jobs re-indexes its backlog
continuously. Reqs posted many months ago routinely report an updated_time of
"about 21 hours". Rows where the two disagree that badly are marked (edited) in
the human output and carry "updated_diverged": true in JSON. Filter and sort on
posted_date; treat updated_time as an edit clock only.

posted_date is day-granular -- the API exposes no sub-day posting timestamp --
so --posted-within is inclusive by date, not by clock: --posted-within 7d means
"posted on or after (today - 7 days)", counting whole dates.

Use this command for one-off live searches. Use 'stats'/'skills' for aggregation
over a synced store, and 'new' for reqs unseen since your last sync of a saved
search.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli find "software engineer" --country USA --limit 10
  amazon-jobs-pp-cli find "solutions architect" --city Seattle --agent --select title,location,posted_date
  amazon-jobs-pp-cli find "software engineer" --manager=false --intern=false
  amazon-jobs-pp-cli find "program manager" --country GBR --posted-within 7d
  amazon-jobs-pp-cli find "" --country SGP --description-not-contains "without sponsorship"
  amazon-jobs-pp-cli find "" --country ARE --description-contains "[Rr]elocation" --posted-within 2w`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := guardDataSource(flags, "live"); err != nil {
				return err
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" && country == "" && state == "" && city == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a query and/or at least one of --country/--state/--city"))
			}
			if limit < 1 {
				limit = 20
			}
			if page < 1 {
				page = 1
			}
			if maxScanPages < 1 {
				maxScanPages = 5
			}

			now := time.Now()
			filters := clientFilters{
				category:   category,
				schedule:   schedule,
				intern:     boolFlag(cmd.Flags().Changed("intern"), intern),
				manager:    boolFlag(cmd.Flags().Changed("manager"), manager),
				university: boolFlag(cmd.Flags().Changed("university"), university),
			}

			if raw := strings.TrimSpace(postedWithin); raw != "" {
				d, derr := cliutil.ParseDurationLoose(raw)
				if derr != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --posted-within %q: use a duration like 24h, 3d, 2w", raw))
				}
				if d <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --posted-within %q: must be a positive duration", raw))
				}
				filters.postedCutoff = postedWithinCutoff(now, d)
				filters.postedWithinRaw = raw
			}

			var perr error
			if filters.descContains, perr = compileDescriptionPattern(descContains); perr != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --description-contains %q: %w", descContains, perr))
			}
			if filters.descExcludes, perr = compileDescriptionPattern(descNotContains); perr != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --description-not-contains %q: %w", descNotContains, perr))
			}

			clientSide := filters.active()

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Without client-side filters, --page maps straight onto the server's
			// offset (server paging). With client-side filters it cannot: the
			// server knows nothing about those filters, so a raw offset would
			// skip unfiltered records and drop matches that live in earlier
			// server pages. Scan from the top instead and discard the first
			// (page-1)*limit matches, so --page walks the filtered result set.
			pageSize := limit
			startOffset := (page - 1) * limit
			skipMatches := 0
			if clientSide {
				startOffset = 0
				skipMatches = (page - 1) * limit
			}

			// PATCH(amend-2026-07-25: --dry-run shows the real request) — this
			// used to print a fixed "would search amazon.jobs live" line before
			// any flag parsing, so it revealed nothing about the query,
			// filters, or paging that would actually be sent and could not be
			// used to check a command before running it. Emitting it here, once
			// the values are resolved, makes the preview faithful.
			if dryRunOK(flags) {
				out := cmd.OutOrStdout()
				values := buildSearchValues(query, country, state, city, sort, pageSize, startOffset)
				fmt.Fprintf(out, "would GET %s%s?%s\n", c.BaseURL, searchPath, values.Encode())
				if clientSide {
					fmt.Fprintf(out, "then filter client-side (%s), scanning up to %d page(s) for %d match(es)\n",
						filters.describe(), maxScanPages, limit)
					if skipMatches > 0 {
						fmt.Fprintf(out, "discarding the first %d match(es) to reach --page %d of the filtered set\n",
							skipMatches, page)
					}
				} else {
					fmt.Fprintf(out, "then return up to %d result(s) from that single page\n", limit)
				}
				return nil
			}

			matches := make([]Job, 0, limit)
			var totalHits, scannedPages, scannedJobs, skipped int
			scanCapHit := false

			for p := 0; p < maxScanPages; p++ {
				offset := startOffset + p*pageSize
				values := buildSearchValues(query, country, state, city, sort, pageSize, offset)
				hits, raw, ferr := searchPage(ctx, c, values)
				if ferr != nil {
					return classifyAPIError(ferr, flags)
				}
				totalHits = hits
				scannedPages++
				if len(raw) == 0 {
					break
				}
				for _, r := range raw {
					scannedJobs++
					j, perr := parseJob(r)
					if perr != nil {
						continue
					}
					if clientSide && !filters.matches(j) {
						continue
					}
					if skipped < skipMatches {
						skipped++
						continue
					}
					matches = append(matches, j)
					if len(matches) >= limit {
						break
					}
				}
				if len(matches) >= limit {
					break
				}
				if !clientSide {
					break // single server page already honored --page/--limit
				}
				if len(raw) < pageSize {
					break // exhausted upstream results
				}
				scanCapHit = true
			}

			// Always convert HTML (<br/>, anchors) to readable text: this is the
			// curated command, and raw HTML is never useful to JSON/agent
			// consumers. The raw `postings search` endpoint remains the faithful
			// escape hatch for callers who want the untouched API payload.
			for i := range matches {
				matches[i] = cleanJob(matches[i])
			}
			annotateFreshness(matches)

			view := findView{
				Hits:         totalHits,
				Returned:     len(matches),
				ScannedPages: scannedPages,
				ScannedJobs:  scannedJobs,
				Results:      matches,
			}
			switch {
			case len(matches) == 0 && skipMatches > 0 && skipped < skipMatches:
				view.Note = fmt.Sprintf("the filtered result set holds only %d match(es), which ends before --page %d; lower --page or raise --max-scan-pages", skipped, page)
			case len(matches) == 0 && clientSide && scanCapHit:
				view.Note = fmt.Sprintf("scanned %d jobs across %d pages without a match; raise --max-scan-pages to widen the search", scannedJobs, scannedPages)
			}

			return emitLiveResult(cmd, flags, view, func(w io.Writer) {
				if len(matches) == 0 {
					fmt.Fprintln(w, "no matching jobs")
					if view.Note != "" {
						fmt.Fprintln(w, view.Note)
					}
					return
				}
				diverged := 0
				for _, j := range matches {
					printJobLine(w, j)
					if j.UpdatedDiverged {
						diverged++
					}
				}
				fmt.Fprintf(w, "\n%d shown of %d total hits (scanned %d jobs across %d page(s))\n",
					len(matches), totalHits, scannedJobs, scannedPages)
				if diverged > 0 {
					fmt.Fprintf(w, "%d row(s) marked (edited): updated_time is far fresher than posted_date, so the req\n"+
						"was re-indexed or edited, not newly posted. Use --posted-within for true recency.\n", diverged)
				}
			})
		},
	}

	cmd.Flags().StringVar(&country, "country", "", "Filter by ISO alpha-3 country code (USA, GBR, IND, ...)")
	cmd.Flags().StringVar(&state, "state", "", "Filter by full state/region name (Washington, ...)")
	cmd.Flags().StringVar(&city, "city", "", "Filter by city name (Seattle, Austin, ...)")
	cmd.Flags().StringVar(&sort, "sort", "recent", "Sort order: recent or relevant")
	cmd.Flags().StringVar(&category, "category", "", "Client-side filter on job/business category substring (e.g. \"software\", \"aws\")")
	cmd.Flags().StringVar(&schedule, "schedule", "", "Client-side filter on job schedule type (e.g. \"Full-Time\")")
	cmd.Flags().BoolVar(&intern, "intern", false, "Client-side filter: internships only (--intern) or exclude (--intern=false)")
	cmd.Flags().BoolVar(&manager, "manager", false, "Client-side filter: manager roles only (--manager) or exclude (--manager=false)")
	cmd.Flags().BoolVar(&university, "university", false, "Client-side filter: university roles only or exclude (--university=false)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matching jobs to return")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-based) for server-side paging when no client-side filters are set")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Maximum pages to scan when client-side filters are active")
	cmd.Flags().StringVar(&postedWithin, "posted-within", "", "Client-side filter on true posted_date (24h, 3d, 2w). Inclusive by date, not by clock")
	cmd.Flags().StringVar(&descContains, "description-contains", "", "Client-side filter: case-insensitive regex over description + qualifications (falls back to literal match)")
	cmd.Flags().StringVar(&descNotContains, "description-not-contains", "", "Client-side filter: exclude jobs whose description + qualifications match this pattern")

	return cmd
}

// printJobLine renders a compact human summary of a job.
//
// Both dates are shown because neither alone is honest: posted_date is the real
// posting date but is day-granular, while updated_time looks precise and fresh
// yet tracks re-indexing rather than posting. Showing them side by side, with
// an (edited) marker when they disagree badly, is what stops a months-old req
// from reading as new. See updatedDiverged in amazonjobs_dates.go.
func printJobLine(w io.Writer, j Job) {
	id := j.IDIcims
	if id == "" {
		id = j.ID
	}
	fmt.Fprintf(w, "%-10s  %s\n", id, j.Title)
	loc := j.Location
	if loc == "" {
		loc = j.NormalizedLocation
	}
	meta := loc
	if j.JobCategory != "" {
		meta = strings.TrimSpace(meta + "  ·  " + j.JobCategory)
	}
	if j.PostedDate != "" {
		meta = strings.TrimSpace(meta + "  ·  posted " + j.PostedDate)
	}
	if j.UpdatedTime != "" {
		meta = strings.TrimSpace(meta + "  ·  updated " + j.UpdatedTime + " ago")
	}
	if j.UpdatedDiverged {
		meta = strings.TrimSpace(meta + "  (edited)")
	}
	if meta != "" {
		fmt.Fprintf(w, "            %s\n", meta)
	}
}
