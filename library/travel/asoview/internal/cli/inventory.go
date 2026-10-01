// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelInventoryCmd(f *rootFlags) *cobra.Command {
	var kind, query string
	var limit int
	var refresh bool
	cmd := &cobra.Command{Use: "inventory", Short: "Resolve first-party Japanese region and category source IDs", Example: "  asoview-pp-cli inventory --kind categories --query 水族館 --agent\n  asoview-pp-cli inventory --kind regions --query 東京 --refresh-inventory --agent", Annotations: sourceAnnotations("--kind=categories;--query=水族館;--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "inventory")
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("inventory uses --kind and --query"))
		}
		c, ctx, cancel, err := sourceClient(cmd, f)
		if err != nil {
			return err
		}
		defer cancel()
		out, err := c.Inventory(ctx, kind, query, limit, refresh)
		if err != nil {
			return sourceFailure(err)
		}
		return sourceOutput(cmd, f, c, out)
	}}
	cmd.Flags().StringVar(&kind, "kind", "categories", "Taxonomy kind: regions or categories (Japanese source IDs)")
	cmd.Flags().StringVar(&query, "query", "", "Local Japanese name or source-ID substring filter")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matching inventory rows returned, bounded to 1..100")
	cmd.Flags().BoolVar(&refresh, "refresh-inventory", false, "Explicitly retrieve current first-party taxonomy page instead of bundle")
	return cmd
}
