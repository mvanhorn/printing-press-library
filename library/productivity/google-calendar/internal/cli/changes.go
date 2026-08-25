// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-account change feed since a caller-held watermark.
// Hand-implemented; classification lives in internal/verdict.

package cli

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
	"github.com/spf13/cobra"
)

// changeRow is one changed event in the machine envelope.
type changeRow struct {
	Account    string `json:"account"`
	Calendar   string `json:"calendar"`
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	Updated    string `json:"updated"`
	Start      string `json:"start"`
	End        string `json:"end"`
	ChangeKind string `json:"change_kind"`
}

type changesOutput struct {
	Since    string           `json:"since"`
	Changes  []changeRow      `json:"changes"`
	Coverage verdict.Coverage `json:"coverage"`
}

// pp:data-source live
func newNovelChangesCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Everything that moved, appeared, or was cancelled across all accounts since a watermark",
		Long: `Lists every event across every calendar in calendars.yaml whose upstream
'updated' stamp is at or after --since — deletions included (they surface
with status "cancelled" and change_kind "cancelled"; everything else is
"new_or_updated", because the API cannot distinguish create from edit).
Results are merged across accounts and sorted by updated time, oldest
first, so the newest row is the natural next --since watermark. No local
state is kept: the caller holds the watermark.

Carries the standard coverage block: exit 4 when any manifest calendar
could not be read (the change list may be missing rows), exit 0 otherwise.`,
		Example: `  google-calendar-pp-cli changes --since 2026-08-17T07:00:00Z
  google-calendar-pp-cli changes --since 2026-08-16 --json
  google-calendar-pp-cli changes --agent --select changes.summary,changes.change_kind,coverage`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, err := parseTimeFlag("--since", flagSince)
			if err != nil {
				return err
			}
			if since.IsZero() {
				since = time.Now().Add(-24 * time.Hour)
			}
			since = since.UTC()
			vc, err := loadVerdictContext(flags)
			if err != nil {
				return err
			}
			events, sources := fetchManifestEvents(cmd.Context(), vc, map[string]string{
				"singleEvents": "true",
				"showDeleted":  "true",
				"updatedMin":   since.Format(time.RFC3339),
			})
			sort.SliceStable(events, func(i, j int) bool {
				if !events[i].Updated.Equal(events[j].Updated) {
					return events[i].Updated.Before(events[j].Updated)
				}
				return events[i].ID < events[j].ID
			})
			out := changesOutput{
				Since:    since.Format(time.RFC3339),
				Changes:  make([]changeRow, 0, len(events)),
				Coverage: verdict.BuildCoverage(sources),
			}
			for _, e := range events {
				ref := e.Ref()
				row := changeRow{
					Account:    e.Account,
					Calendar:   e.Calendar,
					ID:         e.ID,
					Summary:    e.Summary,
					Status:     e.Status,
					Start:      ref.Start,
					End:        ref.End,
					ChangeKind: verdict.ChangeKind(e.Status),
				}
				if !e.Updated.IsZero() {
					row.Updated = e.Updated.Format(time.RFC3339)
				}
				out.Changes = append(out.Changes, row)
			}
			if err := emitVerdict(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "changes since %s: %d\n", out.Since, len(out.Changes))
				for _, r := range out.Changes {
					fmt.Fprintf(w, "%s  %-13s %s/%s  %q  (%s)\n", r.Updated, r.ChangeKind, r.Account, r.Calendar, r.Summary, r.Status)
				}
				fmt.Fprintln(w, coverageSummary(out.Coverage))
				coverageErrorLines(w, out.Coverage)
			}); err != nil {
				return err
			}
			if !out.Coverage.Complete {
				return exitDegraded("coverage incomplete (%d/%d calendars read) — the change list may be missing rows", out.Coverage.Checked, out.Coverage.Of)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Watermark: RFC3339 or YYYY-MM-DD (local midnight); events updated at/after this are listed (default: 24h ago)")
	return cmd
}
