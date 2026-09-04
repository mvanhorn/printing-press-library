// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
	"github.com/spf13/cobra"
)

const reconcilePageSize = 200

type reconcileFinding struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	UpdatedAt   string `json:"updated_at"`
}
type reconcileWindow struct {
	Since string `json:"since"`
	Until string `json:"until"`
}
type reconcileOutput struct {
	Window          reconcileWindow    `json:"window"`
	ScannedLive     int                `json:"scanned_live"`
	ScannedPages    int                `json:"scanned_pages"`
	ScanCapHit      bool               `json:"scan_cap_hit"`
	LocalInWindow   int                `json:"local_in_window"`
	MissingLocally  []reconcileFinding `json:"missing_locally"`
	StaleLocally    []reconcileFinding `json:"stale_locally"`
	DeletedRemotely []reconcileFinding `json:"deleted_remotely"`
	LocalOnly       []reconcileFinding `json:"local_only"`
	InSync          bool               `json:"in_sync"`
	Note            string             `json:"note,omitempty"`
}

func emptyReconcileOutput(since, until string) reconcileOutput {
	return reconcileOutput{Window: reconcileWindow{Since: since, Until: until}, MissingLocally: make([]reconcileFinding, 0), StaleLocally: make([]reconcileFinding, 0), DeletedRemotely: make([]reconcileFinding, 0), LocalOnly: make([]reconcileFinding, 0)}
}

func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	sinceArg, groupArg := "30d", ""
	maxScanPages, limit := 10, 50
	dbPath := ""
	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Verify the local expense mirror against the live API: report missing, stale, and remotely-deleted expenses",
		Long:        "Use this command to verify the local store matches Splitwise before trusting a settle-up or report. Do NOT use it to see recent changes; use 'activity'. Do NOT use it for duplicate/outlier checks; use 'audit'.\n\nExit code 3 means discrepancies were found or the live scan hit its page cap.",
		Example:     "  splitwise-pp-cli reconcile --since 30d --agent\n  splitwise-pp-cli reconcile --group \"Lisbon trip\" --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3", "pp:happy-args": "--since=7d"}, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return novelErr(cmd, flags, usageErr(err))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reconcile")
			}
			dur, err := cliutil.ParseDurationLoose(sinceArg)
			if err != nil || dur <= 0 {
				if err == nil {
					err = errors.New("duration must be positive")
				}
				return novelErr(cmd, flags, usageErr(fmt.Errorf("invalid --since %q: %w", sinceArg, err)))
			}
			if maxScanPages < 1 {
				return novelErr(cmd, flags, usageErr(errors.New("--max-scan-pages must be at least 1")))
			}
			if maxScanPages > 500 {
				return novelErr(cmd, flags, usageErr(errors.New("--max-scan-pages must be at most 500")))
			}
			if limit < 0 {
				return novelErr(cmd, flags, usageErr(errors.New("--limit must be at least 0")))
			}
			if cliutil.IsDogfoodEnv() {
				maxScanPages = 1
			}
			until := time.Now().UTC()
			since := until.Add(-dur)
			out := emptyReconcileOutput(since.Format(time.RFC3339), until.Format(time.RFC3339))
			path := dbPath
			if path == "" {
				path = defaultDBPath("splitwise-pp-cli")
			}
			if _, err := os.Stat(path); err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("checking local mirror: %w", err)
				}
				out.Note = "local expense mirror is missing; run: splitwise-pp-cli sync --resources get-expenses --since 30d"
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: local expense mirror is missing; run: splitwise-pp-cli sync --resources get-expenses --since 30d")
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					err = printReconcileHuman(cmd, out)
				} else {
					err = flags.emitStructured(cmd, out)
				}
				if err != nil {
					return err
				}
				cmd.SilenceErrors = true
				return silentCodeErr(3)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, path)
			if err != nil {
				return fmt.Errorf("opening local mirror: %w", err)
			}
			defer db.Close()
			local, err := loadExpenses(db)
			if err != nil {
				return err
			}
			// The root learning hook may create the database before RunE, so a
			// file-existence check alone cannot identify a never-synced mirror.
			state, stateErr := readSyncHintState(db, "get-expenses")
			if stateErr != nil {
				return stateErr
			}
			if !state.hasState && len(local) == 0 {
				out.Note = "local expense mirror is missing; run: splitwise-pp-cli sync --resources get-expenses --since 30d"
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: local expense mirror is missing; run: splitwise-pp-cli sync --resources get-expenses --since 30d")
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					err = printReconcileHuman(cmd, out)
				} else {
					err = flags.emitStructured(cmd, out)
				}
				if err != nil {
					return err
				}
				cmd.SilenceErrors = true
				return silentCodeErr(3)
			}
			groupID := ""
			if strings.TrimSpace(groupArg) != "" {
				groups, err := loadGroups(db)
				if err != nil {
					return err
				}
				g, ok, err := resolveSettleGroup(groupArg, groups)
				if err != nil {
					return novelErr(cmd, flags, usageErr(err))
				}
				if !ok {
					return novelErr(cmd, flags, notFoundErr(fmt.Errorf("group %q was not found in the local mirror; run sync first", groupArg)))
				}
				groupID = strconv.Itoa(g.ID)
				filtered := make([]Expense, 0)
				for _, e := range local {
					if e.GroupID == g.ID {
						filtered = append(filtered, e)
					}
				}
				local = filtered
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			live := make([]Expense, 0)
			for page := 0; page < maxScanPages; page++ {
				params := map[string]string{"updated_after": out.Window.Since, "limit": strconv.Itoa(reconcilePageSize), "offset": strconv.Itoa(page * reconcilePageSize)}
				if groupID != "" {
					params["group_id"] = groupID
				}
				data, err := c.Get(ctx, "/get_expenses", params)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var env struct {
					Expenses []Expense `json:"expenses"`
				}
				if err := json.Unmarshal(data, &env); err != nil {
					return fmt.Errorf("decoding get_expenses response: %w", err)
				}
				out.ScannedPages++
				live = append(live, env.Expenses...)
				if len(env.Expenses) < reconcilePageSize {
					break
				}
				if page+1 == maxScanPages {
					out.ScanCapHit = true
				}
			}
			out.ScannedLive = len(live)
			reconcileCompare(&out, local, live, since, limit)
			if out.ScanCapHit {
				out.InSync = false
				out.Note = "live scan reached --max-scan-pages; increase --max-scan-pages, then run: splitwise-pp-cli sync --resources get-expenses --since 30d or --full"
			} else if !out.InSync {
				out.Note = "run: splitwise-pp-cli sync --resources get-expenses --since 30d or --full"
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				err = printReconcileHuman(cmd, out)
			} else {
				err = flags.emitStructured(cmd, out)
			}
			if err != nil {
				return err
			}
			if !out.InSync {
				return silentCodeErr(3)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceArg, "since", "30d", "Reconciliation window (e.g. 30d, 7d, 24h)")
	cmd.Flags().StringVar(&groupArg, "group", "", "Limit reconciliation to a locally synced group name or id")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 10, "Maximum live pages of 200 expenses to scan")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows per finding list")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func reconcileCompare(out *reconcileOutput, local, live []Expense, since time.Time, limit int) {
	discrepancy := false
	localByID := make(map[int]Expense, len(local))
	for _, e := range local {
		localByID[e.ID] = e
		if timestampAtOrAfter(e.UpdatedAt, since) {
			out.LocalInWindow++
		}
	}
	seen := make(map[int]bool, len(live))
	for _, e := range live {
		seen[e.ID] = true
		l, ok := localByID[e.ID]
		if !ok {
			discrepancy = true
			appendFinding(&out.MissingLocally, e, limit)
			continue
		}
		if timestampAfter(e.UpdatedAt, l.UpdatedAt) {
			discrepancy = true
			appendFinding(&out.StaleLocally, e, limit)
		}
		if e.DeletedAt != nil && strings.TrimSpace(*e.DeletedAt) != "" && (l.DeletedAt == nil || strings.TrimSpace(*l.DeletedAt) == "") {
			discrepancy = true
			appendFinding(&out.DeletedRemotely, e, limit)
		}
	}
	for _, e := range local {
		if timestampAtOrAfter(e.UpdatedAt, since) && !seen[e.ID] {
			discrepancy = true
			appendFinding(&out.LocalOnly, e, limit)
		}
	}
	for _, list := range []*[]reconcileFinding{&out.MissingLocally, &out.StaleLocally, &out.DeletedRemotely, &out.LocalOnly} {
		sort.Slice(*list, func(i, j int) bool { return (*list)[i].ID < (*list)[j].ID })
	}
	out.InSync = !discrepancy
}

func timestampAfter(a, b string) bool {
	ta, ea := time.Parse(time.RFC3339, strings.TrimSpace(a))
	tb, eb := time.Parse(time.RFC3339, strings.TrimSpace(b))
	if ea == nil && eb == nil {
		return ta.After(tb)
	}
	return strings.TrimSpace(a) > strings.TrimSpace(b)
}
func timestampAtOrAfter(value string, cutoff time.Time) bool {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	return err == nil && !t.Before(cutoff)
}
func appendFinding(dst *[]reconcileFinding, e Expense, limit int) {
	if len(*dst) < limit {
		*dst = append(*dst, reconcileFinding{ID: e.ID, Description: strings.TrimSpace(e.Description), UpdatedAt: strings.TrimSpace(e.UpdatedAt)})
	}
}

func printReconcileHuman(cmd *cobra.Command, out reconcileOutput) error {
	verdict := "NOT in sync"
	if out.InSync {
		verdict = "in sync"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %d live / %d local\n", verdict, out.ScannedLive, out.LocalInWindow)
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	for _, category := range []struct {
		name string
		rows []reconcileFinding
	}{{"missing_locally", out.MissingLocally}, {"stale_locally", out.StaleLocally}, {"deleted_remotely", out.DeletedRemotely}, {"local_only", out.LocalOnly}} {
		_, _ = fmt.Fprintf(tw, "%s\t%d\n", category.name, len(category.rows))
		for _, row := range category.rows {
			_, _ = fmt.Fprintf(tw, "  \t%d\t%s\t%s\n", row.ID, row.UpdatedAt, row.Description)
		}
	}
	if out.Note != "" {
		_, _ = fmt.Fprintf(tw, "note\t%s\n", out.Note)
	}
	return tw.Flush()
}
