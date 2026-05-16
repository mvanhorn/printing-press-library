package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// newDraftsScheduleCmd schedules a draft for future release.
// Verified from Substack's editor JS bundle:
//   POST /api/v1/drafts/{id}/scheduled_release
//   body: { trigger_at, post_audience, email_audience }
func newDraftsScheduleCmd(flags *rootFlags) *cobra.Command {
	var (
		at            string
		postAudience  string
		emailAudience string
		noEmail       bool
	)
	cmd := &cobra.Command{
		Use:   "schedule <id>",
		Short: "Schedule a draft for automatic publication at a future time.",
		Long: `Schedule a draft to publish automatically at a future date/time via
POST /api/v1/drafts/{id}/scheduled_release.

The --at value accepts:
  - RFC3339:        2026-06-01T09:00:00Z
  - Date + time:    "2026-06-01 09:00"
  - Date only:      2026-06-01            (defaults to 09:00 local time)

post_audience controls who can read the published post (everyone | only_paid | only_founding).
email_audience controls who receives the email (only_free | everyone | none).
Use --no-email to schedule a web-only release with no email blast.

Requires --subdomain <publication-subdomain>.`,
		Example: `  # Schedule for a specific UTC instant
  substack-pp-cli drafts schedule 12345 --subdomain mypub --at 2026-06-01T09:00:00Z

  # Schedule, paid-only, no email
  substack-pp-cli drafts schedule 12345 --subdomain mypub --at "2026-06-01 09:00" \
    --post-audience only_paid --no-email

  # Preview without sending
  substack-pp-cli drafts schedule 12345 --subdomain mypub --at 2026-06-01 --dry-run`,
		Annotations: map[string]string{
			"pp:endpoint": "drafts.schedule",
			"pp:method":   "POST",
			"pp:path":     "/drafts/{id}/scheduled_release",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && at == "" {
				// verify probes with --dry-run and no flags
				return nil
			}
			if at == "" {
				return usageErr(fmt.Errorf("--at <datetime> is required"))
			}
			triggerAt, err := parseScheduleTime(at)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"trigger_at":    triggerAt.UTC().Format(time.RFC3339),
				"post_audience": postAudience,
			}
			if !noEmail {
				body["email_audience"] = emailAudience
			}

			path := "/drafts/" + args[0] + "/scheduled_release"
			resp, status, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				envelope := map[string]any{
					"action":       "schedule",
					"resource":     "drafts",
					"path":         path,
					"status":       status,
					"success":      status >= 200 && status < 300,
					"scheduled_at": triggerAt.UTC().Format(time.RFC3339),
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(resp) > 0 {
					var parsed any
					if json.Unmarshal(resp, &parsed) == nil {
						envelope["data"] = parsed
					}
				}
				out, _ := json.Marshal(envelope)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Draft %s scheduled for %s (audience=%s)\n",
				args[0], triggerAt.UTC().Format(time.RFC3339), postAudience)
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "Scheduled publish time (RFC3339, 'YYYY-MM-DD HH:MM', or 'YYYY-MM-DD')")
	cmd.Flags().StringVar(&postAudience, "post-audience", "everyone", "Who can read: everyone | only_paid | only_founding")
	cmd.Flags().StringVar(&emailAudience, "email-audience", "only_free", "Who gets the email: only_free | everyone | none")
	cmd.Flags().BoolVar(&noEmail, "no-email", false, "Schedule a web-only release with no email")
	return cmd
}

// parseScheduleTime accepts several human-friendly datetime formats and
// returns a time.Time. Date-only inputs default to 09:00 local time.
func parseScheduleTime(s string) (time.Time, error) {
	layouts := []struct {
		layout  string
		dateOnly bool
	}{
		{time.RFC3339, false},
		{"2006-01-02T15:04:05", false},
		{"2006-01-02 15:04:05", false},
		{"2006-01-02 15:04", false},
		{"2006-01-02", true},
	}
	for _, l := range layouts {
		t, err := time.Parse(l.layout, s)
		if err == nil {
			if l.dateOnly {
				// default to 09:00 local time
				t = time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, time.Local)
			}
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse --at %q (use RFC3339, 'YYYY-MM-DD HH:MM', or 'YYYY-MM-DD')", s)
}
