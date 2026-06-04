// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// drafts_get.go implements `superhuman-pp-cli drafts get <draft-id>`. The
// command fetches the server-side draft state — subject, body, recipients
// — for one draft id. Pairs with `drafts send <id>` (drafts_send.go) which
// reuses the same fetch primitive before dispatching the email so an
// agent never sends a draft body it never read.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// newDraftsGetCmd registers `drafts get <draft-id>`.
func newDraftsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <draft-id>",
		Short: "Fetch one draft by id (subject, body, recipients)",
		Long: `Fetch a single draft via /v3/userdata.read at
users/<provider-id>/threads/<draft-id>/messages/<draft-id>/draft.

The "draft00..." form returned by 'drafts list' or 'drafts new --json'
is what to pass. Use --json for the full structured payload (writeMessage
schema, every field).`,
		Example: "  superhuman-pp-cli drafts get draft0012ab34cd56ef\n  superhuman-pp-cli drafts get draft0012ab34cd56ef --json",
		Annotations: map[string]string{
			"pp:endpoint":   "drafts.get",
			"pp:method":     "POST",
			"pp:path":       "/v3/userdata.read",
			"mcp:read-only": "true",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("drafts get: requires exactly one <draft-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDraftsGet(cmd, flags, args[0])
		},
	}
	return cmd
}

func runDraftsGet(cmd *cobra.Command, flags *rootFlags, draftID string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	providerID, perr := resolveProviderID(flags)
	if perr != nil {
		return authErr(fmt.Errorf("drafts get: %w", perr))
	}

	dv, statusCode, err := readDraft(c, providerID, draftID)
	if err != nil {
		if errors.Is(err, ErrDraftNotFound) {
			return notFoundErr(fmt.Errorf("drafts get: %s not found", draftID))
		}
		return classifyAPIError(err, flags)
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		envelope := map[string]any{
			"action":   "drafts.get",
			"resource": "drafts",
			"path":     "/v3/userdata.read",
			"draft_id": draftID,
			"status":   statusCode,
			"success":  statusCode >= 200 && statusCode < 300,
			"data":     dv,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	// Human path: compact card form.
	fmt.Fprintf(cmd.OutOrStdout(), "Draft %s\n", dv.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "From:    %s\n", dv.From)
	if len(dv.To) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "To:      %s\n", joinAddresses(dv.To))
	}
	if len(dv.Cc) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Cc:      %s\n", joinAddresses(dv.Cc))
	}
	if len(dv.Bcc) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Bcc:     %s\n", joinAddresses(dv.Bcc))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Subject: %s\n\n", dv.Subject)
	fmt.Fprintln(cmd.OutOrStdout(), dv.Body)
	return nil
}
