// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/store"
)

type typicalFilterView struct {
	Ride       *string `json:"ride"`
	Weekdays   *string `json:"weekdays"`
	HourFrom   *int    `json:"hour_from"`
	HourTo     *int    `json:"hour_to"`
	MinSamples int     `json:"min_samples"`
}

type historySpanView struct {
	First *string `json:"first"`
	Last  *string `json:"last"`
}

type typicalResult struct {
	Park          parkView          `json:"park"`
	DBPath        string            `json:"db_path"`
	Filters       typicalFilterView `json:"filters"`
	StoredSamples int               `json:"stored_samples"`
	HistorySpan   historySpanView   `json:"history_span"`
	Cells         []parks.Typical   `json:"cells"`
	TotalCells    int               `json:"total_cells"`
	Note          string            `json:"note,omitempty"`
}

func newNovelTypicalCmd(flags *rootFlags) *cobra.Command {
	var parkArg string
	var rideFilter string
	var weekdayCSV string
	var hourArg string
	var minSamples int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "typical",
		Short: "Median and p75 waits per ride by weekday and hour from your local snapshots",
		Long: strings.Trim(`
Summarize the local wait history written by 'snapshot' for one park: median
and p75 of open-ride waits per ride, per weekday and hour (Asia/Tokyo), with
the sample count, distinct days and first/last date of the samples used.
Cells with fewer than --min-samples open samples show status "insufficient"
and null statistics. Closed readings are counted separately and never as 0.

This is a description of your own recorded data, not a forecast. Queue-Times
has no history API, so results cover only the times you ran 'snapshot'.

Use this command for historical wait patterns by weekday and hour.
Do NOT use this command for a live comparison on the park day; use 'waits' instead.`, "\n"),
		Example: strings.Trim(`
  japan-theme-parks-pp-cli typical --park 275 --weekday sat --hour 10
  japan-theme-parks-pp-cli typical --park usj --ride "Mario" --weekday weekend
  japan-theme-parks-pp-cli typical --park tdl --hour 9-12 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--park=275"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "typical")
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if parkArg == "" && len(args) > 0 {
				parkArg = args[0]
			}
			if parkArg == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--park is required (one of: %s)", strings.Join(parks.Keys(), ", ")))
			}
			park, err := lookupParkArg(parkArg)
			if err != nil {
				return err
			}
			weekdays, err := parks.ParseWeekdays(weekdayCSV)
			if err != nil {
				return usageErr(err)
			}
			hourLo, hourHi, err := parseHourRange(hourArg)
			if err != nil {
				return usageErr(err)
			}
			if minSamples < 1 {
				return usageErr(fmt.Errorf("--min-samples must be at least 1"))
			}
			dbPath, err = resolveDBPath(dbPath)
			if err != nil {
				return usageErr(err)
			}
			res := typicalResult{
				Park: parkToView(park), DBPath: dbPath, Cells: make([]parks.Typical, 0),
				Filters: typicalFilters(rideFilter, weekdayCSV, hourArg, hourLo, hourHi, minSamples),
			}
			meta := newMeta()
			meta.Sources = append(meta.Sources, sourceRef{
				Name: "local snapshot history", Kind: "local SQLite (recorded from queue-times.com)", URL: dbPath,
				Attribution: parks.QueueTimesAttribution, Link: parks.QueueTimesURL,
			})
			emit := func() error {
				out := struct {
					Meta    respMeta      `json:"meta"`
					Results typicalResult `json:"results"`
				}{meta, res}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFilteredKeep(cmd.OutOrStdout(), out, flags, "median_minutes", "p75_minutes", "samples", "closed_samples", "distinct_days", "first_date", "last_date", "weekday", "hour", "ride_name", "land")
				}
				return printTypicalHuman(cmd, res, meta)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			q := store.WaitQuery{ParkID: park.QueueTimesID, RideLike: strings.TrimSpace(rideFilter)}
			for _, d := range parks.Weekdays {
				if n, _ := parks.WeekdayNumber(d); weekdays[d] {
					q.Weekdays = append(q.Weekdays, n)
				}
			}
			if strings.TrimSpace(hourArg) != "" {
				q.HourSet, q.HourFrom, q.HourTo = true, hourLo, hourHi
			}
			hist, err := loadHistory(ctx, cmd.ErrOrStderr(), flags, dbPath, q)
			if err != nil {
				return err
			}
			if !hist.Exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local history at %s\nrun: japan-theme-parks-pp-cli snapshot --db %s\n", dbPath, dbPath)
				res.Note = "no local history yet; run 'snapshot' on a schedule first"
				return emit()
			}
			res.StoredSamples = hist.Span.Rows
			if hist.Bad > 0 {
				meta.Notes = append(meta.Notes, fmt.Sprintf("%d stored rows skipped: unreadable timestamps", hist.Bad))
			}
			if hist.Span.Rows > 0 {
				first, last := jst(hist.Span.First), jst(hist.Span.Last)
				res.HistorySpan = historySpanView{First: &first, Last: &last}
			}
			samples := samplesOf(hist.Rows)
			cells := parks.Summarize(samples, minSamples)
			res.TotalCells = len(cells)
			if limit > 0 && len(cells) > limit {
				cells = cells[:limit]
				meta.Notes = append(meta.Notes, fmt.Sprintf("showing %d of %d cells; raise --limit or narrow --ride/--weekday/--hour", limit, res.TotalCells))
			}
			res.Cells = cells
			switch {
			case hist.Span.Rows == 0:
				res.Note = "no stored samples for this park; run 'snapshot' on a schedule first"
			case len(samples) == 0:
				res.Note = "no stored samples match these filters"
			}
			return emit()
		},
	}
	cmd.Flags().StringVar(&parkArg, "park", "", "Park key, Queue-Times id or name (tdl, tds, usj, fujiq, legoland)")
	cmd.Flags().StringVar(&rideFilter, "ride", "", "Only rides whose name contains this text (case-insensitive)")
	cmd.Flags().StringVar(&weekdayCSV, "weekday", "", "Weekdays to include: mon..sun, comma-separated, or weekday/weekend (default: all)")
	cmd.Flags().StringVar(&hourArg, "hour", "", "Hour of day in Asia/Tokyo, 0-23, or a range such as 9-12 (default: all)")
	cmd.Flags().IntVar(&minSamples, "min-samples", 6, "Open samples needed before median/p75 are shown instead of \"insufficient\"")
	cmd.Flags().IntVar(&limit, "limit", 100, "Return at most this many ride x weekday x hour cells (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Snapshot history database path (default: the CLI data directory)")
	return cmd
}

func typicalFilters(ride, weekdays, hourArg string, lo, hi, minSamples int) typicalFilterView {
	f := typicalFilterView{MinSamples: minSamples}
	if ride != "" {
		f.Ride = &ride
	}
	if weekdays != "" {
		f.Weekdays = &weekdays
	}
	if strings.TrimSpace(hourArg) != "" {
		f.HourFrom, f.HourTo = &lo, &hi
	}
	return f
}

func parseHourRange(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 23, nil
	}
	lo, hi := s, s
	if i := strings.Index(s, "-"); i > 0 {
		lo, hi = s[:i], s[i+1:]
	}
	a, err1 := strconv.Atoi(strings.TrimSpace(lo))
	b, err2 := strconv.Atoi(strings.TrimSpace(hi))
	if err1 != nil || err2 != nil || a < 0 || b > 23 || a > b {
		return 0, 0, fmt.Errorf("--hour must be 0-23 or a range like 9-12 (got %q)", s)
	}
	return a, b, nil
}

func printTypicalHuman(cmd *cobra.Command, res typicalResult, meta respMeta) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s (%s) — %d stored samples", res.Park.NameEN, res.Park.NameJA, res.StoredSamples)
	if res.HistorySpan.First != nil {
		fmt.Fprintf(w, " from %s to %s", *res.HistorySpan.First, *res.HistorySpan.Last)
	}
	fmt.Fprintln(w)
	if len(res.Cells) == 0 {
		if res.Note != "" {
			fmt.Fprintln(w, "note:", res.Note)
		}
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "RIDE\tDAY\tHOUR\tMEDIAN\tP75\tN\tDAYS\tFIRST..LAST\tSTATUS")
	for _, c := range res.Cells {
		med, p75 := "-", "-"
		if c.Median != nil {
			med = fmt.Sprintf("%.0f", *c.Median)
			p75 = fmt.Sprintf("%.0f", *c.P75)
		}
		span := "-"
		if c.FirstDate != nil {
			span = *c.FirstDate + ".." + *c.LastDate
		}
		fmt.Fprintf(tw, "%s\t%s\t%02d\t%s\t%s\t%d\t%d\t%s\t%s\n", c.RideName, c.Weekday, c.Hour, med, p75, c.Samples, c.DistinctDays, span, c.Status)
	}
	_ = tw.Flush()
	printAttribution(w, meta)
	return nil
}
