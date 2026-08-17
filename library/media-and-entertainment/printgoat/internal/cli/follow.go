// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command family: `follow designer add|list`. No `follow` parent
// command existed in root.go, so it is added here and wired in root.go
// alongside the other novel command families.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

func newNovelFollowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "follow",
		Short:       "follow subcommands: designer",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelFollowDesignerCmd(flags))
	return cmd
}

func newNovelFollowDesignerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "designer",
		Short:       "designer subcommands: add, list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelFollowDesignerAddCmd(flags))
	cmd.AddCommand(newNovelFollowDesignerListCmd(flags))
	return cmd
}

func newNovelFollowDesignerAddCmd(flags *rootFlags) *cobra.Command {
	var flagAs string

	cmd := &cobra.Command{
		Use:         "add <source> <handle>",
		Short:       "Follow a designer on a given source so 'feed' reports their new uploads.",
		Example:     "  printgoat-pp-cli follow designer add thingiverse PrintedSolid --as printed-solid --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("follow designer add requires <source> and <handle>\nUsage: %s <source> <handle> [--as <alias-group>]", cmd.CommandPath()))
			}
			source, handle := args[0], args[1]
			if !validSource(source) {
				return usageErr(fmt.Errorf("invalid source %q: expected printables, thingiverse, or cults3d", source))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			var aliasArg sql.NullString
			if flagAs != "" {
				aliasArg = sql.NullString{String: flagAs, Valid: true}
			}
			if _, err := s.DB().ExecContext(ctx,
				`INSERT INTO printgoat_designer_links (alias_group, source, handle) VALUES (?, ?, ?)
				 ON CONFLICT(source, handle) DO UPDATE SET alias_group = excluded.alias_group`,
				aliasArg, source, handle,
			); err != nil {
				return fmt.Errorf("saving followed designer: %w", err)
			}

			out := map[string]any{"source": source, "handle": handle, "status": "followed"}
			if flagAs != "" {
				out["alias_group"] = flagAs
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagAs, "as", "", "Group this designer under an alias, so the same person's accounts across sites can be tracked together")
	return cmd
}

type followedDesigner struct {
	AliasGroup string `json:"alias_group,omitempty"`
	Source     string `json:"source"`
	Handle     string `json:"handle"`
}

func newNovelFollowDesignerListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all followed designers.",
		Example:     "  printgoat-pp-cli follow designer list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			rows, err := s.DB().QueryContext(ctx, `SELECT alias_group, source, handle FROM printgoat_designer_links ORDER BY alias_group, source, handle`)
			if err != nil {
				return fmt.Errorf("reading followed designers: %w", err)
			}
			defer rows.Close()

			var followed []followedDesigner
			for rows.Next() {
				var d followedDesigner
				var alias sql.NullString
				if err := rows.Scan(&alias, &d.Source, &d.Handle); err != nil {
					return fmt.Errorf("scanning followed designers: %w", err)
				}
				d.AliasGroup = alias.String
				followed = append(followed, d)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading followed designers: %w", err)
			}

			out := map[string]any{"followed": followed, "count": len(followed)}
			if len(followed) == 0 {
				out["message"] = "not following any designers yet; use 'follow designer add <source> <handle>'"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
