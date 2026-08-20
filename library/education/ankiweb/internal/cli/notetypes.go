// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

func newNoteTypesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "notetypes",
		Short:       "List note types and decks available for adding notes (requires login)",
		Long:        "Fetch the note types and decks you can add notes to, your default note type and deck, and the default note type's field names. Use these names with the 'add' command's --type/--deck/--field flags. Requires an AnkiWeb session cookie.",
		Example:     "  ankiweb-pp-cli notetypes --json",
		Annotations: map[string]string{"pp:endpoint": "editor.info", "pp:method": "POST", "pp:path": "/svc/editor/get-info-for-adding", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), svc.AddInfo{}, flags)
			}
			c, _, err := flags.newEditorClient()
			if err != nil {
				return err
			}
			data, status, err := c.PostBytes(cmd.Context(), "/svc/editor/get-info-for-adding", nil)
			if err != nil {
				if status == http.StatusForbidden || status == http.StatusUnauthorized {
					return authErr(errAuthEditor())
				}
				return classifyAPIError(err, flags)
			}
			info, err := svc.DecodeAddInfo(data)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), info, flags)
		},
	}
	return cmd
}
