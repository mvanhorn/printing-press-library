// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// messages_get.go implements `superhuman-pp-cli messages get <id>`. The
// command mirrors the Superhuman MCP's get_message tool: fetch one Gmail
// message by id and surface headers, body (text/plain + text/html), and
// attachment metadata.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/gmail"
)

// newMessagesGetCmd registers `messages get <message-id>`.
func newMessagesGetCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "get <message-id>",
		Short: "Fetch one message by id (headers, body, attachment metadata)",
		Long: `Fetch a single message by id via Gmail's users.messages.get.

Default --format=full returns the full message including decoded text/plain
and text/html bodies plus attachment metadata. Use --format=metadata to
skip the body (lighter response when the user only needs headers).

The message id is the Gmail message id (typically 16 hex chars). Get it
from 'messages query' results or from listing a thread's messages via
'threads get'.`,
		Example: "  superhuman-pp-cli messages get 195e7c2d4f3e8a1b\n  superhuman-pp-cli messages get 195e7c2d4f3e8a1b --format metadata\n  superhuman-pp-cli messages get 195e7c2d4f3e8a1b --json",
		Annotations: map[string]string{
			"pp:endpoint":   "messages.get",
			"pp:method":     "GET",
			"pp:path":       "/users/me/messages/<id>",
			"mcp:read-only": "true",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("messages get: requires exactly one <message-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessagesGet(cmd, flags, args[0], format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "full", "Response format (full, metadata, raw, minimal)")
	return cmd
}

// runMessagesGet is the testable body. Routes through gmail.GetMessage and
// formats the result as either JSON envelope (--json or non-TTY) or a
// human-readable summary.
func runMessagesGet(cmd *cobra.Command, flags *rootFlags, messageID, format string) error {
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would call gmail.googleapis.com/.../messages/%s\n", messageID)
		return nil
	}

	acct, err := resolveActiveAccount(flags)
	if err != nil {
		return authErr(fmt.Errorf("messages get: %w", err))
	}
	gc := gmail.New(acct.Store, acct.Email, acct.GoogleID, acct.AccessToken)
	gc.Stderr = cmd.ErrOrStderr()

	msg, err := gc.GetMessage(cmd.Context(), messageID, format)
	if err != nil {
		if gmail.IsAuth(err) {
			return authErr(fmt.Errorf("messages get: %w", err))
		}
		if ok, status := gmail.IsAPI(err); ok && status == 404 {
			return notFoundErr(fmt.Errorf("messages get %s: not found", messageID))
		}
		return apiErr(fmt.Errorf("messages get %s: %w", messageID, err))
	}

	// JSON envelope (machine consumers).
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		envelope := map[string]any{
			"action":   "messages.get",
			"resource": "messages",
			"path":     "/users/me/messages/" + messageID,
			"success":  true,
			"data": map[string]any{
				"id":           msg.ID,
				"threadId":     msg.ThreadID,
				"labelIds":     msg.LabelIDs,
				"snippet":      msg.Snippet,
				"historyId":    msg.HistoryID,
				"internalDate": msg.InternalDate,
				"headers":      msg.Headers,
				"body":         msg.Body,
				"htmlBody":     msg.HTMLBody,
				"attachments":  msg.Attachments,
			},
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	// Human-readable: header lines + body excerpt + attachment list.
	w := cmd.OutOrStdout()
	for _, h := range msg.Headers {
		if isInterestingHeader(h.Name) {
			fmt.Fprintf(w, "%s: %s\n", h.Name, h.Value)
		}
	}
	fmt.Fprintf(w, "Thread: %s\n", msg.ThreadID)
	if len(msg.LabelIDs) > 0 {
		fmt.Fprintf(w, "Labels: %s\n", strings.Join(msg.LabelIDs, ", "))
	}
	fmt.Fprintln(w)
	if format != "metadata" && msg.Body != "" {
		fmt.Fprintln(w, msg.Body)
	} else if format != "metadata" && msg.HTMLBody != "" {
		// No plain body but we do have HTML — surface the snippet so the
		// user sees something useful without pulling html-render in.
		fmt.Fprintf(w, "(html body only; snippet: %s)\n", msg.Snippet)
	}
	if len(msg.Attachments) > 0 {
		fmt.Fprintf(w, "\nAttachments:\n")
		for _, a := range msg.Attachments {
			fmt.Fprintf(w, "  %s  %s  %d bytes  (id: %s)\n", a.Filename, a.MimeType, a.Size, a.AttachmentID)
		}
	}
	return nil
}

// isInterestingHeader names the subset of RFC822 headers worth printing in
// the human-readable view. Full headers stay accessible via --json.
func isInterestingHeader(name string) bool {
	switch strings.ToLower(name) {
	case "from", "to", "cc", "bcc", "subject", "date", "reply-to":
		return true
	}
	return false
}
