// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
// This command computes results from locally stored history (resource_snapshots)
// built up as the user browses; it does not read a single upstream resource type.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type diffResult struct {
	FromDate string   `json:"from_date"`
	ToDate   string   `json:"to_date"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Changed  []string `json:"changed"`
}

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagType string

	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "See exactly which servers, clients, or skills were added, removed, or changed between two syncs.",
		Example:     "  mcpmarket-pp-cli diff --from 2026-08-01 --to 2026-08-27 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diff")
			}
			resourceType := flagType
			if resourceType == "" {
				resourceType = "server"
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			result, note, err := computeDiff(ctx, resourceType, flagFrom, flagTo)
			if err != nil {
				return err
			}
			if note != "" {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"note": note}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "earlier snapshot date (YYYY-MM-DD); defaults to the oldest available")
	cmd.Flags().StringVar(&flagTo, "to", "", "later snapshot date (YYYY-MM-DD); defaults to today (captures a fresh snapshot)")
	cmd.Flags().StringVar(&flagType, "type", "server", "resource type to diff: server, skill, or mcpclient")
	return cmd
}

func computeDiff(ctx context.Context, resourceType, from, to string) (*diffResult, string, error) {
	dbPath := defaultDBPath("mcpmarket-pp-cli")
	db, err := storeOpenForNovel(ctx, dbPath)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	toDate := to
	if toDate == "" {
		toDate, _, err = db.CaptureSnapshot(ctx, resourceType)
		if err != nil {
			return nil, "", fmt.Errorf("capturing today's snapshot: %w", err)
		}
	}

	dates, err := db.SnapshotDates(ctx)
	if err != nil {
		return nil, "", err
	}
	if len(dates) == 0 {
		return nil, "no snapshots exist yet. Run `mcpmarket-pp-cli server list` (or search/leaderboard) at least twice, on different days, then retry.", nil
	}

	fromDate := from
	if fromDate == "" {
		fromDate = dates[0]
	}
	if fromDate == toDate {
		return nil, fmt.Sprintf("--from and --to both resolved to %s — not enough distinct history yet. Run again on a later day, then retry.", toDate), nil
	}

	fromRows, err := db.SnapshotRows(ctx, fromDate, resourceType)
	if err != nil {
		return nil, "", err
	}
	toRows, err := db.SnapshotRows(ctx, toDate, resourceType)
	if err != nil {
		return nil, "", err
	}
	if len(fromRows) == 0 {
		return nil, fmt.Sprintf("no snapshot found for --from=%s. Run `mcpmarket-pp-cli diff --type %s` without --from/--to to see the available range.", fromDate, resourceType), nil
	}

	fromByID := make(map[string]string, len(fromRows))
	for _, r := range fromRows {
		fromByID[r.ResourceID] = string(r.Data)
	}
	toByID := make(map[string]string, len(toRows))
	for _, r := range toRows {
		toByID[r.ResourceID] = string(r.Data)
	}

	result := &diffResult{
		FromDate: fromDate,
		ToDate:   toDate,
		Added:    []string{},
		Removed:  []string{},
		Changed:  []string{},
	}
	for id, data := range toByID {
		oldData, existed := fromByID[id]
		if !existed {
			result.Added = append(result.Added, id)
			continue
		}
		if oldData != data {
			result.Changed = append(result.Changed, id)
		}
	}
	for id := range fromByID {
		if _, stillExists := toByID[id]; !stillExists {
			result.Removed = append(result.Removed, id)
		}
	}
	return result, "", nil
}
