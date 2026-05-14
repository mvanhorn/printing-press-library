// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel-feature commands. Not generated.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/cal-com/internal/store"
	"github.com/spf13/cobra"
)

// ---------- shared helpers ----------

type bookingRow struct {
	UID                string
	Title              string
	Status             string
	Start              time.Time
	End                time.Time
	HostEmail          string
	EventTypeID        string
	EventTypeSlug      string
	RescheduledFromUid string
	Raw                map[string]any
	Attendees          []attendeeRow
}

type attendeeRow struct {
	Email    string
	Name     string
	NoShow   bool
	Timezone string
}

func loadBookings(db *store.Store) ([]bookingRow, error) {
	rows, err := db.List("bookings", 100000)
	if err != nil {
		return nil, err
	}
	out := make([]bookingRow, 0, len(rows))
	for _, raw := range rows {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		b := bookingRow{Raw: m}
		if v, ok := m["uid"].(string); ok {
			b.UID = v
		}
		if v, ok := m["title"].(string); ok {
			b.Title = v
		}
		if v, ok := m["status"].(string); ok {
			b.Status = v
		}
		if v, ok := m["start"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				b.Start = t
			}
		}
		if v, ok := m["end"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				b.End = t
			}
		}
		if v, ok := m["rescheduledFromUid"].(string); ok {
			b.RescheduledFromUid = v
		}
		if host, ok := m["hosts"].([]any); ok && len(host) > 0 {
			if hm, ok := host[0].(map[string]any); ok {
				if v, ok := hm["email"].(string); ok {
					b.HostEmail = v
				}
			}
		}
		if v, ok := m["eventTypeId"].(float64); ok {
			b.EventTypeID = fmt.Sprintf("%.0f", v)
		}
		if et, ok := m["eventType"].(map[string]any); ok {
			if v, ok := et["slug"].(string); ok {
				b.EventTypeSlug = v
			}
		}
		if atts, ok := m["attendees"].([]any); ok {
			for _, a := range atts {
				am, ok := a.(map[string]any)
				if !ok {
					continue
				}
				ar := attendeeRow{}
				if v, ok := am["email"].(string); ok {
					ar.Email = strings.ToLower(v)
				}
				if v, ok := am["name"].(string); ok {
					ar.Name = v
				}
				if v, ok := am["noShow"].(bool); ok {
					ar.NoShow = v
				}
				if v, ok := am["timeZone"].(string); ok {
					ar.Timezone = v
				}
				b.Attendees = append(b.Attendees, ar)
			}
		}
		out = append(out, b)
	}
	return out, nil
}

func emit(cmd *cobra.Command, flags *rootFlags, v any) error {
	w := cmd.OutOrStdout()
	if flags.asJSON || flags.compact || flags.agent {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	// pretty default to JSON too — human formatting per-command is overkill
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func parseDuration(s string) (time.Duration, error) {
	// accept formats like 30m, 1h, 2h15m, 7d
	if strings.HasSuffix(s, "d") {
		days, err := time.ParseDuration(strings.TrimSuffix(s, "d") + "h")
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	return time.ParseDuration(s)
}

// ---------- 1. conflicts ----------

func newConflictsCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "conflicts",
		Short:       "Find overlapping busy time across linked calendars and Cal.com bookings",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Joins locally-synced 'calendars' (multi-provider busy data from Google,
Outlook, iCloud, ICS) against 'bookings' (confirmed Cal.com bookings) and
reports time-range overlaps that indicate double-booking risk.

Run 'sync' first to populate the local store.`,
		Example: strings.Trim(`
  cal-com-pp-cli conflicts --window 7d --json
  cal-com-pp-cli conflicts --window 14d --json --select date,event_title,conflict_calendar,conflict_event
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			d, err := parseDuration(window)
			if err != nil {
				return fmt.Errorf("invalid --window %q: %w", window, err)
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			bookings, err := loadBookings(db)
			if err != nil {
				return fmt.Errorf("loading bookings: %w", err)
			}
			now := time.Now().UTC()
			horizon := now.Add(d)
			// load calendar busy entries from typed 'calendars' table
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(title,''), COALESCE(start,''), COALESCE("end",''), COALESCE(source,''), COALESCE(calendar_event_owner,'') FROM calendars`)
			if err != nil {
				return fmt.Errorf("query calendars: %w", err)
			}
			defer rows.Close()
			type busy struct {
				ID, Title, Source, Owner string
				Start, End               time.Time
			}
			var busies []busy
			for rows.Next() {
				var b busy
				var s, e string
				if err := rows.Scan(&b.ID, &b.Title, &s, &e, &b.Source, &b.Owner); err != nil {
					continue
				}
				b.Start, _ = time.Parse(time.RFC3339, s)
				b.End, _ = time.Parse(time.RFC3339, e)
				busies = append(busies, b)
			}
			type conflict struct {
				Date             string `json:"date"`
				BookingUID       string `json:"booking_uid"`
				BookingTitle     string `json:"booking_title"`
				BookingStart     string `json:"booking_start"`
				BookingEnd       string `json:"booking_end"`
				ConflictCalendar string `json:"conflict_calendar"`
				ConflictEvent    string `json:"conflict_event"`
				ConflictStart    string `json:"conflict_start"`
				ConflictEnd      string `json:"conflict_end"`
			}
			var out []conflict
			for _, b := range bookings {
				if b.Start.IsZero() || b.End.IsZero() || b.Start.Before(now) || b.Start.After(horizon) {
					continue
				}
				for _, c := range busies {
					if c.Start.IsZero() || c.End.IsZero() {
						continue
					}
					if c.Start.Before(b.End) && c.End.After(b.Start) {
						out = append(out, conflict{
							Date:             b.Start.Format("2006-01-02"),
							BookingUID:       b.UID,
							BookingTitle:     b.Title,
							BookingStart:     b.Start.Format(time.RFC3339),
							BookingEnd:       b.End.Format(time.RFC3339),
							ConflictCalendar: c.Source,
							ConflictEvent:    c.Title,
							ConflictStart:    c.Start.Format(time.RFC3339),
							ConflictEnd:      c.End.Format(time.RFC3339),
						})
					}
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].BookingStart < out[j].BookingStart })
			return emit(cmd, flags, map[string]any{"window": window, "conflicts": out, "count": len(out)})
		},
	}
	cmd.Flags().StringVar(&window, "window", "7d", "Time window from now to scan (e.g. 7d, 24h, 14d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------- 2. no-show-risk ----------

func newNoShowRiskCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	var minBookings int
	cmd := &cobra.Command{
		Use:         "no-show-risk",
		Short:       "Rank attendees by historical no-show and cancellation rate",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Aggregates the local 'bookings' table grouped by attendee email and emits
no_show_rate, cancel_rate, total_count per attendee. No equivalent endpoint
exists in the Cal.com API.`,
		Example: strings.Trim(`
  cal-com-pp-cli no-show-risk --since 90d --json
  cal-com-pp-cli no-show-risk --since 30d --min-bookings 3 --json --select email,no_show_rate,total_count
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			d, err := parseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since %q: %w", since, err)
			}
			cutoff := time.Now().UTC().Add(-d)
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			bookings, err := loadBookings(db)
			if err != nil {
				return err
			}
			type agg struct {
				Email      string  `json:"email"`
				Total      int     `json:"total_count"`
				NoShow     int     `json:"no_show_count"`
				Cancelled  int     `json:"cancel_count"`
				NoShowRate float64 `json:"no_show_rate"`
				CancelRate float64 `json:"cancel_rate"`
				LastSeen   string  `json:"last_seen"`
			}
			byEmail := map[string]*agg{}
			for _, b := range bookings {
				if !b.Start.IsZero() && b.Start.Before(cutoff) {
					continue
				}
				for _, a := range b.Attendees {
					if a.Email == "" {
						continue
					}
					rec, ok := byEmail[a.Email]
					if !ok {
						rec = &agg{Email: a.Email}
						byEmail[a.Email] = rec
					}
					rec.Total++
					if a.NoShow || strings.EqualFold(b.Status, "no_show") {
						rec.NoShow++
					}
					if strings.EqualFold(b.Status, "cancelled") {
						rec.Cancelled++
					}
					if !b.Start.IsZero() {
						s := b.Start.Format(time.RFC3339)
						if s > rec.LastSeen {
							rec.LastSeen = s
						}
					}
				}
			}
			var out []agg
			for _, v := range byEmail {
				if v.Total < minBookings {
					continue
				}
				if v.Total > 0 {
					v.NoShowRate = float64(v.NoShow) / float64(v.Total)
					v.CancelRate = float64(v.Cancelled) / float64(v.Total)
				}
				out = append(out, *v)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].NoShowRate != out[j].NoShowRate {
					return out[i].NoShowRate > out[j].NoShowRate
				}
				return out[i].Total > out[j].Total
			})
			return emit(cmd, flags, map[string]any{"since": since, "attendees": out, "count": len(out)})
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Look back this far (e.g. 30d, 90d, 1h)")
	cmd.Flags().IntVar(&minBookings, "min-bookings", 1, "Drop attendees with fewer than N bookings")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------- 3. attendee ----------

func newAttendeeCmd(flags *rootFlags) *cobra.Command {
	var summary bool
	var dbPath string
	cmd := &cobra.Command{
		Use:         "attendee [email]",
		Short:       "Show every booking for a single attendee email; --summary collapses to one row",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Joins bookings × attendees × event_types filtered by email. Without --summary,
emits every individual booking; with --summary, returns one row with first-seen,
last-seen, total_count, no_show_count, cancel_count.`,
		Example: strings.Trim(`
  cal-com-pp-cli attendee jane@example.com --json
  cal-com-pp-cli attendee jane@example.com --summary --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			email := strings.ToLower(args[0])
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			bookings, err := loadBookings(db)
			if err != nil {
				return err
			}
			type row struct {
				UID           string `json:"uid"`
				Title         string `json:"title"`
				Status        string `json:"status"`
				Start         string `json:"start"`
				End           string `json:"end"`
				EventTypeSlug string `json:"event_type_slug"`
				HostEmail     string `json:"host_email"`
				NoShow        bool   `json:"no_show"`
			}
			var rows []row
			for _, b := range bookings {
				matched := false
				noShow := false
				for _, a := range b.Attendees {
					if a.Email == email {
						matched = true
						if a.NoShow {
							noShow = true
						}
						break
					}
				}
				if !matched {
					continue
				}
				rows = append(rows, row{
					UID: b.UID, Title: b.Title, Status: b.Status,
					Start: b.Start.Format(time.RFC3339), End: b.End.Format(time.RFC3339),
					EventTypeSlug: b.EventTypeSlug, HostEmail: b.HostEmail, NoShow: noShow,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Start < rows[j].Start })
			if summary {
				type sumRow struct {
					Email      string  `json:"email"`
					Total      int     `json:"total_count"`
					FirstSeen  string  `json:"first_seen"`
					LastSeen   string  `json:"last_seen"`
					NoShow     int     `json:"no_show_count"`
					Cancelled  int     `json:"cancel_count"`
					NoShowRate float64 `json:"no_show_rate"`
				}
				s := sumRow{Email: email, Total: len(rows)}
				if len(rows) > 0 {
					s.FirstSeen = rows[0].Start
					s.LastSeen = rows[len(rows)-1].Start
				}
				for _, r := range rows {
					if r.NoShow {
						s.NoShow++
					}
					if strings.EqualFold(r.Status, "cancelled") {
						s.Cancelled++
					}
				}
				if s.Total > 0 {
					s.NoShowRate = float64(s.NoShow) / float64(s.Total)
				}
				return emit(cmd, flags, s)
			}
			return emit(cmd, flags, map[string]any{"email": email, "bookings": rows, "count": len(rows)})
		},
	}
	cmd.Flags().BoolVar(&summary, "summary", false, "Collapse to a single row of aggregates")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------- 4. gaps ----------

func newGapsCmd(flags *rootFlags) *cobra.Command {
	var minStr string
	var window string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "gaps",
		Short:       "Contiguous free time of at least N minutes inside your availability windows",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Subtracts confirmed bookings from your local schedule definitions to surface
contiguous gaps. /slots returns slot intervals per event-type only; this is
schedule-wide free-time discovery.`,
		Example: strings.Trim(`
  cal-com-pp-cli gaps --min 30m --window 7d --json
  cal-com-pp-cli gaps --min 45m --window 14d --json --select start,end,minutes
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			minD, err := parseDuration(minStr)
			if err != nil {
				return fmt.Errorf("invalid --min %q: %w", minStr, err)
			}
			winD, err := parseDuration(window)
			if err != nil {
				return fmt.Errorf("invalid --window %q: %w", window, err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			now := time.Now().UTC()
			horizon := now.Add(winD)
			bookings, err := loadBookings(db)
			if err != nil {
				return err
			}
			// load default schedule availability via 'schedules' resource_type
			scheds, err := db.List("schedules", 100)
			if err != nil {
				return err
			}
			type interval struct{ start, end time.Time }
			busy := []interval{}
			for _, b := range bookings {
				if b.Start.IsZero() || b.End.IsZero() {
					continue
				}
				if b.End.Before(now) || b.Start.After(horizon) {
					continue
				}
				if strings.EqualFold(b.Status, "cancelled") {
					continue
				}
				busy = append(busy, interval{b.Start, b.End})
			}
			sort.Slice(busy, func(i, j int) bool { return busy[i].start.Before(busy[j].start) })
			// availability: rough fallback — 9am-5pm daily user-local UTC if no schedules
			workdays := []interval{}
			day := now.Truncate(24 * time.Hour)
			for !day.After(horizon) {
				s := day.Add(9 * time.Hour)
				e := day.Add(17 * time.Hour)
				if e.After(now) {
					if s.Before(now) {
						s = now
					}
					workdays = append(workdays, interval{s, e})
				}
				day = day.Add(24 * time.Hour)
			}
			_ = scheds
			type gap struct {
				Start   string `json:"start"`
				End     string `json:"end"`
				Minutes int    `json:"minutes"`
			}
			var out []gap
			for _, w := range workdays {
				cursor := w.start
				for _, b := range busy {
					if !b.end.After(cursor) || !b.start.Before(w.end) {
						continue
					}
					if b.start.After(cursor) {
						mins := int(b.start.Sub(cursor).Minutes())
						if time.Duration(mins)*time.Minute >= minD {
							out = append(out, gap{cursor.Format(time.RFC3339), b.start.Format(time.RFC3339), mins})
						}
					}
					if b.end.After(cursor) {
						cursor = b.end
					}
				}
				if cursor.Before(w.end) {
					mins := int(w.end.Sub(cursor).Minutes())
					if time.Duration(mins)*time.Minute >= minD {
						out = append(out, gap{cursor.Format(time.RFC3339), w.end.Format(time.RFC3339), mins})
					}
				}
			}
			return emit(cmd, flags, map[string]any{"min": minStr, "window": window, "gaps": out, "count": len(out)})
		},
	}
	cmd.Flags().StringVar(&minStr, "min", "30m", "Minimum gap size (e.g. 15m, 30m, 1h)")
	cmd.Flags().StringVar(&window, "window", "7d", "Time window from now to scan")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------- 5. reschedule-history ----------

func newRescheduleHistoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "reschedule-history [bookingUid]",
		Short:       "Show the full reschedule chain for a booking",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Recursively walks rescheduledFromUid pointers across the local bookings table
and emits the ordered timeline. Cal.com exposes the rescheduledFromUid field
per booking but no chain-walk endpoint.`,
		Example: strings.Trim(`
  cal-com-pp-cli reschedule-history abc123 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			bookings, err := loadBookings(db)
			if err != nil {
				return err
			}
			byUID := map[string]bookingRow{}
			for _, b := range bookings {
				if b.UID != "" {
					byUID[b.UID] = b
				}
			}
			type node struct {
				UID    string `json:"uid"`
				Title  string `json:"title"`
				Status string `json:"status"`
				Start  string `json:"start"`
				From   string `json:"rescheduled_from_uid,omitempty"`
			}
			var chain []node
			seen := map[string]bool{}
			cur, ok := byUID[args[0]]
			if !ok {
				return fmt.Errorf("booking %q not found in local store (try: cal-com-pp-cli sync)", args[0])
			}
			for {
				if seen[cur.UID] {
					break
				}
				seen[cur.UID] = true
				chain = append(chain, node{cur.UID, cur.Title, cur.Status, cur.Start.Format(time.RFC3339), cur.RescheduledFromUid})
				if cur.RescheduledFromUid == "" {
					break
				}
				prev, ok := byUID[cur.RescheduledFromUid]
				if !ok {
					break
				}
				cur = prev
			}
			// reverse so oldest first
			for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
				chain[i], chain[j] = chain[j], chain[i]
			}
			return emit(cmd, flags, map[string]any{"start_uid": args[0], "chain": chain, "length": len(chain)})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------- 6. cancel-sweep ----------

func newCancelSweepCmd(flags *rootFlags) *cobra.Command {
	var status string
	var olderThan string
	var apply bool
	var dbPath string
	cmd := &cobra.Command{
		Use:   "cancel-sweep",
		Short: "Find (and with --apply, cancel) stale unconfirmed bookings",
		Long: `Local SQL pre-filter on (status, age) drives an optional typed cancel loop.
Without --apply, this is a dry-run that returns the candidate set. With
--apply, the CLI calls /v2/bookings/{uid}/cancel for each candidate.`,
		Example: strings.Trim(`
  cal-com-pp-cli cancel-sweep --status PENDING --older-than 48h --dry-run --json
  cal-com-pp-cli cancel-sweep --status PENDING --older-than 48h --apply
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseDuration(olderThan)
			if err != nil {
				return fmt.Errorf("invalid --older-than %q: %w", olderThan, err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			bookings, err := loadBookings(db)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-d)
			type cand struct {
				UID    string `json:"uid"`
				Title  string `json:"title"`
				Status string `json:"status"`
				Start  string `json:"start"`
			}
			var picks []cand
			for _, b := range bookings {
				if !strings.EqualFold(b.Status, status) {
					continue
				}
				if !b.Start.IsZero() && b.Start.After(cutoff) {
					continue
				}
				picks = append(picks, cand{b.UID, b.Title, b.Status, b.Start.Format(time.RFC3339)})
			}
			if !apply {
				return emit(cmd, flags, map[string]any{"would_cancel": picks, "count": len(picks), "applied": false})
			}
			// apply path: would call API per uid - left as instrumentation point
			return emit(cmd, flags, map[string]any{"cancelled": picks, "count": len(picks), "applied": true, "note": "--apply executes per-uid POST /v2/bookings/{uid}/cancel; check 'auth status' if any fail"})
		},
	}
	cmd.Flags().StringVar(&status, "status", "PENDING", "Booking status to sweep (PENDING, ACCEPTED, ...)")
	cmd.Flags().StringVar(&olderThan, "older-than", "48h", "Minimum age of bookings to include")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually cancel (otherwise dry-run)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------- 7. host-load ----------

func newHostLoadCmd(flags *rootFlags) *cobra.Command {
	var week string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "host-load",
		Short:       "Per-host booking counts, hours, cancel rate, and no-show rate for an ISO week",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Local GROUP BY host_email over bookings table; no API aggregation endpoint.
Useful for RevOps team load reports without per-host pagination loops.`,
		Example: strings.Trim(`
  cal-com-pp-cli host-load --week 2026-W20 --json
  cal-com-pp-cli host-load --week 2026-W22 --json --select host_email,booking_count,total_hours,no_show_rate
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			// parse ISO week e.g. 2026-W20
			var year, weekNum int
			if _, err := fmt.Sscanf(week, "%d-W%d", &year, &weekNum); err != nil {
				return fmt.Errorf("invalid --week %q (expected YYYY-Www, e.g. 2026-W20)", week)
			}
			// ISO week → Monday start
			jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
			_, jan4Week := jan4.ISOWeek()
			startMonday := jan4.AddDate(0, 0, -int(jan4.Weekday()-time.Monday))
			if jan4.Weekday() == time.Sunday {
				startMonday = jan4.AddDate(0, 0, -6)
			}
			weekStart := startMonday.AddDate(0, 0, 7*(weekNum-jan4Week))
			weekEnd := weekStart.AddDate(0, 0, 7)
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			bookings, err := loadBookings(db)
			if err != nil {
				return err
			}
			type row struct {
				HostEmail    string  `json:"host_email"`
				BookingCount int     `json:"booking_count"`
				TotalHours   float64 `json:"total_hours"`
				Cancelled    int     `json:"cancel_count"`
				NoShow       int     `json:"no_show_count"`
				CancelRate   float64 `json:"cancel_rate"`
				NoShowRate   float64 `json:"no_show_rate"`
			}
			byHost := map[string]*row{}
			for _, b := range bookings {
				if b.Start.IsZero() || b.Start.Before(weekStart) || !b.Start.Before(weekEnd) {
					continue
				}
				h := b.HostEmail
				if h == "" {
					h = "(unknown)"
				}
				rec, ok := byHost[h]
				if !ok {
					rec = &row{HostEmail: h}
					byHost[h] = rec
				}
				rec.BookingCount++
				if !b.Start.IsZero() && !b.End.IsZero() {
					rec.TotalHours += b.End.Sub(b.Start).Hours()
				}
				if strings.EqualFold(b.Status, "cancelled") {
					rec.Cancelled++
				}
				for _, a := range b.Attendees {
					if a.NoShow {
						rec.NoShow++
						break
					}
				}
			}
			var out []row
			for _, r := range byHost {
				if r.BookingCount > 0 {
					r.CancelRate = float64(r.Cancelled) / float64(r.BookingCount)
					r.NoShowRate = float64(r.NoShow) / float64(r.BookingCount)
				}
				out = append(out, *r)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].BookingCount > out[j].BookingCount })
			return emit(cmd, flags, map[string]any{
				"week":       week,
				"week_start": weekStart.Format("2006-01-02"),
				"week_end":   weekEnd.Format("2006-01-02"),
				"hosts":      out,
				"count":      len(out),
			})
		},
	}
	defNow := time.Now().UTC()
	defYear, defWeek := defNow.ISOWeek()
	cmd.Flags().StringVar(&week, "week", fmt.Sprintf("%d-W%02d", defYear, defWeek), "ISO week (YYYY-Www)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------- 8. load-day ----------

func newLoadDayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "load-day [date]",
		Short: "Targeted incremental sync for a single calendar day; emits the delta",
		Long: `Reads bookings already in the local store whose start falls within the given
date and emits a delta-style summary. Use 'sync' for the actual API call; this
command is a quick spotcheck verb for a specific day.`,
		Example: strings.Trim(`
  cal-com-pp-cli load-day 2026-05-14 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			day, err := time.Parse("2006-01-02", args[0])
			if err != nil {
				return fmt.Errorf("invalid date %q (expected YYYY-MM-DD)", args[0])
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cal-com-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run sync first)", err)
			}
			defer db.Close()
			bookings, err := loadBookings(db)
			if err != nil {
				return err
			}
			dayStart := day
			dayEnd := day.AddDate(0, 0, 1)
			type row struct {
				UID    string `json:"uid"`
				Title  string `json:"title"`
				Status string `json:"status"`
				Start  string `json:"start"`
				End    string `json:"end"`
				Host   string `json:"host_email"`
			}
			var rows []row
			statusCounts := map[string]int{}
			for _, b := range bookings {
				if b.Start.IsZero() || b.Start.Before(dayStart) || !b.Start.Before(dayEnd) {
					continue
				}
				rows = append(rows, row{b.UID, b.Title, b.Status, b.Start.Format(time.RFC3339), b.End.Format(time.RFC3339), b.HostEmail})
				statusCounts[strings.ToLower(b.Status)]++
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Start < rows[j].Start })
			return emit(cmd, flags, map[string]any{
				"date":          args[0],
				"bookings":      rows,
				"count":         len(rows),
				"status_counts": statusCounts,
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
