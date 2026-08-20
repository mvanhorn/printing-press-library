// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/parallel/internal/store"
	"github.com/spf13/cobra"
)

type monitorDigestEvent struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

type monitorDigestEntry struct {
	MonitorID    string               `json:"monitor_id"`
	EventCount   int                  `json:"event_count"`
	LatestEvents []monitorDigestEvent `json:"latest_events"`
}

func newNovelMonitorsDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagLimit int
	var flagQuietOnly bool

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Mechanical per-monitor event counts and top titles since a duration window.",
		Example: strings.Trim(`
  parallel-pp-cli monitors digest --since 7d --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			sinceStr := flagSince
			if sinceStr == "" {
				sinceStr = "7d"
			}
			sinceTime, err := parseSinceDuration(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", sinceStr, err))
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 5
			}

			db, err := openStoreForRead(cmd.Context(), "parallel-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				out := map[string]any{
					"since":    sinceStr,
					"monitors": []monitorDigestEntry{},
					"quiet":    []map[string]string{},
					"note":     "no local store; run parallel-pp-cli sync first",
				}
				return flags.printJSON(cmd, out)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "monitors")
			hintIfStale(cmd, db, "monitors", flags.maxAge)

			digest, quiet, note, err := buildMonitorDigest(db, sinceTime, limit, flagQuietOnly)
			if err != nil {
				return err
			}

			out := map[string]any{
				"since":    sinceStr,
				"monitors": digest,
				"quiet":    quiet,
			}
			if note != "" {
				out["note"] = note
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Duration window (e.g. 7d, 24h, 30m, 1w)")
	cmd.Flags().IntVar(&flagLimit, "limit", 5, "Top events per monitor")
	cmd.Flags().BoolVar(&flagQuietOnly, "quiet-only", false, "Only list monitors with zero events in the window")
	return cmd
}

func buildMonitorDigest(db *store.Store, since time.Time, limit int, quietOnly bool) ([]monitorDigestEntry, []map[string]string, string, error) {
	monitorIDs, err := listMonitorIDs(db)
	if err != nil {
		return nil, nil, "", err
	}
	if len(monitorIDs) == 0 {
		return []monitorDigestEntry{}, []map[string]string{}, "no monitors in local store", nil
	}

	eventsByMonitor, err := listMonitorEventsSince(db, since)
	if err != nil {
		return nil, nil, "", err
	}

	var digest []monitorDigestEntry
	var quiet []map[string]string
	for _, monitorID := range monitorIDs {
		events := eventsByMonitor[monitorID]
		if quietOnly && len(events) > 0 {
			continue
		}
		if len(events) == 0 {
			quiet = append(quiet, map[string]string{"monitor_id": monitorID})
			if quietOnly {
				continue
			}
		}
		entry := monitorDigestEntry{
			MonitorID:  monitorID,
			EventCount: len(events),
		}
		if len(events) > limit {
			events = events[:limit]
		}
		entry.LatestEvents = events
		if !quietOnly || len(events) > 0 {
			digest = append(digest, entry)
		}
	}
	sort.Slice(digest, func(i, j int) bool {
		if digest[i].EventCount == digest[j].EventCount {
			return digest[i].MonitorID < digest[j].MonitorID
		}
		return digest[i].EventCount > digest[j].EventCount
	})
	return digest, quiet, "", nil
}

func listMonitorIDs(db *store.Store) ([]string, error) {
	rows, err := db.DB().Query(`SELECT COALESCE(monitor_id, id) FROM monitors ORDER BY monitor_id, id`)
	if err != nil {
		if isNoSuchTable(err) {
			return listResourceMonitorIDs(db)
		}
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func listResourceMonitorIDs(db *store.Store) ([]string, error) {
	rows, err := db.DB().Query(`SELECT id FROM resources WHERE resource_type = 'monitors'`)
	if err != nil {
		if isNoSuchTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func listMonitorEventsSince(db *store.Store, since time.Time) (map[string][]monitorDigestEvent, error) {
	out := make(map[string][]monitorDigestEvent)
	cutoff := since.UTC().Format(time.RFC3339)

	rows, err := db.DB().Query(`SELECT monitors_id, id, data, synced_at FROM events WHERE synced_at >= ? ORDER BY synced_at DESC`, cutoff)
	if err != nil {
		if isNoSuchTable(err) {
			return out, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var monitorID, id, data, syncedAt string
		if err := rows.Scan(&monitorID, &id, &data, &syncedAt); err != nil {
			return nil, err
		}
		ev := parseMonitorEvent(id, data, syncedAt)
		out[monitorID] = append(out[monitorID], ev)
	}
	return out, rows.Err()
}

func parseMonitorEvent(id, data, syncedAt string) monitorDigestEvent {
	ev := monitorDigestEvent{ID: id, CreatedAt: syncedAt}
	var obj map[string]any
	if json.Unmarshal([]byte(data), &obj) != nil {
		return ev
	}
	ev.Title = firstString(obj, "title", "event_type", "output", "changed_output")
	ev.URL = firstString(obj, "url", "event_id")
	if t := firstString(obj, "event_date", "timestamp", "created_at"); t != "" {
		ev.CreatedAt = t
	}
	if ev.ID == "" {
		ev.ID = firstString(obj, "event_id", "id")
	}
	return ev
}

func firstString(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isNoSuchTable(err error) bool {
	for err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return true
		}
		err = unwrapErr(err)
	}
	return false
}

func unwrapErr(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}
