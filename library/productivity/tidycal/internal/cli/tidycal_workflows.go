// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type tidycalWindow struct {
	From     time.Time `json:"-"`
	To       time.Time `json:"-"`
	FromDate string    `json:"from"`
	ToDate   string    `json:"to"`
}

type bookingQuestion struct {
	Question string `json:"question,omitempty"`
	Answer   string `json:"answer,omitempty"`
}

type workflowBooking struct {
	ID            string            `json:"booking_id,omitempty"`
	BookingTypeID string            `json:"booking_type_id,omitempty"`
	StartsAt      string            `json:"starts_at,omitempty"`
	EndsAt        string            `json:"ends_at,omitempty"`
	Timezone      string            `json:"timezone,omitempty"`
	MeetingURL    string            `json:"meeting_url,omitempty"`
	MeetingID     string            `json:"meeting_id,omitempty"`
	CancelledAt   string            `json:"cancelled_at,omitempty"`
	ContactName   string            `json:"contact_name,omitempty"`
	ContactEmail  string            `json:"contact_email,omitempty"`
	ContactPhone  string            `json:"contact_phone,omitempty"`
	Questions     []bookingQuestion `json:"questions,omitempty"`
	Payment       map[string]any    `json:"payment,omitempty"`
	Raw           map[string]any    `json:"-"`
}

type tidycalSlot struct {
	StartsAt          string `json:"starts_at"`
	EndsAt            string `json:"ends_at,omitempty"`
	AvailableBookings int    `json:"available_bookings,omitempty"`
	Display           string `json:"display"`
	localStart        time.Time
}

type triageFinding struct {
	Code            string `json:"code"`
	Severity        string `json:"severity"`
	BookingID       string `json:"booking_id,omitempty"`
	ContactEmail    string `json:"contact_email,omitempty"`
	Summary         string `json:"summary"`
	SuggestedAction string `json:"suggested_action"`
}

type followupItem struct {
	BookingID           string            `json:"booking_id,omitempty"`
	ContactName         string            `json:"contact_name,omitempty"`
	ContactEmail        string            `json:"contact_email,omitempty"`
	BookingTypeID       string            `json:"booking_type_id,omitempty"`
	StartsAt            string            `json:"starts_at,omitempty"`
	EndsAt              string            `json:"ends_at,omitempty"`
	Timezone            string            `json:"timezone,omitempty"`
	MeetingURL          string            `json:"meeting_url,omitempty"`
	Questions           []bookingQuestion `json:"questions,omitempty"`
	Payment             map[string]any    `json:"payment,omitempty"`
	SuggestedReason     string            `json:"suggested_reason"`
	SuggestedNextAction string            `json:"suggested_next_action"`
}

func newBriefCmd(flags *rootFlags) *cobra.Command {
	opts := struct {
		date, from, to, timezone, format  string
		includeTeams, offline, syncBefore bool
	}{date: "today", format: "text"}
	cmd := &cobra.Command{
		Use:     "brief",
		Short:   "Produce a contact-aware schedule brief for a day or date range.",
		Example: "  tidycal-pp-cli brief --date today --format json\n  tidycal-pp-cli brief --from 2026-06-01 --to 2026-06-08 --include-teams",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format := workflowFormat(opts.format, flags)
			loc, err := workflowLocation(opts.timezone)
			if err != nil {
				return usageErr(err)
			}
			window, err := resolveWorkflowWindow(opts.date, opts.from, opts.to, loc)
			if err != nil {
				return usageErr(err)
			}
			bookings, err := fetchWorkflowBookings(cmd.Context(), flags, window, opts.includeTeams, false, opts.offline, opts.syncBefore, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			bookings = filterBookingsInWindow(bookings, window, loc, false)
			if format == "json" {
				return flags.printJSON(cmd, map[string]any{"timezone": loc.String(), "window": window, "bookings": bookings})
			}
			return printBriefText(cmd.OutOrStdout(), loc, window, bookings)
		},
	}
	cmd.Flags().StringVar(&opts.date, "date", opts.date, "Local day to brief, or empty when --from/--to are set")
	cmd.Flags().StringVar(&opts.from, "from", "", "Start date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().StringVar(&opts.to, "to", "", "End date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().BoolVar(&opts.includeTeams, "include-teams", false, "Include team bookings when supported by TidyCal")
	cmd.Flags().StringVar(&opts.timezone, "timezone", "", "IANA timezone for display (defaults to local timezone)")
	cmd.Flags().BoolVar(&opts.offline, "offline", false, "Read from the local store instead of the live API")
	cmd.Flags().BoolVar(&opts.syncBefore, "sync-before", false, "Refresh the viewed window from the API before rendering")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: text or json")
	return cmd
}

func newTriageCmd(flags *rootFlags) *cobra.Command {
	opts := struct {
		from, to, timezone, preferredHours, severity, format string
		includeTeams, cancelled, offline, syncBefore         bool
	}{from: "today", to: "+7d", preferredHours: "09:00-17:00", severity: "info", format: "text"}
	cmd := &cobra.Command{
		Use:     "triage",
		Short:   "Identify schedule and booking problems that need attention.",
		Example: "  tidycal-pp-cli triage --from today --to +7d --severity warning --format json\n  tidycal-pp-cli triage --preferred-hours 08:30-16:30 --include-teams",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format := workflowFormat(opts.format, flags)
			loc, err := workflowLocation(opts.timezone)
			if err != nil {
				return usageErr(err)
			}
			window, err := resolveWorkflowWindow("", opts.from, opts.to, loc)
			if err != nil {
				return usageErr(err)
			}
			startMin, endMin, err := parsePreferredHours(opts.preferredHours)
			if err != nil {
				return usageErr(err)
			}
			bookings, err := fetchWorkflowBookings(cmd.Context(), flags, window, opts.includeTeams, opts.cancelled, opts.offline, opts.syncBefore, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			bookings = filterBookingsInWindow(bookings, window, loc, opts.cancelled)
			findings := filterFindings(triageBookings(bookings, loc, startMin, endMin), opts.severity)
			if format == "json" {
				return flags.printJSON(cmd, map[string]any{"timezone": loc.String(), "window": window, "findings": findings})
			}
			return printTriageText(cmd.OutOrStdout(), findings)
		},
	}
	cmd.Flags().StringVar(&opts.from, "from", opts.from, "Start date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().StringVar(&opts.to, "to", opts.to, "End date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().BoolVar(&opts.includeTeams, "include-teams", false, "Include team bookings when supported by TidyCal")
	cmd.Flags().StringVar(&opts.timezone, "timezone", "", "IANA timezone for display (defaults to local timezone)")
	cmd.Flags().StringVar(&opts.preferredHours, "preferred-hours", opts.preferredHours, "Preferred local hours as HH:MM-HH:MM")
	cmd.Flags().BoolVar(&opts.cancelled, "cancelled", false, "Include cancelled bookings in the viewed window")
	cmd.Flags().StringVar(&opts.severity, "severity", opts.severity, "Minimum severity: info, warning, or critical")
	cmd.Flags().BoolVar(&opts.offline, "offline", false, "Read from the local store instead of the live API")
	cmd.Flags().BoolVar(&opts.syncBefore, "sync-before", false, "Refresh the viewed window from the API before triage")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: text or json")
	return cmd
}

func newProposeTimesCmd(flags *rootFlags) *cobra.Command {
	opts := struct {
		from, to, timezone, prefer, format string
		count                              int
		avoidWeekends                      bool
	}{from: "today", to: "+14d", prefer: "any", count: 3, format: "text"}
	cmd := &cobra.Command{
		Use:     "propose-times <booking-type-id>",
		Short:   "Find available timeslots and produce a shortlist.",
		Example: "  tidycal-pp-cli propose-times 123 --from today --to +14d --count 3 --format json\n  tidycal-pp-cli propose-times 123 --prefer morning --avoid-weekends",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format := workflowFormat(opts.format, flags)
			loc, err := workflowLocation(opts.timezone)
			if err != nil {
				return usageErr(err)
			}
			window, err := resolveWorkflowWindow("", opts.from, opts.to, loc)
			if err != nil {
				return usageErr(err)
			}
			slots, err := fetchWorkflowSlots(cmd.Context(), flags, args[0], window, loc, opts.prefer, opts.avoidWeekends, opts.count)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if format == "json" {
				return flags.printJSON(cmd, map[string]any{"booking_type_id": args[0], "timezone": loc.String(), "window": window, "slots": slots})
			}
			return printSlotsText(cmd.OutOrStdout(), slots)
		},
	}
	cmd.Flags().StringVar(&opts.from, "from", opts.from, "Start date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().StringVar(&opts.to, "to", opts.to, "End date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().StringVar(&opts.timezone, "timezone", "", "IANA timezone for display (defaults to local timezone)")
	cmd.Flags().IntVar(&opts.count, "count", opts.count, "Maximum number of slots to return")
	cmd.Flags().StringVar(&opts.prefer, "prefer", opts.prefer, "Slot preference: morning, afternoon, or any")
	cmd.Flags().BoolVar(&opts.avoidWeekends, "avoid-weekends", false, "Skip Saturday and Sunday slots")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: text or json")
	return cmd
}

func newFollowupsCmd(flags *rootFlags) *cobra.Command {
	opts := struct {
		from, to, timezone, format                          string
		includeTeams, excludeCancelled, offline, syncBefore bool
	}{from: "-7d", to: "today", excludeCancelled: true, format: "text"}
	cmd := &cobra.Command{
		Use:     "followups",
		Short:   "Create an AI-ready follow-up queue from recent bookings.",
		Example: "  tidycal-pp-cli followups --from -7d --to today --format json\n  tidycal-pp-cli followups --exclude-cancelled --format csv",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format := workflowFormat(opts.format, flags)
			loc, err := workflowLocation(opts.timezone)
			if err != nil {
				return usageErr(err)
			}
			window, err := resolveWorkflowWindow("", opts.from, opts.to, loc)
			if err != nil {
				return usageErr(err)
			}
			bookings, err := fetchWorkflowBookings(cmd.Context(), flags, window, opts.includeTeams, !opts.excludeCancelled, opts.offline, opts.syncBefore, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			bookings = filterBookingsInWindow(bookings, window, loc, !opts.excludeCancelled)
			items := buildFollowups(bookings)
			switch format {
			case "json":
				return flags.printJSON(cmd, map[string]any{"timezone": loc.String(), "window": window, "followups": items})
			case "csv":
				return printFollowupsCSV(cmd.OutOrStdout(), items)
			default:
				return printFollowupsText(cmd.OutOrStdout(), items)
			}
		},
	}
	cmd.Flags().StringVar(&opts.from, "from", opts.from, "Start date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().StringVar(&opts.to, "to", opts.to, "End date (YYYY-MM-DD, today, +Nd, or -Nd)")
	cmd.Flags().BoolVar(&opts.includeTeams, "include-teams", false, "Include team bookings when supported by TidyCal")
	cmd.Flags().BoolVar(&opts.excludeCancelled, "exclude-cancelled", true, "Exclude cancelled bookings from the queue")
	cmd.Flags().StringVar(&opts.timezone, "timezone", "", "IANA timezone for display (defaults to local timezone)")
	cmd.Flags().BoolVar(&opts.offline, "offline", false, "Read from the local store instead of the live API")
	cmd.Flags().BoolVar(&opts.syncBefore, "sync-before", false, "Refresh the viewed window from the API before queueing")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: text, json, or csv")
	return cmd
}

func newAssistedBookCmd(flags *rootFlags) *cobra.Command {
	opts := struct {
		name, email, phone, timezone, slot, from, to, prefer, format string
		questions                                                    []string
		confirm                                                      bool
	}{from: "today", to: "+14d", prefer: "any", format: "text"}
	cmd := &cobra.Command{
		Use:     "assisted-book <booking-type-id>",
		Short:   "Book on behalf of a contact after an inspectable confirmation step.",
		Example: "  tidycal-pp-cli assisted-book 123 --name 'Ada Lovelace' --email ada@example.com --slot 2026-06-02T15:00:00Z --dry-run --format json\n  tidycal-pp-cli assisted-book 123 --name 'Ada Lovelace' --email ada@example.com --slot 2026-06-02T15:00:00Z --confirm",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := workflowLocation(opts.timezone)
			if err != nil {
				return usageErr(err)
			}
			if strings.TrimSpace(opts.name) == "" || strings.TrimSpace(opts.email) == "" || !strings.Contains(opts.email, "@") {
				return usageErr(fmt.Errorf("--name and a valid --email are required"))
			}
			if opts.timezone == "" {
				opts.timezone = loc.String()
			}
			selected := opts.slot
			var searched *tidycalWindow
			if selected == "" {
				window, err := resolveWorkflowWindow("", opts.from, opts.to, loc)
				if err != nil {
					return usageErr(err)
				}
				searched = &window
				slots, err := fetchWorkflowSlots(cmd.Context(), flags, args[0], window, loc, opts.prefer, false, 1)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if len(slots) == 0 {
					return fmt.Errorf("no available slots found in %s to %s", window.FromDate, window.ToDate)
				}
				selected = slots[0].StartsAt
			}
			bookingQuestions, err := parseQuestionPairs(opts.questions)
			if err != nil {
				return usageErr(err)
			}
			payload := map[string]any{"name": opts.name, "email": opts.email, "timezone": opts.timezone, "starts_at": selected}
			if opts.phone != "" {
				payload["phone"] = opts.phone
			}
			if len(bookingQuestions) > 0 {
				payload["booking_questions"] = bookingQuestions
			}
			if flags.dryRun || !opts.confirm {
				result := map[string]any{"booking_type_id": args[0], "payload": payload, "would_create": opts.confirm && !flags.dryRun, "requires_confirm": !opts.confirm}
				if searched != nil {
					result["searched_window"] = searched
				}
				if workflowFormat(opts.format, flags) == "json" {
					return flags.printJSON(cmd, result)
				}
				return printAssistedBookPreview(cmd.OutOrStdout(), result)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/booking-types/{bookingType}/bookings"
			path = replacePathParam(path, "bookingType", args[0])
			data, statusCode, err := c.PostWithParams(cmd.Context(), path, nil, payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			writeMutationResponseToStore(cmd.Context(), "bookings", data, "")
			if workflowFormat(opts.format, flags) == "json" {
				var parsed any
				_ = json.Unmarshal(data, &parsed)
				return flags.printJSON(cmd, map[string]any{"status": statusCode, "success": statusCode >= 200 && statusCode < 300, "data": parsed})
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&opts.name, "name", "", "Contact name")
	cmd.Flags().StringVar(&opts.email, "email", "", "Contact email")
	cmd.Flags().StringVar(&opts.phone, "phone", "", "Contact phone")
	cmd.Flags().StringVar(&opts.timezone, "timezone", "", "IANA timezone for the booking")
	cmd.Flags().StringVar(&opts.slot, "slot", "", "Exact slot timestamp to book")
	cmd.Flags().StringVar(&opts.from, "from", opts.from, "Start date when selecting a slot")
	cmd.Flags().StringVar(&opts.to, "to", opts.to, "End date when selecting a slot")
	cmd.Flags().StringVar(&opts.prefer, "prefer", opts.prefer, "Slot preference: morning, afternoon, or any")
	cmd.Flags().StringArrayVar(&opts.questions, "question", nil, "Intake answer as question=answer; may be repeated")
	cmd.Flags().BoolVar(&opts.confirm, "confirm", false, "Create the booking after showing the payload")
	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format: text or json")
	return cmd
}

func workflowFormat(format string, flags *rootFlags) string {
	if flags != nil && flags.asJSON {
		return "json"
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return "text"
	}
	return format
}

func workflowLocation(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		return time.Local, nil
	}
	return time.LoadLocation(name)
}

func resolveWorkflowWindow(dateExpr, fromExpr, toExpr string, loc *time.Location) (tidycalWindow, error) {
	now := time.Now().In(loc)
	if strings.TrimSpace(fromExpr) == "" && strings.TrimSpace(toExpr) == "" {
		if strings.TrimSpace(dateExpr) == "" {
			dateExpr = "today"
		}
		day, err := parseWorkflowDate(dateExpr, now, loc)
		if err != nil {
			return tidycalWindow{}, err
		}
		start := dayStart(day, loc)
		end := start.AddDate(0, 0, 1)
		return workflowWindow(start, end), nil
	}
	if strings.TrimSpace(fromExpr) == "" {
		fromExpr = "today"
	}
	if strings.TrimSpace(toExpr) == "" {
		toExpr = fromExpr
	}
	from, err := parseWorkflowDate(fromExpr, now, loc)
	if err != nil {
		return tidycalWindow{}, err
	}
	to, err := parseWorkflowDate(toExpr, now, loc)
	if err != nil {
		return tidycalWindow{}, err
	}
	start := dayStart(from, loc)
	end := dayStart(to, loc)
	if end.Before(start) {
		return tidycalWindow{}, fmt.Errorf("--to %q resolves to a date before --from %q; the range appears to be reversed", toExpr, fromExpr)
	}
	if toExpr != fromExpr {
		end = end.AddDate(0, 0, 1)
	}
	if !end.After(start) {
		end = end.AddDate(0, 0, 1)
	}
	return workflowWindow(start, end), nil
}

func workflowWindow(start, end time.Time) tidycalWindow {
	return tidycalWindow{
		From:     start,
		To:       end,
		FromDate: start.Format("2006-01-02"),
		ToDate:   end.Add(-time.Nanosecond).Format("2006-01-02"),
	}
}

func parseWorkflowDate(expr string, now time.Time, loc *time.Location) (time.Time, error) {
	expr = strings.TrimSpace(strings.ToLower(expr))
	switch expr {
	case "", "today":
		return now, nil
	case "tomorrow":
		return now.AddDate(0, 0, 1), nil
	case "yesterday":
		return now.AddDate(0, 0, -1), nil
	}
	if strings.HasSuffix(expr, "d") && (strings.HasPrefix(expr, "+") || strings.HasPrefix(expr, "-")) {
		days, err := strconv.Atoi(strings.TrimSuffix(expr, "d"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid relative date %q", expr)
		}
		return now.AddDate(0, 0, days), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", expr, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q; use YYYY-MM-DD, today, +Nd, or -Nd", expr)
	}
	return parsed, nil
}

func dayStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func fetchWorkflowBookings(ctx context.Context, flags *rootFlags, window tidycalWindow, includeTeams, cancelled, offline, syncBefore bool, hintWriter io.Writer) ([]workflowBooking, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"starts_at": window.From.UTC().Format("2006-01-02T15:04:05Z"),
		"ends_at":   window.To.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if includeTeams {
		params["include_teams"] = "true"
	}
	if cancelled {
		params["cancelled"] = "true"
	}
	strategy := "auto"
	if offline {
		strategy = "local"
	} else if syncBefore {
		strategy = "live"
	}
	data, _, err := resolvePaginatedReadWithStrategy(ctx, c, flags, strategy, "bookings", "/bookings", params, nil, false, "page", "page", "", "", "", hintWriter)
	if err != nil {
		return nil, err
	}
	return parseWorkflowBookings(data), nil
}

func fetchWorkflowSlots(ctx context.Context, flags *rootFlags, bookingTypeID string, window tidycalWindow, loc *time.Location, prefer string, avoidWeekends bool, count int) ([]tidycalSlot, error) {
	if count < 1 {
		count = 1
	}
	if prefer != "any" && prefer != "morning" && prefer != "afternoon" {
		return nil, usageErr(fmt.Errorf("--prefer must be morning, afternoon, or any"))
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := "/booking-types/{bookingType}/timeslots"
	path = replacePathParam(path, "bookingType", bookingTypeID)
	data, err := c.GetWithHeaders(ctx, path, map[string]string{
		"starts_at": workflowAPIDateTime(window.From),
		"ends_at":   workflowAPIDateTime(window.To),
	}, nil)
	if err != nil {
		return nil, err
	}
	slots := parseWorkflowSlots(data, loc)
	filtered := filterSlotsInWindow(slots, window, avoidWeekends)
	rankSlots(filtered, prefer)
	if len(filtered) > count {
		filtered = filtered[:count]
	}
	return filtered, nil
}

func workflowAPIDateTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func filterSlotsInWindow(slots []tidycalSlot, window tidycalWindow, avoidWeekends bool) []tidycalSlot {
	filtered := slots[:0]
	for _, slot := range slots {
		if slot.localStart.Before(window.From) || !slot.localStart.Before(window.To) {
			continue
		}
		if avoidWeekends && (slot.localStart.Weekday() == time.Saturday || slot.localStart.Weekday() == time.Sunday) {
			continue
		}
		filtered = append(filtered, slot)
	}
	return filtered
}

func parseWorkflowBookings(data json.RawMessage) []workflowBooking {
	items := unwrapArray(data)
	bookings := make([]workflowBooking, 0, len(items))
	for _, raw := range items {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		bookings = append(bookings, workflowBooking{
			ID:            stringValue(obj["id"]),
			BookingTypeID: stringValue(obj["booking_type_id"]),
			StartsAt:      stringValue(obj["starts_at"]),
			EndsAt:        stringValue(obj["ends_at"]),
			Timezone:      stringValue(obj["timezone"]),
			MeetingURL:    stringValue(obj["meeting_url"]),
			MeetingID:     stringValue(obj["meeting_id"]),
			CancelledAt:   stringValue(obj["cancelled_at"]),
			ContactName:   nestedString(obj, "contact", "name"),
			ContactEmail:  nestedString(obj, "contact", "email"),
			ContactPhone:  nestedString(obj, "contact", "phone_number"),
			Questions:     parseQuestions(obj["questions"]),
			Payment:       nestedMap(obj["payment"]),
			Raw:           obj,
		})
	}
	return bookings
}

func parseWorkflowSlots(data json.RawMessage, loc *time.Location) []tidycalSlot {
	items := unwrapArray(data)
	slots := make([]tidycalSlot, 0, len(items))
	for _, raw := range items {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		starts := stringValue(obj["starts_at"])
		startTime, ok := parseAPITime(starts)
		if !ok {
			continue
		}
		local := startTime.In(loc)
		slots = append(slots, tidycalSlot{
			StartsAt:          starts,
			EndsAt:            stringValue(obj["ends_at"]),
			AvailableBookings: intValue(obj["available_bookings"]),
			Display:           local.Format("Mon Jan 2, 3:04 PM ") + loc.String(),
			localStart:        local,
		})
	}
	return slots
}

func unwrapArray(data json.RawMessage) []json.RawMessage {
	var direct []json.RawMessage
	if json.Unmarshal(data, &direct) == nil {
		return direct
	}
	var env map[string]json.RawMessage
	if json.Unmarshal(data, &env) == nil {
		for _, key := range []string{"data", "items", "results"} {
			if raw, ok := env[key]; ok && json.Unmarshal(raw, &direct) == nil {
				return direct
			}
		}
	}
	return nil
}

func filterBookingsInWindow(bookings []workflowBooking, window tidycalWindow, loc *time.Location, includeCancelled bool) []workflowBooking {
	filtered := bookings[:0]
	for _, booking := range bookings {
		if !includeCancelled && booking.CancelledAt != "" {
			continue
		}
		start, ok := parseAPITime(booking.StartsAt)
		if !ok {
			continue
		}
		local := start.In(loc)
		if local.Before(window.From) || !local.Before(window.To) {
			continue
		}
		filtered = append(filtered, booking)
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].StartsAt < filtered[j].StartsAt })
	return filtered
}

func triageBookings(bookings []workflowBooking, loc *time.Location, preferredStart, preferredEnd int) []triageFinding {
	var findings []triageFinding
	seenContactDay := map[string]string{}
	now := time.Now()
	for _, b := range bookings {
		activeFuture := b.CancelledAt == ""
		if start, ok := parseAPITime(b.StartsAt); ok {
			activeFuture = activeFuture && start.After(now)
			mins := start.In(loc).Hour()*60 + start.In(loc).Minute()
			if mins < preferredStart || mins >= preferredEnd {
				findings = append(findings, finding("outside_preferred_hours", "info", b, "Booking is outside preferred hours.", "Review whether this time needs preparation or rescheduling."))
			}
			key := strings.ToLower(b.ContactEmail) + "|" + start.In(loc).Format("2006-01-02")
			if b.ContactEmail != "" {
				if prev := seenContactDay[key]; prev != "" {
					findings = append(findings, finding("duplicate_contact_same_day", "warning", b, "Same contact has multiple active bookings on the same day.", "Compare with booking "+prev+" before taking action."))
				} else if b.CancelledAt == "" {
					seenContactDay[key] = b.ID
				}
			}
		}
		if activeFuture && b.MeetingURL == "" {
			findings = append(findings, finding("missing_meeting_url", "warning", b, "Future active booking has no meeting URL.", "Add or confirm the meeting location before the booking starts."))
		}
		if b.CancelledAt != "" {
			findings = append(findings, finding("cancelled_in_window", "info", b, "Cancelled booking is still inside the viewed window.", "Confirm no follow-up or replacement booking is needed."))
		}
		if b.ContactEmail == "" {
			findings = append(findings, finding("missing_contact_email", "critical", b, "Booking contact is missing an email address.", "Resolve the contact before workflows that require email."))
		}
		if b.ContactName == "" {
			findings = append(findings, finding("missing_contact_name", "warning", b, "Booking contact is missing a name.", "Update the contact before sending personalized follow-up."))
		}
		for _, q := range b.Questions {
			if q.Question != "" && strings.TrimSpace(q.Answer) == "" {
				findings = append(findings, finding("empty_intake_answer", "info", b, "Intake question has an empty answer.", "Review whether the missing answer affects preparation."))
			}
		}
		if len(b.Payment) > 0 && stringValue(b.Payment["payment_id"]) == "" {
			findings = append(findings, finding("missing_payment_data", "warning", b, "Payment object is present but missing payment_id.", "Confirm payment status in TidyCal before treating this as paid."))
		}
	}
	return findings
}

func finding(code, severity string, b workflowBooking, summary, action string) triageFinding {
	return triageFinding{Code: code, Severity: severity, BookingID: b.ID, ContactEmail: b.ContactEmail, Summary: summary, SuggestedAction: action}
}

func filterFindings(findings []triageFinding, minSeverity string) []triageFinding {
	min := severityRank(minSeverity)
	filtered := findings[:0]
	for _, f := range findings {
		if severityRank(f.Severity) >= min {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 3
	case "warning":
		return 2
	default:
		return 1
	}
}

func rankSlots(slots []tidycalSlot, prefer string) {
	sort.SliceStable(slots, func(i, j int) bool {
		if prefer == "morning" || prefer == "afternoon" {
			iPreferred := slotMatchesPreference(slots[i], prefer)
			jPreferred := slotMatchesPreference(slots[j], prefer)
			if iPreferred != jPreferred {
				return iPreferred
			}
		}
		return slots[i].localStart.Before(slots[j].localStart)
	})
}

func slotMatchesPreference(slot tidycalSlot, prefer string) bool {
	hour := slot.localStart.Hour()
	if prefer == "morning" {
		return hour < 12
	}
	if prefer == "afternoon" {
		return hour >= 12
	}
	return true
}

func buildFollowups(bookings []workflowBooking) []followupItem {
	items := make([]followupItem, 0, len(bookings))
	for _, b := range bookings {
		reason := "recent_meeting"
		if b.MeetingURL == "" {
			reason = "missing_meeting_url"
		}
		if b.CancelledAt != "" {
			reason = "cancelled_booking"
		}
		if len(b.Payment) > 0 && reason == "recent_meeting" {
			reason = "paid_booking"
		}
		for _, q := range b.Questions {
			if reason != "cancelled_booking" && strings.Contains(strings.ToLower(q.Answer), "follow") {
				reason = "intake_answer_mentions_followup"
			}
		}
		items = append(items, followupItem{BookingID: b.ID, ContactName: b.ContactName, ContactEmail: b.ContactEmail, BookingTypeID: b.BookingTypeID, StartsAt: b.StartsAt, EndsAt: b.EndsAt, Timezone: b.Timezone, MeetingURL: b.MeetingURL, Questions: b.Questions, Payment: b.Payment, SuggestedReason: reason, SuggestedNextAction: "Draft a follow-up note; do not send automatically."})
	}
	return items
}

func parsePreferredHours(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("--preferred-hours must look like 09:00-17:00")
	}
	start, err := parseClockMinutes(parts[0])
	if err != nil {
		return 0, 0, err
	}
	end, err := parseClockMinutes(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if end <= start {
		return 0, 0, fmt.Errorf("--preferred-hours end must be after start")
	}
	return start, end, nil
}

func parseClockMinutes(value string) (int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid time %q; use HH:MM", value)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func parseQuestionPairs(values []string) ([]map[string]string, error) {
	var out []map[string]string
	for _, value := range values {
		q, a, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(q) == "" {
			return nil, fmt.Errorf("--question must be question=answer")
		}
		out = append(out, map[string]string{"question": strings.TrimSpace(q), "answer": strings.TrimSpace(a)})
	}
	return out, nil
}

func parseQuestions(value any) []bookingQuestion {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	questions := make([]bookingQuestion, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		questions = append(questions, bookingQuestion{Question: stringValue(m["question"]), Answer: stringValue(m["answer"])})
	}
	return questions
}

func parseAPITime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func stringValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

func intValue(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func nestedString(obj map[string]any, key, field string) string {
	return stringValue(nestedMap(obj[key])[field])
}

func nestedMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func printBriefText(w io.Writer, loc *time.Location, window tidycalWindow, bookings []workflowBooking) error {
	fmt.Fprintf(w, "Schedule brief: %s to %s (%s)\n", window.FromDate, window.ToDate, loc.String())
	if len(bookings) == 0 {
		fmt.Fprintln(w, "No bookings found.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "START\tEND\tCONTACT\tEMAIL\tMEETING")
	for _, b := range bookings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", displayTime(b.StartsAt, loc), displayTime(b.EndsAt, loc), b.ContactName, b.ContactEmail, valueOrWarning(b.MeetingURL, "missing meeting URL"))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(w, "Prep notes:")
	for _, b := range bookings {
		fmt.Fprintf(w, "- %s: %s", valueOrWarning(b.ContactName, "unknown contact"), valueOrWarning(b.MeetingURL, "confirm meeting URL"))
		if len(b.Questions) > 0 {
			fmt.Fprint(w, "; intake answers available")
		}
		fmt.Fprintln(w)
	}
	return nil
}

func printTriageText(w io.Writer, findings []triageFinding) error {
	if len(findings) == 0 {
		fmt.Fprintln(w, "No triage findings.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tCODE\tBOOKING\tSUMMARY\tACTION")
	for _, f := range findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.Severity, f.Code, f.BookingID, f.Summary, f.SuggestedAction)
	}
	return tw.Flush()
}

func printSlotsText(w io.Writer, slots []tidycalSlot) error {
	if len(slots) == 0 {
		fmt.Fprintln(w, "No available times found in the searched window.")
		return nil
	}
	fmt.Fprintf(w, "Here are %s available times:\n", numberWord(len(slots)))
	for _, slot := range slots {
		fmt.Fprintf(w, "- %s\n", slot.Display)
	}
	return nil
}

func printFollowupsText(w io.Writer, items []followupItem) error {
	if len(items) == 0 {
		fmt.Fprintln(w, "No follow-ups found.")
		return nil
	}
	for _, item := range items {
		fmt.Fprintf(w, "- %s <%s> booking %s: %s; %s\n", item.ContactName, item.ContactEmail, item.BookingID, item.SuggestedReason, item.SuggestedNextAction)
	}
	return nil
}

func printFollowupsCSV(w io.Writer, items []followupItem) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"booking_id", "contact_name", "contact_email", "booking_type_id", "starts_at", "ends_at", "timezone", "meeting_url", "suggested_reason", "suggested_next_action"})
	for _, item := range items {
		_ = cw.Write([]string{item.BookingID, item.ContactName, item.ContactEmail, item.BookingTypeID, item.StartsAt, item.EndsAt, item.Timezone, item.MeetingURL, item.SuggestedReason, item.SuggestedNextAction})
	}
	cw.Flush()
	return cw.Error()
}

func printAssistedBookPreview(w io.Writer, result map[string]any) error {
	data, _ := json.MarshalIndent(result["payload"], "", "  ")
	fmt.Fprintln(w, "Booking payload preview:")
	fmt.Fprintln(w, string(data))
	if requires, _ := result["requires_confirm"].(bool); requires {
		fmt.Fprintln(w, "No booking was created. Re-run with --confirm to create it.")
	}
	return nil
}

func displayTime(value string, loc *time.Location) string {
	if parsed, ok := parseAPITime(value); ok {
		return parsed.In(loc).Format("Jan 2 15:04")
	}
	return value
}

func valueOrWarning(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func numberWord(n int) string {
	switch n {
	case 1:
		return "one"
	case 2:
		return "two"
	case 3:
		return "three"
	default:
		return strconv.Itoa(n)
	}
}
