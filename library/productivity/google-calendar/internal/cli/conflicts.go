// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-account conflict verdict. Hand-implemented; the pure
// logic lives in internal/verdict (see that package's tests for the
// behavioral contract).

package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
	"github.com/spf13/cobra"
)

// conflictsOutput is the machine envelope for the conflicts command.
type conflictsOutput struct {
	Conflicts   []verdict.Conflict   `json:"conflicts"`
	Mirrors     []verdict.MirrorPair `json:"mirrors"`
	AllDayNotes []verdict.AllDayNote `json:"all_day_notes"`
	Coverage    verdict.Coverage     `json:"coverage"`
}

// pp:data-source live
func newNovelConflictsCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:   "conflicts",
		Short: "Am I double-booked? Cross-account overlap check that refuses a confident all-clear over partial data",
		Long: `Fans out over every calendar in calendars.yaml (all accounts), classifies
busy time (tentative counts as busy; transparent, self-declined, and
cancelled do not), and reports pairwise overlaps of busy timed events.

Same-time same-title events on DIFFERENT accounts are reported as suspected
mirrors, not conflicts. All-day events are never overlap-checked against
timed events; their interactions surface under all_day_notes.

Every output carries a coverage block. The all-clear is only trustworthy
when coverage.complete is true — if any manifest calendar could not be
read, the verdict downgrades explicitly instead of pretending.

Exit codes: 0 = no conflicts with complete coverage; 3 = conflicts found
(even when coverage is incomplete — check coverage.complete); 4 = no
conflicts found but coverage incomplete (degraded all-clear).`,
		Example: `  google-calendar-pp-cli conflicts --from 2026-08-18 --to 2026-08-25
  google-calendar-pp-cli conflicts --from 2026-08-18T09:00:00-06:00 --to 2026-08-18T18:00:00-06:00 --json
  google-calendar-pp-cli conflicts --agent --select conflicts,coverage`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			from, to, err := resolveWindow(flagFrom, flagTo)
			if err != nil {
				return err
			}
			vc, err := loadVerdictContext(flags)
			if err != nil {
				return err
			}
			events, sources := fetchManifestEvents(cmd.Context(), vc, map[string]string{
				"singleEvents": "true",
				"timeMin":      from.Format(time.RFC3339),
				"timeMax":      to.Format(time.RFC3339),
			})
			report := verdict.FindConflicts(events)
			cov := verdict.BuildCoverage(sources)
			out := conflictsOutput{
				Conflicts:   report.Conflicts,
				Mirrors:     report.Mirrors,
				AllDayNotes: report.AllDayNotes,
				Coverage:    cov,
			}
			if err := emitVerdict(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "window: %s → %s\n", from.Format(time.RFC3339), to.Format(time.RFC3339))
				if len(out.Conflicts) == 0 {
					fmt.Fprintln(w, "no conflicts")
				}
				for _, c := range out.Conflicts {
					fmt.Fprintf(w, "CONFLICT %s → %s\n  %s/%s %q\n  %s/%s %q\n",
						c.OverlapStart, c.OverlapEnd,
						c.A.Account, c.A.Calendar, c.A.Summary,
						c.B.Account, c.B.Calendar, c.B.Summary)
				}
				for _, m := range out.Mirrors {
					fmt.Fprintf(w, "mirror (not a conflict): %q on %s and %s at %s\n", m.A.Summary, m.A.Account, m.B.Account, m.A.Start)
				}
				for _, n := range out.AllDayNotes {
					fmt.Fprintf(w, "all-day note (%s): %q (%s) with %q on %s\n", n.Kind, n.AllDay.Summary, n.AllDay.Account, n.Other.Summary, n.Date)
				}
				fmt.Fprintln(w, coverageSummary(cov))
				coverageErrorLines(w, cov)
			}); err != nil {
				return err
			}
			if len(out.Conflicts) > 0 {
				return exitFindings("%d conflict(s) found (coverage %d/%d)", len(out.Conflicts), cov.Checked, cov.Of)
			}
			if !cov.Complete {
				return exitDegraded("no conflicts found, but coverage incomplete (%d/%d calendars read) — all-clear is NOT confident", cov.Checked, cov.Of)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start: YYYY-MM-DD (local midnight) or RFC3339 (default: today)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end: YYYY-MM-DD (local midnight) or RFC3339 (default: --from +7 days)")
	return cmd
}
