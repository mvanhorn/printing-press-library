package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search of locally synced Product Hunt posts",
		Long: strings.Trim(`
Searches the local FTS5 index over synced posts (name + tagline + description).
The store is populated by `+"`producthunt-pp-cli sync`"+`. Empty results before
your first sync are expected.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli search "agentic"
  producthunt-pp-cli search "developer tools" --limit 5 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("producthunt-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			items, err := db.Search(args[0], limit)
			if err != nil {
				return err
			}
			out := []json.RawMessage{}
			for _, raw := range items {
				out = append(out, raw)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results to return")
	return cmd
}
