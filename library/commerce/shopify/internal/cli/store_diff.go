// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopify/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/shopify/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

type storeDiffRow struct {
	ResourceType string `json:"resource_type"`
	ID           string `json:"id"`
	UpdatedAt    string `json:"updated_at"`
}

type storeDiffView struct {
	Since       string         `json:"since"`
	SinceCutoff string         `json:"since_cutoff"`
	Counts      map[string]int `json:"counts_by_resource_type"`
	Rows        []storeDiffRow `json:"rows"`
	Truncated   bool           `json:"truncated"`
	Note        string         `json:"note,omitempty"`
}

func newNovelStoreDiffCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "See which rows were touched by sync since a given time window.",
		Long: "Use this command to see which local rows were written by sync since a given time window (default 24h). " +
			"This reflects local sync write time, not a guaranteed content diff — an unchanged row re-synced in the window still counts. " +
			"Do NOT use this command to check sync freshness or row counts; use 'workflow status' instead.",
		Example:     "  shopify-pp-cli store diff --since 24h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store for rows touched since the given window")
				return nil
			}

			if flagSince == "" {
				flagSince = "24h"
			}
			cutoff, err := resolveStoreDiffCutoff(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("shopify-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: shopify-pp-cli sync --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			cutoffStr := cutoff.UTC().Format(time.RFC3339)
			rows, err := db.DB().QueryContext(ctx, `
				SELECT resource_type, id, updated_at FROM resources
				WHERE updated_at >= ?
				ORDER BY updated_at DESC
				LIMIT ?`, cutoffStr, flagLimit)
			if err != nil {
				return fmt.Errorf("querying resources: %w", err)
			}

			rawRows := make([]storeDiffRow, 0)
			for rows.Next() {
				var r storeDiffRow
				var updatedAt sql.NullString
				if err := rows.Scan(&r.ResourceType, &r.ID, &updatedAt); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan row: %w", err)
				}
				r.UpdatedAt = updatedAt.String
				rawRows = append(rawRows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close rows: %w", err)
			}

			counts := make(map[string]int)
			countRows, err := db.DB().QueryContext(ctx, `
				SELECT resource_type, COUNT(*) FROM resources
				WHERE updated_at >= ?
				GROUP BY resource_type`, cutoffStr)
			if err != nil {
				return fmt.Errorf("counting resources: %w", err)
			}
			for countRows.Next() {
				var rt string
				var n int
				if err := countRows.Scan(&rt, &n); err != nil {
					_ = countRows.Close()
					return fmt.Errorf("scan count row: %w", err)
				}
				counts[rt] = n
			}
			if err := countRows.Err(); err != nil {
				_ = countRows.Close()
				return fmt.Errorf("iterate count rows: %w", err)
			}
			if err := countRows.Close(); err != nil {
				return fmt.Errorf("close count rows: %w", err)
			}

			view := storeDiffView{
				Since:       flagSince,
				SinceCutoff: cutoffStr,
				Counts:      counts,
				Rows:        rawRows,
				Truncated:   len(rawRows) == flagLimit,
			}
			if len(rawRows) == 0 {
				view.Note = fmt.Sprintf("no rows touched since %s; widen --since or run sync first", flagSince)
			}

			if flags.asJSON || wantsMachineOutput(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rawRows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			for rt, n := range counts {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\n", rt, n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Time window to look back (e.g. 24h, 7d) or an RFC3339 timestamp")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// resolveStoreDiffCutoff accepts either a loose duration ("24h", "7d") meaning
// "look back this far from now", or an absolute RFC3339 timestamp.
func resolveStoreDiffCutoff(since string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339, since); err == nil {
		return ts, nil
	}
	d, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return time.Time{}, err
	}
	if d <= 0 {
		// A negative duration (e.g. "-24h") would silently flip the cutoff
		// into the future, making the query return zero rows with no
		// indication why. Reject it with an actionable message instead.
		return time.Time{}, fmt.Errorf("duration must be positive (got %q); use a lookback window like 24h or 7d", since)
	}
	return time.Now().Add(-d), nil
}
