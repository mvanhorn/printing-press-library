// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/store"
	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search cached notebooks by title or id (run sync first)",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli sync --json
  notebooklm-pp-cli search "quarterly report" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ensureFreshForCommand(cmd.Context(), flags, cmd.CommandPath())
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON([]nlm.Notebook{})
				}
				dryRunMessage("search local notebook cache")
				return nil
			}
			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			nbs, err := st.SearchNotebooks(args[0])
			if err != nil {
				return err
			}
			if nbs == nil {
				nbs = []nlm.Notebook{}
			}
			if flags.asJSON {
				return printJSON(nbs)
			}
			for _, nb := range nbs {
				fmt.Printf("%s\t%s\n", nb.ID, nb.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite cache path (default: ~/.local/share/notebooklm-pp-cli/cache.db)")
	return cmd
}
