// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type calendarCabin struct {
	Available      bool    `json:"available"`
	Direct         bool    `json:"direct"`
	Miles          float64 `json:"miles"`
	DirectMiles    float64 `json:"direct_miles"`
	Seats          float64 `json:"seats"`
	CheapestSource string  `json:"cheapest_source"`
}
type calendarDateEntry struct {
	Date     string        `json:"date"`
	Sources  []string      `json:"sources"`
	Economy  calendarCabin `json:"economy"`
	Premium  calendarCabin `json:"premium"`
	Business calendarCabin `json:"business"`
	First    calendarCabin `json:"first"`
}
type calendarRawRow struct {
	date, source string
	cabins       [4]calendarRawCabin
}
type calendarRawCabin struct {
	available, direct         bool
	miles, directMiles, seats sql.NullFloat64
}

func newNovelCalendarCmd(flags *rootFlags) *cobra.Command {
	var origin, destination, source, start, end, flagDB string
	var limit int
	cmd := &cobra.Command{
		Use:         "calendar",
		Short:       "Turn one route's synced availability into a date-by-cabin matrix you can scan at a glance.",
		Long:        "Use this command to view one route's full cabin-by-date matrix from already-synced availability. Do NOT use this to filter across multiple routes/programs for direct-only options under a mileage ceiling; use 'direct-scan' instead.",
		Example:     "  seats-aero-pp-cli calendar --origin JFK --destination NRT --source united --start 2026-10-01 --end 2026-12-31 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--origin=JFK;--destination=NRT;--start=2026-10-01;--end=2026-12-31"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "calendar")
			}
			if flags.dataSource == "live" {
				return novelUsageError(cmd, flags, fmt.Errorf("calendar has no live equivalent; it reads the local store (use --data-source local or auto)"))
			}
			origin, destination = strings.ToUpper(strings.TrimSpace(origin)), strings.ToUpper(strings.TrimSpace(destination))
			if origin == "" {
				return novelUsageError(cmd, flags, fmt.Errorf("--origin is required"))
			}
			if destination == "" {
				return novelUsageError(cmd, flags, fmt.Errorf("--destination is required"))
			}
			if limit <= 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("--limit must be greater than zero"))
			}
			now := time.Now().UTC()
			if start == "" {
				start = now.Format("2006-01-02")
			}
			if end == "" {
				end = now.AddDate(0, 0, 90).Format("2006-01-02")
			}
			startDate, err := time.Parse("2006-01-02", start)
			if err != nil {
				return novelUsageError(cmd, flags, fmt.Errorf("invalid --start %q: use YYYY-MM-DD", start))
			}
			endDate, err := time.Parse("2006-01-02", end)
			if err != nil {
				return novelUsageError(cmd, flags, fmt.Errorf("invalid --end %q: use YYYY-MM-DD", end))
			}
			if endDate.Before(startDate) {
				return novelUsageError(cmd, flags, fmt.Errorf("--end must not be before --start"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path := resolveNovelDBPath(flagDB)
			db, err := openNovelStoreAt(ctx, path)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\n%s\n", path, novelStoreMissingHint(path))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printNovelJSON(cmd.OutOrStdout(), make([]calendarDateEntry, 0), flags, nil)
				}
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "availability") {
				hintIfStale(cmd, db, "availability", flags.maxAge)
			}
			query, params := buildCalendarQuery(origin, destination, start, end, source)
			rows, err := db.DB().QueryContext(ctx, query, params...)
			if err != nil {
				return err
			}
			raw := make([]calendarRawRow, 0)
			for rows.Next() {
				var r calendarRawRow
				var available, direct [4]sql.NullInt64
				dest := []any{&r.date, &r.source}
				for i := range r.cabins {
					dest = append(dest, &available[i], &direct[i], &r.cabins[i].miles, &r.cabins[i].directMiles, &r.cabins[i].seats)
				}
				if err := rows.Scan(dest...); err != nil {
					_ = rows.Close()
					return err
				}
				for i := range r.cabins {
					r.cabins[i].available = available[i].Valid && available[i].Int64 == 1
					r.cabins[i].direct = direct[i].Valid && direct[i].Int64 == 1
				}
				raw = append(raw, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
			result := pivotCalendar(raw, limit)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), result, flags, db)
			}
			if len(result) == 0 {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "No synced availability for %s→%s between %s and %s.\n", origin, destination, start, end)
				return err
			}
			return printCalendarTable(cmd, result)
		},
	}
	cmd.Flags().StringVar(&origin, "origin", "", "Origin IATA airport code (required).")
	cmd.Flags().StringVar(&destination, "destination", "", "Destination IATA airport code (required).")
	cmd.Flags().StringVar(&source, "source", "", "Filter by mileage program source identifier.")
	cmd.Flags().StringVar(&start, "start", "", "First travel date in YYYY-MM-DD (default: today).")
	cmd.Flags().StringVar(&end, "end", "", "Last travel date in YYYY-MM-DD (default: today plus 90 days).")
	cmd.Flags().IntVar(&limit, "limit", 120, "Maximum number of dates to return.")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite store (default: ~/.local/share/seats-aero-pp-cli/data.db)")
	return cmd
}

func buildCalendarQuery(origin, destination, start, end, source string) (string, []any) {
	query := `SELECT substr(date,1,10),source,
	 y_available,y_direct,y_mileage_cost_raw,y_direct_mileage_cost_raw,y_remaining_seats,
	 w_available,w_direct,w_mileage_cost_raw,w_direct_mileage_cost_raw,w_remaining_seats,
	 j_available,j_direct,j_mileage_cost_raw,j_direct_mileage_cost_raw,j_remaining_seats,
	 f_available,f_direct,f_mileage_cost_raw,f_direct_mileage_cost_raw,f_remaining_seats
	 FROM availability_all WHERE json_extract(data,'$.Route.OriginAirport')=? AND json_extract(data,'$.Route.DestinationAirport')=? AND date >= ? AND date < date(?, '+1 day')`
	params := []any{origin, destination, start, end}
	if source != "" {
		query += " AND source=?"
		params = append(params, source)
	}
	return query + " ORDER BY date, source, id", params
}

func pivotCalendar(rows []calendarRawRow, limit int) []calendarDateEntry {
	result := make([]calendarDateEntry, 0)
	byDate := make(map[string]int)
	for _, row := range rows {
		date := row.date
		if len(date) > 10 {
			date = date[:10]
		}
		idx, ok := byDate[date]
		if !ok {
			if len(result) >= limit {
				continue
			}
			result = append(result, calendarDateEntry{Date: date, Sources: make([]string, 0)})
			idx = len(result) - 1
			byDate[date] = idx
		}
		e := &result[idx]
		if !containsCalendarSource(e.Sources, row.source) {
			e.Sources = append(e.Sources, row.source)
			sort.Strings(e.Sources)
		}
		cells := []*calendarCabin{&e.Economy, &e.Premium, &e.Business, &e.First}
		for i, rawCell := range row.cabins {
			mergeCalendarCell(cells[i], rawCell, row.source)
		}
	}
	return result
}

func mergeCalendarCell(cell *calendarCabin, raw calendarRawCabin, source string) {
	if raw.available {
		cell.Available = true
		if raw.miles.Valid && (cell.Miles == 0 || raw.miles.Float64 < cell.Miles) {
			cell.Miles, cell.CheapestSource = raw.miles.Float64, source
		}
		if raw.seats.Valid && raw.seats.Float64 > cell.Seats {
			cell.Seats = raw.seats.Float64
		}
	}
	if raw.direct {
		cell.Direct = true
		if raw.directMiles.Valid && (cell.DirectMiles == 0 || raw.directMiles.Float64 < cell.DirectMiles) {
			cell.DirectMiles = raw.directMiles.Float64
		}
	}
}
func containsCalendarSource(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func printCalendarTable(cmd *cobra.Command, entries []calendarDateEntry) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DATE\tSOURCES\tY\tW\tJ\tF")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Date, strings.Join(e.Sources, ","), calendarCellText(e.Economy), calendarCellText(e.Premium), calendarCellText(e.Business), calendarCellText(e.First))
	}
	return tw.Flush()
}
func calendarCellText(cell calendarCabin) string {
	if !cell.Available {
		return "-"
	}
	text := fmt.Sprintf("%gk", cell.Miles/1000)
	if cell.Direct {
		text += "D"
	}
	return text
}
