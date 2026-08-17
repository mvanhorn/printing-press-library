// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelRegshoThresholdWatchCmd(flags *rootFlags) *cobra.Command {
	var flagSymbol string
	var flagGroup string
	var flagName string
	var flagDays int

	cmd := &cobra.Command{
		Use:   "threshold-watch",
		Short: "See which symbols just crossed the Reg SHO 5-day threshold escalation point before it triggers a mandatory close-out.",
		Long: "See which symbols just crossed the Reg SHO 5-day threshold escalation point before it triggers a mandatory close-out.\n\n" +
			"Scans the trailing --days calendar days of the Threshold List dataset and reports the current consecutive-day\n" +
			"streak for --symbol. FINRA requires mandatory close-out once a security has stayed on the threshold list for\n" +
			"5 consecutive settlement days, so a streak of 5 or more is flagged as escalated.",
		Example:     "--symbol GME --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--symbol=AAPL"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s threshold-list records for the trailing %d day(s) and compute the streak for --symbol\n", flagGroup, flagName, flagDays)
				return nil
			}
			if strings.TrimSpace(flagSymbol) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--symbol is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			today := time.Now().UTC()
			start := today.AddDate(0, 0, -flagDays)
			path := replacePathParam(replacePathParam("/data/group/{group}/name/{name}", "group", flagGroup), "name", flagName)
			body := map[string]any{
				// issueSymbolIdentifier and tradeDate (distinct from Reg SHO
				// Daily's tradeReportDate) are this dataset's confirmed
				// symbol and date fields; a live probe confirmed the API
				// filters on both server-side rather than requiring a
				// client-side scan of an arbitrary unsorted sample.
				"compareFilters":   equalCompareFilter("issueSymbolIdentifier", flagSymbol),
				"dateRangeFilters": dateRangeFilter("tradeDate", start, today),
				"limit":            500,
			}
			data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			records, err := parseDatasetRecords(data)
			if err != nil {
				return apiErr(fmt.Errorf("parsing %s/%s response: %w", flagGroup, flagName, err))
			}

			view := regshoThresholdWatchView{
				Symbol:         flagSymbol,
				CheckedThrough: today.Format("2006-01-02"),
			}
			matchDates := regshoMatchDatesForSymbol(records, flagSymbol)
			if len(matchDates) == 0 {
				view.Note = fmt.Sprintf("no threshold-list records found for %s in the trailing %d day(s)", flagSymbol, flagDays)
			}
			view.ConsecutiveDays = computeStreak(matchDates, today)
			view.Escalated = view.ConsecutiveDays >= 5

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				status := "not on threshold list"
				if view.ConsecutiveDays > 0 {
					status = fmt.Sprintf("%d consecutive day(s)", view.ConsecutiveDays)
					if view.Escalated {
						status += " (ESCALATED: mandatory close-out threshold reached)"
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s as of %s\n", view.Symbol, status, view.CheckedThrough)
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", view.Note)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSymbol, "symbol", "", "Security symbol to check for a Reg SHO threshold-list streak (required)")
	cmd.Flags().StringVar(&flagGroup, "group", "OTCMARKET", "Dataset group for the Threshold List (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "THRESHOLDLIST", "Dataset name for the Threshold List (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().IntVar(&flagDays, "days", 10, "Trailing calendar days to scan for a threshold-list streak")
	return cmd
}

type regshoThresholdWatchView struct {
	Symbol          string `json:"symbol"`
	ConsecutiveDays int    `json:"consecutive_days"`
	Escalated       bool   `json:"escalated"`
	CheckedThrough  string `json:"checked_through"`
	Note            string `json:"note,omitempty"`
}

// regshoMatchDatesForSymbol matches records against the confirmed
// issueSymbolIdentifier field (via matchRecordsByField) rather than
// scanning every string value, then returns one date (from the first
// "date"-suffixed key, e.g. tradeDate) per matching record.
func regshoMatchDatesForSymbol(records []map[string]any, symbol string) []time.Time {
	var dates []time.Time
	for _, rec := range matchRecordsByField(records, "issueSymbolIdentifier", symbol) {
		if d, ok := findRecordDate(rec); ok {
			dates = append(dates, d)
		}
	}
	return dates
}

// parseDatasetRecords unmarshals a Query API response body into a record
// slice. A live probe confirmed FINRA returns HTTP 200/204 with an empty
// body (not a JSON "[]") when compareFilters/dateRangeFilters narrow a
// query to zero matches; treating that as a parse failure would turn a
// legitimate empty result into a hard error, so an empty body parses as no
// records instead.
func parseDatasetRecords(data []byte) ([]map[string]any, error) {
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}
	var records []map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// equalCompareFilter builds a single-element compareFilters body value
// requesting an EQUAL match on field, confirmed live against FINRA's Query
// API to filter server-side rather than scanning an arbitrary sample of
// unfiltered rows client-side.
func equalCompareFilter(field, value string) []map[string]any {
	return []map[string]any{{"fieldName": field, "fieldValue": value, "compareType": "EQUAL"}}
}

// dateRangeFilter builds a single-element dateRangeFilters body value
// bounding field to [start, end], confirmed live against FINRA's Query API.
func dateRangeFilter(field string, start, end time.Time) []map[string]any {
	return []map[string]any{{"fieldName": field, "startDate": start.Format("2006-01-02"), "endDate": end.Format("2006-01-02")}}
}

// matchRecordsByField keeps records whose field value equals needle
// case-insensitively. Used once a dataset's identifier field name (symbol,
// CUSIP, etc.) is confirmed, in place of the broader matchRecordsByValue
// scan.
func matchRecordsByField(records []map[string]any, field, needle string) []map[string]any {
	target := strings.ToLower(strings.TrimSpace(needle))
	var out []map[string]any
	for _, rec := range records {
		if s, ok := rec[field].(string); ok && strings.ToLower(s) == target {
			out = append(out, rec)
		}
	}
	return out
}

// matchRecordsByValue scans every returned record's values for a
// case-insensitive match against needle. Kept as a fallback for datasets
// whose identifier field name (symbol, CUSIP, etc.) is not confirmed —
// matching is done across all string values rather than a single guessed
// field name so those commands stay correct across naming variance.
func matchRecordsByValue(records []map[string]any, needle string) []map[string]any {
	target := strings.ToLower(strings.TrimSpace(needle))
	var out []map[string]any
	for _, rec := range records {
		for _, v := range rec {
			if s, ok := v.(string); ok && strings.ToLower(s) == target {
				out = append(out, rec)
				break
			}
		}
	}
	return out
}

// filterRecordsByDateWindow keeps only records whose date-like field (via
// findRecordDate) falls within [start, end] inclusive. Records with no
// identifiable date field are dropped, since their membership in the window
// can't be confirmed. Used in place of a server-side dateRangeFilters entry
// whenever the date field name for a dataset is unconfirmed.
func filterRecordsByDateWindow(records []map[string]any, start, end time.Time) []map[string]any {
	var out []map[string]any
	for _, rec := range records {
		d, ok := findRecordDate(rec)
		if !ok || d.Before(start) || d.After(end) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// filterRecordsByDateWindowPreferField behaves like filterRecordsByDateWindow
// but tries the confirmed field name first via recordDateAtField, falling
// back to findRecordDate's generic "date"-suffixed key scan only when the
// preferred field is absent from a record. Use this when a dataset's real
// date field is confirmed (e.g. via /metadata) AND a record may carry more
// than one date-like key, where findRecordDate's alphabetical tie-break could
// otherwise pick a different, semantically-wrong date field.
func filterRecordsByDateWindowPreferField(records []map[string]any, preferredField string, start, end time.Time) []map[string]any {
	var out []map[string]any
	for _, rec := range records {
		d, ok := recordDateAtField(rec, preferredField)
		if !ok {
			d, ok = findRecordDate(rec)
		}
		if !ok || d.Before(start) || d.After(end) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// filterDatesByWindow keeps only dates within [start, end] inclusive.
func filterDatesByWindow(dates []time.Time, start, end time.Time) []time.Time {
	var out []time.Time
	for _, d := range dates {
		if d.Before(start) || d.After(end) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// dateValueLayouts are the date/time layouts tried when parsing a FINRA
// date-like field, shared by findRecordDate's generic key scan and
// recordDateAtField's confirmed-field lookup.
var dateValueLayouts = []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05.000", "2006-01-02T15:04:05", "2006-01"}

// parseDateValue tries each of dateValueLayouts against s, returning the
// first successful parse.
func parseDateValue(s string) (time.Time, bool) {
	for _, layout := range dateValueLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// recordDateAtField parses rec[field] as a date using parseDateValue. Used
// once a dataset's date/partition field name is confirmed (e.g. tradeDate,
// beginningOfMonth), in place of findRecordDate's generic "date"-suffixed
// key scan.
func recordDateAtField(rec map[string]any, field string) (time.Time, bool) {
	s, ok := rec[field].(string)
	if !ok {
		return time.Time{}, false
	}
	return parseDateValue(s)
}

// findRecordDate looks for a key containing "date" (case-insensitive) whose
// value parses as a date, trying a handful of common layouts. Map iteration
// order in Go is randomized per run, so candidate keys are collected and
// sorted first; the sorted order is then walked deterministically, returning
// the first key whose value actually parses. This makes the result stable
// across runs even when a record has multiple date-like fields.
func findRecordDate(rec map[string]any) (time.Time, bool) {
	var candidates []string
	for k := range rec {
		if strings.Contains(strings.ToLower(k), "date") {
			candidates = append(candidates, k)
		}
	}
	sort.Strings(candidates)
	for _, k := range candidates {
		s, ok := rec[k].(string)
		if !ok {
			continue
		}
		if t, ok := parseDateValue(s); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// computeStreak walks backward from `through` counting consecutive calendar
// days (skipping weekends) present in dates, stopping at the first gap.
func computeStreak(dates []time.Time, through time.Time) int {
	if len(dates) == 0 {
		return 0
	}
	dateSet := map[string]bool{}
	for _, d := range dates {
		dateSet[d.Format("2006-01-02")] = true
	}
	streak := 0
	cursor := time.Date(through.Year(), through.Month(), through.Day(), 0, 0, 0, 0, time.UTC)
	// If today has no record, start the walk from the most recent matching day.
	if !dateSet[cursor.Format("2006-01-02")] {
		var latest time.Time
		for _, d := range dates {
			if d.After(latest) {
				latest = d
			}
		}
		if latest.IsZero() {
			return 0
		}
		cursor = latest
	}
	for {
		if dateSet[cursor.Format("2006-01-02")] {
			// A matched trading day always counts, weekend or not — the
			// fallback above can land the walk's starting cursor on a
			// Saturday/Sunday when that is the most recent matched date, and
			// that day must not be discounted just because it falls on a
			// weekend.
			streak++
			cursor = cursor.AddDate(0, 0, -1)
			continue
		}
		if cursor.Weekday() == time.Saturday || cursor.Weekday() == time.Sunday {
			// Bridge an unmatched weekend gap between trading days without
			// breaking the streak.
			cursor = cursor.AddDate(0, 0, -1)
			continue
		}
		break
	}
	return streak
}
