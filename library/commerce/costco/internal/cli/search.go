package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco/internal/store"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var db string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search synced line items by description or UPC",
		Long: strings.Trim(`
Search the local archive's line items by description or UPC (case-insensitive
substring), newest first. Run 'sync' first to populate the archive.`, "\n"),
		Example:     "  costco-pp-cli search organic --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search synced line items")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			dbPath := costcoDBPath(db)
			if missingStore(cmd, flags, dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := store.OpenReadOnly(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			out, err := st.SearchItems(ctx, strings.Join(args, " "), limit)
			if err != nil {
				return err
			}
			b, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&db, "db", "", "Archive path (default: per-user data dir)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max items to return")
	return cmd
}
