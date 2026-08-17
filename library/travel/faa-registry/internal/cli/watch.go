// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch owners or tail numbers for registration changes across syncs",
		Long: `Maintain a watch list of owner names and tail numbers. After each sync, run
"watch check" to see registrations added, removed, or changed since the last
check — the scripted version of the FAA's paper mail.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchCheckCmd(flags))
	return cmd
}

func watchKindValue(owner, tail string) (string, string, error) {
	switch {
	case owner != "" && tail != "":
		return "", "", fmt.Errorf("pass exactly one of --owner or --tail, not both")
	case owner != "":
		return "owner", owner, nil
	case tail != "":
		return "tail", tail, nil
	default:
		return "", "", fmt.Errorf("pass --owner NAME or --tail N-NUMBER")
	}
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var owner, tail string
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add an owner or tail number to the watch list",
		Example: "  faa-registry-pp-cli watch add --owner \"NETJETS SALES INC\"\n  faa-registry-pp-cli watch add --tail N101DQ",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "would_add": map[string]string{"owner": owner, "tail": tail}}, flags)
			}
			kind, value, err := watchKindValue(owner, tail)
			if err != nil {
				return err
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			w, err := db.AddWatch(cmd.Context(), kind, value)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), w, flags)
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "", "Owner name to watch (prefix match, e.g. \"NETJETS SALES\")")
	cmd.Flags().StringVar(&tail, "tail", "", "Tail number to watch, e.g. N101DQ")
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var owner, tail string
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove an owner or tail number from the watch list",
		Example: "  faa-registry-pp-cli watch remove --tail N101DQ",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "would_remove": map[string]string{"owner": owner, "tail": tail}}, flags)
			}
			kind, value, err := watchKindValue(owner, tail)
			if err != nil {
				return err
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.RemoveWatch(cmd.Context(), kind, value); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]string{"removed": value, "kind": kind}, flags)
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "", "Owner name watch to remove")
	cmd.Flags().StringVar(&tail, "tail", "", "Tail number watch to remove")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List every owner and tail-number watch currently registered, with kind and watched value",
		Example:     "  faa-registry-pp-cli watch list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			ws, err := db.ListWatches(cmd.Context())
			if err != nil {
				return err
			}
			if ws == nil {
				ws = []registrydb.Watch{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), ws, flags)
		},
	}
	return cmd
}

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Diff every watch against its last snapshot and report changes",
		Long: `Compare each watch's current registry rows against the snapshot stored by the
previous check. The first check after adding a watch records a baseline and
reports no changes. Run sync first so the comparison sees fresh data.`,
		Example: "  faa-registry-pp-cli watch check\n  faa-registry-pp-cli watch check --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			reports, err := db.CheckWatches(cmd.Context())
			if err != nil {
				return err
			}
			if reports == nil {
				reports = []registrydb.WatchReport{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), reports, flags)
		},
	}
	return cmd
}
