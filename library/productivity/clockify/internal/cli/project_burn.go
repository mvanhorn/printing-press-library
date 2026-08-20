// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.
// Transcendence feature: project budget burn — logged vs estimated hours
// per project, joined from synced entries and project estimates.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/store"
	"github.com/spf13/cobra"
)

func newProjectBurnCmd(flags *rootFlags) *cobra.Command {
	var clientFilter, workspace, dbPath string
	var includeArchived bool

	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Hours logged vs each project's time estimate, with percent consumed",
		Long: `Join synced time entries against each project's time estimate to show
logged-vs-estimated hours and the percent of the estimate consumed —
so a project about to blow its budget is visible early.

Reads the local store; run 'clockify-pp-cli sync' first.`,
		Example: `  # Burn for every estimated project
  clockify-pp-cli projects burn

  # One client's projects, as JSON
  clockify-pp-cli projects burn --client "Acme" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("clockify-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clockify-pp-cli sync' first.", err)
			}
			defer db.Close()

			projects := loadProjects(db)
			clients := loadClientNames(db)
			// Burn is a project-lifetime metric, so fetch the full history
			// (the [zero, now+100y] window matches resolveRange("all"))
			// rather than capping a cold/live fetch at the last year.
			entries, err := ensureTimeEntries(db, flags, time.Time{}, time.Now().AddDate(100, 0, 0), workspace)
			if err != nil {
				return fmt.Errorf("loading time entries: %w", err)
			}
			logged := map[string]time.Duration{}
			for _, e := range entries {
				if workspace != "" && e.WorkspaceID != workspace {
					continue
				}
				if e.ProjectID != "" {
					logged[e.ProjectID] += e.Duration
				}
			}

			var rows []projectBurnRow
			for id, p := range projects {
				if p.Archived && !includeArchived {
					continue
				}
				clientName := ""
				if p.ClientID != "" {
					if cn, ok := clients[p.ClientID]; ok {
						clientName = cn
					} else {
						clientName = p.ClientID
					}
				}
				if clientFilter != "" && !strings.EqualFold(clientName, clientFilter) {
					continue
				}
				est := parseISO8601Duration(p.Estimate.Estimate)
				if est == 0 {
					continue // only projects with a time estimate burn
				}
				log := logged[id]
				pct := 0.0
				if est > 0 {
					pct = round2(log.Hours() / est.Hours() * 100)
				}
				rows = append(rows, projectBurnRow{
					Project:       p.Name,
					Client:        clientName,
					LoggedHours:   round2(log.Hours()),
					EstimateHours: round2(est.Hours()),
					PercentUsed:   pct,
					OverBudget:    log > est,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].PercentUsed != rows[j].PercentUsed {
					return rows[i].PercentUsed > rows[j].PercentUsed
				}
				return rows[i].Project < rows[j].Project
			})

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"projects":    rows,
					"over_budget": countOver(rows),
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Project budget burn")
			fmt.Fprintln(out, "")
			if len(rows) == 0 {
				fmt.Fprintln(out, "No projects with a time estimate found.")
				fmt.Fprintf(out, "(Set a time estimate on a project, then sync — or %s)\n", emptyStoreHint)
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "PROJECT\tCLIENT\tLOGGED\tESTIMATE\tUSED\tFLAG")
			for _, r := range rows {
				flag := ""
				if r.OverBudget {
					flag = "OVER BUDGET"
				} else if r.PercentUsed >= 80 {
					flag = "near limit"
				}
				fmt.Fprintf(tw, "%s\t%s\t%.2fh\t%.2fh\t%.1f%%\t%s\n",
					truncate(r.Project, 24), truncate(r.Client, 18), r.LoggedHours, r.EstimateHours, r.PercentUsed, flag)
			}
			tw.Flush()
			if over := countOver(rows); over > 0 {
				fmt.Fprintf(out, "\n%d project(s) over budget.\n", over)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&clientFilter, "client", "", "Filter to one client (name match)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter to one workspace id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include archived projects")
	return cmd
}

// projectBurnRow is one project's logged-vs-estimate burn line.
type projectBurnRow struct {
	Project       string  `json:"project"`
	Client        string  `json:"client,omitempty"`
	LoggedHours   float64 `json:"logged_hours"`
	EstimateHours float64 `json:"estimate_hours"`
	PercentUsed   float64 `json:"percent_used"`
	OverBudget    bool    `json:"over_budget"`
}

func countOver(rows []projectBurnRow) int {
	n := 0
	for _, r := range rows {
		if r.OverBudget {
			n++
		}
	}
	return n
}
