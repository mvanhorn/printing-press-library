// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: training load (Fitness/Fatigue/Form). Not generated.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

type loadView struct {
	FTPUsed      float64     `json:"ftp_used"`
	Days         int         `json:"days"`
	WorkoutsUsed int         `json:"workouts_used"`
	Current      *loadPoint  `json:"current,omitempty"`
	Form         string      `json:"form,omitempty"`
	Series       []loadPoint `json:"series"`
	Note         string      `json:"note"`
}

func newNovelLoadCmd(flags *rootFlags) *cobra.Command {
	var days int
	var ftp float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "load",
		Short: "Compute CTL (Fitness), ATL (Fatigue), and TSB (Form)",
		Long: "Compute the Fitness (CTL, 42-day), Fatigue (ATL, 7-day), and Form (TSB = CTL-ATL)\n" +
			"training-load series from your synced rides — the Performance Management Chart\n" +
			"TrainingPeaks charges for, computed locally from the workout mirror.\n\n" +
			"Per-ride load uses Wahoo's recorded TSS when present, falling back to a\n" +
			"power-vs-FTP estimate (normalized power preferred) then ride duration.\n" +
			"For all-time records use 'bests'; for a fixed recent total use 'digest'.",
		Example:     "  wahoo-pp-cli load --days 90 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			path := dbPathOrDefault(dbPath)
			if mirrorMissing(cmd, flags, path, "workouts", `{"series":[]}`) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "workouts") {
				hintIfStale(cmd, db, "workouts", flags.maxAge)
			}
			ws, err := loadWorkouts(db)
			if err != nil {
				return fmt.Errorf("reading workouts: %w", err)
			}
			usedFTP := ftp
			if usedFTP <= 0 {
				usedFTP, err = latestFTP(db)
				if err != nil {
					return fmt.Errorf("reading power zones: %w", err)
				}
			}
			series := computeLoadSeries(ws, usedFTP, days, time.Now())
			view := loadView{
				FTPUsed:      usedFTP,
				Days:         days,
				WorkoutsUsed: countWithStarts(ws),
				Series:       series,
				Note:         loadNote(usedFTP),
			}
			if n := len(series); n > 0 {
				cur := series[n-1]
				view.Current = &cur
				view.Form = formLabel(cur.TSB)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printLoadHuman(cmd, view)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 90, "Trailing days of the Form curve to show")
	cmd.Flags().Float64Var(&ftp, "ftp", 0, "FTP in watts for load math (default: latest from power-zones)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/wahoo-pp-cli/data.db)")
	return cmd
}

func loadNote(ftp float64) string {
	base := "Load uses Wahoo's recorded per-ride TSS when present, falling back to power-vs-FTP (normalized power preferred) then ride duration."
	if ftp > 0 {
		return base + fmt.Sprintf(" FTP %.0fW from power-zones.", ftp)
	}
	return base + " No FTP set (sync power-zones or pass --ftp); recorded TSS still applies for rides that have it."
}

func countWithStarts(ws []parsedWorkout) int {
	n := 0
	for _, w := range ws {
		if w.HasStarts {
			n++
		}
	}
	return n
}

func printLoadHuman(cmd *cobra.Command, v loadView) error {
	out := cmd.OutOrStdout()
	if v.Current != nil {
		fmt.Fprintf(out, "Form: %s  (CTL %.1f / ATL %.1f / TSB %.1f) as of %s\n",
			v.Form, v.Current.CTL, v.Current.ATL, v.Current.TSB, v.Current.Date)
	} else {
		fmt.Fprintln(out, "No dated workouts in the local mirror yet.")
	}
	fmt.Fprintf(out, "Based on %d workouts; FTP %.0fW.\n\n", v.WorkoutsUsed, v.FTPUsed)
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "DATE\tLOAD\tCTL\tATL\tTSB")
	start := 0
	if len(v.Series) > 14 {
		start = len(v.Series) - 14
	}
	for _, p := range v.Series[start:] {
		fmt.Fprintf(tw, "%s\t%.1f\t%.1f\t%.1f\t%.1f\n", p.Date, p.Load, p.CTL, p.ATL, p.TSB)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%s\n", v.Note)
	return nil
}
