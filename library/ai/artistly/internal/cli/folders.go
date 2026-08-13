// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `folders` manages design folders. Contracts: list from Inertia
// shared props; create = POST /designs/folder {folder_name}; rename = PUT
// /designs/folder/{id} {folder_name}; remove = DELETE /designs/folder/{id}.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newFoldersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "folders",
		Short:       "List and manage your design folders",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFoldersListCmd(flags))
	cmd.AddCommand(newFoldersCreateCmd(flags))
	cmd.AddCommand(newFoldersRenameCmd(flags))
	cmd.AddCommand(newFoldersRemoveCmd(flags))
	return cmd
}

func newFoldersListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your design folders (id and name)",
		Example:     "  artistly-pp-cli folders list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			folders, err := foldersFromProps(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), folders, flags)
			}
			if len(folders) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No folders.")
				return nil
			}
			for _, f := range folders {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10d %s\n", f.ID, f.Name)
			}
			return nil
		},
	}
	return cmd
}

func newFoldersCreateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "create <name>",
		Short:       "Create a new folder",
		Example:     "  artistly-pp-cli folders create \"Client Logos\"",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return writeNoop(flags, "dry_run", "would create folder")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a folder name is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := writeCall(ctx, c, "POST", "/designs/folder", map[string]any{"folder_name": args[0]}); err != nil {
				return classifyAPIError(err, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created folder %q. Run 'folders list' to see its id.\n", args[0])
			return nil
		},
	}
	return cmd
}

func newFoldersRenameCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rename <folder-id> <new-name>",
		Short:       "Rename a design folder by id to a new name",
		Example:     "  artistly-pp-cli folders rename 155902 \"Archived\"",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return writeNoop(flags, "dry_run", "would rename folder")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("folder id and new name are required"))
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("folder id must be a number"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := writeCall(ctx, c, "PUT", fmt.Sprintf("/designs/folder/%d", id), map[string]any{"folder_name": args[1]}); err != nil {
				return classifyAPIError(err, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Renamed folder %d to %q.\n", id, args[1])
			return nil
		},
	}
	return cmd
}

func newFoldersRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove <folder-id>",
		Short:       "Delete a folder (destructive). Requires --yes.",
		Example:     "  artistly-pp-cli folders remove 155902 --yes",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return writeNoop(flags, "dry_run", "would remove folder")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a folder id is required"))
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("removing a folder is destructive; pass --yes to confirm"))
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("folder id must be a number"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := writeCall(ctx, c, "DELETE", fmt.Sprintf("/designs/folder/%d", id), nil); err != nil {
				return classifyAPIError(err, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed folder %d.\n", id)
			return nil
		},
	}
	return cmd
}
