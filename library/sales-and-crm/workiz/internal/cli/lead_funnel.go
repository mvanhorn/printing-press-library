// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/internal/cliutil"
)

type funnelSource struct {
	Source         string  `json:"source"`
	Leads          int     `json:"leads"`
	Converted      int     `json:"converted"`
	ConversionRate float64 `json:"conversion_rate"`
	AvgJobValue    float64 `json:"avg_job_value"`
}

func newNovelLeadFunnelCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "funnel",
		Short:       "See which lead sources actually turn into paid jobs, with conversion rate and average resulting job value per source.",
		Long:        "Use this for lead-source conversion rate and ROI ranking together. For raw dollar totals by source/status across all jobs (not just lead-originated), use 'job revenue' instead.",
		Example:     "  workiz-pp-cli lead funnel --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute lead-to-job conversion funnel from the local mirror")
				return nil
			}
			ctx := cmd.Context()
			var bail bool
			if dbPath, bail = checkNovelMirror(cmd, flags, dbPath, "lead,job", []funnelSource{}); bail {
				return nil
			}
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			leads, err := loadLeads(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading leads: %w", err)
			}
			jobs, err := loadJobs(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading jobs: %w", err)
			}

			if flagSince != "" {
				dur, perr := cliutil.ParseDurationLoose(flagSince)
				if perr != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since duration %q: %w", flagSince, perr))
				}
				cutoff := time.Now().Add(-dur)
				filtered := leads[:0]
				for _, l := range leads {
					if t, ok := parseWorkizTime(l.CreatedDate); ok && t.Before(cutoff) {
						continue
					}
					filtered = append(filtered, l)
				}
				leads = filtered
			}

			matchJob := func(l wzLead) (wzJob, bool) {
				return matchLeadToJob(l, jobs)
			}

			type agg struct {
				leads     int
				converted int
				jobValue  float64
			}
			bySource := map[string]*agg{}
			for _, l := range leads {
				source := l.LeadSource
				if source == "" {
					source = "(unknown)"
				}
				a, ok := bySource[source]
				if !ok {
					a = &agg{}
					bySource[source] = a
				}
				a.leads++
				if job, matched := matchJob(l); matched {
					a.converted++
					a.jobValue += parseMoney(job.JobTotalPrice)
				}
			}

			sources := make([]string, 0, len(bySource))
			for s := range bySource {
				sources = append(sources, s)
			}
			sort.Strings(sources)

			results := make([]funnelSource, 0, len(sources))
			for _, s := range sources {
				a := bySource[s]
				var rate, avg float64
				if a.leads > 0 {
					rate = float64(a.converted) / float64(a.leads)
				}
				if a.converted > 0 {
					avg = a.jobValue / float64(a.converted)
				}
				results = append(results, funnelSource{
					Source:         s,
					Leads:          a.leads,
					Converted:      a.converted,
					ConversionRate: rate,
					AvgJobValue:    avg,
				})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].ConversionRate > results[j].ConversionRate })

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d leads\t%d converted\t%.1f%%\t$%.2f avg job\n", r.Source, r.Leads, r.Converted, r.ConversionRate*100, r.AvgJobValue)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only include leads created since this duration ago (e.g. 30d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/workiz-pp-cli/data.db)")
	return cmd
}

// leadJobChronologyGrace tolerates a job being created slightly before its
// originating lead. Confirmed against live data: Workiz's own "AI Call"
// intake integration creates the job record 2-3 seconds before the lead
// record for the same contact (both are written near-simultaneously by the
// same automated flow), so a strict jobCreated >= leadCreated guard rejected
// every real conversion from that source. The grace window absorbs
// same-flow timing noise while still rejecting a genuinely older job from a
// previous, unrelated visit (typically days or weeks apart).
const leadJobChronologyGrace = 10 * time.Minute

// matchLeadToJob finds the job a lead likely became: same contact identity
// (email, or phone, or first+last name) and, when both dates are known, the
// job was created at or after the lead (within leadJobChronologyGrace).
// Workiz exposes no direct convert-link field between the two resources, so
// this is a best-effort heuristic.
//
// Collects every contact-matching job rather than returning the first one
// found: iteration order over synced jobs is not guaranteed chronological, so
// an early-return could pick an arbitrary older job for a repeat customer.
// Preference order: a candidate with a known CreatedDate always beats one
// without (an undated first match must not permanently block a later,
// dated candidate — the zero time.Time value would otherwise never be
// "before" by a real date and the undated match would stick). Among two
// dated candidates, the earliest-created wins. An undated candidate is only
// used as a last resort when no dated candidate exists.
func matchLeadToJob(l wzLead, jobs []wzJob) (wzJob, bool) {
	leadCreated, leadHasCreated := parseWorkizTime(l.CreatedDate)
	var best wzJob
	var bestCreated time.Time
	bestHasCreated := false
	found := false
	for _, j := range jobs {
		sameContact := (l.Email != "" && strings.EqualFold(l.Email, j.Email)) ||
			(l.Phone != "" && l.Phone == j.Phone) ||
			(l.FirstName != "" && l.LastName != "" && strings.EqualFold(l.FirstName, j.FirstName) && strings.EqualFold(l.LastName, j.LastName))
		if !sameContact {
			continue
		}
		jobCreated, jobHasCreated := parseWorkizTime(j.CreatedDate)
		if leadHasCreated && jobHasCreated && jobCreated.Before(leadCreated.Add(-leadJobChronologyGrace)) {
			continue // job meaningfully predates the lead; not a plausible conversion
		}
		switch {
		case !found:
			best, bestCreated, bestHasCreated, found = j, jobCreated, jobHasCreated, true
		case jobHasCreated && !bestHasCreated:
			best, bestCreated, bestHasCreated = j, jobCreated, true
		case jobHasCreated && bestHasCreated && jobCreated.Before(bestCreated):
			best, bestCreated = j, jobCreated
		}
	}
	return best, found
}
