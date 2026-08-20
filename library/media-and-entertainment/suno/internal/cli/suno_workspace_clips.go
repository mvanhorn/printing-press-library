// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `workspace add` / `workspace remove` — add or remove clips from a
// workspace (Suno "project"). POST /api/project/{workspace_id}/clips with
// body {update_type:"add"|"remove", metadata:{clip_ids:[...]}}. Mutating.
// Each subcommand has a literal Use string and is registered directly in
// workspace.go so static tooling (verify-skill) can resolve the path.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newWorkspaceAddCmd builds `workspace add <workspace_id> --clip <id>...`.
func newWorkspaceAddCmd(flags *rootFlags) *cobra.Command {
	var clips []string
	cmd := &cobra.Command{
		Use:         "add <workspace_id>",
		Short:       "Add clip(s) to a workspace",
		Long:        "Add one or more clips to a workspace (Suno \"project\").\n\nPOSTs {update_type:\"add\", metadata:{clip_ids:[...]}} to /api/project/<id>/clips.\nUse --dry-run to preview the request without sending it.",
		Example:     "  suno-pp-cli project add WS_ID --clip CLIP_ID1 --clip CLIP_ID2",
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/api/project/{workspace_id}/clips"},
		RunE:        workspaceClipMutateRunE(flags, "add", &clips),
	}
	cmd.Flags().StringArrayVar(&clips, "clip", nil, "Clip ID to add/remove (repeatable)")
	return cmd
}

// newWorkspaceRemoveCmd builds `workspace remove <workspace_id> --clip <id>...`.
func newWorkspaceRemoveCmd(flags *rootFlags) *cobra.Command {
	var clips []string
	cmd := &cobra.Command{
		Use:         "remove <workspace_id>",
		Short:       "Remove clip(s) from a workspace",
		Long:        "Remove one or more clips from a workspace (Suno \"project\").\n\nPOSTs {update_type:\"remove\", metadata:{clip_ids:[...]}} to /api/project/<id>/clips.\nUse --dry-run to preview the request without sending it.",
		Example:     "  suno-pp-cli project remove WS_ID --clip CLIP_ID1 --clip CLIP_ID2",
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/api/project/{workspace_id}/clips"},
		RunE:        workspaceClipMutateRunE(flags, "remove", &clips),
	}
	cmd.Flags().StringArrayVar(&clips, "clip", nil, "Clip ID to add/remove (repeatable)")
	return cmd
}

// workspaceClipMutateRunE is the shared add/remove handler.
func workspaceClipMutateRunE(flags *rootFlags, updateType string, clips *[]string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) == 0 {
			return usageErr(fmt.Errorf("a workspace_id is required"))
		}
		workspaceID := args[0]
		if len(*clips) == 0 {
			return usageErr(fmt.Errorf("at least one --clip <id> is required"))
		}

		if dryRunOK(flags) {
			fmt.Fprintf(cmd.OutOrStdout(), "would %s %d clip(s) %s workspace %s: %s\n",
				updateType, len(*clips), preposition(updateType), workspaceID, strings.Join(*clips, ", "))
			return nil
		}

		c, err := flags.newClient()
		if err != nil {
			return err
		}
		path := replacePathParam("/api/project/{workspace_id}/clips", "workspace_id", workspaceID)
		body := map[string]any{
			"update_type": updateType,
			"metadata":    map[string]any{"clip_ids": *clips},
		}
		data, _, err := c.Post(cmd.Context(), path, body)
		if err != nil {
			return classifyAPIError(err, flags)
		}

		if flags.asJSON {
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d clip(s) %s workspace %s\n",
			pastTense(updateType), len(*clips), preposition(updateType), workspaceID)
		return nil
	}
}

func preposition(updateType string) string {
	if updateType == "remove" {
		return "from"
	}
	return "to"
}

func pastTense(updateType string) string {
	if updateType == "remove" {
		return "removed"
	}
	return "added"
}
