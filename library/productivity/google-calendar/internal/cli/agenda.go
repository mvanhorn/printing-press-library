// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written absorbed feature: merged cross-account agenda (manifest row #3).

package cli

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
	"github.com/spf13/cobra"
)

type agendaRow struct {
	Account   string `json:"account"`
	Calendar  string `json:"calendar"`
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	Start     string `json:"start"`
	End       string `json:"end"`
	AllDay    bool   `json:"all_day"`
	EventType string `json:"event_type,omitempty"`
	Busy      bool   `json:"busy"`
}

type agendaOutput struct {
	Events   []agendaRow      `json:"events"`
	Coverage verdict.Coverage `json:"coverage"`
}

func newAgendaCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo string
	cmd := &cobra.Command{
		Use:   "agenda",
		Short: "Merged agenda across every manifest calendar, sorted by start, with per-source freshness",
		Long: `Lists every event in the window across ALL calendars in calendars.yaml,
merged and sorted by start time. Each row names its account and calendar;
the output carries the standard coverage block — if any source could not
be read, the agenda says so instead of silently narrowing.

Exit codes: 0 = complete coverage; 4 = one or more sources unreadable.`,
		Example: `  google-calendar-pp-cli agenda --from 2026-08-18 --to 2026-08-19
  google-calendar-pp-cli agenda --from today --to +2d --agent --select events.summary,events.start,events.account`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
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
			sort.Slice(events, func(i, j int) bool { return events[i].Start.Before(events[j].Start) })
			out := agendaOutput{Events: make([]agendaRow, 0, len(events)), Coverage: verdict.BuildCoverage(sources)}
			for _, e := range events {
				if e.Status == "cancelled" {
					continue
				}
				out.Events = append(out.Events, agendaRow{
					Account: e.Account, Calendar: e.Calendar, ID: e.ID, Summary: e.Summary,
					Start: e.Start.Format(time.RFC3339), End: e.End.Format(time.RFC3339),
					AllDay: e.AllDay, EventType: e.EventType, Busy: verdict.IsBusy(e),
				})
			}
			if err := emitVerdict(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "agenda %s → %s (%d events)\n", from.Format(time.RFC3339), to.Format(time.RFC3339), len(out.Events))
				for _, r := range out.Events {
					marker := " "
					if r.AllDay {
						marker = "◻"
					}
					fmt.Fprintf(w, "%s %s  %s/%s  %q\n", marker, r.Start, r.Account, r.Calendar, r.Summary)
				}
				fmt.Fprintln(w, coverageSummary(out.Coverage))
				coverageErrorLines(w, out.Coverage)
			}); err != nil {
				return err
			}
			if !out.Coverage.Complete {
				return exitDegraded("agenda incomplete: %d/%d sources", out.Coverage.Checked, out.Coverage.Of)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start (YYYY-MM-DD or RFC3339; default today)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end (YYYY-MM-DD, RFC3339, or +Nd; default +7d)")
	return cmd
}
