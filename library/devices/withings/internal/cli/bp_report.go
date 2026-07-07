// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `bp-report` — dated blood-pressure + AFib table with user annotations.
// pp:data-source local
//
// Hand-authored implementation (RunE body replaced from the generated stub).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
	"github.com/spf13/cobra"
)

// bpRow is one dated blood-pressure row.
type bpRow struct {
	Date      string `json:"date"`
	Systolic  int    `json:"systolic"`
	Diastolic int    `json:"diastolic"`
	Pulse     int    `json:"pulse"`
	Afib      string `json:"afib"`
	Note      string `json:"note"`
}

func newNovelBpReportCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string
	var notes []string

	cmd := &cobra.Command{
		Use:         "bp-report",
		Short:       "A dated blood-pressure + AFib table with your own annotations (medication changes, symptoms)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		Long: `Builds a dated blood-pressure history from the local mirror — systolic,
diastolic, pulse, and any AFib classification from ECG recordings — and lets
you attach free-text notes per date (e.g. medication changes, symptoms) that
persist locally in a bp_notes table. The clean history to hand a cardiologist.

Reads local data only (and writes only your annotations). Sync first with:
  withings-pp-cli sync --resources measure,heart

Add a note:
  withings-pp-cli bp-report --note 2026-06-10="started 5mg lisinopril"`,
		Example: "  withings-pp-cli bp-report\n" +
			"  withings-pp-cli bp-report --since 180d\n" +
			"  withings-pp-cli bp-report --note 2026-06-10=\"started 5mg lisinopril\"\n" +
			"  withings-pp-cli bp-report --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			since, err := parseSinceFlag(flagSince, 90*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-since)

			db, handled, err := openLocalForAnalyticsRW(cmd, flags, dbPath, "measure,heart", true)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			defer db.Close()

			if err := db.EnsureBPNotesTable(); err != nil {
				return err
			}
			// Persist any --note DATE=TEXT annotations before rendering.
			for _, spec := range notes {
				date, text, ok := strings.Cut(spec, "=")
				if !ok || strings.TrimSpace(date) == "" {
					return usageErr(fmt.Errorf("invalid --note %q: expected DATE=TEXT (e.g. 2026-06-10=\"note\")", spec))
				}
				if _, valid := parseYMD(date); !valid {
					return usageErr(fmt.Errorf("invalid --note date %q: expected YYYY-MM-DD", date))
				}
				if err := db.UpsertBPNote(strings.TrimSpace(date), text); err != nil {
					return err
				}
			}

			rows, err := computeBPReport(db, cutoff)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Lookback window (e.g. 90d, 12w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard data dir)")
	cmd.Flags().StringArrayVar(&notes, "note", nil, "Attach a dated note: --note YYYY-MM-DD=TEXT (repeatable)")
	return cmd
}

// computeBPReport builds the dated BP rows from measures + heart recordings in
// the window, joined with stored annotations. Returns rows sorted by date.
func computeBPReport(db *store.Store, cutoff time.Time) ([]bpRow, error) {
	noteByDay, err := db.BPNotes()
	if err != nil {
		return nil, err
	}

	type agg struct {
		systolic, diastolic, pulse int
	}
	byDay := map[string]*agg{}

	groups, err := loadMeasureGroups(db, cutoff)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		day := epochToYMD(g.Date)
		if day == "" {
			continue
		}
		sys, hasSys := g.scaledOfType(10)
		dia, hasDia := g.scaledOfType(9)
		pulse, hasPulse := g.scaledOfType(11)
		if !hasSys && !hasDia {
			continue // not a BP measurement group
		}
		a := byDay[day]
		if a == nil {
			a = &agg{}
			byDay[day] = a
		}
		if hasSys {
			a.systolic = int(sys)
		}
		if hasDia {
			a.diastolic = int(dia)
		}
		if hasPulse {
			a.pulse = int(pulse)
		}
	}

	// AFib events from heart recordings: ecg.afib > 0 on a given day.
	afibByDay := map[string]int{}
	cutYMD := cutoff.UTC().Format("2006-01-02")
	hRows, err := localRows(db, "heart")
	if err != nil {
		return nil, err
	}
	for _, raw := range hRows {
		var h struct {
			Timestamp int64 `json:"timestamp"`
			Data      struct {
				ECG struct {
					Afib int `json:"afib"`
				} `json:"ecg"`
			} `json:"data"`
			ECG struct {
				Afib int `json:"afib"`
			} `json:"ecg"`
		}
		if json.Unmarshal(raw, &h) != nil {
			continue
		}
		day := epochToYMD(h.Timestamp)
		if day == "" || day < cutYMD {
			continue
		}
		afib := h.Data.ECG.Afib
		if afib == 0 {
			afib = h.ECG.Afib // tolerate a flattened shape
		}
		if afib > afibByDay[day] {
			afibByDay[day] = afib
		}
	}

	// Union of BP days, AFib days, and annotated days (so a note on a day with
	// no reading still surfaces).
	daySet := map[string]struct{}{}
	for d := range byDay {
		daySet[d] = struct{}{}
	}
	for d := range afibByDay {
		daySet[d] = struct{}{}
	}
	for d := range noteByDay {
		if d >= cutYMD {
			daySet[d] = struct{}{}
		}
	}

	days := make([]string, 0, len(daySet))
	for d := range daySet {
		days = append(days, d)
	}
	sort.Strings(days)

	out := make([]bpRow, 0, len(days))
	for _, d := range days {
		row := bpRow{Date: d, Afib: "negative", Note: noteByDay[d]}
		if a := byDay[d]; a != nil {
			row.Systolic = a.systolic
			row.Diastolic = a.diastolic
			row.Pulse = a.pulse
		}
		if afib, ok := afibByDay[d]; ok {
			row.Afib = withingsAfibLabel(afib)
		}
		out = append(out, row)
	}
	return out, nil
}
