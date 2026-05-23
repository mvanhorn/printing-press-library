package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Walk the vault and reconcile the local SQLite index.",
		Long: "Walk the vault on disk and reconcile the local SQLite index that\n" +
			"powers search, layers stats, lint, and every other command that\n" +
			"reads from the store. Defaults to incremental (mtime-based); use\n" +
			"--full to rebuild from scratch.",
		Example: "  obsidian-pp-cli sync\n  obsidian-pp-cli sync --full --json",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			stats, err := store.Sync(cmd.Context(), vc.S, vc.V, !full)
			if err != nil {
				return apiErr(fmt.Errorf("sync vault: %w", err))
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(stats)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Indexed %d notes (%d new/updated, %d unchanged, %d deleted)\n  facts=%d links=%d tags=%d\n",
				stats.NotesIndexed, stats.NotesUpdated, stats.NotesUnchanged, stats.NotesDeleted,
				stats.FactsIndexed, stats.LinksIndexed, stats.TagsIndexed)
			if len(stats.Errors) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %d errors during sync\n", len(stats.Errors))
				for _, e := range stats.Errors {
					fmt.Fprintf(cmd.ErrOrStderr(), "    - %s\n", e)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Rebuild the index from scratch (default: incremental by mtime)")
	return cmd
}
