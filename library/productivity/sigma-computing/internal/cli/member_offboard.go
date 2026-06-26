// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: member offboard. Hand-filled scaffold.

// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/sigma-computing/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelMemberOffboardCmd(flags *rootFlags) *cobra.Command {
	var flagTransferTo string

	cmd := &cobra.Command{
		Use:   "offboard [member-email]",
		Short: "Deactivate a member and reassign every workbook and file they own to another member in one command.",
		Example: strings.Trim(`
  sigma-computing-pp-cli member offboard leaver@acme.com --transfer-to manager@acme.com --dry-run
  sigma-computing-pp-cli member offboard leaver@acme.com --transfer-to manager@acme.com`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				return cmd.Help()
			}
			memberEmail := strings.TrimSpace(args[0])
			if strings.TrimSpace(flagTransferTo) == "" {
				return fmt.Errorf("missing required flag --transfer-to: the member email or memberId to receive the leaver's files")
			}

			out := cmd.OutOrStdout()

			// Short-circuit for dry-run / verify env BEFORE any HTTP.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would deactivate %s and reassign its files to %s\n", memberEmail, flagTransferTo)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			memberID, err := resolveRecipientID(cmd, flags, c, memberEmail)
			if err != nil {
				return fmt.Errorf("resolving member to offboard: %w", err)
			}
			transferID, err := resolveRecipientID(cmd, flags, c, flagTransferTo)
			if err != nil {
				return fmt.Errorf("resolving --transfer-to: %w", err)
			}

			// Step 2: list ALL files (following pagination), filter to those
			// OWNED by the member. A single-page read would silently leave a
			// heavy owner's overflow files behind.
			fileEntries, err := getAllEntries(cmd, c, fmt.Sprintf("/v2/members/%s/files", memberID), nil)
			if err != nil {
				return fmt.Errorf("listing files for member %s: %w", memberEmail, err)
			}
			owned := ownedInodeIDs(fileEntries, memberID)

			// Step 3: reassign each owned inode, accumulating per-inode results so
			// a mid-run failure reports exactly what already moved (the
			// reassign+deactivate sequence is not atomic — be honest about partial
			// state rather than aborting with only the failing inode named).
			reassignedIDs := make([]string, 0, len(owned))
			var failedID, failDetail string
			for _, inodeID := range owned {
				body := map[string]any{"ownerId": transferID}
				resp, status, perr := c.Patch(cmd.Context(), fmt.Sprintf("/v2/files/%s", inodeID), body)
				if perr != nil {
					failedID, failDetail = inodeID, perr.Error()
					break
				}
				if status < 200 || status >= 300 {
					failedID, failDetail = inodeID, fmt.Sprintf("status %d: %s", status, string(resp))
					break
				}
				reassignedIDs = append(reassignedIDs, inodeID)
			}
			if failedID != "" {
				// Do NOT deactivate the member — leave them active so the operator
				// can re-run after fixing the failure. Report what already moved.
				return fmt.Errorf("reassigned %d of %d file(s) to %s, then failed on inode %s: %s; member %s left ACTIVE — re-run after resolving (already-reassigned inodes: %s)",
					len(reassignedIDs), len(owned), flagTransferTo, failedID, failDetail, memberEmail, strings.Join(reassignedIDs, ","))
			}

			// Step 4: all files reassigned — deactivate the member (isArchived per
			// the OpenAPI spec).
			deactBody := map[string]any{"isArchived": true}
			dResp, dStatus, err := c.Patch(cmd.Context(), fmt.Sprintf("/v2/members/%s", memberID), deactBody)
			if err != nil {
				return fmt.Errorf("reassigned %d file(s) but failed to deactivate member %s: %w", len(reassignedIDs), memberEmail, err)
			}
			if dStatus < 200 || dStatus >= 300 {
				return fmt.Errorf("reassigned %d file(s) but failed to deactivate member %s: status %d: %s", len(reassignedIDs), memberEmail, dStatus, string(dResp))
			}

			result := map[string]any{
				"deactivated":     memberEmail,
				"deactivatedId":   memberID,
				"reassignedTo":    flagTransferTo,
				"reassignedToId":  transferID,
				"filesReassigned": len(reassignedIDs),
			}
			if wantJSON(flags, cmd) {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(out, "Deactivated %s and reassigned %d file(s) to %s\n", memberEmail, len(reassignedIDs), flagTransferTo)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTransferTo, "transfer-to", "", "Member email or memberId to receive the offboarded member's owned files")
	return cmd
}

// ownedInodeIDs returns inode ids from /v2/members/{id}/files entries that are
// OWNED by the member (ownerId == memberID).
func ownedInodeIDs(entries []map[string]any, memberID string) []string {
	var out []string
	for _, e := range entries {
		if firstString(e, "ownerId") == memberID {
			if id := firstString(e, "id", "inodeId"); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}
