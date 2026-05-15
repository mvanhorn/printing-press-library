// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// Hand-built v1.1 novel verb: schedule. A cron-safe wrapper over
// chat.scheduleMessage with --dry-run and verify-env short-circuits.

package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/cliutil"
)

func newScheduleCmd(flags *rootFlags) *cobra.Command {
	var channel string
	var text string
	var postAt string

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Schedule a message for future delivery (cron-safe chat.scheduleMessage)",
		Long: `Schedule a Slack message to be delivered at a future time. A cron-safe
wrapper over chat.scheduleMessage: --dry-run prints the request without
sending it, and the command short-circuits under the printing-press
verifier so a verify pass never schedules anything in a real workspace.

--post-at accepts a Unix epoch timestamp or an RFC3339 datetime
(2026-05-20T09:00:00Z).`,
		// Mutating verb — no mcp:read-only annotation.
		Example: strings.Trim(`
  # Schedule a digest for a specific epoch time
  slack-pp-cli schedule --channel "#csm" --text "weekly digest" --post-at 1747900800

  # Schedule with an RFC3339 datetime
  slack-pp-cli schedule --channel "#data" --text "KPI review" --post-at 2026-05-20T09:00:00Z

  # Preview without scheduling (cron safety)
  slack-pp-cli schedule --channel "#churnsales" --text "reminder" --post-at 1747900800 --dry-run
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if channel == "" && text == "" && postAt == "" {
				return cmd.Help()
			}
			if channel == "" {
				return usageErr(fmt.Errorf("--channel is required"))
			}
			if text == "" {
				return usageErr(fmt.Errorf("--text is required"))
			}
			if postAt == "" {
				return usageErr(fmt.Errorf("--post-at is required (Unix epoch or RFC3339 datetime)"))
			}

			epoch, err := parsePostAt(postAt)
			if err != nil {
				return usageErr(err)
			}

			fields := url.Values{}
			fields.Set("channel", channel)
			fields.Set("text", text)
			fields.Set("post_at", strconv.FormatInt(epoch, 10))

			if dryRunOK(flags) {
				printPostPreview(cmd, "schedule", "/chat.scheduleMessage", fields)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				printPostPreview(cmd, "schedule", "/chat.scheduleMessage", fields)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostForm("/chat.scheduleMessage", fields)
			if err != nil {
				if rl, ok := err.(*cliutil.RateLimitError); ok {
					return rateLimitErr(rl)
				}
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "", "Channel id or name to post to (e.g. #csm or C0123ABCD)")
	cmd.Flags().StringVar(&text, "text", "", "Message text")
	cmd.Flags().StringVar(&postAt, "post-at", "", "When to deliver: Unix epoch seconds or an RFC3339 datetime")
	return cmd
}

// parsePostAt accepts a Unix epoch (seconds) or an RFC3339 datetime and
// returns the epoch-seconds value. A future bound is not enforced here —
// Slack rejects past timestamps itself with a clearer error.
func parsePostAt(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("--post-at is empty")
	}
	if epoch, err := strconv.ParseInt(s, 10, 64); err == nil {
		if epoch <= 0 {
			return 0, fmt.Errorf("invalid --post-at %q: epoch must be positive", s)
		}
		return epoch, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("invalid --post-at %q: expected a Unix epoch or RFC3339 datetime (2026-05-20T09:00:00Z)", s)
}
