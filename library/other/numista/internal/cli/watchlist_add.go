// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

// PATCH: local watchlist mutation with zero API cost.
func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "add [type_id]",
		Short: "Add a type_id to the watchlist (zero API cost).",
		Example: "  numista-pp-cli watchlist add 11013\n" +
			"  numista-pp-cli watchlist add 11013 --label 'Australia 3 pence George VI'",
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
			if err := s.WatchlistAdd(cmd.Context(), typeID, label); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"type_id": typeID, "label": label, "status": "added"}, flags)
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Label for this type")
	return cmd
}
