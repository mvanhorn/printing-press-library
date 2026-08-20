// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

// newJournalCompactCmd reclaims unused space from the SQLite journal database
// and rebuilds the FTS5 indexes from scratch. Useful after large delete
// operations, after schema upgrades that left orphan FTS rows behind, or
// when a corrupted index produces inconsistent search results. The --confirm
// flag is mandatory because a VACUUM rewrites the entire database file and
// briefly holds an exclusive lock; running it accidentally during an active
// session is disruptive.
func newJournalCompactCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var confirm bool

	cmd := &cobra.Command{
		Use:   "compact",
		Short: "Reclaim space and rebuild FTS5 indexes on the journal database",
		Long: `Runs SQLite VACUUM to reclaim unused space and rebuilds the works/sits FTS5
indexes from scratch. Safe to run while the CLI is idle; avoid running it
during an active sit because VACUUM briefly acquires an exclusive lock.

The --confirm flag is mandatory to prevent accidental invocation; VACUUM
rewrites the entire database file and can take a few seconds on large
journals.`,
		Example: `  art-goat-pp-cli journal compact --confirm
  art-goat-pp-cli journal compact --confirm --json`,
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return emitCompactVerifyEnvelope(cmd, flags)
			}
			if !confirm {
				return fmt.Errorf("compact requires --confirm to run")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}
			rebuilt, err := db.CompactAndReindex(cmd.Context())
			if err != nil {
				return fmt.Errorf("compact: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"compacted":      true,
					"fts_rows_built": rebuilt,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "compacted; rebuilt %d FTS rows.\n", rebuilt)
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Explicit acknowledgement that VACUUM will rewrite the database file")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	_ = cmd.MarkFlagRequired("confirm")
	return cmd
}

func emitCompactVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "journal compact",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "compact rewrites the database; PRINTING_PRESS_VERIFY=1 short-circuits the write",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
