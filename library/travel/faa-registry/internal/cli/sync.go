// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/cliutil"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Download the FAA daily Releasable Aircraft Database into the local registry",
		Long: `Download the FAA's daily Releasable Aircraft Database (~73 MB zip, refreshed
each federal working day) and import all of it into the local SQLite registry:
active registrations (MASTER), deregistered aircraft (DEREG), reserved
N-numbers (RESERVED), aircraft model reference (ACFTREF), and engine
reference (ENGINE). Offline commands (fleet report, hex resolve, aircraft
history, expiring, nnumber available, search, watch) read this database.

Re-running is cheap: the download is skipped when the upstream file has not
changed since the last sync (Last-Modified check). Use --force to re-download
and re-import regardless.`,
		Example: "  faa-registry-pp-cli sync\n  faa-registry-pp-cli sync --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync: download", "https://registry.faa.gov/database/ReleasableAircraft.zip")
				return nil
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.Sync(cmd.Context(), registryZipPath(), force, func(msg string) {
				fmt.Fprintln(cmd.ErrOrStderr(), msg)
			})
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Re-download and re-import even when the upstream file is unchanged")
	return cmd
}
