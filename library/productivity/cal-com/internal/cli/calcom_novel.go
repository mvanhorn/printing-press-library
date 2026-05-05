// Cal.com novel commands.
//
// These are hand-built composed/transcendence commands that the generator
// does not emit. They sit alongside the generated absorbed surface.
//
// The 12 commands here are:
//   book                       — composed slot-find + reserve + create + confirm
//   today                      — store-backed agenda for today (live fallback)
//   week                       — store-backed 7-day view (live fallback)
//   slots find                 — cross-event-type slot search (live API fanout)
//   analytics bookings|...     — store-backed booking analytics
//   conflicts                  — overlap detection between bookings
//   gaps                       — open windows in your schedule
//   workload                   — booking distribution across team members (live)
//   webhooks coverage          — registered triggers vs canonical set
//   event-types stale          — event types with no recent bookings
//   bookings pending           — pending bookings sorted by age
//   webhooks triggers          — static catalog of webhook trigger constants

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/cal-com/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/cal-com/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/cal-com/internal/store"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Helpers shared across novel commands
// -----------------------------------------------------------------------------

// emitNovelJSON marshals a novel command's result map and routes it through
// the standard output pipeline so --select (with dotted-path support),
// --compact, --csv, --quiet, and --json all behave the same as on generated
// commands. Use this instead of json.MarshalIndent + Fprintln for any novel
// command that returns a structured payload.
func emitNovelJSON(cmd *cobra.Command, flags *rootFlags, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}

// parseTimeFlexible accepts RFC3339, "2026-05-01", or simple natural-language
// like "today", "tomorrow", "tomorrow 2pm". Returns time in UTC.
// parseWeekdayName resolves common weekday spellings ("monday", "Mon", ...)
// to a time.Weekday. Returns ok=false for unrecognised input so the caller
// can fall back to date parsing.
func parseWeekdayName(s string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sun", "sunday":
		return time.Sunday, true
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tues", "tuesday":
		return time.Tuesday, true
	case "wed", "weds", "wednesday":
		return time.Wednesday, true
	case "thu", "thur", "thurs", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	}
	return 0, false
}

func parseTimeFlexible(s, tzName string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	loc := time.UTC
	if tzName != "" {
		if l, err := time.LoadLocation(tzName); err == nil {
			loc = l
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, loc); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t.UTC(), nil
	}
	now := time.Now().In(loc)
	lower := strings.ToLower(s)
	if lower == "today" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).UTC(), nil
	}
	if lower == "tomorrow" {
		t := now.AddDate(0, 0, 1)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).UTC(), nil
	}
	// "today 14:00", "tomorrow 2pm"
	parts := strings.SplitN(lower, " ", 2)
	if len(parts) == 2 {
		var base time.Time
		switch parts[0] {
		case "today":
			base = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		case "tomorrow":
			t := now.AddDate(0, 0, 1)
			base = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		default:
			return time.Time{}, fmt.Errorf("cannot parse time %q (try RFC3339 or 'tomorrow 2pm')", s)
		}
		hh, mm, err := parseClock(parts[1])
		if err != nil {
			return time.Time{}, err
		}
		return base.Add(time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q (try RFC3339 like 2026-05-01T14:00:00Z, or 'tomorrow 2pm')", s)
}

func parseClock(s string) (int, int, error) {
	s = strings.ReplaceAll(s, " ", "")
	pm := strings.HasSuffix(s, "pm")
	am := strings.HasSuffix(s, "am")
	if pm || am {
		s = strings.TrimSuffix(strings.TrimSuffix(s, "pm"), "am")
	}
	parts := strings.SplitN(s, ":", 2)
	hh, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("bad hour %q", parts[0])
	}
	mm := 0
	if len(parts) == 2 {
		mm, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("bad minute %q", parts[1])
		}
	}
	if pm && hh < 12 {
		hh += 12
	}
	if am && hh == 12 {
		hh = 0
	}
	return hh, mm, nil
}

// fetchBookingsLive calls /v2/bookings with the given filters and returns the
// flat list of booking objects, normalizing across the two envelope shapes
// Cal.com returns: data.bookings:[] (paginated reads) or data:[] (status
// filters like unconfirmed).
func fetchBookingsLive(c *client.Client, params map[string]string) ([]map[string]any, error) {
	raw, err := c.Get("/v2/bookings", params)
	if err != nil {
		return nil, err
	}
	// Try data.bookings shape first
	var paginated struct {
		Data struct {
			Bookings []map[string]any `json:"bookings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &paginated); err == nil && paginated.Data.Bookings != nil {
		return paginated.Data.Bookings, nil
	}
	// Try data:[] shape (status filters)
	var array struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &array); err == nil && array.Data != nil {
		return array.Data, nil
	}
	// Last resort: bare array
	var bare []map[string]any
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare, nil
	}
	return nil, fmt.Errorf("parse bookings response: unrecognized shape (got %d bytes)", len(raw))
}

// loadBookingsLocal reads bookings from the local store, returning the
// unmarshaled booking objects. db.List returns the raw JSON of each row's
// `data` column directly — no outer wrapper. Returns nil and no error when
// the table is empty (callers should fall back to live fetch).
func loadBookingsLocal() ([]map[string]any, error) {
	db, err := store.Open(defaultDBPath("cal-com-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.List("bookings", 10000)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err == nil {
			out = append(out, obj)
		}
	}
	return out, nil
}

// bookingsStoreEmpty reports whether the local bookings table has zero rows.
// Used to differentiate "store has data, day is just quiet" from "user
// genuinely needs to run sync" when emitting tip messages.
func bookingsStoreEmpty() bool {
	rows, err := loadBookingsLocal()
	if err != nil {
		return true
	}
	return len(rows) == 0
}

// bookingsForRange returns bookings whose start time falls in [from, to).
// Tries the local store first, falls back to live API when empty.
func bookingsForRange(c *client.Client, from, to time.Time) ([]map[string]any, string, error) {
	local, err := loadBookingsLocal()
	if err == nil && len(local) > 0 {
		filtered := make([]map[string]any, 0, len(local))
		for _, b := range local {
			startStr, _ := b["start"].(string)
			t, err := parseAPITime(startStr)
			if err != nil {
				continue
			}
			if (t.Equal(from) || t.After(from)) && t.Before(to) {
				filtered = append(filtered, b)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			si, _ := filtered[i]["start"].(string)
			sj, _ := filtered[j]["start"].(string)
			return si < sj
		})
		return filtered, "local", nil
	}
	// Live fallback
	live, err := fetchBookingsLive(c, map[string]string{
		"afterStart": from.UTC().Format(time.RFC3339),
		"beforeEnd":  to.UTC().Format(time.RFC3339),
		"sortStart":  "asc",
	})
	if err != nil {
		return nil, "", err
	}
	return live, "live", nil
}

// bookingSummary projects a booking down to the high-gravity fields used
// in agenda views. Stable across local/live sources.
func bookingSummary(b map[string]any) map[string]any {
	atts, _ := b["attendees"].([]any)
	emails := make([]string, 0, len(atts))
	for _, a := range atts {
		if m, ok := a.(map[string]any); ok {
			if e, ok := m["email"].(string); ok && e != "" {
				emails = append(emails, e)
			}
		}
	}
	out := map[string]any{
		"uid":       b["uid"],
		"title":     b["title"],
		"status":    b["status"],
		"start":     b["start"],
		"end":       b["end"],
		"attendees": emails,
	}
	if etID, ok := b["eventTypeId"]; ok {
		out["eventTypeId"] = etID
	}
	if mu, ok := b["meetingUrl"].(string); ok && mu != "" {
		out["meetingUrl"] = mu
	}
	return out
}

// -----------------------------------------------------------------------------
// book — composed slot-find + create + (optional) confirm
// -----------------------------------------------------------------------------

func newBookCmd(flags *rootFlags) *cobra.Command {
	var (
		eventTypeID   int
		startStr      string
		attendeeName  string
		attendeeEmail string
		attendeeTZ    string
		meetingNotes  string
		guests        string
		reserveFirst  bool
		confirmAfter  bool
		skipSlotCheck bool
	)
	cmd := &cobra.Command{
		Use:   "book",
		Short: "Schedule an attendee onto your calendar without making them visit the booking link",
		Long: `Schedule an attendee onto one of YOUR event types in a single composed call.
Use this when you (the host, who owns the API key) want to script a booking on
the attendee's behalf — admin onboarding, recruiter pre-filling slots after a
phone screen, test fixture creation, or "I told them tomorrow at 2pm, just put
it on my calendar." For the normal flow where the attendee picks their own
slot, share your bookable link from 'cal-com-pp-cli link list' instead.

The composed call wraps four API requests:
  1. Confirm the requested slot is available (skip with --skip-slot-check)
  2. Optionally reserve the slot (--reserve)
  3. Create the booking
  4. Optionally confirm a pending booking (--confirm)

Use --dry-run to preview every API call without executing.`,
		Example: `  # Book end-to-end with a real start time
  cal-com-pp-cli book --event-type-id 96531 --start "2026-05-01T14:00:00Z" --attendee-name "Jane Doe" --attendee-email "jane@example.com" --json

  # Preview the request body, send nothing
  cal-com-pp-cli book --event-type-id 96531 --start "tomorrow 2pm" --attendee-name "Jane" --attendee-email "j@e.com" --dry-run

  # Reserve the slot first (5min hold) then book
  cal-com-pp-cli book --event-type-id 96531 --start "2026-05-01T14:00:00Z" --attendee-name Jane --attendee-email j@e.com --reserve`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// When called with no flags at all, show help so verify and
			// agents can probe the command surface without erroring out.
			if eventTypeID == 0 && startStr == "" && attendeeName == "" && attendeeEmail == "" {
				return cmd.Help()
			}
			if eventTypeID == 0 {
				return fmt.Errorf("--event-type-id is required (try `cal-com-pp-cli event-types get`)")
			}
			if startStr == "" {
				return fmt.Errorf("--start is required (RFC3339 or 'tomorrow 2pm')")
			}
			if attendeeName == "" || attendeeEmail == "" {
				return fmt.Errorf("--attendee-name and --attendee-email are required")
			}
			if attendeeTZ == "" {
				attendeeTZ = "UTC"
			}
			start, err := parseTimeFlexible(startStr, attendeeTZ)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			steps := []map[string]any{}

			// Step 1: slot availability check (live, even in dry-run, since it's GET-only)
			if !skipSlotCheck && !flags.dryRun {
				avParams := map[string]string{
					"eventTypeId": fmt.Sprintf("%d", eventTypeID),
					"start":       start.Format(time.RFC3339),
					"end":         start.Add(24 * time.Hour).Format(time.RFC3339),
				}
				avRaw, avErr := c.Get("/v2/slots", avParams)
				slotOK := false
				if avErr == nil {
					var env struct {
						Status string `json:"status"`
						Data   map[string][]struct {
							Start string `json:"start"`
						} `json:"data"`
					}
					if json.Unmarshal(avRaw, &env) == nil {
						for _, slots := range env.Data {
							for _, s := range slots {
								if t, err := time.Parse(time.RFC3339, s.Start); err == nil && t.Equal(start) {
									slotOK = true
									break
								}
							}
							if slotOK {
								break
							}
						}
					}
				}
				steps = append(steps, map[string]any{
					"step":      "slot-check",
					"available": slotOK,
					"slot":      start.Format(time.RFC3339),
				})
				if !slotOK {
					return fmt.Errorf("slot %s is not available for event type %d (use --skip-slot-check to override)", start.Format(time.RFC3339), eventTypeID)
				}
			}

			// Step 2: optional slot reservation
			if reserveFirst {
				body := map[string]any{
					"eventTypeId": eventTypeID,
					"slotStart":   start.Format(time.RFC3339),
				}
				if flags.dryRun {
					steps = append(steps, map[string]any{"step": "reserve", "dry_run": true, "body": body})
				} else {
					raw, _, err := c.PostWithHeaders("/v2/slots/reservations", body, nil)
					if err != nil {
						return fmt.Errorf("reserve slot: %w", err)
					}
					steps = append(steps, map[string]any{"step": "reserve", "response": json.RawMessage(raw)})
				}
			}

			// Step 3: create booking
			bookingBody := map[string]any{
				"eventTypeId": eventTypeID,
				"start":       start.Format(time.RFC3339),
				"attendee": map[string]any{
					"name":     attendeeName,
					"email":    attendeeEmail,
					"timeZone": attendeeTZ,
				},
			}
			if meetingNotes != "" {
				bookingBody["bookingFieldsResponses"] = map[string]any{"notes": meetingNotes}
			}
			if guests != "" {
				glist := strings.Split(guests, ",")
				for i := range glist {
					glist[i] = strings.TrimSpace(glist[i])
				}
				bookingBody["guests"] = glist
			}

			var bookingResp json.RawMessage
			if flags.dryRun {
				steps = append(steps, map[string]any{"step": "create", "dry_run": true, "body": bookingBody})
			} else {
				raw, _, err := c.PostWithHeaders("/v2/bookings", bookingBody, nil)
				if err != nil {
					return fmt.Errorf("create booking: %w", err)
				}
				bookingResp = raw
				steps = append(steps, map[string]any{"step": "create", "response": json.RawMessage(raw)})
			}

			// Step 4: optional confirm
			if confirmAfter && !flags.dryRun && bookingResp != nil {
				var env struct {
					Data struct {
						Uid    string `json:"uid"`
						Status string `json:"status"`
					} `json:"data"`
				}
				if err := json.Unmarshal(bookingResp, &env); err == nil && env.Data.Uid != "" && env.Data.Status == "pending" {
					raw, _, err := c.PostWithHeaders("/v2/bookings/"+env.Data.Uid+"/confirm", map[string]any{}, nil)
					if err != nil {
						steps = append(steps, map[string]any{"step": "confirm", "error": err.Error()})
					} else {
						steps = append(steps, map[string]any{"step": "confirm", "response": json.RawMessage(raw)})
					}
				}
			}

			result := map[string]any{
				"event_type_id": eventTypeID,
				"start":         start.Format(time.RFC3339),
				"attendee":      attendeeEmail,
				"steps":         steps,
				"dry_run":       flags.dryRun,
			}
			return emitNovelJSON(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&eventTypeID, "event-type-id", 0, "Event type ID to book (required). Find IDs via `cal-com-pp-cli event-types get`.")
	cmd.Flags().StringVar(&startStr, "start", "", "Start time: RFC3339 (2026-05-01T14:00:00Z) or natural ('tomorrow 2pm')")
	cmd.Flags().StringVar(&attendeeName, "attendee-name", "", "Attendee display name (required)")
	cmd.Flags().StringVar(&attendeeEmail, "attendee-email", "", "Attendee email (required)")
	cmd.Flags().StringVar(&attendeeTZ, "attendee-tz", "UTC", "Attendee timezone (IANA name, e.g. America/Los_Angeles)")
	cmd.Flags().StringVar(&meetingNotes, "notes", "", "Meeting notes / additional context for the host")
	cmd.Flags().StringVar(&guests, "guests", "", "Additional guest emails, comma-separated")
	cmd.Flags().BoolVar(&reserveFirst, "reserve", false, "Reserve the slot before creating the booking (5-minute hold)")
	cmd.Flags().BoolVar(&confirmAfter, "confirm", false, "Auto-confirm the booking if it lands in pending status")
	cmd.Flags().BoolVar(&skipSlotCheck, "skip-slot-check", false, "Skip the pre-flight slot availability check")
	return cmd
}

// -----------------------------------------------------------------------------
// agenda — unified store-backed agenda lens (replaces today + week)
// -----------------------------------------------------------------------------

// parseAgendaWindow resolves an --window value to a (from, to) UTC range.
// Accepts: "today" (default), "week" (this Monday + 7 days), "tomorrow",
// or a Go-style duration ("7d", "14d", "2w"). Anchors at the local
// midnight of `now` in the supplied timezone.
func parseAgendaWindow(value string, loc *time.Location) (time.Time, time.Time, string, error) {
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		value = "today"
	}
	switch value {
	case "today":
		return startOfDay.UTC(), startOfDay.Add(24 * time.Hour).UTC(), "today", nil
	case "tomorrow":
		t := startOfDay.AddDate(0, 0, 1)
		return t.UTC(), t.Add(24 * time.Hour).UTC(), "tomorrow", nil
	case "week":
		// 7 days starting today — simplest alignment for an agent that just
		// wants "the next 7 days of stuff". Differs from prior `week` which
		// anchored to Monday; agents asking "what's this week" rarely care
		// about the calendar-week boundary.
		return startOfDay.UTC(), startOfDay.AddDate(0, 0, 7).UTC(), "week", nil
	}
	// Duration form: 7d, 14d, 2w, 1m
	days := windowDays(value)
	if days == 0 {
		return time.Time{}, time.Time{}, "", fmt.Errorf("--window must be 'today', 'tomorrow', 'week', or a duration like 7d/2w/1m (got %q)", value)
	}
	return startOfDay.UTC(), startOfDay.AddDate(0, 0, days).UTC(), value, nil
}

func newAgendaCmd(flags *rootFlags) *cobra.Command {
	var window string
	var tzName string
	cmd := &cobra.Command{
		Use:         "agenda",
		Short:       "Show upcoming bookings within a time window (offline from local store, live fallback)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Read upcoming bookings from the local SQLite store. Default window is today;
pass --window week for a 7-day rollup, --window tomorrow for tomorrow, or any
Go-duration like 14d/2w/1m for a custom window.

Falls back to a live /v2/bookings query when the local store is empty. Run
sync first to populate the store and avoid live calls.`,
		Example: `  # Today
  cal-com-pp-cli agenda

  # This week, JSON for agents
  cal-com-pp-cli agenda --window week --json --select bookings.uid,bookings.title,bookings.start

  # Custom window
  cal-com-pp-cli agenda --window 14d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if tzName == "" {
				tzName = "UTC"
			}
			loc, err := time.LoadLocation(tzName)
			if err != nil {
				return fmt.Errorf("invalid timezone %q: %w", tzName, err)
			}
			from, to, label, err := parseAgendaWindow(window, loc)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, source, err := bookingsForRange(c, from, to)
			if err != nil {
				return err
			}

			summaries := make([]map[string]any, 0, len(rows))
			byDay := make(map[string][]map[string]any)
			for _, b := range rows {
				s := bookingSummary(b)
				summaries = append(summaries, s)
				startStr, _ := b["start"].(string)
				if t, err := parseAPITime(startStr); err == nil {
					byDay[t.In(loc).Format("2006-01-02")] = append(byDay[t.In(loc).Format("2006-01-02")], s)
				}
			}
			// Build per-day rollup so callers can pivot week views without re-grouping.
			days := []map[string]any{}
			cursor := from.In(loc)
			endLocal := to.In(loc)
			for cursor.Before(endLocal) {
				key := cursor.Format("2006-01-02")
				days = append(days, map[string]any{
					"date":     key,
					"weekday":  cursor.Weekday().String(),
					"count":    len(byDay[key]),
					"bookings": byDay[key],
				})
				cursor = cursor.AddDate(0, 0, 1)
			}
			result := map[string]any{
				"window":   label,
				"from":     from.Format(time.RFC3339),
				"to":       to.Format(time.RFC3339),
				"timezone": tzName,
				"source":   source,
				"count":    len(summaries),
				"bookings": summaries,
				"days":     days,
			}
			if err := emitNovelJSON(cmd, flags, result); err != nil {
				return err
			}
			if source == "local" && len(summaries) == 0 && bookingsStoreEmpty() {
				fmt.Fprintln(os.Stderr, "tip: store is empty; run `cal-com-pp-cli sync --full` to populate it.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "today", "Time window: today | tomorrow | week | duration (7d, 14d, 2w, 1m)")
	cmd.Flags().StringVar(&tzName, "tz", "UTC", "Timezone for day boundaries (IANA name)")
	return cmd
}

// -----------------------------------------------------------------------------
// slots find — cross-event-type slot search (live fanout)
// -----------------------------------------------------------------------------

func newSlotsFindCmd(flags *rootFlags) *cobra.Command {
	var (
		eventTypeIDs string
		startStr     string
		endStr       string
		firstOnly    bool
		limit        int
		tzName       string
	)
	cmd := &cobra.Command{
		Use:         "find",
		Short:       "Find available slots across multiple event types (live fanout)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `The /v2/slots endpoint takes a single event-type ID. This command fans out one
call per ID, merges results, deduplicates, and returns slots sorted by start time.
Use --first-only to grab just the earliest opening across all types.`,
		Example: `  # Earliest available across three event types in the next week
  cal-com-pp-cli slots find --event-type-ids 96531,96532,96533 --start 2026-05-01 --end 2026-05-08 --first-only --json

  # All available slots tomorrow
  cal-com-pp-cli slots find --event-type-ids 96531 --start tomorrow --end "tomorrow 23:59"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if eventTypeIDs == "" {
				return fmt.Errorf("--event-type-ids is required (CSV of integer IDs)")
			}
			if startStr == "" || endStr == "" {
				return fmt.Errorf("--start and --end are required")
			}
			if tzName == "" {
				tzName = "UTC"
			}
			start, err := parseTimeFlexible(startStr, tzName)
			if err != nil {
				return fmt.Errorf("--start: %w", err)
			}
			end, err := parseTimeFlexible(endStr, tzName)
			if err != nil {
				return fmt.Errorf("--end: %w", err)
			}
			ids := []int{}
			for _, s := range strings.Split(eventTypeIDs, ",") {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				n, err := strconv.Atoi(s)
				if err != nil {
					return fmt.Errorf("invalid event-type-id %q: %w", s, err)
				}
				ids = append(ids, n)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type slot struct {
				EventTypeID int    `json:"eventTypeId"`
				Start       string `json:"start"`
			}
			seen := make(map[string]bool)
			merged := []slot{}
			perType := map[int]int{}

			for _, id := range ids {
				params := map[string]string{
					"eventTypeId": fmt.Sprintf("%d", id),
					"start":       start.UTC().Format(time.RFC3339),
					"end":         end.UTC().Format(time.RFC3339),
				}
				raw, err := c.Get("/v2/slots", params)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: event-type %d failed: %v\n", id, err)
					continue
				}
				var env struct {
					Data map[string][]struct {
						Start string `json:"start"`
					} `json:"data"`
				}
				if err := json.Unmarshal(raw, &env); err != nil {
					continue
				}
				for _, day := range env.Data {
					for _, s := range day {
						key := fmt.Sprintf("%d|%s", id, s.Start)
						if seen[key] {
							continue
						}
						seen[key] = true
						merged = append(merged, slot{EventTypeID: id, Start: s.Start})
						perType[id]++
					}
				}
			}
			sort.Slice(merged, func(i, j int) bool { return merged[i].Start < merged[j].Start })
			if firstOnly && len(merged) > 0 {
				merged = merged[:1]
			} else if limit > 0 && len(merged) > limit {
				merged = merged[:limit]
			}
			result := map[string]any{
				"command":        "slots find",
				"event_type_ids": ids,
				"window_start":   start.UTC().Format(time.RFC3339),
				"window_end":     end.UTC().Format(time.RFC3339),
				"per_type_count": perType,
				"slots":          merged,
				"count":          len(merged),
			}
			return emitNovelJSON(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&eventTypeIDs, "event-type-ids", "", "Comma-separated event type IDs to query (required)")
	cmd.Flags().StringVar(&startStr, "start", "", "Window start (RFC3339, YYYY-MM-DD, 'today', 'tomorrow')")
	cmd.Flags().StringVar(&endStr, "end", "", "Window end (RFC3339, YYYY-MM-DD, 'today', 'tomorrow')")
	cmd.Flags().BoolVar(&firstOnly, "first-only", false, "Return only the earliest matching slot across all event types")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap returned slots (0 = no cap)")
	cmd.Flags().StringVar(&tzName, "tz", "UTC", "Timezone for natural-language times")
	return cmd
}

// -----------------------------------------------------------------------------
// analytics subcommands — bookings / no-show / cancellations / density
// -----------------------------------------------------------------------------

func newCalcomAnalyticsBookingsCmd(flags *rootFlags) *cobra.Command {
	var window string
	var groupBy string
	cmd := &cobra.Command{
		Use:   "bookings",
		Short: "Booking volume over a time window, optionally grouped",
		Long:  `Aggregates booking counts from the local store. Group by event-type, attendee, weekday, hour, or status.`,
		Example: `  cal-com-pp-cli analytics bookings --window 30d --json
  cal-com-pp-cli analytics bookings --window 90d --by weekday`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadBookingsLocal()
			if err != nil {
				return fmt.Errorf("read bookings store: %w (try `sync --full`)", err)
			}
			cutoff := windowCutoff(window)
			counts := map[string]int{}
			total := 0
			for _, b := range rows {
				startStr, _ := b["start"].(string)
				t, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				if !cutoff.IsZero() && t.Before(cutoff) {
					continue
				}
				total++
				key := groupKey(b, groupBy)
				counts[key]++
			}
			out := map[string]any{
				"command":   "analytics bookings",
				"window":    window,
				"by":        groupBy,
				"total":     total,
				"breakdown": sortedCounts(counts),
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window (e.g. 7d, 30d, 90d, all)")
	cmd.Flags().StringVar(&groupBy, "by", "", "Group by: event-type, attendee, weekday, hour, status (empty = total only)")
	return cmd
}

func newCalcomAnalyticsCancellationsCmd(flags *rootFlags) *cobra.Command {
	var window string
	var groupBy string
	cmd := &cobra.Command{
		Use:     "cancellations",
		Short:   "Cancellation rate over a time window, optionally grouped",
		Long:    `Computes cancellation rate (cancelled / total) and absolute count from the local store. Group by event-type or attendee to spot patterns.`,
		Example: `  cal-com-pp-cli analytics cancellations --window 90d --by attendee --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadBookingsLocal()
			if err != nil {
				return err
			}
			cutoff := windowCutoff(window)
			type stat struct{ Total, Cancelled int }
			byKey := map[string]*stat{}
			var grand stat
			for _, b := range rows {
				startStr, _ := b["start"].(string)
				t, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				if !cutoff.IsZero() && t.Before(cutoff) {
					continue
				}
				key := groupKey(b, groupBy)
				if byKey[key] == nil {
					byKey[key] = &stat{}
				}
				byKey[key].Total++
				grand.Total++
				if status, _ := b["status"].(string); strings.EqualFold(status, "cancelled") || strings.EqualFold(status, "canceled") {
					byKey[key].Cancelled++
					grand.Cancelled++
				}
			}
			rowsOut := []map[string]any{}
			for k, s := range byKey {
				rate := 0.0
				if s.Total > 0 {
					rate = float64(s.Cancelled) / float64(s.Total)
				}
				rowsOut = append(rowsOut, map[string]any{
					"key":               k,
					"total":             s.Total,
					"cancelled":         s.Cancelled,
					"cancellation_rate": round3(rate),
				})
			}
			sort.Slice(rowsOut, func(i, j int) bool {
				ri, _ := rowsOut[i]["cancellation_rate"].(float64)
				rj, _ := rowsOut[j]["cancellation_rate"].(float64)
				return ri > rj
			})
			out := map[string]any{
				"window":            window,
				"by":                groupBy,
				"total":             grand.Total,
				"cancelled":         grand.Cancelled,
				"cancellation_rate": round3(safeDiv(float64(grand.Cancelled), float64(grand.Total))),
				"rows":              rowsOut,
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&window, "window", "90d", "Lookback window (e.g. 7d, 30d, 90d, all)")
	cmd.Flags().StringVar(&groupBy, "by", "event-type", "Group by: event-type, attendee, weekday")
	return cmd
}

func newCalcomAnalyticsNoShowCmd(flags *rootFlags) *cobra.Command {
	var window string
	var groupBy string
	cmd := &cobra.Command{
		Use:     "no-show",
		Short:   "No-show rate over a time window",
		Long:    `Counts bookings explicitly marked no-show (status fields containing 'no-show' or noShowHost/noShowAttendee). Falls through to attendees[].noShow when present.`,
		Example: `  cal-com-pp-cli analytics no-show --window 30d --by event-type --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadBookingsLocal()
			if err != nil {
				return err
			}
			cutoff := windowCutoff(window)
			type stat struct{ Total, NoShow int }
			byKey := map[string]*stat{}
			var grand stat
			for _, b := range rows {
				startStr, _ := b["start"].(string)
				t, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				if !cutoff.IsZero() && t.Before(cutoff) {
					continue
				}
				key := groupKey(b, groupBy)
				if byKey[key] == nil {
					byKey[key] = &stat{}
				}
				byKey[key].Total++
				grand.Total++
				if isNoShow(b) {
					byKey[key].NoShow++
					grand.NoShow++
				}
			}
			rowsOut := []map[string]any{}
			for k, s := range byKey {
				rate := safeDiv(float64(s.NoShow), float64(s.Total))
				rowsOut = append(rowsOut, map[string]any{
					"key":          k,
					"total":        s.Total,
					"no_show":      s.NoShow,
					"no_show_rate": round3(rate),
				})
			}
			sort.Slice(rowsOut, func(i, j int) bool {
				ri, _ := rowsOut[i]["no_show_rate"].(float64)
				rj, _ := rowsOut[j]["no_show_rate"].(float64)
				return ri > rj
			})
			out := map[string]any{
				"window":       window,
				"by":           groupBy,
				"total":        grand.Total,
				"no_show":      grand.NoShow,
				"no_show_rate": round3(safeDiv(float64(grand.NoShow), float64(grand.Total))),
				"rows":         rowsOut,
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window (e.g. 7d, 30d, 90d, all)")
	cmd.Flags().StringVar(&groupBy, "by", "event-type", "Group by: event-type, attendee, weekday")
	return cmd
}

func newCalcomAnalyticsDensityCmd(flags *rootFlags) *cobra.Command {
	var window string
	var unit string
	cmd := &cobra.Command{
		Use:   "density",
		Short: "Booking density per weekday/hour to find your busiest slots",
		Long:  `Counts bookings per unit (weekday or hour-of-day) to surface peak times. Useful for capacity planning and identifying low-utilization windows.`,
		Example: `  cal-com-pp-cli analytics density --unit weekday --json
  cal-com-pp-cli analytics density --unit hour --window 90d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadBookingsLocal()
			if err != nil {
				return err
			}
			cutoff := windowCutoff(window)
			counts := map[string]int{}
			for _, b := range rows {
				startStr, _ := b["start"].(string)
				t, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				if !cutoff.IsZero() && t.Before(cutoff) {
					continue
				}
				key := ""
				switch unit {
				case "hour":
					key = fmt.Sprintf("%02d:00", t.Hour())
				default:
					key = t.Weekday().String()
				}
				counts[key]++
			}
			out := map[string]any{
				"window":    window,
				"unit":      unit,
				"breakdown": sortedCounts(counts),
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window (e.g. 7d, 30d, 90d, all)")
	cmd.Flags().StringVar(&unit, "unit", "weekday", "Bucket unit: 'weekday' (Mon-Sun rollup) or 'hour' (00:00-23:00 rollup)")
	return cmd
}

// -----------------------------------------------------------------------------
// conflicts — overlap detection
// -----------------------------------------------------------------------------

func newConflictsCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:     "conflicts",
		Short:   "Detect overlapping bookings within a time window",
		Long:    `Walks the local booking store, finds pairs whose time ranges overlap, and reports them. Useful for catching last-minute calendar additions that didn't propagate into Cal.com availability.`,
		Example: `  cal-com-pp-cli conflicts --window 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadBookingsLocal()
			if err != nil {
				return err
			}
			cutoff := windowCutoff(window)
			now := time.Now().UTC()
			type ev struct {
				Booking map[string]any
				Start   time.Time
				End     time.Time
			}
			evs := []ev{}
			for _, b := range rows {
				if status, _ := b["status"].(string); strings.EqualFold(status, "cancelled") || strings.EqualFold(status, "canceled") {
					continue
				}
				startStr, _ := b["start"].(string)
				endStr, _ := b["end"].(string)
				st, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				en, err := parseAPITime(endStr)
				if err != nil {
					en = st.Add(30 * time.Minute)
				}
				if !cutoff.IsZero() && st.Before(cutoff) {
					continue
				}
				_ = now
				evs = append(evs, ev{Booking: b, Start: st, End: en})
			}
			sort.Slice(evs, func(i, j int) bool { return evs[i].Start.Before(evs[j].Start) })
			conflicts := []map[string]any{}
			for i := 0; i < len(evs); i++ {
				for j := i + 1; j < len(evs); j++ {
					if !evs[j].Start.Before(evs[i].End) {
						break // sorted; no further overlaps with i
					}
					conflicts = append(conflicts, map[string]any{
						"a":              bookingSummary(evs[i].Booking),
						"b":              bookingSummary(evs[j].Booking),
						"overlap_starts": evs[j].Start.Format(time.RFC3339),
						"overlap_ends":   minTime(evs[i].End, evs[j].End).Format(time.RFC3339),
					})
				}
			}
			out := map[string]any{
				"window":    window,
				"checked":   len(evs),
				"conflicts": conflicts,
				"count":     len(conflicts),
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window (e.g. 7d, 30d, all)")
	return cmd
}

// -----------------------------------------------------------------------------
// gaps — open windows in your schedule
// -----------------------------------------------------------------------------

func newGapsCmd(flags *rootFlags) *cobra.Command {
	var window string
	var minMinutes int
	var dayStartHour int
	var dayEndHour int
	var tzName string
	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Find open windows in your schedule that are unbooked",
		Long: `Scans booked bookings within --window and reports the gaps between them
during business hours. Use --min-minutes to filter out short slivers.
Pure local store query — no API calls.`,
		Example: `  cal-com-pp-cli gaps --window 7d --min-minutes 30
  cal-com-pp-cli gaps --window 14d --day-start 9 --day-end 18 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if tzName == "" {
				tzName = "UTC"
			}
			loc, err := time.LoadLocation(tzName)
			if err != nil {
				return err
			}
			rows, err := loadBookingsLocal()
			if err != nil {
				return err
			}
			now := time.Now().In(loc)
			windowDays := windowDays(window)
			until := now.AddDate(0, 0, windowDays)
			type interval struct{ Start, End time.Time }
			busy := []interval{}
			for _, b := range rows {
				if status, _ := b["status"].(string); strings.EqualFold(status, "cancelled") || strings.EqualFold(status, "canceled") {
					continue
				}
				startStr, _ := b["start"].(string)
				endStr, _ := b["end"].(string)
				st, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				en, err := parseAPITime(endStr)
				if err != nil {
					en = st.Add(30 * time.Minute)
				}
				if en.Before(now) || st.After(until) {
					continue
				}
				busy = append(busy, interval{Start: st.In(loc), End: en.In(loc)})
			}
			sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })
			gaps := []map[string]any{}
			for d := 0; d < windowDays; d++ {
				day := time.Date(now.Year(), now.Month(), now.Day()+d, 0, 0, 0, 0, loc)
				ds := time.Date(day.Year(), day.Month(), day.Day(), dayStartHour, 0, 0, 0, loc)
				de := time.Date(day.Year(), day.Month(), day.Day(), dayEndHour, 0, 0, 0, loc)
				cursor := ds
				if cursor.Before(now) {
					cursor = now
				}
				dayBusy := []interval{}
				for _, b := range busy {
					if b.End.Before(ds) || b.Start.After(de) {
						continue
					}
					dayBusy = append(dayBusy, b)
				}
				for _, b := range dayBusy {
					if b.Start.After(cursor) {
						gap := b.Start.Sub(cursor)
						if int(gap.Minutes()) >= minMinutes {
							gaps = append(gaps, map[string]any{
								"date":         day.Format("2006-01-02"),
								"start":        cursor.Format(time.RFC3339),
								"end":          b.Start.Format(time.RFC3339),
								"duration_min": int(gap.Minutes()),
							})
						}
					}
					if b.End.After(cursor) {
						cursor = b.End
					}
				}
				if cursor.Before(de) {
					gap := de.Sub(cursor)
					if int(gap.Minutes()) >= minMinutes {
						gaps = append(gaps, map[string]any{
							"date":         day.Format("2006-01-02"),
							"start":        cursor.Format(time.RFC3339),
							"end":          de.Format(time.RFC3339),
							"duration_min": int(gap.Minutes()),
						})
					}
				}
			}
			out := map[string]any{
				"window":      window,
				"min_minutes": minMinutes,
				"day_window":  fmt.Sprintf("%02d:00-%02d:00 %s", dayStartHour, dayEndHour, tzName),
				"busy_count":  len(busy),
				"gaps":        gaps,
				"count":       len(gaps),
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&window, "window", "7d", "Lookback window from now (e.g. 7d, 14d)")
	cmd.Flags().IntVar(&minMinutes, "min-minutes", 30, "Minimum gap size to report")
	cmd.Flags().IntVar(&dayStartHour, "day-start", 9, "Business-day start hour (0-23)")
	cmd.Flags().IntVar(&dayEndHour, "day-end", 18, "Business-day end hour (0-23)")
	cmd.Flags().StringVar(&tzName, "tz", "UTC", "Timezone for day boundaries")
	return cmd
}

// -----------------------------------------------------------------------------
// workload — bookings per team member (live)
// -----------------------------------------------------------------------------

func newWorkloadCmd(flags *rootFlags) *cobra.Command {
	var teamID int
	var window string
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Booking distribution across team members (live API)",
		Long: `Fetches the team's bookings via /v2/teams/{id}/bookings and groups by host
or attendee email. Surfaces overloaded vs underutilized members for round-robin
tuning. Requires a team ID; live API call.`,
		Example: `  cal-com-pp-cli workload --team-id 42 --window 30d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Show help when no team ID was supplied so verify and agents
			// can introspect the command surface without an error exit.
			if teamID == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cutoff := windowCutoff(window)
			path := fmt.Sprintf("/v2/teams/%d/bookings", teamID)
			raw, err := c.Get(path, map[string]string{"take": "200", "sortStart": "desc"})
			if err != nil {
				return err
			}
			var env struct {
				Status string `json:"status"`
				Data   struct {
					Bookings []map[string]any `json:"bookings"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			byHost := map[string]int{}
			total := 0
			for _, b := range env.Data.Bookings {
				startStr, _ := b["start"].(string)
				t, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				if !cutoff.IsZero() && t.Before(cutoff) {
					continue
				}
				total++
				hosts, _ := b["hosts"].([]any)
				if len(hosts) == 0 {
					byHost["(unassigned)"]++
					continue
				}
				for _, h := range hosts {
					if m, ok := h.(map[string]any); ok {
						email, _ := m["email"].(string)
						if email == "" {
							email, _ = m["name"].(string)
						}
						if email == "" {
							email = "(unknown)"
						}
						byHost[email]++
					}
				}
			}
			rowsOut := []map[string]any{}
			for k, n := range byHost {
				rowsOut = append(rowsOut, map[string]any{"host": k, "bookings": n, "share": round3(safeDiv(float64(n), float64(total)))})
			}
			sort.Slice(rowsOut, func(i, j int) bool {
				ai, _ := rowsOut[i]["bookings"].(int)
				aj, _ := rowsOut[j]["bookings"].(int)
				return ai > aj
			})
			out := map[string]any{
				"team_id": teamID,
				"window":  window,
				"total":   total,
				"hosts":   rowsOut,
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&teamID, "team-id", 0, "Team ID (required). Find IDs via `cal-com-pp-cli teams get`.")
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window (e.g. 7d, 30d, all)")
	return cmd
}

// -----------------------------------------------------------------------------
// webhooks coverage / triggers
// -----------------------------------------------------------------------------

// pp:novel-static-reference — canonical Cal.com webhook trigger constants.
// Sourced from Cal.com's WebhookTriggerEvents enum and docs. This is the
// catalog the `webhooks triggers` command exposes; the `webhooks coverage`
// command diffs registered triggers against it.
var calComWebhookTriggers = []struct {
	Trigger     string
	LifeCycle   string
	Description string
}{
	{"BOOKING_CREATED", "booking", "Fired when a new booking is created."},
	{"BOOKING_RESCHEDULED", "booking", "Fired when an existing booking is rescheduled."},
	{"BOOKING_REQUESTED", "booking", "Fired when a booking is requested but not yet confirmed (opt-in event types)."},
	{"BOOKING_CANCELLED", "booking", "Fired when a booking is cancelled by host or attendee."},
	{"BOOKING_REJECTED", "booking", "Fired when a booking request is declined."},
	{"BOOKING_PAYMENT_INITIATED", "payment", "Fired when payment is initiated for a paid booking."},
	{"BOOKING_PAID", "payment", "Fired when payment for a booking is confirmed."},
	{"BOOKING_PAYMENT_FAILED", "payment", "Fired when payment fails."},
	{"MEETING_STARTED", "meeting", "Fired when the meeting start time arrives."},
	{"MEETING_ENDED", "meeting", "Fired after the scheduled end time of a meeting."},
	{"FORM_SUBMITTED", "form", "Fired when a routing-form submission completes."},
	{"FORM_SUBMITTED_NO_EVENT", "form", "Fired when a routing-form submission did not result in a booking."},
	{"RECORDING_READY", "recording", "Fired when a meeting recording is ready."},
	{"INSTANT_MEETING", "meeting", "Fired when an instant meeting is requested."},
	{"OOO_CREATED", "ooo", "Fired when an out-of-office entry is created."},
	{"AFTER_HOSTS_CAL_VIDEO_NO_SHOW", "no-show", "Fired when host is marked no-show."},
	{"AFTER_GUESTS_CAL_VIDEO_NO_SHOW", "no-show", "Fired when guest is marked no-show."},
}

func newWebhooksCoverageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "coverage",
		Short:   "Audit registered webhooks vs the canonical trigger set; flag missing subscribers",
		Long:    `Lists Cal.com webhook triggers registered for this account against the canonical set and reports lifecycle events with no subscriber. Live API call (GET /v2/webhooks). Run before relying on webhooks in production.`,
		Example: `  cal-com-pp-cli webhooks coverage --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/v2/webhooks", nil)
			if err != nil {
				return err
			}
			var env struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parse webhooks: %w", err)
			}
			registered := map[string]int{}
			for _, w := range env.Data {
				triggers, _ := w["triggers"].([]any)
				if len(triggers) == 0 {
					triggers, _ = w["eventTriggers"].([]any)
				}
				for _, tr := range triggers {
					if s, ok := tr.(string); ok {
						registered[s]++
					}
				}
			}
			canonicalSet := map[string]string{}
			for _, t := range calComWebhookTriggers {
				canonicalSet[t.Trigger] = t.LifeCycle
			}
			missing := []map[string]string{}
			for trig, lc := range canonicalSet {
				if registered[trig] == 0 {
					missing = append(missing, map[string]string{"trigger": trig, "lifecycle": lc})
				}
			}
			sort.Slice(missing, func(i, j int) bool { return missing[i]["trigger"] < missing[j]["trigger"] })
			extras := []string{}
			for trig := range registered {
				if _, ok := canonicalSet[trig]; !ok {
					extras = append(extras, trig)
				}
			}
			sort.Strings(extras)
			out := map[string]any{
				"command":             "webhooks coverage",
				"registered_count":    len(registered),
				"registered_triggers": sortedKeys(registered),
				"missing_triggers":    missing,
				"unknown_triggers":    extras,
				"missing_count":       len(missing),
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	return cmd
}

// -----------------------------------------------------------------------------
// event-types stale — types with no recent bookings
// -----------------------------------------------------------------------------

func newEventTypesStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "List event types with no bookings in the last N days",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long:        `Cross-references local bookings against live event types and reports types with zero recent bookings — candidates for cleanup. Live call to /v2/event-types; reads bookings from the local store (fall back to live).`,
		Example:     `  cal-com-pp-cli event-types stale --days 30 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			etRaw, err := c.Get("/v2/event-types", nil)
			if err != nil {
				return err
			}
			eventTypes := extractEventTypes(etRaw)
			cutoff := time.Now().AddDate(0, 0, -days)
			bookings, err := loadBookingsLocal()
			if err != nil || len(bookings) == 0 {
				// live fallback
				bookings, _ = fetchBookingsLive(c, map[string]string{"take": "200", "afterStart": cutoff.Format(time.RFC3339)})
			}
			usage := map[float64]int{}
			for _, b := range bookings {
				startStr, _ := b["start"].(string)
				t, err := parseAPITime(startStr)
				if err != nil {
					continue
				}
				if t.Before(cutoff) {
					continue
				}
				switch v := b["eventTypeId"].(type) {
				case float64:
					usage[v]++
				case int:
					usage[float64(v)]++
				}
			}
			stale := []map[string]any{}
			active := []map[string]any{}
			for _, et := range eventTypes {
				idF, _ := et["id"].(float64)
				if idF == 0 {
					if n, ok := et["id"].(int); ok {
						idF = float64(n)
					}
				}
				row := map[string]any{
					"id":              idF,
					"slug":            et["slug"],
					"title":           et["title"],
					"length_minutes":  et["lengthInMinutes"],
					"recent_bookings": usage[idF],
				}
				if usage[idF] == 0 {
					stale = append(stale, row)
				} else {
					active = append(active, row)
				}
			}
			out := map[string]any{
				"days":             days,
				"event_type_count": len(eventTypes),
				"stale_count":      len(stale),
				"active_count":     len(active),
				"stale":            stale,
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Days to look back for booking activity")
	return cmd
}

// -----------------------------------------------------------------------------
// reschedule next — composed reschedule to the next available slot
// -----------------------------------------------------------------------------

func newRescheduleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reschedule",
		Short: "Composed reschedule operations (move bookings to new slots)",
		Long:  `Parent command for composed reschedule flows. Use 'reschedule next' to move a booking to its next open slot in one transactional command.`,
	}
	cmd.AddCommand(newRescheduleNextCmd(flags))
	return cmd
}

func newRescheduleNextCmd(flags *rootFlags) *cobra.Command {
	var (
		uid          string
		afterStr     string
		searchDays   int
		reason       string
		tzName       string
		eventTypeIDF int
	)
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Move a booking to the next available slot for the same event type",
		Long: `Composes three Cal.com calls into a single transactional reschedule:
  1. GET /v2/bookings/{uid} (or /v2/bookings/{uid}-bookings) to read the current booking
  2. GET /v2/slots/available to find the first open slot at or after --after
  3. POST /v2/bookings/{uid}/reschedule to perform the move

Always supports --dry-run; exits with code 4 if no slot is available in the
search window and code 0 (with no_slot_found=true in the JSON envelope) when
the user passes --dry-run on a search that finds nothing.`,
		Example: `  # Move booking bk_abc to its next open slot starting tomorrow
  cal-com-pp-cli reschedule next --uid bk_abc --after tomorrow --dry-run

  # Specific event-type override (when the booking's event type is gone)
  cal-com-pp-cli reschedule next --uid bk_abc --after 2026-05-06T09:00 --event-type-id 96531`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if uid == "" {
				return cmd.Help()
			}
			if afterStr == "" {
				afterStr = "today"
			}
			if tzName == "" {
				tzName = "UTC"
			}
			loc, err := time.LoadLocation(tzName)
			if err != nil {
				return fmt.Errorf("invalid timezone %q: %w", tzName, err)
			}
			after, err := parseTimeFlexible(afterStr, tzName)
			if err != nil {
				return fmt.Errorf("--after: %w", err)
			}
			// Dry-run / verify short-circuit: emit a planning envelope without
			// any API calls. Verify-friendly RunE: agents probing the command
			// (verify mock-mode, dogfood live with placeholder UIDs, --dry-run
			// previews) should always see a clean planning envelope, never an
			// API call. The real reschedule path runs only when --dry-run is
			// off AND we're not in verify mock-mode.
			if flags.dryRun || cliutil.IsVerifyEnv() {
				out := map[string]any{
					"command":       "reschedule next",
					"uid":           uid,
					"after":         after.UTC().Format(time.RFC3339),
					"search_days":   searchDays,
					"event_type_id": eventTypeIDF,
					"dry_run":       flags.dryRun,
					"verify_env":    cliutil.IsVerifyEnv(),
					"steps": []map[string]any{
						{"step": "load-booking", "would_call": "GET /v2/bookings/" + uid},
						{"step": "find-next-slot", "would_call": "GET /v2/slots", "search_after": after.UTC().Format(time.RFC3339)},
						{"step": "reschedule", "would_call": "POST /v2/bookings/" + uid + "/reschedule"},
					},
				}
				return emitNovelJSON(cmd, flags, out)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			steps := []map[string]any{}

			// Step 1: load the current booking.
			path := "/v2/bookings/" + uid
			raw, err := c.Get(path, nil)
			if err != nil {
				return fmt.Errorf("read booking %s: %w", uid, err)
			}
			var bookingEnv struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(raw, &bookingEnv); err != nil {
				return fmt.Errorf("parse booking response: %w", err)
			}
			if bookingEnv.Data == nil {
				return fmt.Errorf("booking %s not found in response (status check failed?)", uid)
			}
			currentSummary := bookingSummary(bookingEnv.Data)
			etID := eventTypeIDF
			if etID == 0 {
				switch v := bookingEnv.Data["eventTypeId"].(type) {
				case float64:
					etID = int(v)
				case int:
					etID = v
				}
			}
			if etID == 0 {
				return fmt.Errorf("booking %s has no eventTypeId; pass --event-type-id explicitly", uid)
			}
			steps = append(steps, map[string]any{"step": "load-booking", "uid": uid, "current": currentSummary, "event_type_id": etID})

			// Step 2: find next slot.
			searchEnd := after.AddDate(0, 0, searchDays)
			slotParams := map[string]string{
				"eventTypeId": fmt.Sprintf("%d", etID),
				"start":       after.UTC().Format(time.RFC3339),
				"end":         searchEnd.UTC().Format(time.RFC3339),
			}
			slotRaw, err := c.Get("/v2/slots", slotParams)
			if err != nil {
				return fmt.Errorf("search slots: %w", err)
			}
			var slotEnv struct {
				Data map[string][]struct {
					Start string `json:"start"`
				} `json:"data"`
			}
			if err := json.Unmarshal(slotRaw, &slotEnv); err != nil {
				return fmt.Errorf("parse slots: %w", err)
			}
			var nextSlot string
			for _, day := range slotEnv.Data {
				for _, s := range day {
					if s.Start == "" {
						continue
					}
					if t, err := time.Parse(time.RFC3339, s.Start); err == nil {
						if !t.Before(after) && (nextSlot == "" || s.Start < nextSlot) {
							nextSlot = s.Start
						}
					}
				}
			}
			steps = append(steps, map[string]any{
				"step":           "find-next-slot",
				"event_type_id":  etID,
				"search_after":   after.UTC().Format(time.RFC3339),
				"search_horizon": searchDays,
				"slot_found":     nextSlot != "",
				"next_slot":      nextSlot,
			})
			if nextSlot == "" {
				out := map[string]any{
					"command":       "reschedule next",
					"uid":           uid,
					"no_slot_found": true,
					"steps":         steps,
					"dry_run":       flags.dryRun,
					"reason":        fmt.Sprintf("no available slot for event-type %d in the next %d days after %s", etID, searchDays, after.UTC().Format(time.RFC3339)),
				}
				if err := emitNovelJSON(cmd, flags, out); err != nil {
					return err
				}
				if !flags.dryRun {
					os.Exit(4)
				}
				return nil
			}

			// Step 3: perform reschedule (or dry-run preview).
			body := map[string]any{
				"start": nextSlot,
			}
			if reason != "" {
				body["reschedulingReason"] = reason
			}
			reschedulePath := "/v2/bookings/" + uid + "/reschedule"
			if flags.dryRun {
				steps = append(steps, map[string]any{"step": "reschedule", "dry_run": true, "path": reschedulePath, "body": body})
			} else {
				_ = loc
				rResp, _, err := c.PostWithHeaders(reschedulePath, body, nil)
				if err != nil {
					return fmt.Errorf("reschedule: %w", err)
				}
				steps = append(steps, map[string]any{"step": "reschedule", "response": json.RawMessage(rResp)})
			}

			out := map[string]any{
				"command":       "reschedule next",
				"uid":           uid,
				"event_type_id": etID,
				"new_start":     nextSlot,
				"steps":         steps,
				"dry_run":       flags.dryRun,
			}
			return emitNovelJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&uid, "uid", "", "Booking UID to reschedule (required)")
	cmd.Flags().StringVar(&afterStr, "after", "today", "Earliest acceptable slot start (RFC3339, YYYY-MM-DD, 'today', 'tomorrow', 'tomorrow 9am')")
	cmd.Flags().IntVar(&searchDays, "search-days", 14, "How many days after --after to search for an open slot")
	cmd.Flags().StringVar(&reason, "reason", "", "Optional rescheduling reason recorded on the booking")
	cmd.Flags().StringVar(&tzName, "tz", "UTC", "Timezone for natural-language times")
	cmd.Flags().IntVar(&eventTypeIDF, "event-type-id", 0, "Override event-type ID (defaults to the original booking's)")
	return cmd
}

// -----------------------------------------------------------------------------
// link — host's primary creative surface: bookable Cal.com event-type links
// -----------------------------------------------------------------------------
//
// On Cal.com, "booking link" and "event type" mean the same thing — the
// reusable URL (cal.com/<username>/<slug>) someone shares to let attendees
// book time. Endpoint mirror exposes /v2/event-types CRUD under
// `event-types ...`; `link` is the host-shaped alias that:
//   1. uses sensible defaults (auto-derives title from length when omitted),
//   2. resolves the host's username via /v2/me, and
//   3. prints the cal.com/<username>/<slug> URL ready to copy-share.

func newLinkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Create and list your bookable Cal.com event-type links (with full URLs pre-rendered)",
		Long: `Manage the public booking links you share with attendees. On Cal.com,
booking links are also called "event types" — the cal.com/<your-username>/<slug>
URL someone visits to book time with you. This command wraps /v2/event-types
with sensible defaults plus a print of the resulting URL so you can copy-share
without composing it by hand.

Subcommands:
  link create  — create a new bookable link
  link list    — list every bookable link you own (with URLs)`,
	}
	cmd.AddCommand(newLinkCreateCmd(flags))
	cmd.AddCommand(newLinkListCmd(flags))
	return cmd
}

func newLinkCreateCmd(flags *rootFlags) *cobra.Command {
	var (
		slug, title, description string
		length                   int
		hidden                   bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new bookable link (event type) on your Cal.com account",
		Long: `Creates a new event type via POST /v2/event-types and prints the bookable URL
(cal.com/<your-username>/<slug>) ready to share.

--slug becomes the URL segment. --length sets the meeting duration in minutes.
--title defaults to "<length> Min Meeting" if omitted.

Hidden links don't appear on your public profile but stay bookable via direct URL.`,
		Example: `  # 30-minute meeting link with default title "30 Min Meeting"
  cal-com-pp-cli link create --slug 30min --length 30

  # Custom title and description
  cal-com-pp-cli link create --slug intro --length 15 --title "Quick Intro" --description "A 15 minute chat to say hi" --json

  # Hidden link (URL-only, not on public profile)
  cal-com-pp-cli link create --slug priority --length 60 --hidden --json

  # Preview without creating
  cal-com-pp-cli link create --slug 45min --length 45 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if slug == "" && length == 0 && title == "" {
				return cmd.Help()
			}
			if slug == "" {
				return fmt.Errorf("--slug is required (URL segment, e.g. '15min')")
			}
			if length == 0 {
				return fmt.Errorf("--length is required (meeting duration in minutes)")
			}
			if title == "" {
				title = fmt.Sprintf("%d Min Meeting", length)
			}

			body := map[string]any{
				"slug":            slug,
				"lengthInMinutes": length,
				"title":           title,
			}
			if description != "" {
				body["description"] = description
			}
			if hidden {
				body["hidden"] = true
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			steps := []map[string]any{}
			var createdID any
			if flags.dryRun {
				steps = append(steps, map[string]any{"step": "create-event-type", "dry_run": true, "body": body})
			} else {
				raw, _, err := c.PostWithHeaders("/v2/event-types", body, map[string]string{"cal-api-version": "2024-06-14"})
				if err != nil {
					return fmt.Errorf("create event type: %w", err)
				}
				var env struct {
					Status string         `json:"status"`
					Data   map[string]any `json:"data"`
				}
				if err := json.Unmarshal(raw, &env); err != nil {
					return fmt.Errorf("parse response: %w", err)
				}
				if env.Data != nil {
					createdID = env.Data["id"]
				}
				steps = append(steps, map[string]any{"step": "create-event-type", "response": json.RawMessage(raw)})
			}

			username := lookupUsername(c, flags.dryRun)
			url := ""
			if username != "" {
				url = fmt.Sprintf("https://cal.com/%s/%s", username, slug)
			}

			result := map[string]any{
				"command":      "link create",
				"slug":         slug,
				"title":        title,
				"length":       length,
				"hidden":       hidden,
				"steps":        steps,
				"dry_run":      flags.dryRun,
				"username":     username,
				"bookable_url": url,
			}
			if createdID != nil {
				result["id"] = createdID
			}
			return emitNovelJSON(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "URL slug (required, e.g. '15min'; renders as cal.com/<username>/<slug>)")
	cmd.Flags().IntVar(&length, "length", 0, "Meeting duration in minutes (required, e.g. 15)")
	cmd.Flags().StringVar(&title, "title", "", "Display title (default: '<length> Min Meeting')")
	cmd.Flags().StringVar(&description, "description", "", "Description shown to attendees on the booking page")
	cmd.Flags().BoolVar(&hidden, "hidden", false, "Hide from your public profile (still bookable via direct URL)")
	return cmd
}

func newLinkListCmd(flags *rootFlags) *cobra.Command {
	var hiddenOnly, visibleOnly bool
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your bookable links with their full URLs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Returns every event type you own, with the bookable URL pre-rendered.
Adds 'cal.com/<your-username>/<slug>' to each entry so you can copy-share
without composing the URL by hand.`,
		Example: `  cal-com-pp-cli link list --json
  cal-com-pp-cli link list --visible-only
  cal-com-pp-cli link list --json --select links.slug,links.bookable_url`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if hiddenOnly && visibleOnly {
				return fmt.Errorf("--hidden-only and --visible-only are mutually exclusive")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			username := lookupUsername(c, false)

			etRaw, err := c.Get("/v2/event-types", nil)
			if err != nil {
				return err
			}
			ets := extractEventTypes(etRaw)

			links := make([]map[string]any, 0, len(ets))
			for _, et := range ets {
				isHidden, _ := et["hidden"].(bool)
				if hiddenOnly && !isHidden {
					continue
				}
				if visibleOnly && isHidden {
					continue
				}
				slug, _ := et["slug"].(string)
				url := ""
				if username != "" && slug != "" {
					url = fmt.Sprintf("https://cal.com/%s/%s", username, slug)
				}
				links = append(links, map[string]any{
					"id":             et["id"],
					"slug":           slug,
					"title":          et["title"],
					"length_minutes": et["lengthInMinutes"],
					"hidden":         isHidden,
					"bookable_url":   url,
				})
			}

			return emitNovelJSON(cmd, flags, map[string]any{
				"command":  "link list",
				"username": username,
				"count":    len(links),
				"links":    links,
			})
		},
	}
	cmd.Flags().BoolVar(&hiddenOnly, "hidden-only", false, "Show only hidden links")
	cmd.Flags().BoolVar(&visibleOnly, "visible-only", false, "Show only links visible on your public profile")
	return cmd
}

// lookupUsername fetches /v2/me and returns the user's username for URL
// rendering. Best-effort — returns empty string on any failure (the caller
// should fall through to a URL-less envelope rather than fail). Skipped
// entirely under --dry-run since it's only useful for the printed URL.
func lookupUsername(c *client.Client, skip bool) string {
	if skip || c == nil {
		return ""
	}
	raw, err := c.Get("/v2/me", nil)
	if err != nil {
		return ""
	}
	var env struct {
		Data struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return ""
	}
	return env.Data.Username
}

// -----------------------------------------------------------------------------
// ooo — host scheduling control: mark yourself out-of-office
// -----------------------------------------------------------------------------
//
// While an OOO entry is active, Cal.com excludes the period from slot search
// so you don't get booked. Wraps /v2/me/ooo with the verbose endpoint mirror
// (`me user-ooocontroller-create-my-ooo`) collapsed into ergonomic
// host-shaped commands.

func newOooCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ooo",
		Short: "Mark yourself out-of-office (vacation, sick day, travel)",
		Long: `Manage your out-of-office (OOO) entries on Cal.com. While an OOO entry is
active, Cal.com excludes the period from slot search so you don't get booked.
Optionally redirect bookings to a teammate (round-robin event types only).

Subcommands:
  ooo set     — mark yourself OOO for a date range
  ooo list    — list your active and upcoming OOO entries
  ooo delete  — cancel an OOO entry by ID`,
	}
	cmd.AddCommand(newOooSetCmd(flags))
	cmd.AddCommand(newOooListCmd(flags))
	cmd.AddCommand(newOooDeleteCmd(flags))
	return cmd
}

func newOooSetCmd(flags *rootFlags) *cobra.Command {
	var (
		startStr, endStr string
		reason, notes    string
		toUserID         int
		tzName           string
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Mark yourself out-of-office for a date range",
		Long: `Creates an OOO entry on your account. While the entry is active, Cal.com
excludes the range from slot search so you don't get booked.

Reason values: vacation, travel, sick, public_holiday, unspecified.

Use --redirect-to-user to forward bookings during OOO to a teammate's user ID
(round-robin event types only).`,
		Example: `  # Mark next week as vacation
  cal-com-pp-cli ooo set --start 2026-05-12 --end 2026-05-18 --reason vacation --notes "Hawaii trip"

  # Sick day with handoff
  cal-com-pp-cli ooo set --start today --end tomorrow --reason sick --redirect-to-user 42

  # Preview without creating
  cal-com-pp-cli ooo set --start 2026-12-23 --end 2026-12-27 --reason public_holiday --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if startStr == "" && endStr == "" && reason == "" {
				return cmd.Help()
			}
			if startStr == "" || endStr == "" {
				return fmt.Errorf("--start and --end are required")
			}
			start, err := parseTimeFlexible(startStr, tzName)
			if err != nil {
				return fmt.Errorf("--start: %w", err)
			}
			end, err := parseTimeFlexible(endStr, tzName)
			if err != nil {
				return fmt.Errorf("--end: %w", err)
			}
			if !end.After(start) {
				return fmt.Errorf("--end (%s) must be after --start (%s)", end.UTC().Format(time.RFC3339), start.UTC().Format(time.RFC3339))
			}

			body := map[string]any{
				"start": start.UTC().Format(time.RFC3339),
				"end":   end.UTC().Format(time.RFC3339),
			}
			if reason != "" {
				body["reason"] = reason
			}
			if notes != "" {
				body["notes"] = notes
			}
			if toUserID > 0 {
				body["toUserId"] = toUserID
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			steps := []map[string]any{}
			if flags.dryRun {
				steps = append(steps, map[string]any{"step": "set-ooo", "dry_run": true, "body": body})
			} else {
				raw, _, err := c.PostWithHeaders("/v2/me/ooo", body, nil)
				if err != nil {
					return fmt.Errorf("set OOO: %w", err)
				}
				steps = append(steps, map[string]any{"step": "set-ooo", "response": json.RawMessage(raw)})
			}

			return emitNovelJSON(cmd, flags, map[string]any{
				"command": "ooo set",
				"start":   start.UTC().Format(time.RFC3339),
				"end":     end.UTC().Format(time.RFC3339),
				"reason":  reason,
				"steps":   steps,
				"dry_run": flags.dryRun,
			})
		},
	}
	cmd.Flags().StringVar(&startStr, "start", "", "Start time (RFC3339, YYYY-MM-DD, 'today', 'tomorrow') — required")
	cmd.Flags().StringVar(&endStr, "end", "", "End time (RFC3339, YYYY-MM-DD, 'today', 'tomorrow') — required")
	cmd.Flags().StringVar(&reason, "reason", "vacation", "Reason: vacation, travel, sick, public_holiday, unspecified")
	cmd.Flags().StringVar(&notes, "notes", "", "Optional notes (e.g. 'Hawaii trip')")
	cmd.Flags().IntVar(&toUserID, "redirect-to-user", 0, "Forward bookings to this user ID during OOO (round-robin only)")
	cmd.Flags().StringVar(&tzName, "tz", "UTC", "Timezone for natural-language times")
	return cmd
}

func newOooListCmd(flags *rootFlags) *cobra.Command {
	var take int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your active and upcoming out-of-office entries",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  cal-com-pp-cli ooo list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"sortStart": "asc"}
			if take > 0 {
				params["take"] = strconv.Itoa(take)
			}
			raw, err := c.Get("/v2/me/ooo", params)
			if err != nil {
				return err
			}
			var env struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			return emitNovelJSON(cmd, flags, map[string]any{
				"command": "ooo list",
				"count":   len(env.Data),
				"entries": env.Data,
			})
		},
	}
	cmd.Flags().IntVar(&take, "take", 50, "Max entries to return")
	return cmd
}

func newOooDeleteCmd(flags *rootFlags) *cobra.Command {
	var oooID int
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Cancel an OOO entry by ID",
		Example: `  cal-com-pp-cli ooo delete --id 42 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if oooID == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/v2/me/ooo/%d", oooID)
			steps := []map[string]any{}
			if flags.dryRun {
				steps = append(steps, map[string]any{"step": "delete-ooo", "dry_run": true, "path": path})
			} else {
				raw, _, err := c.Delete(path)
				if err != nil {
					return fmt.Errorf("delete OOO %d: %w", oooID, err)
				}
				steps = append(steps, map[string]any{"step": "delete-ooo", "response": json.RawMessage(raw)})
			}
			return emitNovelJSON(cmd, flags, map[string]any{
				"command": "ooo delete",
				"id":      oooID,
				"steps":   steps,
				"dry_run": flags.dryRun,
			})
		},
	}
	cmd.Flags().IntVar(&oooID, "id", 0, "OOO entry ID (find via 'cal-com-pp-cli ooo list')")
	return cmd
}

// -----------------------------------------------------------------------------
// Shared small helpers
// -----------------------------------------------------------------------------

func windowCutoff(w string) time.Time {
	if w == "" || w == "all" {
		return time.Time{}
	}
	d := windowDays(w)
	if d == 0 {
		return time.Time{}
	}
	return time.Now().AddDate(0, 0, -d)
}

func windowDays(w string) int {
	if w == "" || w == "all" {
		return 365
	}
	w = strings.TrimSpace(w)
	if strings.HasSuffix(w, "d") {
		n, _ := strconv.Atoi(strings.TrimSuffix(w, "d"))
		return n
	}
	if strings.HasSuffix(w, "w") {
		n, _ := strconv.Atoi(strings.TrimSuffix(w, "w"))
		return n * 7
	}
	if strings.HasSuffix(w, "m") {
		n, _ := strconv.Atoi(strings.TrimSuffix(w, "m"))
		return n * 30
	}
	n, _ := strconv.Atoi(w)
	if n == 0 {
		return 30
	}
	return n
}

func groupKey(b map[string]any, by string) string {
	switch by {
	case "event-type", "eventtype", "event_type":
		switch v := b["eventTypeId"].(type) {
		case float64:
			return fmt.Sprintf("%d", int(v))
		case int:
			return fmt.Sprintf("%d", v)
		}
		return "(unknown)"
	case "attendee", "attendees":
		atts, _ := b["attendees"].([]any)
		if len(atts) > 0 {
			if m, ok := atts[0].(map[string]any); ok {
				if e, ok := m["email"].(string); ok {
					return e
				}
			}
		}
		return "(unknown)"
	case "weekday":
		startStr, _ := b["start"].(string)
		if t, err := parseAPITime(startStr); err == nil {
			return t.Weekday().String()
		}
		return "(unknown)"
	case "hour":
		startStr, _ := b["start"].(string)
		if t, err := parseAPITime(startStr); err == nil {
			return fmt.Sprintf("%02d:00", t.Hour())
		}
		return "(unknown)"
	case "status":
		s, _ := b["status"].(string)
		if s == "" {
			s = "(unknown)"
		}
		return s
	}
	return "total"
}

func sortedCounts(m map[string]int) []map[string]any {
	rows := make([]map[string]any, 0, len(m))
	for k, v := range m {
		rows = append(rows, map[string]any{"key": k, "count": v})
	}
	sort.Slice(rows, func(i, j int) bool {
		ci, _ := rows[i]["count"].(int)
		cj, _ := rows[j]["count"].(int)
		return ci > cj
	})
	return rows
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isNoShow(b map[string]any) bool {
	if v, ok := b["noShowHost"].(bool); ok && v {
		return true
	}
	if status, _ := b["status"].(string); strings.Contains(strings.ToLower(status), "no_show") || strings.Contains(strings.ToLower(status), "noshow") {
		return true
	}
	atts, _ := b["attendees"].([]any)
	for _, a := range atts {
		if m, ok := a.(map[string]any); ok {
			if v, ok := m["noShow"].(bool); ok && v {
				return true
			}
		}
	}
	return false
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func round3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}

// extractEventTypes normalizes the several shapes Cal.com returns from
// /v2/event-types: data.eventTypeGroups[].eventTypes[] (default), data:[]
// (some filter combinations), or data:{eventTypes:[]} (rare).
func extractEventTypes(raw []byte) []map[string]any {
	// Shape 1: data.eventTypeGroups[].eventTypes[]
	var grouped struct {
		Data struct {
			EventTypeGroups []struct {
				EventTypes []map[string]any `json:"eventTypes"`
			} `json:"eventTypeGroups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &grouped); err == nil && len(grouped.Data.EventTypeGroups) > 0 {
		out := []map[string]any{}
		for _, g := range grouped.Data.EventTypeGroups {
			out = append(out, g.EventTypes...)
		}
		if len(out) > 0 {
			return out
		}
	}
	// Shape 2: data:[]
	var array struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &array); err == nil && len(array.Data) > 0 {
		return array.Data
	}
	// Shape 3: data:{eventTypes:[]}
	var nested struct {
		Data struct {
			EventTypes []map[string]any `json:"eventTypes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &nested); err == nil && len(nested.Data.EventTypes) > 0 {
		return nested.Data.EventTypes
	}
	return nil
}

// parseAPITime accepts Cal.com's response timestamps which use either
// RFC3339 ("2026-05-01T14:00:00Z") or RFC3339 with fractional seconds
// ("2026-05-01T14:00:00.000Z"). RFC3339Nano covers both shapes.
func parseAPITime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp %q", s)
}

// silence "imported and not used" for url, kept for future needs
var _ = url.PathEscape
