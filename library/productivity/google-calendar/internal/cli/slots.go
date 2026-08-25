// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-account open-slot finder. Hand-implemented; interval
// algebra lives in internal/verdict.

package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
	"github.com/spf13/cobra"
)

// slotOut is one ranked open window in the machine envelope.
type slotOut struct {
	Start           string `json:"start"`
	End             string `json:"end"`
	DurationMinutes int    `json:"duration_minutes"`
}

type slotsOutput struct {
	Slots    []slotOut        `json:"slots"`
	Coverage verdict.Coverage `json:"coverage"`
}

// pp:data-source live
func newNovelSlotsCmd(flags *rootFlags) *cobra.Command {
	var flagDuration string
	var flagFrom string
	var flagTo string
	var flagBetween string
	var flagTZ string

	cmd := &cobra.Command{
		Use:   "slots",
		Short: "Ranked open windows of a requested duration across every account's manifest calendars",
		Long: `Queries live free/busy for every calendar in calendars.yaml (one freeBusy
call per account), merges the busy intervals, and inverts them within
[--from, --to] — optionally clipped to a daily --between wall-clock window
in --tz. Slots of at least --duration are ranked longest first.

An "open" claim follows the same coverage contract as conflicts: if any
manifest calendar could not be read, the output marks coverage.complete
false and the command exits 4 — a slot over partial data is a guess, not
an answer. Exit 0 (with or without qualifying slots) requires complete
coverage.`,
		Example: `  google-calendar-pp-cli slots --duration 90m --from 2026-08-18 --to 2026-08-22 --between 09:00-17:00
  google-calendar-pp-cli slots --duration 30m --from 2026-08-18 --to 2026-08-19 --tz America/Denver --json
  google-calendar-pp-cli slots --duration 2h --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagDuration == "" {
				return usageErr(fmt.Errorf("--duration is required (e.g. --duration 90m)"))
			}
			dur, err := time.ParseDuration(flagDuration)
			if err != nil || dur <= 0 {
				return usageErr(fmt.Errorf("invalid --duration %q: use Go duration syntax like 90m, 1h30m", flagDuration))
			}
			from, to, err := resolveWindow(flagFrom, flagTo)
			if err != nil {
				return err
			}
			loc, err := resolveLocation(flagTZ)
			if err != nil {
				return err
			}
			windows := []verdict.Interval{{Start: from, End: to}}
			if flagBetween != "" {
				startMin, endMin, berr := parseBetween(flagBetween)
				if berr != nil {
					return berr
				}
				windows = verdict.DailyWindows(from, to, startMin, endMin, loc)
			}
			vc, err := loadVerdictContext(flags)
			if err != nil {
				return err
			}
			var busy []verdict.Interval
			var sources []verdict.Source
			accountOrder, byAccount := vc.manifest.ByAccount()
			for _, account := range accountOrder {
				entries := byAccount[account]
				ids := make([]string, 0, len(entries))
				for _, e := range entries {
					ids = append(ids, e.ID)
				}
				c, cerr := vc.clientFor(account)
				if cerr != nil {
					fetchedAt := time.Now().UTC().Format(time.RFC3339)
					for _, id := range ids {
						sources = append(sources, verdict.Source{Account: account, Calendar: id, FetchedAt: fetchedAt, Error: cerr.Error()})
					}
					continue
				}
				accBusy, accSources := fetchFreeBusy(cmd.Context(), c, account, ids, from, to)
				busy = append(busy, accBusy...)
				sources = append(sources, accSources...)
			}
			cov := verdict.BuildCoverage(sources)
			free := verdict.FreeWithinWindows(windows, busy)
			ranked := verdict.FindSlots(free, dur)
			out := slotsOutput{Slots: make([]slotOut, 0, len(ranked)), Coverage: cov}
			for _, s := range ranked {
				out.Slots = append(out.Slots, slotOut{
					Start:           s.Start.UTC().Format(time.RFC3339),
					End:             s.End.UTC().Format(time.RFC3339),
					DurationMinutes: int(s.Duration() / time.Minute),
				})
			}
			if err := emitVerdict(cmd, flags, out, func(w io.Writer) {
				if len(ranked) == 0 {
					fmt.Fprintf(w, "no open slots of at least %s in %s → %s\n", dur, from.Format(time.RFC3339), to.Format(time.RFC3339))
				}
				for i, s := range ranked {
					fmt.Fprintf(w, "%2d. %s → %s  (%d min)\n", i+1,
						s.Start.In(loc).Format("Mon 2006-01-02 15:04 MST"),
						s.End.In(loc).Format("15:04 MST"),
						int(s.Duration()/time.Minute))
				}
				fmt.Fprintln(w, coverageSummary(cov))
				coverageErrorLines(w, cov)
			}); err != nil {
				return err
			}
			if !cov.Complete {
				return exitDegraded("coverage incomplete (%d/%d calendars read) — open-slot claims are NOT confident", cov.Checked, cov.Of)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDuration, "duration", "", "Minimum slot length, Go duration syntax (e.g. 90m, 1h30m); required")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Search window start: YYYY-MM-DD (local midnight) or RFC3339 (default: today)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Search window end: YYYY-MM-DD (local midnight) or RFC3339 (default: --from +7 days)")
	cmd.Flags().StringVar(&flagBetween, "between", "", "Daily wall-clock window HH:MM-HH:MM in --tz (e.g. 09:00-17:00); default: the whole window")
	cmd.Flags().StringVar(&flagTZ, "tz", "local", "Time zone for --between and human output: 'local' or an IANA name like America/Denver")
	return cmd
}
