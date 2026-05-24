// Hand-authored novel feature: conflict-guarded create. Survives regen.
package cli

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/cliutil"
)

// exitConflict is the typed exit code `book` returns when --on-conflict abort
// hits an overlapping event. Distinct from the generic API failure codes so an
// agent can branch on "the slot was taken" versus "the request failed".
const exitConflict = 9

func newBookCmd(flags *rootFlags) *cobra.Command {
	var calendar, summary, startStr, endStr, description, location, attendeesCSV, onConflict, sendUpdates string
	cmd := &cobra.Command{
		Use:   "book",
		Short: "Create an event, aborting if it overlaps an existing one",
		Long: "Create an event with a built-in conflict guard. Before the create call, it checks the local event\n" +
			"store for overlaps on the target calendar; with --on-conflict abort (the default) it refuses and exits\n" +
			"with code 9, so an agent booking on a user's behalf never silently double-books.\n\n" +
			"--on-conflict warn books anyway after printing the clash; --on-conflict allow skips the check entirely.",
		Example: "  google-calendar-pp-cli book --summary 'Design review' --start 2026-05-25T15:00:00Z --end 2026-05-25T16:00:00Z --on-conflict abort",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,9",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// A bare `book --dry-run` (no inputs) is the verify probe: succeed quietly.
			if dryRunOK(flags) && summary == "" && startStr == "" && endStr == "" {
				return nil
			}
			if summary == "" || startStr == "" || endStr == "" {
				if flags == nil || !flags.asJSON {
					_ = cmd.Help()
				}
				return usageErr(fmt.Errorf("--summary, --start, and --end are required"))
			}
			switch onConflict {
			case "abort", "warn", "allow":
			default:
				return usageErr(fmt.Errorf("invalid --on-conflict %q: use abort, warn, or allow", onConflict))
			}
			start, startAllDay, err := parseBookBound(startStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --start %q: %w", startStr, err))
			}
			end, endAllDay, err := parseBookBound(endStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --end %q: %w", endStr, err))
			}
			if !end.After(start) {
				return usageErr(fmt.Errorf("--end must be after --start"))
			}
			cal := calendar
			if cal == "" {
				cal = "primary"
			}

			// Conflict guard (skipped for --on-conflict allow). Read-only; safe
			// under verify since the guard only loads events.
			if onConflict != "allow" {
				events, _, lerr := gcalLoadEvents(cmd, flags, eventQuery{calendars: []string{cal}, timeMin: start, timeMax: end})
				if lerr != nil {
					// The guard could not run. Under --on-conflict abort (the
					// default, used as a hard double-booking guard) we must NOT
					// book blindly — a silent fall-through here would defeat the
					// entire purpose of the command. Fail loudly and tell the
					// user how to override.
					if onConflict == "abort" {
						return apiErr(fmt.Errorf("conflict check failed, refusing to book (could double-book): %w\nhint: pass --on-conflict allow to book without checking, or --on-conflict warn to book and continue", lerr))
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: conflict check failed (%v); booking anyway (--on-conflict warn)\n", lerr)
				} else {
					var clashes []conflictEndpoint
					for _, ev := range events {
						if ev.Status == "cancelled" || ev.AllDay || ev.Transparency == "transparent" || ev.Start.IsZero() || ev.End.IsZero() {
							continue
						}
						if ev.Start.Before(end) && ev.End.After(start) {
							clashes = append(clashes, endpointOf(ev))
						}
					}
					if len(clashes) > 0 {
						if onConflict == "abort" {
							if flags != nil && flags.asJSON {
								_ = flags.printJSON(cmd, map[string]any{"status": "conflict", "conflicts": clashes})
							} else {
								fmt.Fprintf(cmd.ErrOrStderr(), "conflict: %d overlapping event(s); not booked (use --on-conflict warn|allow to override)\n", len(clashes))
							}
							return &cliError{code: exitConflict, err: fmt.Errorf("event overlaps %d existing event(s) on %s", len(clashes), cal)}
						}
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d overlapping event(s) on %s; booking anyway\n", len(clashes), cal)
					}
				}
			}

			body := map[string]any{"summary": summary}
			if startAllDay {
				body["start"] = map[string]string{"date": start.Format("2006-01-02")}
			} else {
				body["start"] = map[string]string{"dateTime": start.Format(time.RFC3339)}
			}
			if endAllDay {
				body["end"] = map[string]string{"date": end.Format("2006-01-02")}
			} else {
				body["end"] = map[string]string{"dateTime": end.Format(time.RFC3339)}
			}
			if description != "" {
				body["description"] = description
			}
			if location != "" {
				body["location"] = location
			}
			if attendeesCSV != "" {
				var att []map[string]string
				for _, e := range strings.Split(attendeesCSV, ",") {
					if e = strings.TrimSpace(e); e != "" {
						att = append(att, map[string]string{"email": e})
					}
				}
				if len(att) > 0 {
					body["attendees"] = att
				}
			}

			// Side-effect guard: never create during --dry-run or verify runs.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"status": "would_create", "calendar": cal, "event": body})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if sendUpdates != "" {
				params["sendUpdates"] = sendUpdates
			}
			path := "/calendars/" + url.PathEscape(cal) + "/events"
			data, _, err := c.PostWithParams(cmd.Context(), path, params, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&calendar, "calendar", "primary", "Calendar ID to create the event on")
	cmd.Flags().StringVar(&summary, "summary", "", "Event title (required)")
	cmd.Flags().StringVar(&startStr, "start", "", "Start time: RFC3339 (timed) or YYYY-MM-DD (all-day) (required)")
	cmd.Flags().StringVar(&endStr, "end", "", "End time: RFC3339 (timed) or YYYY-MM-DD (all-day) (required)")
	cmd.Flags().StringVar(&description, "description", "", "Event description")
	cmd.Flags().StringVar(&location, "location", "", "Event location")
	cmd.Flags().StringVar(&attendeesCSV, "attendees", "", "Comma-separated attendee emails")
	cmd.Flags().StringVar(&onConflict, "on-conflict", "abort", "On overlap: abort (exit 9), warn (book anyway), or allow (skip check)")
	cmd.Flags().StringVar(&sendUpdates, "send-updates", "", "Notify guests: all, externalOnly, or none")
	return cmd
}

// parseBookBound parses a --start/--end value: RFC3339 (timed) or YYYY-MM-DD (all-day).
func parseBookBound(s string) (time.Time, bool, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, nil
	}
	if isoDateRe.MatchString(s) {
		t, err := time.ParseInLocation("2006-01-02", s, time.Local)
		return t, true, err
	}
	return time.Time{}, false, fmt.Errorf("expected RFC3339 (2026-05-25T15:00:00Z) or YYYY-MM-DD")
}
