// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
	"github.com/spf13/cobra"
)

// PATCH: local watchlist mutation with zero API cost.
func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove [type_id]",
		Short: "Remove a type_id from the watchlist (zero API cost).",
		Example: "  numista-pp-cli watchlist remove 11013\n" +
			"  numista-pp-cli watchlist remove 11013 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			typeID, err := parsePositiveInt64Arg("type_id", args[0])
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.WatchlistRemove(cmd.Context(), typeID); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"type_id": typeID, "status": "removed"}, flags)
		},
	}
}
