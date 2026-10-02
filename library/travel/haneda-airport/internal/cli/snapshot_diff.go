// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/haneda"
	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelSnapshotDiffCmd(flags *rootFlags) *cobra.Command {
	var before, after string
	var limit, offset int
	cmd := &cobra.Command{Use: "diff", Short: "Compare status, times and facilities between compatible saved scopes", Long: "Compare --before and --after, or the latest compatible cached pair with matching origin/date/kind/direction. Automatic selection reads at most 64 MiB. If older files stop selection after a valid pair is found, compare that pair and report cache_selection_complete:false with notes; a newer pair may remain unexamined. Fewer than two compatible observations yield an empty baseline. Coverage mismatch is an error; a disappearing service is no_longer_reported and never inferred canceled.", Annotations: hanedaAnnotations("computed"), Example: "  haneda-airport-pp-cli snapshot diff --limit 20 --agent"}
	cmd.Flags().StringVar(&before, "before", "", "Earlier complete observation file; pair with --after")
	cmd.Flags().StringVar(&after, "after", "", "Later compatible observation file; pair with --before")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum changed service groups returned (1–200)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Local offset within material changes")
	cmd.Annotations["pp:happy-args"] = "--limit=5"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "snapshot diff")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("snapshot diff uses --before/--after, not positionals"))
		}
		if flags.dataSource == "live" {
			return usageErr(fmt.Errorf("snapshot diff computes saved observations locally; use --data-source auto or local"))
		}
		if limit < 1 || limit > 200 || offset < 0 || offset > 20000 {
			return usageErr(fmt.Errorf("--limit must be 1–200 and --offset 0–20000"))
		}
		if (before == "") != (after == "") {
			return usageErr(fmt.Errorf("provide both --before and --after, or neither for the latest compatible cache baseline"))
		}
		selectionNotes := []string{}
		if before == "" {
			paths, err := hanedaLatestPaths()
			if err != nil {
				return configErr(err)
			}
			before, after, selectionNotes, err = hanedaLatestCompatiblePair(paths)
			if err != nil {
				return configErr(err)
			}
			if before == "" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"changes": []haneda.Change{}, "total_changes": 0, "baseline_sufficient": false, "cache_selection_complete": len(selectionNotes) == 0, "cache_selection_notes": selectionNotes, "budget": haneda.Budget{}, "notes": []string{"At least two compatible saved observations are needed. Run snapshot save again for the same origin/date/kind/direction, or provide --before and --after."}}, flags)
			}
		}
		a, err := haneda.LoadSnapshot(before)
		if err != nil {
			return configErr(fmt.Errorf("read --before: %w", err))
		}
		b, err := haneda.LoadSnapshot(after)
		if err != nil {
			return configErr(fmt.Errorf("read --after: %w", err))
		}
		r, err := haneda.DiffSnapshots(a, b)
		if err != nil {
			return usageErr(err)
		}
		start := offset
		if start > len(r.Changes) {
			start = len(r.Changes)
		}
		end := start + limit
		if end > len(r.Changes) {
			end = len(r.Changes)
		}
		var next *int
		if end < len(r.Changes) {
			n := end
			next = &n
		}
		r.Changes = append([]haneda.Change{}, r.Changes[start:end]...)
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"comparison": r, "baseline_sufficient": true, "cache_selection_complete": len(selectionNotes) == 0, "cache_selection_notes": selectionNotes, "limit": limit, "offset": offset, "next_offset": next, "budget": haneda.Budget{}}, flags)
	}
	return cmd
}
