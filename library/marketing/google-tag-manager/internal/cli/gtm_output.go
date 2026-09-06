// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared output + mirror-access helpers for the read-only GTM commands.
// Hand-authored.

package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-tag-manager/internal/store"
	"github.com/spf13/cobra"
)

func gtmDBPath(dbPath string) string {
	if dbPath != "" {
		return dbPath
	}
	return defaultDBPath("google-tag-manager-pp-cli")
}

// gtmEmit writes v as JSON (honoring --select/--compact/--csv) when machine
// output is requested, otherwise renders the human table.
func gtmEmit(cmd *cobra.Command, flags *rootFlags, v any, table func(io.Writer)) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}
	table(cmd.OutOrStdout())
	return nil
}

// gtmReadSnapshots opens the local mirror READ-ONLY and returns every
// snapshot, newest-first. Read-only opens never take a write lock, so several
// query commands can run against one mirror concurrently. When the mirror is
// absent or empty it prints a sync hint, emits an empty JSON array for machine
// consumers, and returns ok=false so the caller can return nil (an empty
// mirror is a state, not an error).
func gtmReadSnapshots(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, []gtmSnapshot, bool, error) {
	noMirror := func() {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no local mirror at %s\nrun: google-tag-manager-pp-cli pull --account <id> --container <id> --live --db %s\n",
			dbPath, dbPath)
		if flags.asJSON {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		noMirror()
		return nil, nil, false, nil
	}
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, nil, false, err
	}
	snaps, err := listSnapshots(ctx, s.DB())
	if err != nil {
		// A DB that exists but was never pulled has no gtm_snapshot table;
		// treat that as an empty mirror rather than a hard error.
		s.Close()
		noMirror()
		return nil, nil, false, nil
	}
	if len(snaps) == 0 {
		s.Close()
		noMirror()
		return nil, nil, false, nil
	}
	return s, snaps, true, nil
}
