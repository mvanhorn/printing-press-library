// pp:data-source auto
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/asoview/internal/asoview"
	"github.com/spf13/cobra"
)

func newNovelCompareCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "compare <id> <id> [id...]", Short: "Compare terms and advertised bands for two to five products", Example: "  asoview-pp-cli compare ticket0000049223 ticket0000012233 --agent", Annotations: sourceAnnotations("id=ticket0000049223;id=ticket0000012233;--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "compare")
		}
		ids, err := joinedIDs(args)
		if err != nil {
			return err
		}
		c, ctx, cancel, err := sourceClient(cmd, f)
		if err != nil {
			return err
		}
		defer cancel()
		results := []asoview.Object{}
		for _, id := range ids {
			p, e := c.Product(ctx, id, false)
			if e != nil {
				return sourceFailure(e)
			}
			delete(p, "related_products")
			delete(p, "location")
			results = append(results, p)
		}
		return sourceOutput(cmd, f, c, asoview.Object{"results": results, "comparison": asoview.Object{"price_basis": "advertised_bands_only", "date_party_total": nil, "terms_may_differ": true, "complete": true}})
	}}
	return cmd
}
