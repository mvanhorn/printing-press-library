// Hand-authored: live catalog search over the Creative Fabrica Algolia index.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var (
		itemType, category, designer, format, sortBy string
		pod, free, onSale, noSub                     bool
		maxPrice                                     float64
		page, limit                                  int
	)
	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "Search the Creative Fabrica catalog with filters the web UI can't express",
		Long: `Search Creative Fabrica's 20M+ catalog with rich filters.

Beyond the web UI, this adds --format (file format: svg/dxf/png/eps/pes, matched
against tags and titles since Creative Fabrica has no format facet) and
--no-subscription (assets usable without an active subscription). Combine any
filters in one call.

Use this command for live catalog discovery. For deepest-discount ranking use
'deals'; for a designer's whole catalog use 'designer'.`,
		Example: strings.Trim(`
  creativefabrica-pp-cli find "watercolor flowers" --limit 10
  creativefabrica-pp-cli find "mandala" --format svg,dxf --pod --agent
  creativefabrica-pp-cli find "valentine" --type Graphics --no-subscription --sort newest
  creativefabrica-pp-cli find "logo" --max-price 2 --on-sale --json --select name,price,url`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			q := catalogQuery{
				itemType: itemType, category: category, designer: designer,
				formats: splitCSV(format), pod: pod, free: free, onSale: onSale,
				noSubscription: noSub, maxPrice: maxPrice, sortBy: sortBy,
				page: page, limit: limit,
			}
			if len(args) > 0 {
				q.query = args[0]
			}
			if q.query == "" && q.itemType == "" && q.category == "" && q.designer == "" &&
				!q.pod && !q.free && !q.onSale && !q.noSubscription && q.maxPrice == 0 && len(q.formats) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a query or at least one filter (e.g. --type, --category, --free)"))
			}
			return runCatalogSearch(cmd, flags, q)
		},
	}
	cmd.Flags().StringVar(&itemType, "type", "", "Product type (Graphics, Fonts, Crafts, Embroidery, Laser Cutting, 3D SVG, Bundles, ...)")
	cmd.Flags().StringVar(&category, "category", "", "Category facet (see 'categories'), e.g. Icons, Patterns, T-shirt Designs")
	cmd.Flags().StringVar(&designer, "designer", "", "Restrict to a designer (numeric id or exact name)")
	cmd.Flags().StringVar(&format, "format", "", "File format(s), comma-separated: svg,dxf,png,eps,pes (matched in tags/titles)")
	cmd.Flags().BoolVar(&pod, "pod", false, "Only print-on-demand / commercial-license assets")
	cmd.Flags().BoolVar(&free, "free", false, "Only free assets")
	cmd.Flags().BoolVar(&onSale, "on-sale", false, "Only assets currently on promotion")
	cmd.Flags().BoolVar(&noSub, "no-subscription", false, "Only assets usable without an active subscription")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price (0 = no limit)")
	cmd.Flags().StringVar(&sortBy, "sort", "relevance", "Sort order: relevance | newest")
	cmd.Flags().IntVar(&page, "page", 0, "Result page (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results to return")
	return cmd
}
