// Copyright 2026 melanson633. Licensed under Apache-2.0. See LICENSE.
// Transcendence feature: billable leakage audit — joins synced entries
// against projects, clients, and tags to flag misfiled billable time.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/store"
	"github.com/spf13/cobra"
)

func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit synced time data for misfiled or leaking entries",
		Long: `Cross-check synced time entries against projects, clients, and tags
to surface the misfiles that quietly cost money or break reports.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAuditBillableCmd(flags))
	return cmd
}

func newAuditBillableCmd(flags *rootFlags) *cobra.Command {
	var rangeFlag, workspace, dbPath string

	cmd := &cobra.Command{
		Use:   "billable",
		Short: "Flag billable entries that will leak off invoices",
		Long: `Join synced time entries against projects and tags to flag the
misfiles that silently drop billable time off invoices:

  - billable entries with no project
  - billable entries with no tags
  - billable entries whose project is marked non-billable
  - non-billable entries on an otherwise billable project

Each flagged entry is money or reporting accuracy at risk.`,
		Example: `  # Audit all synced entries
  clockify-pp-cli audit billable

  # Audit last month only, as JSON
  clockify-pp-cli audit billable --range last-month --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("clockify-pp-cli")
			}
			start, end, label, err := resolveRange(rangeFlag, time.Now())
			if err != nil {
				return usageErr(err)
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clockify-pp-cli sync' first.", err)
			}
			defer db.Close()

			entries, err := ensureTimeEntries(db, flags, start, end, workspace)
			if err != nil {
				return fmt.Errorf("loading time entries: %w", err)
			}
			projects := loadProjects(db)

			type finding struct {
				EntryID     string  `json:"entry_id"`
				Description string  `json:"description"`
				Date        string  `json:"date"`
				Hours       float64 `json:"hours"`
				Issue       string  `json:"issue"`
			}
			var findings []finding
			var atRisk time.Duration
			scanned := 0
			for _, e := range entries {
				if e.Start.IsZero() || e.Start.Before(start) || !e.Start.Before(end) {
					continue
				}
				if workspace != "" && e.WorkspaceID != workspace {
					continue
				}
				scanned++
				p, hasProject := projects[e.ProjectID]
				var issue string
				switch {
				case e.Billable && e.ProjectID == "":
					issue = "billable, no project"
				case e.Billable && len(e.TagIDs) == 0:
					issue = "billable, untagged"
				case e.Billable && hasProject && !p.Billable:
					issue = "billable on non-billable project"
				case !e.Billable && hasProject && p.Billable && e.ProjectID != "":
					issue = "non-billable on billable project"
				}
				if issue == "" {
					continue
				}
				desc := e.Description
				if desc == "" {
					desc = "(no description)"
				}
				date := ""
				if !e.Start.IsZero() {
					date = e.Start.Format("2006-01-02")
				}
				findings = append(findings, finding{
					EntryID: e.ID, Description: desc, Date: date,
					Hours: round2(e.Duration.Hours()), Issue: issue,
				})
				if e.Billable {
					atRisk += e.Duration
				}
			}
			sort.Slice(findings, func(i, j int) bool {
				if findings[i].Issue != findings[j].Issue {
					return findings[i].Issue < findings[j].Issue
				}
				return findings[i].Hours > findings[j].Hours
			})

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"range":                  label,
					"entries_scanned":        scanned,
					"findings":               findings,
					"finding_count":          len(findings),
					"billable_at_risk_hours": round2(atRisk.Hours()),
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Billable audit — %s (%d entries scanned)\n\n", label, scanned)
			if len(findings) == 0 {
				if scanned == 0 {
					fmt.Fprintf(out, "No entries in range. (%s)\n", emptyStoreHint)
				} else {
					fmt.Fprintln(out, "Clean — no misfiled billable entries found.")
				}
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "DATE\tHOURS\tISSUE\tDESCRIPTION")
			for _, f := range findings {
				fmt.Fprintf(tw, "%s\t%.2fh\t%s\t%s\n", f.Date, f.Hours, f.Issue, truncate(f.Description, 40))
			}
			tw.Flush()
			fmt.Fprintf(out, "\n%d issue(s); %.2fh of billable time at risk.\n", len(findings), atRisk.Hours())
			return nil
		},
	}

	cmd.Flags().StringVar(&rangeFlag, "range", "all", "Date range: today, this-week, this-month, last-month, all, or Nd")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter to one workspace id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
