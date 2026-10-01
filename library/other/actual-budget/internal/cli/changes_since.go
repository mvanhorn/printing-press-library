// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil"
)

// parseSince resolves a "since" expression: a duration (7d, 2w, 24h) back from
// ref, a YYYY-MM-DD or YYYYMMDD date (midnight UTC), or an RFC3339 timestamp.
func parseSince(when string, ref time.Time) (time.Time, error) {
	when = strings.TrimSpace(when)
	if when == "" {
		return time.Time{}, errors.New("empty time")
	}
	if d, err := actual.ParseDate(when); err == nil {
		return actual.DateTime(d), nil
	}
	if t, err := time.Parse(time.RFC3339, when); err == nil {
		return t, nil
	}
	d, err := cliutil.ParseDurationLoose(when)
	if err != nil || d < 0 {
		return time.Time{}, fmt.Errorf("invalid time %q: want a duration like 7d/24h/2w, a date YYYY-MM-DD or YYYYMMDD, or RFC3339", when)
	}
	return ref.Add(-d), nil
}

func newNovelChangesSinceCmd(flags *rootFlags) *cobra.Command {
	var flagDataset string
	var flagAsOf string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "since <when>",
		Short: "See what was created, edited, or deleted in the budget since a point in time, with names resolved.",
		Long: `Decodes Actual's sync change log (messages_crdt) in the mirror and groups every
change since <when> by row. <when> is a duration back from now (7d, 24h, 2w),
a date (YYYY-MM-DD or YYYYMMDD, midnight UTC) or an RFC3339 timestamp. With --as-of
YYYY-MM-DD, durations count back from the end of that day and later changes
are excluded.

Each row reports the dataset (table), row id, action ("deleted" when the row
was tombstoned in the window, "restored" when un-tombstoned, otherwise
"edited" — the log does not reliably distinguish creation from editing), the
changed columns with their newest values (payee, account, category and group
ids shown as names with the id kept in "<column>_id", dates as YYYY-MM-DD,
raw bank-sync payloads omitted), a human label for the entity
(transactions as payee/amount/date, categories/payees/accounts by name, even
when deleted), and device (the sync node id that made the latest change).
Amounts are integer minor units. Reads the local mirror only; run
'mirror pull' first to see the newest changes.`,
		Example: strings.Trim(`
  actual-budget-pp-cli changes since 7d
  actual-budget-pp-cli changes since 30d --dataset transactions --agent
  actual-budget-pp-cli changes since 2026-09-01 --limit 20 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "when=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if done, err := mirrorPreflight(cmd, flags, "changes since"); done {
				return err
			}
			if len(args) == 0 {
				return usageErr(errors.New("missing <when>: e.g. 'changes since 7d' or 'changes since 2026-09-01'"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("expected one <when> argument, got %d", len(args)))
			}
			ref := time.Now().UTC()
			var until time.Time
			if d, ok, err := parseAsOfFlag(flagAsOf); err != nil {
				return err
			} else if ok {
				ref = actual.DateTime(d).Add(24 * time.Hour)
				until = ref
			}
			since, err := parseSince(args[0], ref)
			if err != nil {
				return usageErr(err)
			}
			if err := checkLimit(flagLimit); err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]actual.RowChange, 0), func(ctx context.Context, h *mirrorHandle) error {
				msgs, err := h.Ledger.CRDTMessages(ctx, since, until, strings.TrimSpace(flagDataset))
				if err != nil {
					return err
				}
				rows := actual.GroupChanges(msgs)
				if flagLimit > 0 && len(rows) > flagLimit {
					rows = rows[:flagLimit]
				}
				names := h.Ledger.NewNameResolver()
				for i := range rows {
					rows[i].Entity = h.Ledger.DescribeEntity(ctx, rows[i].Dataset, rows[i].Row)
					names.HumanizeChanges(ctx, &rows[i])
				}
				return emitRows(cmd, flags, rows, func() error {
					w := cmd.OutOrStdout()
					if len(rows) == 0 {
						fmt.Fprintf(w, "No changes since %s.\n", since.Format(time.RFC3339))
						return nil
					}
					table := make([][]string, 0, len(rows))
					for _, r := range rows {
						cols := make([]string, 0, len(r.Changes))
						for k, v := range r.Changes {
							cols = append(cols, fmt.Sprintf("%s=%v", k, v))
						}
						sort.Strings(cols)
						entity := r.Entity
						if entity == "" {
							entity = r.Row
						}
						table = append(table, []string{r.LastChange[:min(19, len(r.LastChange))], r.Action, r.Dataset, entity, strings.Join(cols, " "), r.Device})
					}
					return printOrderedTable(w, []string{"WHEN", "ACTION", "DATASET", "ENTITY", "CHANGES", "DEVICE"}, table)
				})
			})
		},
	}
	cmd.Flags().StringVar(&flagDataset, "dataset", "", "Only changes to this table (transactions, categories, payees, accounts, zero_budgets, ...)")
	cmd.Flags().StringVar(&flagAsOf, "as-of", "", "Treat this date (YYYY-MM-DD or YYYYMMDD) as now: durations count back from its end and later changes are excluded")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum changed rows to return, newest first (0 = all)")
	return cmd
}
