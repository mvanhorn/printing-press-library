// Copyright 2026 melanson633. Licensed under Apache-2.0. See LICENSE.
// Transcendence feature: backfill — reconstruct time entries you forgot to
// track from a CSV export, shell history, or a CLI session log, then
// optionally commit them to Clockify.

package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/cliutil"
	"github.com/spf13/cobra"
)

// draftEntry is a reconstructed time entry awaiting review or commit.
type draftEntry struct {
	Description string    `json:"description"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Billable    bool      `json:"billable"`
	Source      string    `json:"source"`
}

func (d draftEntry) hours() float64 { return d.End.Sub(d.Start).Hours() }

func newBackfillCmd(flags *rootFlags) *cobra.Command {
	var fromFlag, file, workspace, project, task string
	var idleGap time.Duration
	var billable, commit bool

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Draft time entries from a CSV, shell history, or CLI session log",
		Long: `Reconstruct the time you forgot to track. backfill parses an artifact
you already have and turns it into draft time entries:

  --from csv           a CSV export (columns: description, start, end,
                       or description, date, duration)
  --from shell-history a timestamped shell history file (bash HISTTIMEFORMAT
                       extended format: #<epoch> lines)
  --from session-log   a JSONL CLI/agent session log with per-line timestamps

By default backfill only previews the drafts. Pass --commit to write
them to Clockify as real time entries (a live API write).`,
		Example: `  # Preview drafts from a CSV
  clockify-pp-cli backfill --from csv --file timesheet.csv

  # Reconstruct from shell history, 15-minute idle gap
  clockify-pp-cli backfill --from shell-history --file ~/.bash_history --idle-gap 15m

  # Draft from a session log and commit to a project
  clockify-pp-cli backfill --from session-log --file ./session.jsonl --project 64a1f... --commit`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return cmd.Help()
			}
			drafts, err := parseBackfillSource(fromFlag, file, idleGap, billable)
			if err != nil {
				return usageErr(err)
			}
			sort.Slice(drafts, func(i, j int) bool { return drafts[i].Start.Before(drafts[j].Start) })

			out := cmd.OutOrStdout()
			var total time.Duration
			for _, d := range drafts {
				total += d.End.Sub(d.Start)
			}

			// When --commit is set, defer all JSON to commitDrafts so a single
			// top-level object is written to stdout (preview + result combined).
			// Without this gate, --json --commit emits two separate JSON
			// objects, which is not valid JSON and breaks downstream parsers.
			if flags.asJSON && !commit {
				return flags.printJSON(cmd, map[string]any{
					"source":      fromFlag,
					"file":        file,
					"drafts":      drafts,
					"draft_count": len(drafts),
					"total_hours": round2(total.Hours()),
					"committed":   false,
				})
			}
			if !flags.asJSON {
				fmt.Fprintf(out, "Backfill drafts from %s (%s)\n\n", fromFlag, file)
				if len(drafts) == 0 {
					fmt.Fprintln(out, "No entries could be reconstructed from this source.")
					return nil
				}
				tw := newTabWriter(out)
				fmt.Fprintln(tw, "START\tEND\tHOURS\tDESCRIPTION")
				for _, d := range drafts {
					fmt.Fprintf(tw, "%s\t%s\t%.2f\t%s\n",
						d.Start.Format("2006-01-02 15:04"), d.End.Format("15:04"),
						d.hours(), truncate(d.Description, 44))
				}
				tw.Flush()
				fmt.Fprintf(out, "\n%d draft entr(ies), %.2fh total.\n", len(drafts), total.Hours())
				if !commit {
					fmt.Fprintln(out, "Preview only — re-run with --commit to write these to Clockify.")
					return nil
				}
			}
			// --commit path below; len(drafts)==0 still emits a single JSON
			// object so a piped parser sees a valid response.
			if len(drafts) == 0 {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{
						"source":      fromFlag,
						"file":        file,
						"drafts":      drafts,
						"draft_count": 0,
						"total_hours": 0.0,
						"committed":   true,
						"created":     0,
						"failed":      0,
					})
				}
				return nil
			}
			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{
						"source":      fromFlag,
						"file":        file,
						"drafts":      drafts,
						"draft_count": len(drafts),
						"total_hours": round2(total.Hours()),
						"committed":   false,
						"verify_env":  true,
					})
				}
				fmt.Fprintf(out, "would commit %d draft entr(ies) to Clockify\n", len(drafts))
				return nil
			}
			return commitDrafts(cmd, flags, workspace, project, task, drafts, fromFlag, file, total)
		},
	}

	cmd.Flags().StringVar(&fromFlag, "from", "csv", "Source type: csv, shell-history, or session-log")
	cmd.Flags().StringVar(&file, "file", "", "Path to the source file")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace id for --commit (default: your active workspace)")
	cmd.Flags().StringVar(&project, "project", "", "Project id to assign committed entries")
	cmd.Flags().StringVar(&task, "task", "", "Task id to assign committed entries (some workspaces require a task)")
	cmd.Flags().DurationVar(&idleGap, "idle-gap", 15*time.Minute, "Gap that ends one activity window (shell-history, session-log)")
	cmd.Flags().BoolVar(&billable, "billable", false, "Mark drafted entries billable")
	cmd.Flags().BoolVar(&commit, "commit", false, "Write the drafts to Clockify (live API write)")
	return cmd
}

// commitDrafts posts each draft to the Clockify time-entries endpoint.
// In --json mode it emits a single top-level object combining the draft
// preview with the commit result, so the caller's printJSON is suppressed.
func commitDrafts(cmd *cobra.Command, flags *rootFlags, workspace, project, task string, drafts []draftEntry, fromFlag, file string, total time.Duration) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	wsID := workspace
	if wsID == "" {
		wsID, _, err = resolveWorkspaceUser(c, "")
		if err != nil {
			return err
		}
	}
	out := cmd.OutOrStdout()
	created, failed := 0, 0
	for _, d := range drafts {
		body := map[string]any{
			"start":       d.Start.UTC().Format("2006-01-02T15:04:05Z"),
			"end":         d.End.UTC().Format("2006-01-02T15:04:05Z"),
			"description": d.Description,
			"billable":    d.Billable,
		}
		if project != "" {
			body["projectId"] = project
		}
		if task != "" {
			body["taskId"] = task
		}
		if _, _, err := c.Post(fmt.Sprintf("/v1/workspaces/%s/time-entries", wsID), body); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to create entry %q: %v\n", truncate(d.Description, 40), err)
			failed++
			continue
		}
		created++
	}
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{
			"source":      fromFlag,
			"file":        file,
			"drafts":      drafts,
			"draft_count": len(drafts),
			"total_hours": round2(total.Hours()),
			"committed":   true,
			"created":     created,
			"failed":      failed,
		})
	}
	fmt.Fprintf(out, "\nCommitted: %d created, %d failed.\n", created, failed)
	if failed > 0 {
		return apiErr(fmt.Errorf("%d entr(ies) failed to commit", failed))
	}
	return nil
}

// parseBackfillSource dispatches to the right parser for the source type.
func parseBackfillSource(from, file string, idleGap time.Duration, billable bool) ([]draftEntry, error) {
	switch strings.ToLower(from) {
	case "csv":
		return parseCSVDrafts(file, billable)
	case "shell-history", "shell", "history":
		return parseShellHistoryDrafts(file, idleGap, billable)
	case "session-log", "session", "jsonl":
		return parseSessionLogDrafts(file, idleGap, billable)
	default:
		return nil, fmt.Errorf("unknown --from %q (use csv, shell-history, or session-log)", from)
	}
}

var flexTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006-01-02",
	"01/02/2006 15:04",
	"01/02/2006",
}

// unixSecsOrMillis decodes a raw integer epoch as seconds or milliseconds.
// Values above ~year 2286 in seconds (>9_999_999_999) are treated as ms —
// Node.js, Discord, Slack, and most JS-based loggers emit ms, and feeding
// a ms value to time.Unix as seconds lands the result in the year 57,000s.
func unixSecsOrMillis(n int64) time.Time {
	if n > 9_999_999_999 {
		return time.UnixMilli(n).Local()
	}
	return time.Unix(n, 0).Local()
}

// parseFlexTime parses a timestamp in any of several common layouts.
func parseFlexTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if epoch, err := strconv.ParseInt(s, 10, 64); err == nil && epoch > 100000000 {
		return unixSecsOrMillis(epoch), true
	}
	for _, layout := range flexTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseCSVDrafts builds drafts from a header-bearing CSV. Recognized columns
// (case-insensitive): description/desc/task, start, end, date, duration.
func parseCSVDrafts(file string, billable bool) ([]draftEntry, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	pick := func(rec []string, names ...string) string {
		for _, n := range names {
			if idx, ok := col[n]; ok && idx < len(rec) {
				return strings.TrimSpace(rec[idx])
			}
		}
		return ""
	}
	var drafts []draftEntry
	for _, rec := range rows[1:] {
		if len(rec) == 0 {
			continue
		}
		desc := pick(rec, "description", "desc", "task", "summary")
		if desc == "" {
			desc = "Imported entry"
		}
		startRaw := pick(rec, "start", "start_time", "begin")
		endRaw := pick(rec, "end", "end_time", "finish")
		dateRaw := pick(rec, "date", "day")
		durRaw := pick(rec, "duration", "hours", "length")

		var start, end time.Time
		var ok bool
		if start, ok = parseFlexTime(startRaw); !ok {
			if d, dok := parseFlexTime(dateRaw); dok {
				start = d.Add(9 * time.Hour) // default workday start
			}
		}
		if end, ok = parseFlexTime(endRaw); !ok {
			if dur := parseHoursDuration(durRaw); dur > 0 && !start.IsZero() {
				end = start.Add(dur)
			}
		}
		if start.IsZero() || end.IsZero() || !end.After(start) {
			continue // not enough to reconstruct a window
		}
		drafts = append(drafts, draftEntry{
			Description: desc, Start: start, End: end, Billable: billable, Source: "csv",
		})
	}
	return drafts, nil
}

// parseHoursDuration parses a duration cell that may be ISO-8601 (PT1H30M),
// a Go duration (1h30m), or a bare decimal-hours number (1.5).
func parseHoursDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if d := parseISO8601Duration(s); d > 0 {
		return d
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	if h, err := strconv.ParseFloat(s, 64); err == nil && h > 0 {
		return time.Duration(h * float64(time.Hour))
	}
	return 0
}

// timedEvent is one timestamped record from a log source.
type timedEvent struct {
	t     time.Time
	label string
}

// windowEvents groups timestamped events into activity windows split by an
// idle gap, producing one draft per window.
func windowEvents(events []timedEvent, idleGap time.Duration, source string, billable bool) []draftEntry {
	if len(events) == 0 {
		return nil
	}
	sort.Slice(events, func(i, j int) bool { return events[i].t.Before(events[j].t) })
	var drafts []draftEntry
	winStart := 0
	flush := func(start, end int) {
		s := events[start].t
		e := events[end].t
		if !e.After(s) {
			e = s.Add(5 * time.Minute) // floor for a single-event window
		}
		labels := make([]string, 0, 3)
		for i := start; i <= end && len(labels) < 3; i++ {
			if l := strings.TrimSpace(events[i].label); l != "" {
				labels = append(labels, l)
			}
		}
		desc := fmt.Sprintf("%s: %d event(s)", source, end-start+1)
		if len(labels) > 0 {
			desc = fmt.Sprintf("%s — %s", desc, strings.Join(labels, ", "))
		}
		drafts = append(drafts, draftEntry{
			Description: truncate(desc, 200), Start: s, End: e, Billable: billable, Source: source,
		})
	}
	for i := 1; i < len(events); i++ {
		if events[i].t.Sub(events[i-1].t) > idleGap {
			flush(winStart, i-1)
			winStart = i
		}
	}
	flush(winStart, len(events)-1)
	return drafts
}

// parseShellHistoryDrafts reconstructs windows from a bash extended-history
// file (lines `#<epoch>` followed by the command).
func parseShellHistoryDrafts(file string, idleGap time.Duration, billable bool) ([]draftEntry, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("opening shell history: %w", err)
	}
	defer f.Close()
	var events []timedEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 256*1024)
	var pendingTS time.Time
	havePending := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r\n")
		if strings.HasPrefix(line, "#") {
			if epoch, err := strconv.ParseInt(strings.TrimSpace(line[1:]), 10, 64); err == nil && epoch > 100000000 {
				pendingTS = unixSecsOrMillis(epoch)
				havePending = true
			}
			continue
		}
		if !havePending || strings.TrimSpace(line) == "" {
			continue
		}
		events = append(events, timedEvent{t: pendingTS, label: firstWord(line)})
		havePending = false
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading shell history: %w", err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("no timestamped commands found — shell history needs the bash extended format (HISTTIMEFORMAT set)")
	}
	return windowEvents(events, idleGap, "shell", billable), nil
}

// parseSessionLogDrafts reconstructs windows from a JSONL session log; each
// line is a JSON object carrying a timestamp field.
func parseSessionLogDrafts(file string, idleGap time.Duration, billable bool) ([]draftEntry, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("opening session log: %w", err)
	}
	defer f.Close()
	var events []timedEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] != '{' {
			continue
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &obj) != nil {
			continue
		}
		ts, ok := timeFromJSON(obj, "timestamp", "time", "ts", "created_at", "createdAt", "date")
		if !ok {
			continue
		}
		label := stringFromJSON(obj, "type", "role", "event", "name")
		events = append(events, timedEvent{t: ts, label: label})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading session log: %w", err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("no timestamped JSON records found in session log")
	}
	return windowEvents(events, idleGap, "session", billable), nil
}

func timeFromJSON(obj map[string]json.RawMessage, keys ...string) (time.Time, bool) {
	for _, k := range keys {
		raw, ok := obj[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if t, ok := parseFlexTime(s); ok {
				return t, true
			}
		}
		var n int64
		if json.Unmarshal(raw, &n) == nil && n > 100000000 {
			return unixSecsOrMillis(n), true
		}
	}
	return time.Time{}, false
}

func stringFromJSON(obj map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if raw, ok := obj[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// firstWord returns the first whitespace-delimited token, basename-trimmed.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		s = s[:i]
	}
	return filepath.Base(s)
}
