// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// drafts_discard.go implements `superhuman-pp-cli drafts discard <draft-id>`.
// Mirrors the Superhuman MCP's discard_draft tool: remove an unsent draft.
//
// Implementation-time unknown (plan 2026-05-14-003 U9): the exact endpoint
// shape. Two reasonable hypotheses from the bundle:
//   1. POST /v3/userdata.write with {writes:[{path:"users/<gid>/threads/<id>/messages/<id>/draft", value:null}]}
//   2. POST /v3/userdata.write with {deletes:[{path:"users/<gid>/threads/<id>"}]}
// This implementation chooses (1) because writeMessage uses the writes-shape
// and the userdata.write naming suggests a generic write API. If a live
// probe shows discard routes through a dedicated `/messages/discard` or
// only succeeds with the deletes-shape, swap the body in discardDraftBody.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newDraftsDiscardCmd registers `drafts discard <draft-id>`.
func newDraftsDiscardCmd(flags *rootFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "discard <draft-id>",
		Short: "Discard an unsent draft by id",
		Long: `Discard a draft via /v3/userdata.write. The draft id is the
"draft00..." form that 'drafts list' returns (see 'drafts list --json').

Destructive: the draft is gone from Superhuman after this fires. Pass
--yes to skip the confirmation prompt; --json implies --yes.`,
		Example: "  superhuman-pp-cli drafts discard draft0012ab34cd56ef\n  superhuman-pp-cli drafts discard draft0012ab34cd56ef --yes",
		Annotations: map[string]string{
			"pp:endpoint": "drafts.discard",
			"pp:method":   "POST",
			"pp:path":     "/v3/userdata.write",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("drafts discard: requires exactly one <draft-id> argument"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDraftsDiscard(cmd, flags, args[0], yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

// discardDraftBody is centralized so swapping the userdata.write shape
// (deletes vs writes-with-null) is a one-liner if a live probe disagrees.
func discardDraftBody(providerID, draftID string) map[string]any {
	return map[string]any{
		"writes": []map[string]any{
			{
				"path":  fmt.Sprintf("users/%s/threads/%s/messages/%s/draft", providerID, draftID, draftID),
				"value": nil,
			},
		},
	}
}

func runDraftsDiscard(cmd *cobra.Command, flags *rootFlags, draftID string, yes bool) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	providerID, perr := resolveProviderID(flags)
	if perr != nil {
		return authErr(fmt.Errorf("drafts discard: %w", perr))
	}

	// Confirmation gate: --yes or --json bypasses; otherwise we need a TTY
	// with a 'y' reply. Non-TTY without --yes is a usage error so a
	// scripted-invocation never accidentally discards.
	if !yes && !flags.asJSON {
		if !isTerminalReader(cmd.InOrStdin()) {
			return usageErr(fmt.Errorf("drafts discard: confirmation required and stdin is not a terminal; pass --yes to skip"))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Discard draft %s? [y/N]: ", draftID)
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Discard cancelled.")
			return nil
		}
	}

	body := discardDraftBody(providerID, draftID)
	data, statusCode, err := c.Post("/v3/userdata.write", body)
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
			"action":   "drafts.discard",
			"resource": "drafts",
			"path":     "/v3/userdata.write",
			"draft_id": draftID,
			"status":   statusCode,
			"success":  statusCode >= 200 && statusCode < 300,
			"data":     parsed,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Discarded draft %s.\n", draftID)
	return nil
}
