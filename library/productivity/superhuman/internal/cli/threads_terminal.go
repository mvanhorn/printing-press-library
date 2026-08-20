// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// threads_terminal.go implements three destructive thread-state verbs
// kept out of `threads update` (see plan 2026-05-14-003 KD5):
//
//   - threads trash       — add TRASH, remove INBOX
//   - threads mark-spam   — add SPAM, remove INBOX
//   - threads unsubscribe — honor RFC 8058 One-Click unsubscribe headers,
//                           then archive (remove INBOX)
//
// Each prompts unless --yes is passed or --json is on (machine consumers).

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/gmail"
)

// newThreadsTrashCmd registers `threads trash <id>`.
func newThreadsTrashCmd(flags *rootFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "trash <thread-id>",
		Short: "Trash a thread (add TRASH label, remove INBOX)",
		Long: `Move a thread to Gmail's Trash. Recoverable from Gmail's Trash
folder for ~30 days, after which Gmail permanently deletes it.`,
		Example: "  superhuman-pp-cli threads trash 19abc --yes",
		Annotations: map[string]string{
			"pp:endpoint": "threads.trash",
			"pp:method":   "POST",
			"pp:path":     "/users/me/threads/<id>/modify",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("threads trash: requires exactly one <thread-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTerminalLabel(cmd, flags, args[0], yes, terminalActionTrash)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

// newThreadsMarkSpamCmd registers `threads mark-spam <id>`.
func newThreadsMarkSpamCmd(flags *rootFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "mark-spam <thread-id>",
		Short: "Mark a thread as spam (add SPAM, remove INBOX)",
		Long: `Mark a thread as spam. Gmail's classifier may use this signal
to improve future spam detection for the account.`,
		Example: "  superhuman-pp-cli threads mark-spam 19abc --yes",
		Annotations: map[string]string{
			"pp:endpoint": "threads.mark-spam",
			"pp:method":   "POST",
			"pp:path":     "/users/me/threads/<id>/modify",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("threads mark-spam: requires exactly one <thread-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTerminalLabel(cmd, flags, args[0], yes, terminalActionMarkSpam)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

// newThreadsUnsubscribeCmd registers `threads unsubscribe <id>`.
func newThreadsUnsubscribeCmd(flags *rootFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "unsubscribe <thread-id>",
		Short: "Honor RFC 8058 One-Click unsubscribe, then archive",
		Long: `Fetch the latest message in the thread, parse its
List-Unsubscribe and List-Unsubscribe-Post headers, and fire a
One-Click POST when both headers indicate RFC 8058 support. When only
a mailto: form is present, the address is printed (Claude does not auto-
send email on the user's behalf). The thread is archived (INBOX removed)
regardless of unsubscribe outcome.`,
		Example: "  superhuman-pp-cli threads unsubscribe 19abc --yes",
		Annotations: map[string]string{
			"pp:endpoint": "threads.unsubscribe",
			"pp:method":   "POST",
			"pp:path":     "/users/me/threads/<id>/modify",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("threads unsubscribe: requires exactly one <thread-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnsubscribe(cmd, flags, args[0], yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

// terminalAction is the closed set of terminal-label verbs.
type terminalAction int

const (
	terminalActionTrash terminalAction = iota
	terminalActionMarkSpam
)

// String returns the verb name for output formatting.
func (a terminalAction) String() string {
	switch a {
	case terminalActionTrash:
		return "trash"
	case terminalActionMarkSpam:
		return "mark-spam"
	}
	return "unknown"
}

// labelOps returns the (add, remove) Gmail labels for this action.
func (a terminalAction) labelOps() (add, remove []string) {
	switch a {
	case terminalActionTrash:
		return []string{gmail.SystemLabelTrash}, []string{gmail.SystemLabelInbox}
	case terminalActionMarkSpam:
		return []string{gmail.SystemLabelSpam}, []string{gmail.SystemLabelInbox}
	}
	return nil, nil
}

func runTerminalLabel(cmd *cobra.Command, flags *rootFlags, threadID string, yes bool, action terminalAction) error {
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would %s thread %s\n", action, threadID)
		return nil
	}

	if !yes && !flags.asJSON {
		if !isTerminalReader(cmd.InOrStdin()) {
			return usageErr(fmt.Errorf("threads %s: confirmation required and stdin is not a terminal; pass --yes to skip", action))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s thread %s? [y/N]: ", action, threadID)
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
			return nil
		}
	}

	acct, err := resolveActiveAccount(flags)
	if err != nil {
		return authErr(fmt.Errorf("threads %s: %w", action, err))
	}
	gc := gmail.New(acct.Store, acct.Email, acct.GoogleID, acct.AccessToken)
	gc.Stderr = cmd.ErrOrStderr()

	add, remove := action.labelOps()
	newLabels, err := gc.ModifyThreadLabels(cmd.Context(), threadID, add, remove)
	if err != nil {
		if gmail.IsAuth(err) {
			return authErr(fmt.Errorf("threads %s: %w", action, err))
		}
		if ok, status := gmail.IsAPI(err); ok && status == 404 {
			return notFoundErr(fmt.Errorf("threads %s: thread %s not found", action, threadID))
		}
		return apiErr(fmt.Errorf("threads %s %s: %w", action, threadID, err))
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		envelope := map[string]any{
			"action":     "threads." + action.String(),
			"resource":   "threads",
			"thread_id":  threadID,
			"new_labels": newLabels,
			"success":    true,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	switch action {
	case terminalActionTrash:
		fmt.Fprintf(cmd.OutOrStdout(), "Trashed %s (restorable from Gmail Trash for ~30 days)\n", threadID)
	case terminalActionMarkSpam:
		fmt.Fprintf(cmd.OutOrStdout(), "Marked spam %s (Gmail's classifier may use this signal)\n", threadID)
	}
	return nil
}

// runUnsubscribe is the multi-step unsubscribe-and-archive flow described
// in plan 2026-05-14-003 U11. Phase order: fetch thread -> parse headers
// -> attempt One-Click POST -> archive.
func runUnsubscribe(cmd *cobra.Command, flags *rootFlags, threadID string, yes bool) error {
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would unsubscribe + archive thread %s\n", threadID)
		return nil
	}

	if !yes && !flags.asJSON {
		if !isTerminalReader(cmd.InOrStdin()) {
			return usageErr(fmt.Errorf("threads unsubscribe: confirmation required and stdin is not a terminal; pass --yes to skip"))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Unsubscribe + archive thread %s? [y/N]: ", threadID)
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
			return nil
		}
	}

	acct, err := resolveActiveAccount(flags)
	if err != nil {
		return authErr(fmt.Errorf("threads unsubscribe: %w", err))
	}
	gc := gmail.New(acct.Store, acct.Email, acct.GoogleID, acct.AccessToken)
	gc.Stderr = cmd.ErrOrStderr()

	// Phase 1: fetch the latest message in the thread. Gmail's threads.get
	// returns messages newest-last; we read the last entry. (The MCP's
	// unsubscribe tool fetches the last message too.)
	tmsg, err := fetchLatestThreadMessage(cmd.Context(), gc, threadID)
	if err != nil {
		return err
	}

	// Phase 2: parse List-Unsubscribe / List-Unsubscribe-Post headers and
	// decide on the unsubscribe path.
	oneClickURL, mailtoAddr := pickUnsubscribeTargets(tmsg.Headers)
	unsubResult := "no List-Unsubscribe header found"
	switch {
	case oneClickURL != "":
		if perr := postOneClickUnsubscribe(cmd.Context(), oneClickURL); perr != nil {
			unsubResult = fmt.Sprintf("One-Click POST to %s failed: %v", oneClickURL, perr)
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", unsubResult)
		} else {
			unsubResult = "One-Click POST sent to " + oneClickURL
		}
	case mailtoAddr != "":
		unsubResult = "mailto: " + mailtoAddr + " (send manually to unsubscribe; not auto-sent)"
		fmt.Fprintf(cmd.OutOrStdout(), "Unsubscribe address: %s\n", mailtoAddr)
	}

	// Phase 3: archive the thread regardless of phase-2 outcome.
	newLabels, archiveErr := gc.ModifyThreadLabels(cmd.Context(), threadID, nil, []string{gmail.SystemLabelInbox})
	if archiveErr != nil {
		if gmail.IsAuth(archiveErr) {
			return authErr(fmt.Errorf("threads unsubscribe (archive): %w", archiveErr))
		}
		return apiErr(fmt.Errorf("threads unsubscribe (archive): %w", archiveErr))
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		envelope := map[string]any{
			"action":            "threads.unsubscribe",
			"resource":          "threads",
			"thread_id":         threadID,
			"unsubscribe":       unsubResult,
			"one_click_url":     oneClickURL,
			"mailto":            mailtoAddr,
			"archived":          true,
			"new_labels":        newLabels,
			"success":           true,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Unsubscribe: %s\nArchived: %s\n", unsubResult, threadID)
	return nil
}

// fetchLatestThreadMessage returns the most-recent message in the thread.
// Implemented by listing messages.list with the threadId filter and taking
// the first result (Gmail returns newest-first for messages.list).
func fetchLatestThreadMessage(ctx context.Context, gc *gmail.Client, threadID string) (*gmail.Message, error) {
	// users.threads.get returns the message list in order; helper does
	// not yet exist in internal/gmail. For v1.1, use messages.list with
	// the q=threadId:<id> shortcut (Gmail's q grammar supports this).
	q := url.Values{}
	q.Set("q", "threadId:"+threadID)
	q.Set("maxResults", "1")
	var raw struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := gc.GetJSON(ctx, "/users/me/messages?"+q.Encode(), &raw); err != nil {
		return nil, apiErr(fmt.Errorf("threads unsubscribe (fetch thread messages): %w", err))
	}
	if len(raw.Messages) == 0 {
		return nil, notFoundErr(fmt.Errorf("threads unsubscribe: thread %s has no messages", threadID))
	}
	return gc.GetMessage(ctx, raw.Messages[0].ID, "full")
}

// pickUnsubscribeTargets parses List-Unsubscribe and List-Unsubscribe-Post
// headers per RFC 8058 + RFC 2369.
//
// RFC 8058 One-Click: BOTH headers must be present. List-Unsubscribe-Post:
// "List-Unsubscribe=One-Click" + List-Unsubscribe value containing at least
// one https:// or http:// URL between angle brackets. When both are
// present we return the first URL form for One-Click POST.
//
// When only mailto: forms are present we return the mailto address (for
// the human to send manually). Auto-sending an unsubscribe mail on behalf
// of the user is intentionally NOT supported.
func pickUnsubscribeTargets(headers []gmail.Header) (oneClickURL, mailtoAddr string) {
	var listValue, postValue string
	for _, h := range headers {
		switch strings.ToLower(h.Name) {
		case "list-unsubscribe":
			listValue = h.Value
		case "list-unsubscribe-post":
			postValue = h.Value
		}
	}
	if listValue == "" {
		return "", ""
	}

	urlForm, mailtoForm := splitUnsubscribeForms(listValue)

	// RFC 8058 One-Click gate: the post header must indicate One-Click,
	// AND the list-value must contain at least one URL form.
	if strings.Contains(strings.ToLower(postValue), "one-click") && urlForm != "" {
		return urlForm, mailtoForm
	}
	return "", mailtoForm
}

// splitUnsubscribeForms parses the comma-separated <url> / <mailto:addr>
// list shape RFC 2369 specifies for List-Unsubscribe.
func splitUnsubscribeForms(value string) (urlForm, mailtoForm string) {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		p := strings.TrimSpace(part)
		p = strings.TrimPrefix(p, "<")
		p = strings.TrimSuffix(p, ">")
		switch {
		case strings.HasPrefix(p, "http://"), strings.HasPrefix(p, "https://"):
			if urlForm == "" {
				urlForm = p
			}
		case strings.HasPrefix(strings.ToLower(p), "mailto:"):
			if mailtoForm == "" {
				mailtoForm = strings.TrimPrefix(p, "mailto:")
				mailtoForm = strings.TrimPrefix(mailtoForm, "MAILTO:")
			}
		}
	}
	return urlForm, mailtoForm
}

// postOneClickUnsubscribe fires the RFC 8058 One-Click POST. The request
// body is the literal byte sequence "List-Unsubscribe=One-Click" per the
// RFC; no other parameters are required.
//
// Test seam: oneClickHTTPClient defaults to the package's stdlib client;
// tests override this to point at an httptest server.
var oneClickHTTPClient = http.DefaultClient

func postOneClickUnsubscribe(ctx context.Context, url string) error {
	body := strings.NewReader("List-Unsubscribe=One-Click")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := oneClickHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("one-click POST returned HTTP %d", resp.StatusCode)
	}
	return nil
}
