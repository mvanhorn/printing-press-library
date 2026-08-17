// Hand-authored offline full-text search over stored offers. Not
// generator-emitted.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newFindCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "find <term>",
		Short:       "Search stored offers offline (full-text over the local store)",
		Long:        "Full-text search over offers already synced into the local store. Offline and free; run 'search' or 'sync' first to populate the store.",
		Example:     "  1688-pp-cli find 硅胶 --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local store")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("search term is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("1688-pp-cli")
			}
			db, err := openLocalStore(ctx, cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return nil
			}
			defer db.Close()

			results, err := db.FindOffers(ctx, args[0], limit)
			if err != nil {
				return err
			}
			if results == nil {
				results = []json.RawMessage{}
			}
			raw, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum matches to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
