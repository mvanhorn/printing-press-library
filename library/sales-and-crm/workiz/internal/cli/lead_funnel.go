// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/internal/cliutil"
	"github.com/spf13/cobra"
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

			// Match a lead to the job it likely became: same contact identity
			// (email, or phone, or first+last name) and the job was created
			// on or after the lead. Workiz exposes no direct convert-link
			// field between the two resources.
			matchJob := func(l wzLead) (wzJob, bool) {
				leadCreated, _ := parseWorkizTime(l.CreatedDate)
				for _, j := range jobs {
					sameContact := (l.Email != "" && strings.EqualFold(l.Email, j.Email)) ||
						(l.Phone != "" && l.Phone == j.Phone) ||
						(l.FirstName != "" && l.LastName != "" && strings.EqualFold(l.FirstName, j.FirstName) && strings.EqualFold(l.LastName, j.LastName))
					if !sameContact {
						continue
					}
					if jobCreated, ok := parseWorkizTime(j.CreatedDate); ok && !leadCreated.IsZero() && jobCreated.Before(leadCreated) {
						continue // job predates the lead; not a plausible conversion
					}
					return j, true
				}
				return wzJob{}, false
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
