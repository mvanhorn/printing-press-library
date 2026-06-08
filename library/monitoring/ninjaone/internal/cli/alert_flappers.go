// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source live

type flapper struct {
	DeviceID      int64  `json:"deviceId"`
	ConditionName string `json:"conditionName"`
	Cycles        int    `json:"cycles"`
	FirstSeen     string `json:"firstSeen"`
	LastSeen      string `json:"lastSeen"`
}

type alertFlappersView struct {
	Flappers       []flapper `json:"flappers"`
	Count          int       `json:"count"`
	WindowSecs     int64     `json:"window_secs"`
	MinCycles      int       `json:"min_cycles"`
	EventsRecorded int       `json:"events_recorded"`
	Note           string    `json:"note,omitempty"`
}

func newNovelAlertFlappersCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWindow    string
		flagMinCycles int
		flagLimit     int
	)

	cmd := &cobra.Command{
		Use:   "alert-flappers",
		Short: "Rank conditions that repeatedly fire and auto-resolve over a window, by cycle count per device and condition.",
		Long: `Record current alert events into a local history table, then from history within
--window count fire events per (device, condition) and report those that fired at
least --min-cycles times. Builds signal over repeated runs.

Examples:
  ninjaone-pp-cli alert-flappers
  ninjaone-pp-cli alert-flappers --window 30d --min-cycles 5
  ninjaone-pp-cli alert-flappers --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRunPreview(cmd, flags, "would record alert events into local history and rank conditions that flap within --window")
			}

			window := 7 * 24 * time.Hour
			if flagWindow != "" {
				d, err := cliutil.ParseDurationLoose(flagWindow)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --window: %w", err))
				}
				window = d
			}
			if flagMinCycles < 1 {
				flagMinCycles = 3
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			alerts, err := fetchAlerts(ctx, c)
			if err != nil {
				return err
			}

			db, err := store.OpenWithContext(ctx, defaultDBPath("ninjaone-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			const kind = "alert"
			recorded := 0
			for _, a := range alerts {
				cond := a.ConditionName
				if cond == "" {
					cond = a.SourceName
				}
				key := fmt.Sprintf("%d:%s", a.DeviceID, cond)
				// Use the alert UID as the run_id so distinct fire events
				// dedupe per-uid but accumulate across runs and conditions.
				runID := a.UID
				if runID == "" {
					runID = strconv.FormatInt(a.createSeconds(), 10)
				}
				obs := a.createSeconds()
				if obs <= 0 {
					obs = time.Now().Unix()
				}
				if err := db.RecordHistory(ctx, kind, key, runID, obs, map[string]any{
					"condition": cond, "severity": a.Severity,
				}); err != nil {
					return err
				}
				recorded++
			}

			since := time.Now().Add(-window).Unix()
			entities, err := db.EventCountsSince(ctx, kind, since, flagMinCycles)
			if err != nil {
				return err
			}

			flappers := make([]flapper, 0, len(entities))
			for _, e := range entities {
				devID, cond := splitEntityKey(e.EntityKey)
				flappers = append(flappers, flapper{
					DeviceID:      devID,
					ConditionName: cond,
					Cycles:        e.Count,
					FirstSeen:     isoFromEpoch(e.FirstSeen),
					LastSeen:      isoFromEpoch(e.LastSeen),
				})
			}
			if n := boundLimit(len(flappers), flagLimit); n < len(flappers) {
				flappers = flappers[:n]
			}

			view := alertFlappersView{
				Flappers:       flappers,
				Count:          len(flappers),
				WindowSecs:     int64(window.Seconds()),
				MinCycles:      flagMinCycles,
				EventsRecorded: recorded,
			}
			if len(flappers) == 0 {
				view.Note = fmt.Sprintf("recorded %d alert event(s); no (device,condition) has fired >= %d times within %s yet — run again over time to build history", recorded, flagMinCycles, window)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(flappers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			headers := []string{"DEVICE", "CONDITION", "CYCLES", "FIRST", "LAST"}
			rows := make([][]string, 0, len(flappers))
			for _, f := range flappers {
				rows = append(rows, []string{strconv.FormatInt(f.DeviceID, 10), f.ConditionName, strconv.Itoa(f.Cycles), f.FirstSeen, f.LastSeen})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Lookback window for counting flap cycles (e.g. 7d, 30d)")
	cmd.Flags().IntVar(&flagMinCycles, "min-cycles", 3, "Minimum fire events for a condition to be reported")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of flappers to return (0 = all)")
	return cmd
}
