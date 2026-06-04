// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.
// Transcendence feature: unbilled billable balance — the invoice-ready
// number, summed per client from synced entries.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/store"
	"github.com/spf13/cobra"
)

func newBillableCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billable",
		Short: "Inspect billable time across synced entries",
		Long:  `Commands for reviewing billable time held in the local store.`,
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBillablePendingCmd(flags))
	return cmd
}

func newBillablePendingCmd(flags *rootFlags) *cobra.Command {
	var rangeFlag, workspace, clientFilter, dbPath string

	cmd := &cobra.Command{
		Use:   "pending",
		Short: "Sum uninvoiced billable time per client",
		Long: `Sum billable time on synced entries that are not yet locked
(Clockify locks an entry once it is invoiced or approved), grouped by
client — the "what can I invoice right now" number.`,
		Example: `  # Pending billable time, all clients
  clockify-pp-cli billable pending

  # One client, last month, as JSON
  clockify-pp-cli billable pending --client "Acme" --range last-month --json`,
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
			clients := loadClientNames(db)

			type row struct {
				Client  string        `json:"client"`
				Hours   time.Duration `json:"-"`
				Entries int           `json:"entries"`
			}
			byClient := map[string]*row{}
			var total time.Duration
			lockedSkipped := 0
			for _, e := range entries {
				if e.Start.IsZero() || e.Start.Before(start) || !e.Start.Before(end) {
					continue
				}
				if workspace != "" && e.WorkspaceID != workspace {
					continue
				}
				if !e.Billable || e.Locked {
					if e.Billable && e.Locked {
						lockedSkipped++
					}
					continue
				}
				client := "(no client)"
				if p, ok := projects[e.ProjectID]; ok && p.ClientID != "" {
					if cn, ok := clients[p.ClientID]; ok && cn != "" {
						client = cn
					} else {
						client = p.ClientID
					}
				}
				if clientFilter != "" && !strings.EqualFold(client, clientFilter) {
					continue
				}
				r := byClient[client]
				if r == nil {
					r = &row{Client: client}
					byClient[client] = r
				}
				r.Hours += e.Duration
				r.Entries++
				total += e.Duration
			}

			rows := make([]*row, 0, len(byClient))
			for _, r := range byClient {
				rows = append(rows, r)
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Hours != rows[j].Hours {
					return rows[i].Hours > rows[j].Hours
				}
				return rows[i].Client < rows[j].Client
			})

			if flags.asJSON {
				type jrow struct {
					Client       string  `json:"client"`
					PendingHours float64 `json:"pending_hours"`
					Entries      int     `json:"entries"`
				}
				jrows := make([]jrow, 0, len(rows))
				for _, r := range rows {
					jrows = append(jrows, jrow{r.Client, round2(r.Hours.Hours()), r.Entries})
				}
				return flags.printJSON(cmd, map[string]any{
					"range":               label,
					"clients":             jrows,
					"total_pending_hours": round2(total.Hours()),
					"locked_skipped":      lockedSkipped,
				})
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Pending billable time — %s\n\n", label)
			if len(rows) == 0 {
				if len(entries) == 0 {
					fmt.Fprintf(out, "No entries. (%s)\n", emptyStoreHint)
				} else {
					fmt.Fprintln(out, "No uninvoiced billable time in range.")
				}
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "CLIENT\tPENDING\tENTRIES")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%.2fh\t%d\n", truncate(r.Client, 32), r.Hours.Hours(), r.Entries)
			}
			tw.Flush()
			fmt.Fprintf(out, "\n%.2fh uninvoiced billable across %d client(s).\n", total.Hours(), len(rows))
			if lockedSkipped > 0 {
				fmt.Fprintf(out, "(%d billable entries already locked/invoiced, excluded.)\n", lockedSkipped)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&rangeFlag, "range", "all", "Date range: today, this-week, this-month, last-month, all, or Nd")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter to one workspace id")
	cmd.Flags().StringVar(&clientFilter, "client", "", "Filter to one client (name match)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
