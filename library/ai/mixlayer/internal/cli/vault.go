// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/shield"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newVaultCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage the local reversible pseudonym vault",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       "List vault tokens",
		Example:     `  mixlayer-pp-cli vault list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.VaultEntries(cmd.Context(), false)
			if err != nil {
				return err
			}
			return outputJSON(cmd, rows)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:         "rehydrate <text-or-file>",
		Short:       "Replace vault tokens with original values locally",
		Example:     `  mixlayer-pp-cli vault rehydrate masked-answer.txt`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			text, err := readTextArg(args[0])
			if err != nil {
				return err
			}
			out, err := shield.Rehydrate(cmd.Context(), s, text)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:         "rotate",
		Short:       "Report rotation support for the current vault",
		Example:     `  mixlayer-pp-cli vault rotate --json`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return outputJSON(cmd, map[string]any{"rotated": false, "reason": "rotation is deferred to preserve existing audit receipts"})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:         "purge",
		Short:       "Delete every local vault entry",
		Example:     `  mixlayer-pp-cli vault purge --yes --json`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flags.yes && !flags.agent {
				return usageErr(fmt.Errorf("pass --yes to purge the vault"))
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.PurgeVault(cmd.Context()); err != nil {
				return err
			}
			return outputJSON(cmd, map[string]any{"purged": true})
		},
	})
	return cmd
}
