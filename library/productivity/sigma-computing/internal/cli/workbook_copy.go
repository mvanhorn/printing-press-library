// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: workbook copy + reassign ownership. Hand-filled scaffold.

// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/sigma-computing/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelWorkbookCopyCmd(flags *rootFlags) *cobra.Command {
	var flagTo string
	var flagName string
	var flagFolder string

	cmd := &cobra.Command{
		Use:   "copy [workbookId]",
		Short: "Copy a workbook and automatically reassign ownership to the intended recipient instead of the calling admin.",
		Example: strings.Trim(`
  sigma-computing-pp-cli workbook copy 2Xpsl5dB1qD --to analyst@acme.com --folder 7fHomeFolder
  sigma-computing-pp-cli workbook copy 2Xpsl5dB1qD --to analyst@acme.com --name "Q3 Metrics (copy)" --dry-run`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				return cmd.Help()
			}
			workbookID := strings.TrimSpace(args[0])
			if strings.TrimSpace(flagTo) == "" {
				return fmt.Errorf("missing required flag --to: the recipient member email or memberId to own the copy")
			}
			name := flagName
			if strings.TrimSpace(name) == "" {
				name = "Copy of " + workbookID
			}

			out := cmd.OutOrStdout()

			// Short-circuit for dry-run / verify env BEFORE any HTTP.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would copy workbook %s (name %q, folder %q) then reassign owner to %s\n",
					workbookID, name, folderOrDefault(flagFolder), flagTo)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve recipient email -> memberId (store first, API fallback).
			recipientID, err := resolveRecipientID(cmd, flags, c, flagTo)
			if err != nil {
				return err
			}

			// Step 1: copy the workbook.
			copyBody := map[string]any{
				"name":                name,
				"destinationFolderId": flagFolder,
			}
			resp, status, err := c.Post(cmd.Context(), fmt.Sprintf("/v2/workbooks/%s/copy", workbookID), copyBody)
			if err != nil {
				return fmt.Errorf("copying workbook %s: %w", workbookID, err)
			}
			if status < 200 || status >= 300 {
				return fmt.Errorf("copying workbook %s: unexpected status %d: %s", workbookID, status, string(resp))
			}
			newID := extractNewWorkbookID(resp)
			if newID == "" {
				return fmt.Errorf("copy succeeded but could not determine the new workbook id from response: %s", string(resp))
			}

			// Step 2: reassign ownership of the new workbook to the recipient.
			reassignBody := map[string]any{"ownerId": recipientID}
			rResp, rStatus, err := c.Patch(cmd.Context(), fmt.Sprintf("/v2/files/%s", newID), reassignBody)
			if err != nil {
				return fmt.Errorf("reassigning owner of new workbook %s to %s: %w", newID, flagTo, err)
			}
			if rStatus < 200 || rStatus >= 300 {
				return fmt.Errorf("reassigning owner of new workbook %s: unexpected status %d: %s", newID, rStatus, string(rResp))
			}

			result := map[string]any{
				"copiedFrom":     workbookID,
				"newWorkbookId":  newID,
				"name":           name,
				"reassignedTo":   flagTo,
				"reassignedToId": recipientID,
			}
			if wantJSON(flags, cmd) {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(out, "Copied workbook %s -> %s (%q); owner reassigned to %s\n", workbookID, newID, name, flagTo)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTo, "to", "", "Recipient member email or memberId who should own the copy")
	cmd.Flags().StringVar(&flagName, "name", "", "Name for the copied workbook (default: \"Copy of <workbookId>\")")
	cmd.Flags().StringVar(&flagFolder, "folder", "", "destinationFolderId for the copy (default: recipient/admin default folder)")
	return cmd
}

func folderOrDefault(folder string) string {
	if strings.TrimSpace(folder) == "" {
		return "(default)"
	}
	return folder
}

// extractNewWorkbookID pulls the new workbook's id from a copy response. The
// copy endpoint returns workbookId (verified against the OpenAPI spec); we also
// accept id/inodeId as fallbacks.
func extractNewWorkbookID(resp json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(resp, &obj); err != nil {
		return ""
	}
	return firstString(obj, "workbookId", "inodeId", "id")
}
