// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
)

const plannerHumanUnavailable = "unavailable"

// plannerWantsHumanTerminal deliberately requires a real terminal. The
// generic output router supports explicit human-friendly output in pipes, but
// Planner's default pipe contract is JSON so shell consumers never receive a
// table by accident.
func plannerWantsHumanTerminal(w io.Writer, flags *rootFlags) bool {
	if flags != nil && wantsMachineOutput(flags) {
		return false
	}
	return isTerminal(w)
}

func writePlannerCollectionHuman(w io.Writer, value plannerCollectionOutput) error {
	snapshot := value.Snapshot
	status := plannerHumanText(snapshot.Status)
	if status == "" {
		status = plannerHumanUnavailable
	}
	window := plannerHumanWindow(snapshot.RequestedStart, snapshot.RequestedEnd)
	currency := plannerHumanText(snapshot.CurrencyCode)
	if currency == "" {
		currency = plannerHumanUnavailable
	}

	fmt.Fprintf(w, "Snapshot: %s\n", plannerHumanText(snapshot.ID))
	fmt.Fprintf(w, "Endpoint: %s\n", plannerHumanText(snapshot.Endpoint))
	fmt.Fprintf(w, "Status: %s; complete=%s\n", status, plannerHumanComplete(snapshot.Complete))
	fmt.Fprintf(w, "Window: %s; currency=%s\n", window, currency)
	fmt.Fprintf(w, "Counts: receipts=%d; keywords=%d; monthly_rows=%d\n",
		value.ReceiptCount, value.StoredKeywordCount, value.StoredMonthlyCount)
	if value.RenderLimit > 0 {
		state := "within limit"
		if value.RowsTruncated {
			state = "truncated"
		}
		fmt.Fprintf(w, "Rows: showing %d of %d (limit=%d; %s)\n",
			value.RenderedRowCount, value.StoredMonthlyCount, value.RenderLimit, state)
	} else {
		fmt.Fprintf(w, "Rows: showing %d of %d\n", value.RenderedRowCount, value.StoredMonthlyCount)
	}
	if len(value.Warnings) > 0 {
		fmt.Fprintf(w, "Warnings: %s\n", plannerHumanList(value.Warnings))
	}
	fmt.Fprintln(w)
	return writePlannerRowsTable(w, value.Rows, "Monthly rows")
}

func writePlannerSnapshotsHuman(w io.Writer, snapshots []portfolio.Snapshot) error {
	fmt.Fprintln(w, "Planner snapshots")
	fmt.Fprintf(w, "Rendered %d snapshots\n", len(snapshots))
	if len(snapshots) == 0 {
		fmt.Fprintln(w, "(no snapshots)")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "SNAPSHOT\tENDPOINT\tSTATUS\tCOMPLETE\tTERMS\tWINDOW\tFETCHED_AT")
	for _, snapshot := range snapshots {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			plannerHumanText(snapshot.ID),
			plannerHumanText(snapshot.Endpoint),
			plannerHumanStatus(snapshot.Status),
			plannerHumanComplete(snapshot.Complete),
			plannerSnapshotTermCount(snapshot),
			plannerHumanWindow(snapshot.RequestedStart, snapshot.RequestedEnd),
			plannerHumanTime(snapshot.FetchedAt),
		)
	}
	return tw.Flush()
}

func writePlannerRowsHuman(w io.Writer, rows []portfolio.Row, title string) error {
	return writePlannerRowsTable(w, rows, title)
}

func writePlannerRowsHumanJSON(w io.Writer, data []byte, title string) error {
	var rows []portfolio.Row
	if err := json.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("decode Planner export rows for terminal output: %w", err)
	}
	return writePlannerRowsTable(w, rows, title)
}

func writePlannerRowsTable(w io.Writer, rows []portfolio.Row, title string) error {
	if strings.TrimSpace(title) != "" {
		fmt.Fprintln(w, plannerHumanText(title))
	}
	fmt.Fprintf(w, "Rendered %d monthly rows\n", len(rows))
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no monthly rows)")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "KEYWORD\tMONTH\tMONTHLY_SEARCHES\tFLAGS\tSTATUS\tCOMPLETE")
	for _, row := range rows {
		month := plannerHumanText(row.Month)
		if month == "" {
			month = plannerHumanUnavailable
		}
		flags := plannerHumanList(row.Flags)
		if flags == "" {
			flags = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			plannerHumanText(row.Keyword),
			month,
			plannerHumanInt64(row.MonthlySearches),
			flags,
			plannerHumanStatus(row.Status),
			plannerHumanComplete(row.Complete),
		)
	}
	return tw.Flush()
}

func plannerHumanText(value string) string {
	return truncate(cliutil.ScrubTerminal(strings.TrimSpace(value)), 80)
}

func plannerHumanStatus(value string) string {
	value = plannerHumanText(value)
	if value == "" {
		return plannerHumanUnavailable
	}
	return value
}

func plannerHumanComplete(complete bool) string {
	if complete {
		return "complete"
	}
	return "incomplete"
}

func plannerHumanWindow(start, end string) string {
	start = plannerHumanText(start)
	end = plannerHumanText(end)
	switch {
	case start == "" && end == "":
		return plannerHumanUnavailable
	case end == "":
		return start + ".."
	case start == "":
		return ".." + end
	default:
		return start + ".." + end
	}
}

func plannerHumanTime(value time.Time) string {
	if value.IsZero() {
		return plannerHumanUnavailable
	}
	return value.UTC().Format(time.RFC3339)
}

func plannerHumanInt64(value *int64) string {
	if value == nil {
		return plannerHumanUnavailable
	}
	return strconv.FormatInt(*value, 10)
}

func plannerHumanList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if text := plannerHumanText(value); text != "" {
			clean = append(clean, text)
		}
	}
	return strings.Join(clean, ", ")
}

func plannerSnapshotTermCount(snapshot portfolio.Snapshot) int {
	if snapshot.Endpoint == portfolio.EndpointIdeas {
		return len(snapshot.SubmittedSeeds)
	}
	if snapshot.Endpoint == portfolio.EndpointHistorical {
		return len(snapshot.SubmittedKeywords)
	}
	return len(snapshot.SubmittedSeeds) + len(snapshot.SubmittedKeywords)
}
