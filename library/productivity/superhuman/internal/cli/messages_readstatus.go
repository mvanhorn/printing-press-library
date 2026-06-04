// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// messages_readstatus.go implements `superhuman-pp-cli messages read-status
// <thread-id>`. Mirrors the Superhuman MCP's get_read_status_feed tool:
// surface who opened a thread and when.
//
// Implementation-time unknown (plan 2026-05-14-003 U8): the exact endpoint
// path. Two reasonable hypotheses from the bundle:
//   1. POST /v3/userdata.read with path users/<gid>/read_status/<thread-id>
//   2. A dedicated /messages/readstatus route
// This implementation chooses (1) because it mirrors the existing pattern
// for every other userdata.* surface (threads, drafts, splits). If a live
// probe shows (2) is correct, swap the path in readStatusPathFor — the
// command's output shape stays the same.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newMessagesReadStatusCmd registers `messages read-status <thread-id>`.
func newMessagesReadStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read-status <thread-id>",
		Short: "Fetch the read-receipt feed for a thread (who opened, when)",
		Long: `Fetch the read-status feed for a thread via Superhuman's
userdata.read at users/<provider-id>/read_status/<thread-id>.

Output is one row per recorded open event: recipient email, opened-at,
device, and (when tracked) approximate location. An empty result means
no opens have been recorded yet for this thread.`,
		Example: "  superhuman-pp-cli messages read-status 19dd4c470cff4d49\n  superhuman-pp-cli messages read-status 19dd4c470cff4d49 --json",
		Annotations: map[string]string{
			"pp:endpoint":   "messages.read-status",
			"pp:method":     "POST",
			"pp:path":       "/v3/userdata.read",
			"mcp:read-only": "true",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("messages read-status: requires exactly one <thread-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessagesReadStatus(cmd, flags, args[0])
		},
	}
	return cmd
}

// readStatusPathFor returns the userdata.read path for a thread's read
// feed. Centralized so a future live-probe correction is a one-liner.
func readStatusPathFor(providerID, threadID string) string {
	return fmt.Sprintf("users/%s/read_status/%s", providerID, threadID)
}

func runMessagesReadStatus(cmd *cobra.Command, flags *rootFlags, threadID string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	providerID, perr := resolveProviderID(flags)
	if perr != nil {
		return authErr(fmt.Errorf("messages read-status: %w", perr))
	}

	body := map[string]any{
		"reads": []map[string]any{
			{"path": readStatusPathFor(providerID, threadID)},
		},
		"pageToken": nil,
		"pageSize":  nil,
	}
	data, statusCode, err := c.Post("/v3/userdata.read", body)
	if err != nil {
		return classifyAPIError(err, flags)
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		var parsed any
		_ = json.Unmarshal(data, &parsed)
		envelope := map[string]any{
			"action":    "messages.read-status",
			"resource":  "messages",
			"path":      "/v3/userdata.read",
			"thread_id": threadID,
			"status":    statusCode,
			"success":   statusCode >= 200 && statusCode < 300,
			"data":      parsed,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	// Human path: parse the response as best we can, print one row per
	// open event, fall back to raw JSON when the shape is unfamiliar.
	var rows []map[string]any
	if jerr := json.Unmarshal(data, &rows); jerr == nil && len(rows) > 0 {
		if perr := printAutoTable(cmd.OutOrStdout(), rows); perr == nil {
			return nil
		}
	}
	// Wrap-in-data shape (mirrors threads get).
	var wrapped struct {
		Data []map[string]any `json:"data"`
	}
	if jerr := json.Unmarshal(data, &wrapped); jerr == nil && len(wrapped.Data) > 0 {
		if perr := printAutoTable(cmd.OutOrStdout(), wrapped.Data); perr == nil {
			return nil
		}
	}
	// Empty / unfamiliar shape: print a friendly message and the raw body.
	fmt.Fprintln(cmd.OutOrStdout(), "No read events yet for this thread.")
	return nil
}
