// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(reminders-endpoint-rewrite): see reminders_create.go for the
// background. Cancel mirrors create but writes a null value on the same
// path -- Superhuman's standard "delete via userdata.write" convention,
// already used by drafts_discard. The generator's --reminder-id flag is
// meaningless under this shape (reminders are keyed by thread, not by a
// separate id); it stays as a deprecated alias for --thread-id so
// scripts that pass it keep working.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newRemindersCancelCmd(flags *rootFlags) *cobra.Command {
	var bodyReminderId string
	var bodyThreadId string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a snooze (un-snooze a thread)",
		Long: `Cancel a thread snooze by writing null to its reminder path
via /v3/userdata.write. --reminder-id is accepted as a deprecated
alias for --thread-id; pass --thread-id directly going forward.`,
		Example: "  superhuman-pp-cli reminders cancel --thread-id 19e2dc46a8b281fe",
		Annotations: map[string]string{
			"pp:endpoint": "reminders.cancel",
			"pp:method":   "POST",
			"pp:path":     "/v3/userdata.write",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				// --reminder-id is the deprecated alias; fall back to it when
				// the caller hasn't passed --thread-id explicitly.
				if bodyThreadId == "" && bodyReminderId != "" {
					bodyThreadId = bodyReminderId
				}
				if bodyThreadId == "" && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "thread-id")
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
					return authErr(fmt.Errorf("reminders cancel: %w", perr))
				}
				body = map[string]any{
					"writes": []map[string]any{
						{
							"path":  fmt.Sprintf("users/%s/threads/%s/reminder", providerID, bodyThreadId),
							"value": nil,
						},
					},
				}
			}

			// PATCH: --dry-run must not fire the userdata.write delete.
			// Same fix shape as reminders create — greptile P1 on PR #595.
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
	cmd.Flags().StringVar(&bodyReminderId, "reminder-id", "", "Deprecated alias for --thread-id (kept for back-compat)")
	cmd.Flags().StringVar(&bodyThreadId, "thread-id", "", "Thread to un-snooze")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin (raw /v3/userdata.write payload)")

	return cmd
}
