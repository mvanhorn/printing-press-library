package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/zoomurl"
)

// newZoomTodayCmd: T4 today + conflicts.
func newZoomTodayCmd(flags *rootFlags) *cobra.Command {
	var (
		withRecordings bool
		since          string
	)
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Everything on your plate today: cloud meetings + saved bookmarks + recordings, with conflict detection",
		Long: "UNIONs cloud_meetings (cached by `sync` or `meetings list`), saved_meetings (your bookmarks " +
			"with a scheduled_for in today's window), and optionally today's local_recordings. Computes overlapping " +
			"intervals → `conflict_with` per row. With --since 7d, widens the window.",
		Example: `  zoom-pp-cli today --json
  zoom-pp-cli today --with-recordings --json
  zoom-pp-cli today --since 7d --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			windowStart, windowEnd := todayWindow(since)
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			if err := localstore.EnsureSchema(cmd.Context(), db); err != nil {
				return err
			}

			type row struct {
				Source       string    `json:"source"`
				ID           string    `json:"id,omitempty"`
				Topic        string    `json:"topic"`
				StartTime    time.Time `json:"start_time"`
				DurationMin  int       `json:"duration_min,omitempty"`
				JoinURL      string    `json:"join_url,omitempty"`
				MeetingID    string    `json:"meeting_id,omitempty"`
				Path         string    `json:"path,omitempty"`
				ConflictWith []string  `json:"conflict_with,omitempty"`
			}
			var rows []row

			// Cloud meetings: pull from the generic resources table that the
			// generated sync uses (resource_type = 'meetings').
			cm, err := db.QueryContext(cmd.Context(),
				`SELECT id, data FROM resources WHERE resource_type IN ('meetings', 'users_meetings')`)
			if err == nil {
				defer cm.Close()
				for cm.Next() {
					var id string
					var data []byte
					if err := cm.Scan(&id, &data); err != nil {
						continue
					}
					topic, start, dur, joinURL := extractMeetingFields(data)
					if start.IsZero() || start.Before(windowStart) || start.After(windowEnd) {
						continue
					}
					rows = append(rows, row{
						Source: "cloud_meeting", ID: id, Topic: topic, StartTime: start,
						DurationMin: dur, JoinURL: joinURL, MeetingID: id,
					})
				}
			}

			// Saved bookmarks scheduled for today.
			bms, _ := localstore.ListBookmarks(cmd.Context(), db)
			for _, bm := range bms {
				if bm.ScheduledFor == nil {
					continue
				}
				if bm.ScheduledFor.Before(windowStart) || bm.ScheduledFor.After(windowEnd) {
					continue
				}
				url, _ := zoomurl.Build(zoomurl.Params{Action: zoomurl.ActionJoin, ConfNo: bm.MeetingID, Pwd: bm.Pwd})
				rows = append(rows, row{
					Source: "saved_bookmark", ID: bm.Name, Topic: bm.Name, StartTime: *bm.ScheduledFor,
					JoinURL: url, MeetingID: bm.MeetingID,
				})
			}

			// Local recordings made today.
			if withRecordings {
				rec, _ := localstore.ListLocalRecordings(cmd.Context(), db, localstore.ListLocalOpts{Since: windowStart})
				for _, r := range rec {
					if r.Start.Before(windowStart) || r.Start.After(windowEnd) {
						continue
					}
					rows = append(rows, row{
						Source: "local_recording", ID: r.ID, Topic: r.Topic, StartTime: r.Start,
						MeetingID: r.MeetingID, Path: r.Path,
					})
				}
			}

			// Sort by start time and compute conflict overlaps.
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].StartTime.Before(rows[j].StartTime) })
			for i := range rows {
				for j := range rows {
					if i == j {
						continue
					}
					a := rows[i]
					b := rows[j]
					if a.StartTime.IsZero() || b.StartTime.IsZero() {
						continue
					}
					ae := a.StartTime.Add(time.Duration(a.DurationMin) * time.Minute)
					if a.DurationMin == 0 && a.Source == "local_recording" {
						// best-effort: assume 60 min for recordings without duration
						ae = a.StartTime.Add(60 * time.Minute)
					}
					if a.DurationMin == 0 && a.Source == "saved_bookmark" {
						ae = a.StartTime.Add(30 * time.Minute)
					}
					if a.DurationMin == 0 && a.Source == "cloud_meeting" {
						ae = a.StartTime.Add(60 * time.Minute)
					}
					if b.StartTime.Before(ae) && b.StartTime.After(a.StartTime) {
						label := b.Topic
						if label == "" {
							label = b.ID
						}
						rows[i].ConflictWith = append(rows[i].ConflictWith, label)
					}
				}
			}

			// Normalize nil to an empty slice so the result stays a self-describing
			// list envelope ({count, items:[]}) even when nothing matches. This
			// also lets --select descend into items[] via the envelope fallback
			// rather than collapsing to {} when only row-level fields are selected.
			if rows == nil {
				rows = []row{}
			}
			return flags.printJSON(cmd, map[string]any{
				"window_start": windowStart,
				"window_end":   windowEnd,
				"count":        len(rows),
				"items":        rows,
			})
		},
	}
	cmd.Flags().BoolVar(&withRecordings, "with-recordings", false, "Include today's local recordings in the output")
	cmd.Flags().StringVar(&since, "since", "", "Widen window backward (e.g. 7d covers the last week + today)")
	return cmd
}

func todayWindow(since string) (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.Add(24 * time.Hour)
	if since != "" {
		if t, err := parseSince(since); err == nil && !t.IsZero() {
			start = t
		}
	}
	return start, end
}

func extractMeetingFields(data []byte) (topic string, start time.Time, durationMin int, joinURL string) {
	// Lightweight JSON field extraction — avoids pulling a strict schema for
	// the generic resources table.
	s := string(data)
	topic = jsonStringField(s, "topic")
	if t := jsonStringField(s, "start_time"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			start = parsed
		}
	}
	if d := jsonNumberField(s, "duration"); d > 0 {
		durationMin = int(d)
	}
	if u := jsonStringField(s, "join_url"); u != "" {
		joinURL = u
	}
	return topic, start, durationMin, joinURL
}
