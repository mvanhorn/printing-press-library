// Provider-specific domain parser replaces the generated HTML text extraction.
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/onsen"
	"github.com/spf13/cobra"
)

// pp:data-source auto
func newBathSearchCmd(f *rootFlags) *cobra.Command {
	var query, region string
	var filters []string
	var page, limit int
	var all bool
	cmd := &cobra.Command{Use: "search", Short: "Find organic day-use listings by prefecture, keyword and source filters", Long: "Fetch one source page without detail requests. Defaults to the source day-use classification. Missing facts stay null. --limit truncates this page; --page selects the next source page and does not continue a truncated page.", Example: "  nifty-onsen-pp-cli bath search --region=tokyo --filter=sauna --limit=5 --agent\n  nifty-onsen-pp-cli bath search --query=草津 --page=2 --select=id,name,url --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--region=tokyo;--limit=3"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "GET one public Nifty search page")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("search takes flags; use --query=草津"))
		}
		if e := checkOnsenLimit(limit, 30); e != nil {
			return e
		}
		o := onsen.SearchOptions{Region: region, Query: query, Page: page, Filters: onsenFilters(filters, all)}
		if _, e := onsen.SearchURL(o); e != nil {
			return usageErr(e)
		}
		c, e := onsenClient(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		v, p, e := c.Search(ctx, o)
		if e != nil {
			return e
		}
		count := len(v.Items)
		if count > limit {
			v.Items = v.Items[:limit]
		}
		next := any(nil)
		if v.NextURL != nil {
			next = page + 1
		}
		coverage := map[string]any{"scope": "one organic source page", "exhaustive": false, "page": page, "source_count": count, "returned": len(v.Items), "source_total": v.Total, "next_page": next, "next_url": v.NextURL, "truncated": count > limit, "note": "Source ranking; excludes sponsored cards. Truncated page items are omitted; raise --limit to 30 before changing --page. Listings do not establish live admission or capacity."}
		return onsenOutput(cmd, f, v.Items, p, coverage)
	}}
	cmd.Flags().StringVar(&query, "query", "", "Japanese facility or place keywords, up to 120 characters")
	cmd.Flags().StringVar(&region, "region", "", "Source prefecture slug or Japanese name; see regions")
	cmd.Flags().StringSliceVar(&filters, "filter", nil, "Comma-separated source filters; see filters for meanings and caveats")
	cmd.Flags().BoolVar(&all, "all-types", false, "Include all source facility types by omitting default day-use filter")
	cmd.Flags().IntVar(&page, "page", 1, "Source page number to fetch, from 1 to 100")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum organic rows returned from this page, from 1 to 30")
	return cmd
}
