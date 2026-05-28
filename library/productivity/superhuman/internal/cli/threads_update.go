// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// threads_update.go implements `superhuman-pp-cli threads update <id>
// --action <verb>`. Mirrors the Superhuman MCP's update_thread tool with
// a closed verb set: archive, read, unread, star, unstar.
//
// Per plan 2026-05-14-003 KD4 + KD5:
//   - Verbs are mapped to Gmail label add/remove operations (KD4).
//   - Terminal labels (TRASH, SPAM, unsubscribe) are NOT in this verb
//     set — they live in dedicated commands so the destructive
//     nature is visible at the command surface (KD5).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/gmail"
)

// threadUpdateAction names one supported verb. add and remove are the
// Gmail labels that get applied or stripped.
type threadUpdateAction struct {
	Add    []string
	Remove []string
}

// threadUpdateActions is the closed verb set. Lookups in lowercase.
var threadUpdateActions = map[string]threadUpdateAction{
	"archive": {Remove: []string{gmail.SystemLabelInbox}},
	"read":    {Remove: []string{gmail.SystemLabelUnread}},
	"unread":  {Add: []string{gmail.SystemLabelUnread}},
	"star":    {Add: []string{gmail.SystemLabelStarred}},
	"unstar":  {Remove: []string{gmail.SystemLabelStarred}},
}

// supportedActionsList returns the verb names sorted for stable error
// output.
func supportedActionsList() []string {
	out := make([]string, 0, len(threadUpdateActions))
	for k := range threadUpdateActions {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// newThreadsUpdateCmd registers `threads update <thread-id> --action <verb>`.
func newThreadsUpdateCmd(flags *rootFlags) *cobra.Command {
	var action string
	cmd := &cobra.Command{
		Use:   "update <thread-id>",
		Short: "Mutate a thread's state (archive, read, unread, star, unstar)",
		Long: `Apply one of the supported verbs to a thread via Gmail's
users.threads.modify endpoint.

Supported --action values:
  archive   Remove INBOX label
  read      Remove UNREAD label
  unread    Add UNREAD label
  star      Add STARRED label
  unstar    Remove STARRED label

For terminal moves (trash, spam, unsubscribe) use the dedicated
'threads trash', 'threads mark-spam', 'threads unsubscribe' commands —
they share the same Gmail label primitives but separate destructive
intent at the command surface.`,
		Example: "  superhuman-pp-cli threads update 19abc --action archive\n  superhuman-pp-cli threads update 19abc --action star --json",
		Annotations: map[string]string{
			"pp:endpoint": "threads.update",
			"pp:method":   "POST",
			"pp:path":     "/users/me/threads/<id>/modify",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("threads update: requires exactly one <thread-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if action == "" {
				return usageErr(fmt.Errorf("threads update: --action <verb> is required (supported: %s)", strings.Join(supportedActionsList(), ", ")))
			}
			return runThreadsUpdate(cmd, flags, args[0], action)
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "Update verb: archive, read, unread, star, unstar")
	return cmd
}

func runThreadsUpdate(cmd *cobra.Command, flags *rootFlags, threadID, action string) error {
	verb, ok := threadUpdateActions[strings.ToLower(action)]
	if !ok {
		return usageErr(fmt.Errorf("threads update: unsupported --action %q (supported: %s)",
			action, strings.Join(supportedActionsList(), ", ")))
	}

	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would call gmail.googleapis.com/.../threads/%s/modify (action=%s)\n", threadID, action)
		return nil
	}

	acct, err := resolveActiveAccount(flags)
	if err != nil {
		return authErr(fmt.Errorf("threads update: %w", err))
	}
	gc := gmail.New(acct.Store, acct.Email, acct.GoogleID, acct.AccessToken)
	gc.Stderr = cmd.ErrOrStderr()

	newLabels, err := gc.ModifyThreadLabels(cmd.Context(), threadID, verb.Add, verb.Remove)
	if err != nil {
		if gmail.IsAuth(err) {
			return authErr(fmt.Errorf("threads update: %w", err))
		}
		if ok, status := gmail.IsAPI(err); ok && status == 404 {
			return notFoundErr(fmt.Errorf("threads update: thread %s not found", threadID))
		}
		return apiErr(fmt.Errorf("threads update %s --action %s: %w", threadID, action, err))
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		envelope := map[string]any{
			"action":     "threads.update",
			"resource":   "threads",
			"path":       "/users/me/threads/" + threadID + "/modify",
			"thread_id":  threadID,
			"verb":       action,
			"add":        verb.Add,
			"remove":     verb.Remove,
			"new_labels": newLabels,
			"success":    true,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s thread %s (now: %s)\n", action, threadID, strings.Join(newLabels, ", "))
	return nil
}
