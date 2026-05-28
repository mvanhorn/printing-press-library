package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/cliutil"
	"github.com/spf13/cobra"
)

func newActivityCmd(flags *rootFlags) *cobra.Command {
	limit := 20
	since := "7d"
	cmd := &cobra.Command{
		Use:         "activity",
		Short:       "Show recent notifications and expenses changed within a time window",
		Example:     "  splitwise-pp-cli activity --since 7d --agent\n  splitwise-pp-cli activity --since 24h --limit 50 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would show activity")
				return nil
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-notifications")
			hintIfUnsynced(cmd, db, "get-expenses")

			type notification struct {
				ID        json.Number `json:"id"`
				Type      any         `json:"type"`
				CreatedAt string      `json:"created_at"`
				Content   string      `json:"content"`
			}

			notifications := make([]notification, 0)
			rows, err := db.List("get-notifications", 0)
			if err != nil {
				return err
			}
			for _, row := range rows {
				var n notification
				if err := json.Unmarshal(row, &n); err != nil {
					continue
				}
				n.Content = stripHTML(n.Content)
				notifications = append(notifications, n)
			}
			sort.Slice(notifications, func(i, j int) bool {
				return strings.TrimSpace(notifications[i].CreatedAt) > strings.TrimSpace(notifications[j].CreatedAt)
			})
			if limit < 0 {
				limit = 0
			}
			if len(notifications) > limit {
				notifications = notifications[:limit]
			}

			// "Changed expenses" is a recency window, not the last-sync time:
			// every locally-stored expense has updated_at <= the last sync
			// completion time, so comparing against GetLastSyncedAt would make
			// this list always empty. Diff against now-minus-window instead.
			dur, derr := cliutil.ParseDurationLoose(since)
			if derr != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, derr))
			}
			cutoff := time.Now().UTC().Add(-dur).Format(time.RFC3339)
			type changedExpense struct {
				ID          int    `json:"id"`
				Description string `json:"description"`
				UpdatedAt   string `json:"updated_at"`
			}
			changed := make([]changedExpense, 0)
			if cutoff != "" {
				expenses, err := loadExpenses(db)
				if err != nil {
					return err
				}
				for _, e := range expenses {
					if strings.TrimSpace(e.UpdatedAt) > cutoff {
						changed = append(changed, changedExpense{ID: e.ID, Description: strings.TrimSpace(e.Description), UpdatedAt: strings.TrimSpace(e.UpdatedAt)})
					}
				}
				sort.Slice(changed, func(i, j int) bool {
					return changed[i].UpdatedAt > changed[j].UpdatedAt
				})
			}

			out := struct {
				Since           string           `json:"since"`
				Notifications   []notification   `json:"notifications"`
				ChangedExpenses []changedExpense `json:"changed_expenses"`
			}{
				Since:           cutoff,
				Notifications:   notifications,
				ChangedExpenses: changed,
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Since: %s\n\n", cutoff)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "CREATED AT\tID\tTYPE\tCONTENT")
			for _, n := range notifications {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%v\t%s\n", n.CreatedAt, n.ID.String(), n.Type, n.Content)
			}
			_ = tw.Flush()

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			tw2 := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw2, "UPDATED AT\tID\tDESCRIPTION")
			for _, e := range changed {
				_, _ = fmt.Fprintf(tw2, "%s\t%d\t%s\n", e.UpdatedAt, e.ID, e.Description)
			}
			return tw2.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum notifications to return")
	cmd.Flags().StringVar(&since, "since", "7d", "Recency window for changed expenses (e.g. 24h, 7d, 4w)")
	return cmd
}
