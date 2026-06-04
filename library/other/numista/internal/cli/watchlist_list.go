// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
	"github.com/spf13/cobra"
)

// PATCH: local watchlist read with zero API cost.
func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list [--json]",
		Short: "List all watched type_ids (zero API cost).",
		Example: "  numista-pp-cli watchlist list --json\n" +
			"  numista-pp-cli watchlist list --compact",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			entries, err := s.WatchlistList(cmd.Context())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}
}
