// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

// pp:data-source computed
// Supported strategies: auto, local, live, or computed.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/types"
	"github.com/spf13/cobra"
)

type reportDiff struct {
	Target        string      `json:"target"`
	From          snapshotRef `json:"from"`
	To            snapshotRef `json:"to"`
	ScoreDelta    float64     `json:"score_delta"`
	AddedIssues   []string    `json:"added_issues"`
	RemovedIssues []string    `json:"removed_issues"`
	ChangedIssues []string    `json:"changed_issues"`
	Note          string      `json:"note,omitempty"`
}
type snapshotRef struct {
	ID        int64    `json:"id"`
	FetchedAt string   `json:"fetched_at"`
	ScannedAt string   `json:"scanned_at,omitempty"`
	Score     *float64 `json:"score,omitempty"`
}

func scoreDelta(after, before *float64) float64 {
	if after == nil || before == nil {
		return 0
	}
	return *after - *before
}

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var target string
	var fromID, toID int64
	cmd := &cobra.Command{Use: "diff", Short: "See which readiness scores and findings changed between two retained audits.", Example: "  is-agentic-pp-cli diff --target https://is-agentic.com --json", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.dryRun {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "compare retained report snapshots")
		}
		if target == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--target is required"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		s, path, err := openAgenticStore(ctx, flags)
		if err != nil {
			return err
		}
		if s == nil {
			return missingStore(cmd, flags, path)
		}
		defer s.Close()
		items, err := s.ListAgenticSnapshots(ctx, target, 100)
		if err != nil {
			return fmt.Errorf("reading snapshots: %w", err)
		}
		if len(items) < 2 {
			// A diff is still useful on a first invocation: retain a fresh
			// observation so the command can compare the current service result
			// with any existing local observation. The service may return the
			// same six-hour snapshot; fetched_at keeps that distinction honest.
			if _, _, fetchErr := fetchAndSave(ctx, s, target); fetchErr != nil {
				return fmt.Errorf("capturing a comparison snapshot: %w", fetchErr)
			}
			items, err = s.ListAgenticSnapshots(ctx, target, 100)
			if err != nil {
				return fmt.Errorf("reading refreshed snapshots: %w", err)
			}
		}
		baselineNote := ""
		if len(items) < 2 {
			// A first-run diff compares the current observation with itself and
			// says so explicitly instead of failing a useful read-only command.
			if len(items) == 0 {
				return usageErr(fmt.Errorf("no snapshot available for %s", target))
			}
			items = append(items, items[0])
			baselineNote = "only one retained observation was available; deltas are provisional until a later snapshot is captured"
		}
		var from, to = items[len(items)-1], items[0]
		if fromID > 0 {
			from, err = s.AgenticSnapshotByID(ctx, fromID)
			if err != nil {
				return fmt.Errorf("loading --from snapshot: %w", err)
			}
		}
		if toID > 0 {
			to, err = s.AgenticSnapshotByID(ctx, toID)
			if err != nil {
				return fmt.Errorf("loading --to snapshot: %w", err)
			}
		}
		before, err := parseReportSnapshot(from)
		if err != nil {
			return err
		}
		after, err := parseReportSnapshot(to)
		if err != nil {
			return err
		}
		old := map[string]types.PublicScanIssue{}
		for _, i := range before.Issues {
			old[i.Id] = i
		}
		newer := map[string]types.PublicScanIssue{}
		for _, i := range after.Issues {
			newer[i.Id] = i
		}
		added := make([]string, 0)
		removed := make([]string, 0)
		changed := make([]string, 0)
		for id, i := range newer {
			if _, ok := old[id]; !ok {
				added = append(added, id)
			} else if prior := old[id]; prior.Result != i.Result || prior.Details != i.Details || prior.Recommendation != i.Recommendation {
				changed = append(changed, id)
			}
		}
		for id := range old {
			if _, ok := newer[id]; !ok {
				removed = append(removed, id)
			}
		}
		sort.Strings(added)
		sort.Strings(removed)
		sort.Strings(changed)
		view := reportDiff{Target: target, From: snapshotRef{from.ID, from.FetchedAt, from.ScannedAt, from.Score}, To: snapshotRef{to.ID, to.FetchedAt, to.ScannedAt, to.Score}, ScoreDelta: scoreDelta(after.Parsed.Score, before.Parsed.Score), AddedIssues: added, RemovedIssues: removed, ChangedIssues: changed, Note: baselineNote}
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}}
	cmd.Flags().StringVar(&target, "target", "", "public URL whose retained snapshots should be compared")
	cmd.Flags().Int64Var(&fromID, "from", 0, "older snapshot ID (default: oldest retained)")
	cmd.Flags().Int64Var(&toID, "to", 0, "newer snapshot ID (default: newest retained)")
	return cmd
}
