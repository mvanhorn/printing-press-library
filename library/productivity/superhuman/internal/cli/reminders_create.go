// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(reminders-endpoint-rewrite): the generator wired this command to
// POST /reminders/create with a flat snake_case body. That endpoint
// returns 400 "missing threadId" on every body variant against ingested
// threads (see .manuscripts/20260515-165115/discovery/reminders-sniff-report.md).
// The Superhuman web client snoozes threads by writing to a path under
// /v3/userdata.write -- the same generic write surface drafts_discard
// and send.go use. This file is hand-maintained, not generator output;
// see .printing-press-patches.json for the manifest entry.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// reminderConditionAlways and reminderConditionIfNoReply are the values the
// CLI accepts on --condition. They map to keepOnReply on the wire:
//   - always       -> keepOnReply: true  (reminder fires regardless of reply)
//   - if-no-reply  -> keepOnReply: false (reminder is cancelled on reply)
// The wire shape was recovered by reading an existing reminder via
// /v3/userdata.read on a thread snoozed from the web client; see
// .manuscripts/20260515-165115/discovery/reminders-sniff-report.md.
const (
	reminderConditionAlways    = "always"
	reminderConditionIfNoReply = "if-no-reply"
)

func newRemindersCreateCmd(flags *rootFlags) *cobra.Command {
	var bodyThreadId string
	var bodyTriggerAt string
	var bodyCondition string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a snooze reminder for a thread",
		Long: `Snooze a thread until --trigger-at via /v3/userdata.write.

--trigger-at accepts RFC3339 (e.g. 2026-05-16T09:00:00Z) or unix
milliseconds (e.g. 1778900000000). --condition defaults to "always"
(reminder fires unconditionally); pass "if-no-reply" to skip the
reminder if a reply arrives first.`,
		Example: "  superhuman-pp-cli reminders create --thread-id 19e2dc46a8b281fe --trigger-at 2026-05-16T09:00:00Z\n  superhuman-pp-cli reminders create --thread-id 19e2dc46a8b281fe --trigger-at 2026-05-16T09:00:00Z --condition if-no-reply",
		Annotations: map[string]string{
			"pp:endpoint": "reminders.create",
			"pp:method":   "POST",
			"pp:path":     "/v3/userdata.write",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("thread-id") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "thread-id")
				}
				if !cmd.Flags().Changed("trigger-at") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "trigger-at")
				}
				if bodyCondition != reminderConditionAlways && bodyCondition != reminderConditionIfNoReply {
					return usageErr(fmt.Errorf("reminders create: --condition must be %q or %q, got %q", reminderConditionAlways, reminderConditionIfNoReply, bodyCondition))
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			const path = "/v3/userdata.write"
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				providerID, perr := resolveProviderID(flags)
				if perr != nil {
					return authErr(fmt.Errorf("reminders create: %w", perr))
				}
				triggerAt, terr := parseTriggerAtISO(bodyTriggerAt)
				if terr != nil {
					return usageErr(fmt.Errorf("reminders create: invalid --trigger-at %q: %w", bodyTriggerAt, terr))
				}
				// Step 1: read the thread so we can populate messageIds on
				// the reminder value. Empty/missing messageIds causes the
				// backend's reminder validator to reject the write with 400.
				// Skipped under --dry-run so no network call fires; the
				// preview body carries a placeholder string in place of the
				// real message ids.
				var messageIDs []string
				if flags.dryRun {
					messageIDs = []string{"<dry-run: messageIds resolved from /v3/userdata.read in live mode>"}
				} else {
					readBody := map[string]any{
						"reads": []map[string]any{
							{"path": fmt.Sprintf("users/%s/threads/%s", providerID, bodyThreadId)},
						},
					}
					readData, _, rerr := c.Post("/v3/userdata.read", readBody)
					if rerr != nil {
						return classifyAPIError(fmt.Errorf("reminders create: read thread: %w", rerr), flags)
					}
					ids, mErr := extractThreadMessageIDs(readData)
					if mErr != nil {
						return apiErr(fmt.Errorf("reminders create: extract messageIds from thread %s: %w", bodyThreadId, mErr))
					}
					messageIDs = ids
				}
				body = map[string]any{
					"writes": []map[string]any{
						{
							"path": fmt.Sprintf("users/%s/threads/%s/reminder", providerID, bodyThreadId),
							"value": map[string]any{
								"clientCreatedAt": time.Now().UTC().Format(superhumanReminderTimeFormat),
								"keepOnReply":     bodyCondition == reminderConditionAlways,
								"messageIds":      messageIDs,
								"onDesktop":       false,
								"reminderId":      uuid.NewString(),
								"source":          "USER",
								"threadId":        bodyThreadId,
								"triggerAt":       triggerAt,
							},
						},
					},
				}
			}

			// PATCH: --dry-run must not fire the userdata.write. The
			// envelope at line ~174 only decorates the response with
			// `dry_run: true` AFTER the write has already executed,
			// which was greptile P1 on PR #595. Skip the actual POST
			// when dry-run is set and synthesize a planned-payload
			// response so the user can preview the operation without
			// snoozing the thread.
			var data []byte
			var statusCode int
			if flags.dryRun {
				preview := map[string]any{
					"dry_run":          true,
					"path":             path,
					"method":           "POST",
					"planned_body":     body,
					"would_send_write": true,
				}
				data, _ = json.Marshal(preview)
				statusCode = 0
			} else {
				var err error
				data, statusCode, err = c.Post(path, body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					return nil
				}
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "reminders",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyThreadId, "thread-id", "", "Thread to snooze")
	cmd.Flags().StringVar(&bodyTriggerAt, "trigger-at", "", "When the reminder should fire: RFC3339 (2026-05-16T09:00:00Z) or unix milliseconds (1778900000000)")
	cmd.Flags().StringVar(&bodyCondition, "condition", reminderConditionAlways, "Fire condition: \"always\" or \"if-no-reply\"")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin (raw /v3/userdata.write payload)")

	return cmd
}

// superhumanReminderTimeFormat is the ISO-with-nanos format the backend
// uses on the wire (e.g. "2026-05-15T23:50:00.000000000Z"). Recovered
// from a real reminder read off a snoozed thread.
const superhumanReminderTimeFormat = "2006-01-02T15:04:05.000000000Z"

// parseTriggerAtISO normalizes --trigger-at into the ISO-with-nanos format
// the Superhuman reminder value expects. Accepts RFC3339 (with or without
// fractional seconds) and integer ms strings.
func parseTriggerAtISO(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("empty")
	}
	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t.UTC().Format(superhumanReminderTimeFormat), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, input); err == nil {
		return t.UTC().Format(superhumanReminderTimeFormat), nil
	}
	var ms int64
	if _, err := fmt.Sscanf(input, "%d", &ms); err != nil {
		return "", fmt.Errorf("not RFC3339 and not an integer ms: %w", err)
	}
	if ms <= 0 {
		return "", fmt.Errorf("ms must be positive")
	}
	return time.UnixMilli(ms).UTC().Format(superhumanReminderTimeFormat), nil
}

// extractThreadMessageIDs walks the userdata.read response shape and
// returns the message ids on the requested thread, sorted by their
// natural string order (Gmail thread ids sort by send time when hex).
func extractThreadMessageIDs(data json.RawMessage) ([]string, error) {
	var parsed struct {
		Results []struct {
			Value struct {
				Messages map[string]any `json:"messages"`
			} `json:"value"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal userdata.read: %w", err)
	}
	if len(parsed.Results) == 0 {
		return nil, fmt.Errorf("no results in userdata.read response")
	}
	msgs := parsed.Results[0].Value.Messages
	if len(msgs) == 0 {
		return nil, fmt.Errorf("thread has no messages")
	}
	ids := make([]string, 0, len(msgs))
	for id := range msgs {
		ids = append(ids, id)
	}
	// Stable order: alphabetical also matches send-time for Gmail hex ids.
	sortStrings(ids)
	return ids, nil
}

// sortStrings is sort.Strings without dragging in the sort package import
// at file scope (and matches how a few other files in this package sort
// small slices of identifiers).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
