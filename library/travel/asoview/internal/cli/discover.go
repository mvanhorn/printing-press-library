// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/asoview/internal/asoview"
	"github.com/spf13/cobra"
)

func newNovelDiscoverCmd(f *rootFlags) *cobra.Command {
	p := asoview.SearchParams{}
	cmd := &cobra.Command{Use: "discover", Aliases: []string{"search"}, Short: "Find leisure products by region, category, date and party", Long: "Native source filters narrow discovery. --query is a local substring over bounded scanned cards, not a global keyword search. A date/party filter does not confirm a booking or final price.", Example: "  asoview-pp-cli discover --region prf130000 --category 192 --limit 5 --agent\n  asoview-pp-cli discover --region 東京都 --category 水族館 --query 葛西 --agent", Annotations: sourceAnnotations("--region=prf130000;--category=192;--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "discover")
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("use --query for local text filtering"))
		}
		c, ctx, cancel, err := sourceClient(cmd, f)
		if err != nil {
			return err
		}
		defer cancel()
		out, err := c.Discover(ctx, p)
		if err != nil {
			return sourceFailure(err)
		}
		return sourceOutput(cmd, f, c, out)
	}}
	cmd.Flags().StringVar(&p.Region, "region", "", "Source region ID or exact Japanese name; resolve with inventory")
	cmd.Flags().StringVar(&p.Category, "category", "", "Source category/genre ID or exact Japanese name")
	cmd.Flags().StringVar(&p.Query, "query", "", "Local title/base/category substring over scanned source cards")
	cmd.Flags().StringVar(&p.Kind, "kind", "", "Local product kind filter: ticket or activity")
	cmd.Flags().StringVar(&p.Date, "date", "", "Native candidate date filter in YYYY-MM-DD, Japan time")
	cmd.Flags().StringVar(&p.Cursor, "cursor", "", "Continue from the returned opaque page:offset cursor")
	cmd.Flags().IntVar(&p.Page, "page", 1, "Source starting page, bounded to 1..200")
	cmd.Flags().IntVar(&p.Pages, "pages", 1, "Maximum source pages to scan, bounded to 1..5")
	cmd.Flags().IntVar(&p.Limit, "limit", 10, "Maximum matching products returned, bounded to 1..50")
	cmd.Flags().IntVar(&p.Adults, "adults", 1, "Native search adult quantity; total party bounded to 50")
	cmd.Flags().IntVar(&p.Children, "children", 0, "Native search child quantity; ages are verified in options")
	return cmd
}
