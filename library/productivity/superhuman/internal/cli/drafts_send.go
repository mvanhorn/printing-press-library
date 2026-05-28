// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// drafts_send.go implements `superhuman-pp-cli drafts send <draft-id>`.
// The motivating bug (2026-05-22): the agent created a draft via
// `drafts new`, the user edited the draft in the Superhuman UI, and the
// agent then called `send` — which built a brand-new draft from its
// local --to/--subject/--body flags and dispatched THAT, silently
// discarding the user's UI edits. `send` had no notion of the user's
// in-flight draft id. This subcommand closes that gap: fetch the
// current server-side draft via /v3/userdata.read, then dispatch via
// the same Gmail-API path send.go uses for new drafts.
//
// Why a new subcommand instead of a flag on `send`: `send` already
// validates --to and --subject as required and constructs identity
// fields (rfc822Id, superhumanId) from scratch. Making those mutually
// exclusive with --draft-id would muddle the contracts. A dedicated
// verb keeps the surface clean and mirrors the existing
// `drafts discard <id>` shape.
//
// What this command intentionally does NOT do:
//   - It does NOT re-run step1WriteMessage. The draft is already at the
//     desired server state; rewriting risks racing with another active
//     client (Superhuman web) that may have just persisted a fresh
//     edit. The "is this draft current?" decision belongs to the
//     caller, not the dispatch path.
//   - It does NOT mint fresh rfc822Id / superhumanId. The fetched draft
//     already carries the canonical Message-Id continuity for threading.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
	"github.com/spf13/cobra"
)

// newDraftsSendCmd registers `drafts send <draft-id>`.
func newDraftsSendCmd(flags *rootFlags) *cobra.Command {
	var undo time.Duration
	cmd := &cobra.Command{
		Use:   "send <draft-id>",
		Short: "Send an existing draft by id (honors UI edits)",
		Long: `Send the current server-side draft for <draft-id>. The body, subject,
and recipients are read from Superhuman immediately before dispatch,
so any edits made in the Superhuman web or desktop UI between draft
creation and this call are respected.

Use this instead of 'send' when an agent created a draft via
'drafts new', the user touched the draft in the UI, and the agent
needs to ship that state.

--undo accepts a duration (e.g. 30s) and holds the send locally until
the timer elapses or 'unsend' is invoked. The CLI process must stay
foreground for the duration.`,
		Example: "  superhuman-pp-cli drafts send draft0012ab34cd56ef\n  superhuman-pp-cli drafts send draft0012ab34cd56ef --undo 30s --json",
		Annotations: map[string]string{
			"pp:endpoint": "drafts.send",
			"pp:method":   "POST",
			"pp:path":     "/v3/userdata.read+gmail.messages.send",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("drafts send: requires exactly one <draft-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDraftsSend(cmd, flags, args[0], undo)
		},
	}
	cmd.Flags().DurationVar(&undo, "undo", 0, "Hold the send locally for this duration; cancel with 'unsend' or Ctrl-C")
	return cmd
}

func runDraftsSend(cmd *cobra.Command, flags *rootFlags, draftID string, undo time.Duration) error {
	acct, err := resolveActiveAccount(flags)
	if err != nil {
		return authErr(fmt.Errorf("drafts send: %w", err))
	}

	// --dry-run short-circuit fires BEFORE the userdata.read fetch. The HTTP
	// client's global dry-run mode would return a synthetic envelope here
	// rather than the real server response, which would surface as a
	// spurious "draft not found" error.
	if flags.dryRun {
		return printDraftsSendDryRun(cmd, draftID, acct.Email)
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	dv, _, err := readDraft(c, acct.GoogleID, draftID)
	if err != nil {
		if errors.Is(err, ErrDraftNotFound) {
			return notFoundErr(fmt.Errorf("drafts send: %s not found (or empty)", draftID))
		}
		return classifyAPIError(err, flags)
	}

	inputs := draftValueToSendInputs(dv, acct.Email)

	announceSender(cmd, acct.Email, inputs.FromName)

	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would send draft %s: %q to %v\n", draftID, inputs.Subject, inputs.To)
		return nil
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Step 2 (send/log) — analytics parity with the new-draft send path.
	// Step 1 (writeMessage) is intentionally skipped: the draft already
	// exists at the desired state and re-writing risks a race with the
	// Superhuman web client. See file header.
	om := buildOutgoingMessage(inputs)
	if err := step2SendLog(ctx, c, dv, om, inputs.SuperhumanID, draftID); err != nil {
		return apiErr(fmt.Errorf("drafts send step 2 (send/log): %w", err))
	}

	fromDisplay := formatAddressString(acct.Email, inputs.FromName)
	if undo > 0 {
		return enqueueWithUndo(cmd, c, acct.Email, acct.GoogleID, acct.AccessToken, fromDisplay, acct.Store, inputs, om, undo)
	}

	gmailID, err := sendGmailWithRefresh(ctx, cmd.ErrOrStderr(), acct.Store, acct.Email, acct.GoogleID, acct.AccessToken, fromDisplay, inputs)
	if err != nil {
		return apiErr(fmt.Errorf("drafts send (gmail api): %w", err))
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		envelope := map[string]any{
			"action":   "drafts.send",
			"resource": "drafts",
			"draft_id": draftID,
			"gmail_id": gmailID,
			"from":     inputs.FromEmail,
			"to":       inputs.To,
			"cc":       inputs.Cc,
			"bcc":      inputs.Bcc,
			"subject":  inputs.Subject,
			"success":  true,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Sent. draftId=%s, gmailId=%s\n", draftID, gmailID)
	return nil
}

// draftValueToSendInputs translates a fetched draftValue into the
// sendInputs shape the existing Gmail dispatch path expects. The DraftID,
// Rfc822ID, and SuperhumanID identity fields carry over from the draft
// (preserving Message-Id continuity for threading); the body is treated
// as already-final HTML since drafts persisted in Superhuman are HTML-
// shaped in the UI editor.
func draftValueToSendInputs(dv draftValue, fallbackEmail string) sendInputs {
	fromEmail, fromName := splitAddressLine(dv.From, fallbackEmail)
	return sendInputs{
		FromEmail:    fromEmail,
		FromName:     fromName,
		To:           append([]string(nil), dv.To...),
		Cc:           append([]string(nil), dv.Cc...),
		Bcc:          append([]string(nil), dv.Bcc...),
		Subject:      dv.Subject,
		Body:         dv.Body,
		HTMLBody:     true,
		Reminder:     dv.Reminder,
		Schedule:     dv.ScheduledFor,
		DraftID:      dv.ID,
		Rfc822ID:     dv.Rfc822ID,
		SuperhumanID: deriveSuperhumanID(dv),
		Now:          time.Now(),
	}
}

// splitAddressLine parses a "Name <email>" form (or bare email) into its
// two parts. Falls back to fallbackEmail when the from line is empty or
// unparseable so the dispatch never fires with an empty FromEmail.
func splitAddressLine(line, fallbackEmail string) (email, name string) {
	if line == "" {
		return fallbackEmail, ""
	}
	if addr, err := mail.ParseAddress(line); err == nil {
		return addr.Address, addr.Name
	}
	return line, ""
}

// deriveSuperhumanID returns dv's superhumanId if the bundle persists one,
// or mints a fresh one. The cancel-schedule precedent stores the value
// under different field names across server versions; we look in two
// likely places before falling back. The minted id is byte-stable across
// retries because it derives from the draft id.
func deriveSuperhumanID(dv draftValue) string {
	// The draftValue struct has no SuperhumanID field, so the bundle
	// either does not return it on read or stores it elsewhere on the
	// thread record. Fall back to deriving from the draft id, which is
	// what auth.NewSuperhumanID emits anyway. The step2SendLog server
	// does not appear to validate the id beyond uniqueness.
	if dv.Rfc822ID != "" {
		return "sh-" + dv.Rfc822ID
	}
	return "sh-" + dv.ID
}

func printDraftsSendDryRun(cmd *cobra.Command, draftID, fromEmail string) error {
	envelope := map[string]any{
		"action":   "drafts.send",
		"resource": "drafts",
		"draft_id": draftID,
		"from":     fromEmail,
		"note":     "live mode fetches the server-side draft via /v3/userdata.read before dispatch",
		"dry_run":  true,
	}
	envelopeJSON, jerr := json.Marshal(envelope)
	if jerr != nil {
		return jerr
	}
	return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
}
