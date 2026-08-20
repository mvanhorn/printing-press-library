// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
)

func newNovelNnumberAvailableCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "available [n-number...]",
		Short: "Check whether N-numbers are assigned, reserved, or free — computed locally, with the reason",
		Long: `Check one or more N-numbers against the local registry: assigned in the
active registry, reserved (with the reservation's purge date), or free. The
FAA's own availability page rejects scripted requests; this answers offline
and batch-scale. Requires a prior sync; availability reflects the last-synced
snapshot.`,
		Example:     "  faa-registry-pp-cli nnumber available N500XA\n  faa-registry-pp-cli nnumber available N1 N101DQ N500XA --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			emitRegistryStaleHint(cmd, db, flags)
			results := make([]*registrydb.Availability, 0, len(args))
			for _, n := range args {
				av, err := db.Available(cmd.Context(), n)
				if err != nil {
					return err
				}
				results = append(results, av)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	return cmd
}
