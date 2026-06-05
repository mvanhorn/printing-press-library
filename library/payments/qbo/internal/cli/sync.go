// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/store"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Synchronize QuickBooks Online ledger tables to local SQLite cache",
		Example: "  qbo-pp-cli sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer s.Close()

			return store.Sync(cmd.Context(), c, s, cmd.OutOrStdout())
		},
	}
	return cmd
}
