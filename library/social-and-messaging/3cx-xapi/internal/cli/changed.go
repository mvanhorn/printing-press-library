// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command: live-state merge. A single time-windowed feed merging the
// activity log, event log, active calls, and system status from the local
// mirror into one "what's happening on this PBX since T" view.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/internal/cliutil"
	"github.com/spf13/cobra"
)

type changedFeed struct {
	Since            string            `json:"since"`
	CutoffUTC        string            `json:"cutoff_utc"`
	ActivityLog      []json.RawMessage `json:"activity_log"`
	EventLogs        []json.RawMessage `json:"event_logs"`
	ActiveCalls      []json.RawMessage `json:"active_calls"`
	SystemStatus     json.RawMessage   `json:"system_status,omitempty"`
	ActivityLogCount int               `json:"activity_log_count"`
	EventLogCount    int               `json:"event_log_count"`
	ActiveCallCount  int               `json:"active_call_count"`
	Note             string            `json:"note,omitempty"`
}

// filterSince keeps objects whose extracted timestamp is at or after cutoff.
// Objects with no parseable timestamp are dropped from time-series views.
func filterSince(objs []map[string]json.RawMessage, raws []json.RawMessage, cutoff time.Time) []json.RawMessage {
	out := []json.RawMessage{}
	for i, o := range objs {
		if t, ok := extractTimestamp(o); ok && !t.Before(cutoff) {
			out = append(out, raws[i])
		}
	}
	return out
}

func newNovelChangedCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "changed",
		Short: "Time-windowed merge of activity log, events, active calls, and system status",
		Long: "Merge the activity log, event log, active calls, and current system status from the\n" +
			"local mirror into a single feed scoped to a recent time window. Reads local data only.\n\n" +
			"Use this command for the live activity/event/status merge over a recent window. For\n" +
			"structural config drift between captures use 'diff'; for a security focus use 'posture'.",
		Example:     "  3cx-xapi-pp-cli changed --since 2h --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would merge activity/event/active-call/system-status over the time window")
				return nil
			}
			if flagSince == "" {
				flagSince = "24h"
			}
			dur, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			cutoff := time.Now().Add(-dur).UTC()

			db, ok, err := openLocalMirror(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, rtActivityLog) {
				hintIfStale(cmd, db, rtActivityLog, flags.maxAge)
			}

			feed := changedFeed{Since: flagSince, CutoffUTC: cutoff.Format(time.RFC3339)}

			// Time-series: filter by --since.
			actRaw, err := db.List(rtActivityLog, 0)
			if err != nil {
				return err
			}
			actObjs, err := listObjects(db, rtActivityLog)
			if err != nil {
				return err
			}
			feed.ActivityLog = filterSince(actObjs, actRaw, cutoff)

			evtRaw, err := db.List(rtEventLogs, 0)
			if err != nil {
				return err
			}
			evtObjs, err := listObjects(db, rtEventLogs)
			if err != nil {
				return err
			}
			feed.EventLogs = filterSince(evtObjs, evtRaw, cutoff)

			// Current state: include as-is (no useful per-row timestamp).
			callRaw, err := db.List(rtActiveCalls, 0)
			if err != nil {
				return err
			}
			feed.ActiveCalls = callRaw
			if feed.ActiveCalls == nil {
				feed.ActiveCalls = []json.RawMessage{}
			}

			sysRaw, err := db.List(rtSystemStatus, 0)
			if err != nil {
				return err
			}
			if len(sysRaw) > 0 {
				feed.SystemStatus = sysRaw[0]
			}

			feed.ActivityLogCount = len(feed.ActivityLog)
			feed.EventLogCount = len(feed.EventLogs)
			feed.ActiveCallCount = len(feed.ActiveCalls)
			if feed.ActivityLogCount == 0 && feed.EventLogCount == 0 && feed.ActiveCallCount == 0 && len(sysRaw) == 0 {
				feed.Note = "nothing in the window; sync activity-log,event-logs,active-calls,system-status first"
			}

			if machineOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), feed, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Changes since %s (cutoff %s)\n", flagSince, feed.CutoffUTC)
			fmt.Fprintf(w, "  activity-log: %d   events: %d   active calls: %d\n", feed.ActivityLogCount, feed.EventLogCount, feed.ActiveCallCount)
			if feed.Note != "" {
				fmt.Fprintln(w, feed.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Time window to merge (e.g. 2h, 30m, 7d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
