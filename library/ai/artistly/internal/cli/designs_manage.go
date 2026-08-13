// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `designs delete` and `designs move` — contracts captured from a
// user HAR: delete = POST /api/remove-design {uuid, selectedImage, user_id};
// move = POST /change-folder {folder, uuid, design_id}.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newDesignsDeleteCmd(flags *rootFlags) *cobra.Command {
	var image int
	cmd := &cobra.Command{
		Use:   "delete <id|uuid>",
		Short: "Delete a design image (destructive). Requires --yes.",
		Long: trimLong(`
Delete a design's image by design id or uuid. This is destructive and permanent.
For a single-image design, the default --image 0 removes the design. Requires
--yes (or --agent) to run non-interactively.`),
		Example:     "  artistly-pp-cli designs delete ml9ec9e6-034c-73ee-9358-ef7854a022db --yes",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return writeNoop(flags, "dry_run", "would delete design image")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a design id or uuid is required"))
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("delete is destructive; pass --yes to confirm"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			d, err := findDesign(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			body := map[string]any{"uuid": d.UUID, "selectedImage": image, "user_id": d.UserID}
			if err := writeCall(ctx, c, "POST", "/api/remove-design", body); err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": d.UUID, "image": image}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted image %d of design %s.\n", image, d.UUID)
			return nil
		},
	}
	cmd.Flags().IntVar(&image, "image", 0, "Index of the image to remove (default 0)")
	return cmd
}

func newDesignsMoveCmd(flags *rootFlags) *cobra.Command {
	var folder int
	cmd := &cobra.Command{
		Use:   "move <id|uuid> --folder <folder-id>",
		Short: "Move a design into a folder",
		Long: trimLong(`
Move a design (by id or uuid) into a folder. Find folder ids with
'artistly-pp-cli folders list'.`),
		Example:     "  artistly-pp-cli designs move 57628746 --folder 155902",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return writeNoop(flags, "dry_run", "would move design to folder")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a design id or uuid is required"))
			}
			if !cmd.Flags().Changed("folder") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--folder <folder-id> is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// /change-folder accepts design_id alone (verified), so a numeric id
			// works for any design without needing it in the recent list. A uuid
			// is resolved to its design_id via the recent list.
			body := map[string]any{"folder": fmt.Sprintf("%d", folder)}
			var movedID int
			if idNum, aerr := strconv.Atoi(strings.TrimSpace(args[0])); aerr == nil {
				body["design_id"] = idNum
				movedID = idNum
			} else {
				d, derr := findDesign(ctx, c, args[0])
				if derr != nil {
					return classifyAPIError(derr, flags)
				}
				body["design_id"] = d.ID
				body["uuid"] = d.UUID
				movedID = d.ID
			}
			if err := writeCall(ctx, c, "POST", "/change-folder", body); err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"moved": movedID, "folder": folder}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Moved design %d to folder %d.\n", movedID, folder)
			return nil
		},
	}
	cmd.Flags().IntVar(&folder, "folder", 0, "Destination folder id (required)")
	return cmd
}
