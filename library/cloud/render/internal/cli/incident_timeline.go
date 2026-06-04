// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// timelineEvent is one row in the merged chronological view. Kind is one of
// deploy | event | audit and tells the caller which table the row came
// from; a deterministic per-row primary key would force a synthetic id, so
// we rely on (kind, timestamp, summary) for human ordering.
type timelineEvent struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"`
	Actor     string `json:"actor,omitempty"`
	Summary   string `json:"summary"`
}

// parseTimelineWindow parses an `--since` / `--until` value. Accepts
// duration suffix forms (2h, 30m, 1d, 7d) and RFC3339 absolute times. The
// special value "now" returns the current time.
func parseTimelineWindow(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "now") {
		return now, nil
	}
	// Day suffix isn't part of time.ParseDuration, so unfold it manually
	// before delegating.
	if strings.HasSuffix(s, "d") {
		var n int
		_, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%d", &n)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid day-relative duration %q: %w", s, err)
		}
		return now.AddDate(0, 0, -n), nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse %q as duration (e.g. 2h, 30m, 1d) or RFC3339 time", s)
}

func newIncidentTimelineCmd(flags *rootFlags) *cobra.Command {
	var (
		since  string
		until  string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "incident-timeline <serviceId>",
		Short: "Merge deploys, service events, and audit-logs for one service into one chronological view.",
		Example: strings.Trim(`
  render-pp-cli incident-timeline srv-d12abc --since 2h
  render-pp-cli incident-timeline srv-d12abc --since 1d --until now
  render-pp-cli incident-timeline srv-d12abc --since 2026-05-09T09:00:00Z --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "incident-timeline"}`)
				return nil
			}
			now := time.Now().UTC()
			start, err := parseTimelineWindow(orDefault(since, "2h"), now)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			end, err := parseTimelineWindow(orDefault(until, "now"), now)
			if err != nil {
				return fmt.Errorf("--until: %w", err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			events, err := buildIncidentTimeline(db, args[0], start, end)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			if len(events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No deploys, events, or audit entries in the requested window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-8s %-25s %s\n", "TIMESTAMP", "KIND", "ACTOR", "SUMMARY")
			for _, e := range events {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-8s %-25s %s\n", e.Timestamp, e.Kind, e.Actor, e.Summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "2h", "Window start: duration (2h, 30m, 1d) or RFC3339 timestamp")
	cmd.Flags().StringVar(&until, "until", "now", "Window end: duration (1h), 'now', or RFC3339 timestamp")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// buildIncidentTimeline UNIONs the three tables and filters by [start,end].
// Events that lack a parseable timestamp are silently dropped to avoid
// polluting the chronology.
func buildIncidentTimeline(db *store.Store, serviceID string, start, end time.Time) ([]timelineEvent, error) {
	out := []timelineEvent{}
	if start.After(end) {
		return out, nil
	}

	addRowsFromQuery := func(query, kind string, args ...any) error {
		rows, err := db.DB().Query(query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			ts := pickTimestamp(obj)
			if ts == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				continue
			}
			if t.Before(start) || t.After(end) {
				continue
			}
			out = append(out, timelineEvent{
				Timestamp: ts,
				Kind:      kind,
				Actor:     pickActor(obj),
				Summary:   pickSummary(obj, kind),
			})
		}
		return rows.Err()
	}

	if err := addRowsFromQuery(
		`SELECT data FROM deploys WHERE services_id = ?`,
		"deploy", serviceID,
	); err != nil {
		return out, err
	}
	if err := addRowsFromQuery(
		`SELECT data FROM services_events WHERE services_id = ?`,
		"event", serviceID,
	); err != nil {
		return out, err
	}
	// Audit logs reference resources, not services; we filter on the
	// JSON path for resourceId / targetId / details.serviceId so any of
	// the common shapes hit.
	if err := addRowsFromQuery(
		`SELECT data FROM owners_audit_logs
		 WHERE json_extract(data, '$.resourceId') = ?
		    OR json_extract(data, '$.targetId') = ?
		    OR json_extract(data, '$.details.serviceId') = ?`,
		"audit", serviceID, serviceID, serviceID,
	); err != nil {
		// audit table may be empty for some workspaces; non-fatal
		_ = err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out, nil
}

func pickTimestamp(obj map[string]any) string {
	for _, k := range []string{"createdAt", "created_at", "timestamp", "occurredAt", "finishedAt", "updatedAt"} {
		if v := strFromAny(obj[k]); v != "" {
			return v
		}
	}
	return ""
}

func pickActor(obj map[string]any) string {
	for _, k := range []string{"actor", "userId", "user_id", "triggeredBy", "trigger"} {
		if v := strFromAny(obj[k]); v != "" {
			return v
		}
	}
	if details, ok := obj["details"].(map[string]any); ok {
		for _, k := range []string{"actor", "userId", "user_id"} {
			if v := strFromAny(details[k]); v != "" {
				return v
			}
		}
	}
	return ""
}

func pickSummary(obj map[string]any, kind string) string {
	switch kind {
	case "deploy":
		status := strFromAny(obj["status"])
		commit := strFromAny(obj["commit"])
		if commit == "" {
			if c, ok := obj["commit"].(map[string]any); ok {
				commit = strFromAny(c["id"])
			}
		}
		if commit != "" && len(commit) > 8 {
			commit = commit[:8]
		}
		out := "deploy"
		if status != "" {
			out += " status=" + status
		}
		if commit != "" {
			out += " commit=" + commit
		}
		return out
	case "event":
		typ := strFromAny(obj["type"])
		if typ == "" {
			typ = "event"
		}
		details := strFromAny(obj["details"])
		if details == "" {
			details = strFromAny(obj["message"])
		}
		out := typ
		if details != "" && len(details) < 80 {
			out += " " + details
		}
		return out
	case "audit":
		action := strFromAny(obj["action"])
		target := strFromAny(obj["target"])
		if target == "" {
			target = strFromAny(obj["resourceId"])
		}
		out := action
		if target != "" {
			out += " " + target
		}
		if out == "" {
			out = "audit"
		}
		return out
	}
	return kind
}
