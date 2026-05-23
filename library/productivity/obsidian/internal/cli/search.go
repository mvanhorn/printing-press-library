package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search the synced vault via SQLite FTS5.",
		Long: "Run a full-text search against the local FTS5 index. The index is\n" +
			"populated by `sync`; if you've never run sync, search returns no rows.\n" +
			"Query syntax follows SQLite FTS5: bare tokens AND-combined, prefix\n" +
			"with - to exclude, quote phrases.",
		Example: "  obsidian-pp-cli search buttermilk\n  obsidian-pp-cli search '\"servosity pricing\"' --json --select path,description",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
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
			hits, err := vc.S.Search(cmd.Context(), args[0], limit)
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(hits)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no matches)")
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", h.Path, h.Description)
				if h.Snippet != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", h.Snippet)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of hits to return")
	return cmd
}
