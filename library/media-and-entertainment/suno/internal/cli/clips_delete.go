// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `clips delete` — trash (or, with --undo, restore) one or more clips.
// POST /api/feed/trash with body {ids:[...]}. Mutating.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newClipsDeleteCmd(flags *rootFlags) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "delete <clip_id> [<clip_id>...]",
		Short: "Trash (or restore) one or more clips",
		Long: `Trash one or more clips (POST /api/feed/trash).

Pass --undo to restore previously trashed clips instead. Use --dry-run to
preview which clips would be affected without sending the request.`,
		Example: `  suno-pp-cli clips delete 550e8400-e29b-41d4-a716-446655440000
  suno-pp-cli clips delete <id1> <id2> --dry-run
  suno-pp-cli clips delete <id1> --undo`,
		Annotations: map[string]string{"pp:endpoint": "clips.delete", "pp:method": "POST", "pp:path": "/api/feed/trash"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("at least one clip_id is required"))
			}
			verb, pastVerb := "trash", "trashed"
			if undo {
				verb, pastVerb = "restore", "restored"
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would %s %d clip(s): %s\n", verb, len(args), strings.Join(args, ", "))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"ids": args}
			if undo {
				body["undo_trash"] = true
			}
			data, _, err := c.Post(cmd.Context(), "/api/feed/trash", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d clip(s)\n", pastVerb, len(args))
			return nil
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "Restore (un-trash) the clips instead of trashing them")
	return cmd
}
