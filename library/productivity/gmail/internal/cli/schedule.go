// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature (user-vision): scheduled send queue. The Gmail API has no
// schedule-send endpoint — Gmail's own UI feature is unreachable
// programmatically — so this builds it from a local SQLite outbox plus a
// due-time processor. Idempotent by construction: items are claimed with an
// atomic pending->sending transition before any network call.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

func newNovelScheduleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Schedule emails to send later (the API feature Gmail never shipped)",
		Long: `Queue emails locally and send them at their due time. The Gmail API has
no schedule-send endpoint, so the queue lives in the local database and
"schedule run" delivers due items — invoke it from cron/launchd, or keep
it resident with --watch. Items are claimed atomically, so a message is
never sent twice even with overlapping runs.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newScheduleSendCmd(flags))
	cmd.AddCommand(newScheduleListCmd(flags))
	cmd.AddCommand(newScheduleCancelCmd(flags))
	cmd.AddCommand(newScheduleEditCmd(flags))
	cmd.AddCommand(newScheduleRunCmd(flags))
	return cmd
}

func newScheduleSendCmd(flags *rootFlags) *cobra.Command {
	var f sendFlags
	var at, in string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Queue an email for later delivery, at an absolute (--at) or relative (--in) time",
		Example: strings.Trim(`
  gmail-pp-cli schedule send --to alex@example.com --subject "Follow up" --body-file pitch.txt --at "2026-08-04 09:00"
  gmail-pp-cli schedule send --to alex@example.com --subject "Reminder" --body "Ping" --in 18h`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "--to=dest@example.com;--subject=Example;--body=Example body;--in=2h",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "queue an email for later delivery")
			}
			body, err := f.resolveBody()
			if err != nil {
				return usageErr(err)
			}
			if len(f.to) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is required"))
			}
			if strings.TrimSpace(body) == "" && strings.TrimSpace(f.htmlBody) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--body, --body-file, or --html-body is required"))
			}
			sendAt, err := parseSendAt(at, in, time.Now())
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			atts := "[]"
			if len(f.attach) > 0 {
				enc, err := json.Marshal(f.attach)
				if err != nil {
					return err
				}
				atts = string(enc)
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			id, err := db.CreateScheduledSend(store.ScheduledSend{
				To: strings.Join(f.to, ", "), Cc: strings.Join(f.cc, ", "), Bcc: strings.Join(f.bcc, ", "),
				Subject: f.subject, BodyText: body, BodyHTML: f.htmlBody,
				Attachments: json.RawMessage(atts),
				ThreadID:    f.threadID,
				SendAt:      sendAt,
			})
			if err != nil {
				return err
			}
			out := map[string]any{
				"id":      id,
				"send_at": sendAt.Format(time.RFC3339),
				"to":      strings.Join(f.to, ", "),
				"subject": f.subject,
				"status":  "pending",
				"note":    "run 'gmail-pp-cli schedule run' (cron) or 'schedule run --watch' to deliver due items",
			}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "queued #%d for %s (deliver with: gmail-pp-cli schedule run)\n", id, sendAt.Local().Format("2006-01-02 15:04 MST"))
			return nil
		},
	}
	addComposeFlags(cmd, &f)
	cmd.Flags().StringVar(&f.threadID, "thread-id", "", "Send inside an existing thread")
	cmd.Flags().StringVar(&at, "at", "", "Absolute delivery time (RFC3339 or \"2006-01-02 15:04\" local)")
	cmd.Flags().StringVar(&in, "in", "", "Relative delivery delay (e.g. 2h, 18h, 3d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}

func newScheduleListCmd(flags *rootFlags) *cobra.Command {
	var status string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List queued, sent, failed, and canceled scheduled emails",
		Example:     "  gmail-pp-cli schedule list\n  gmail-pp-cli schedule list --status pending --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list the scheduled-send queue")
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			items, err := db.ListScheduledSends(status, limit)
			if err != nil {
				return err
			}
			if wantsJSONOutput(cmd, flags) {
				if items == nil {
					items = []store.ScheduledSend{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), items, flags)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "schedule queue is empty")
				return nil
			}
			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "#%d  %-8s  %s  to=%s  %q\n",
					it.ID, it.Status, it.SendAt.Local().Format("2006-01-02 15:04"), it.To, truncateCell(it.Subject, 40))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: pending, sent, failed, canceled")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum items to list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}

func newScheduleCancelCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "cancel <id>",
		Short:       "Cancel a pending scheduled email",
		Example:     "  gmail-pp-cli schedule cancel 3",
		Annotations: map[string]string{"pp:happy-args": "id=1", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cancel a scheduled email")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a queue id argument is required"))
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid queue id %q", args[0]))
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			ok, err := db.CancelScheduledSend(id)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("item #%d is not pending (already sent, failed, canceled, or missing)", id)
			}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"id": id, "status": "canceled"}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "canceled #%d\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}

func newScheduleEditCmd(flags *rootFlags) *cobra.Command {
	var at, in string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "edit <id>",
		Short:       "Move a pending item's delivery time",
		Example:     "  gmail-pp-cli schedule edit 3 --at \"2026-08-05 08:30\"\n  gmail-pp-cli schedule edit 3 --in 4h",
		Annotations: map[string]string{"pp:happy-args": "id=1;--in=2h", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reschedule a queued email")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a queue id argument is required"))
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid queue id %q", args[0]))
			}
			sendAt, err := parseSendAt(at, in, time.Now())
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			ok, err := db.UpdateScheduledSendTime(id, sendAt)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("item #%d is not pending; only pending items can be rescheduled", id)
			}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"id": id, "send_at": sendAt.Format(time.RFC3339)}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rescheduled #%d for %s\n", id, sendAt.Local().Format("2006-01-02 15:04 MST"))
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "New absolute delivery time")
	cmd.Flags().StringVar(&in, "in", "", "New relative delivery delay (e.g. 4h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}

type scheduleRunResult struct {
	Sent     []int64 `json:"sent_ids"`
	Failed   []int64 `json:"failed_ids"`
	Requeued int     `json:"requeued,omitempty"`
	Pending  int     `json:"still_pending"`
}

// deliverDue processes every due item once. Shared by single-shot and --watch.
func deliverDue(cmd *cobra.Command, flags *rootFlags, db *store.Store) (scheduleRunResult, error) {
	res := scheduleRunResult{Sent: []int64{}, Failed: []int64{}}
	// Recover items whose previous run died mid-send before looking for work,
	// otherwise an interrupted attempt sits in 'sending' forever.
	if n, err := db.RequeueStaleClaims(time.Now()); err != nil {
		return res, err
	} else if n > 0 {
		res.Requeued = n
		fmt.Fprintf(cmd.ErrOrStderr(), "requeued %d interrupted send(s)\n", n)
	}
	due, err := db.DueScheduledSends(time.Now())
	if err != nil {
		return res, err
	}
	if len(due) == 0 {
		pending, _ := db.ListScheduledSends("pending", 1000)
		res.Pending = len(pending)
		return res, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return res, err
	}
	for _, item := range due {
		claimed, err := db.ClaimScheduledSend(item.ID)
		if err != nil {
			return res, err
		}
		if !claimed {
			continue // another runner got it first
		}
		var attach []string
		_ = json.Unmarshal(item.Attachments, &attach)
		compose := gmailmail.Compose{
			To:          splitAddressList(item.To),
			Cc:          splitAddressList(item.Cc),
			Bcc:         splitAddressList(item.Bcc),
			Subject:     item.Subject,
			Text:        item.BodyText,
			HTML:        item.BodyHTML,
			Attachments: attach,
			InReplyTo:   item.InReplyTo,
			References:  item.References,
		}
		raw, buildErr := compose.BuildRaw()
		if buildErr != nil {
			if finishErr := db.FinishScheduledSend(item.ID, "", buildErr); finishErr != nil {
				return res, finishErr
			}
			res.Failed = append(res.Failed, item.ID)
			continue
		}
		gmailID, sendErr := gmailSendRaw(cmd.Context(), c, raw, item.ThreadID)
		if finishErr := db.FinishScheduledSend(item.ID, gmailID, sendErr); finishErr != nil {
			return res, finishErr
		}
		if sendErr != nil {
			res.Failed = append(res.Failed, item.ID)
		} else {
			res.Sent = append(res.Sent, item.ID)
		}
	}
	pending, _ := db.ListScheduledSends("pending", 1000)
	res.Pending = len(pending)
	return res, nil
}

func newScheduleRunCmd(flags *rootFlags) *cobra.Command {
	var watch bool
	var interval string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Deliver due items (cron-friendly); --watch stays resident",
		Long: `Sends every queued email whose time has come, exactly once. Wire it to
cron or launchd for set-and-forget delivery:

  */5 * * * * gmail-pp-cli schedule run --quiet

or keep a resident worker with --watch.`,
		Example: "  gmail-pp-cli schedule run\n  gmail-pp-cli schedule run --watch --interval 60s",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "deliver due scheduled emails")
			}
			pollEvery, err := cliutil.ParseDurationLoose(interval)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --interval %q: %w", interval, err))
			}
			if pollEvery < 10*time.Second {
				pollEvery = 10 * time.Second
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would deliver due scheduled emails")
				return nil
			}
			if cliutil.IsDogfoodEnv() {
				watch = false
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			for {
				res, err := deliverDue(cmd, flags, db)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if !watch {
					if wantsJSONOutput(cmd, flags) {
						return printJSONFiltered(cmd.OutOrStdout(), res, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "sent %d, failed %d, %d still pending\n", len(res.Sent), len(res.Failed), res.Pending)
					return nil
				}
				if len(res.Sent) > 0 || len(res.Failed) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s sent=%d failed=%d pending=%d\n",
						time.Now().Format(time.RFC3339), len(res.Sent), len(res.Failed), res.Pending)
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(pollEvery):
				}
			}
		},
	}
	cmd.Flags().BoolVar(&watch, "watch", false, "Stay resident and poll for due items")
	cmd.Flags().StringVar(&interval, "interval", "30s", "Poll interval with --watch (min 10s)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}
