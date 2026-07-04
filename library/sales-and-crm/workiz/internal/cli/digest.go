// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/internal/cliutil"
	"github.com/spf13/cobra"
)

type digestEntry struct {
	ID      string `json:"id"`
	Change  string `json:"change"` // "new" or "changed"
	Summary string `json:"summary"`
}

type digestReport struct {
	Since string        `json:"since"`
	Jobs  []digestEntry `json:"jobs"`
	Leads []digestEntry `json:"leads"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "See everything new or changed across jobs and leads since your last check, grouped by entity.",
		Example:     "  workiz-pp-cli digest --since 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report jobs/leads changed since the given duration from the local mirror")
				return nil
			}
			since := flagSince
			if since == "" {
				since = "24h"
			}
			dur, perr := cliutil.ParseDurationLoose(since)
			if perr != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since duration %q: %w", since, perr))
			}
			cutoff := time.Now().Add(-dur)

			ctx := cmd.Context()
			var bail bool
			empty := digestReport{Since: since, Jobs: []digestEntry{}, Leads: []digestEntry{}}
			if dbPath, bail = checkNovelMirror(cmd, flags, dbPath, "job,lead", empty); bail {
				return nil
			}
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			jobs, err := loadJobs(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading jobs: %w", err)
			}
			leads, err := loadLeads(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading leads: %w", err)
			}

			report := digestReport{Since: since, Jobs: []digestEntry{}, Leads: []digestEntry{}}
			for _, j := range jobs {
				created, hasCreated := parseWorkizTime(j.CreatedDate)
				updated, hasUpdated := parseWorkizTime(j.LastStatusUpdate)
				switch {
				case hasCreated && created.After(cutoff):
					report.Jobs = append(report.Jobs, digestEntry{ID: j.UUID, Change: "new", Summary: fmt.Sprintf("%s %s (%s)", j.FirstName, j.LastName, j.Status)})
				case hasUpdated && updated.After(cutoff):
					report.Jobs = append(report.Jobs, digestEntry{ID: j.UUID, Change: "changed", Summary: fmt.Sprintf("status now %s", j.Status)})
				}
			}
			for _, l := range leads {
				created, hasCreated := parseWorkizTime(l.CreatedDate)
				updated, hasUpdated := parseWorkizTime(l.LastStatusUpdate)
				switch {
				case hasCreated && created.After(cutoff):
					report.Leads = append(report.Leads, digestEntry{ID: l.UUID, Change: "new", Summary: fmt.Sprintf("%s %s (%s)", l.FirstName, l.LastName, l.Status)})
				case hasUpdated && updated.After(cutoff):
					report.Leads = append(report.Leads, digestEntry{ID: l.UUID, Change: "changed", Summary: fmt.Sprintf("status now %s", l.Status)})
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if len(report.Jobs) == 0 && len(report.Leads) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "nothing new or changed since %s\n", since)
				return nil
			}
			for _, j := range report.Jobs {
				fmt.Fprintf(cmd.OutOrStdout(), "job %s [%s]: %s\n", j.ID, j.Change, j.Summary)
			}
			for _, l := range report.Leads {
				fmt.Fprintf(cmd.OutOrStdout(), "lead %s [%s]: %s\n", l.ID, l.Change, l.Summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Only include jobs/leads new or changed since this duration ago (e.g. 24h, 3d, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/workiz-pp-cli/data.db)")
	return cmd
}
