// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
)

func newNovelAircraftHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history [n-number]",
		Short: "Chronological ownership timeline stitching deregistration records with the current registration",
		Long: `Build an aircraft's ownership timeline from the local registry: every
deregistration record for the tail number (with cancel dates and export
country) followed by the current registration when one exists. The FAA
website shows only the current owner; the history lives in the deregistered
file most tools drop. Requires a prior sync.`,
		Example:     "  faa-registry-pp-cli aircraft history N101DQ\n  faa-registry-pp-cli aircraft history N123AB --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := registrydb.ValidTail(args[0]); err != nil {
				return err
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			emitRegistryStaleHint(cmd, db, flags)
			events, err := db.History(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if events == nil {
				events = []registrydb.HistoryEvent{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), events, flags)
		},
	}
	return cmd
}
