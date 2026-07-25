// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. Preserved across `generate --force`.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
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

	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "Search Amazon jobs live, with location and client-side pipeline filters",
		Long: strings.Trim(`
Search amazon.jobs live by keyword and location, then apply client-side filters
(--category, --schedule, --intern, --manager, --university) for the facets the
API cannot filter server-side.

Use this command for one-off live searches. Use 'stats'/'skills' for aggregation
over a synced store, and 'new' for reqs unseen since your last sync of a saved
search.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli find "software engineer" --country USA --limit 10
  amazon-jobs-pp-cli find "solutions architect" --city Seattle --agent --select title,location,posted_date
  amazon-jobs-pp-cli find "software engineer" --manager=false --intern=false`, "\n"),
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

			wantIntern := boolFlag(cmd.Flags().Changed("intern"), intern)
			wantManager := boolFlag(cmd.Flags().Changed("manager"), manager)
			wantUniversity := boolFlag(cmd.Flags().Changed("university"), university)
			clientSide := hasClientFilters(category, schedule, wantIntern, wantManager, wantUniversity)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Without client-side filters, honor --page directly (server paging).
			// With client-side filters, scan forward up to --max-scan-pages,
			// filtering locally until we collect --limit matches.
			pageSize := limit
			startOffset := (page - 1) * limit

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
						describeClientFilters(category, schedule, wantIntern, wantManager, wantUniversity), maxScanPages, limit)
				} else {
					fmt.Fprintf(out, "then return up to %d result(s) from that single page\n", limit)
				}
				return nil
			}

			matches := make([]Job, 0, limit)
			var totalHits, scannedPages, scannedJobs int
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
					if clientSide && !matchesClientFilters(j, category, schedule, wantIntern, wantManager, wantUniversity) {
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

			view := findView{
				Hits:         totalHits,
				Returned:     len(matches),
				ScannedPages: scannedPages,
				ScannedJobs:  scannedJobs,
				Results:      matches,
			}
			if len(matches) == 0 && clientSide && scanCapHit {
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
				for _, j := range matches {
					printJobLine(w, j)
				}
				fmt.Fprintf(w, "\n%d shown of %d total hits (scanned %d jobs across %d page(s))\n",
					len(matches), totalHits, scannedJobs, scannedPages)
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

	return cmd
}

// printJobLine renders a compact human summary of a job.
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
	if meta != "" {
		fmt.Fprintf(w, "            %s\n", meta)
	}
}
