// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/service"
	"github.com/spf13/cobra"
	"unicode/utf8"
)

func newServicesDiscoverCmd(f *rootFlags) *cobra.Command {
	var query, kind string
	var pages, limit int
	c := &cobra.Command{Use: "discover", Short: "Discover a bounded Japanese guide or experience shortlist", Long: "Read native JAPAN47GO guide/experience category pages and keyword search. Listing records are candidates; inspect details before deciding on request terms. Coverage distinguishes matching inventory from the global site count.", Example: "  japan47go-pp-cli services discover --query 妻籠 --kind guides --max-pages 1 --limit 3 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--query=妻籠;--kind=guides;--max-pages=1;--limit=3"}}
	c.RunE = func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "services discover")
		}
		if len(args) > 0 {
			return serviceError(c, f, usageErr(fmt.Errorf("services discover takes no positional input; use --query")))
		}
		if f.dataSource == "local" {
			return serviceError(c, f, usageErr(fmt.Errorf("services discover is live-only; use services saved for observations")))
		}
		if pages < 1 || pages > 5 || limit < 1 || limit > 50 || (kind != "guides" && kind != "experiences") || utf8.RuneCountInString(query) > 120 {
			return serviceError(c, f, usageErr(fmt.Errorf("--max-pages must be 1..5, --limit 1..50, --kind guides|experiences and --query at most 120 characters")))
		}
		if cliutil.IsDogfoodEnv() && pages > 1 {
			pages = 1
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		out, e := service.NewClient(f.rateLimit).Discover(ctx, service.DiscoverOptions{Query: query, Kind: kind, MaxPages: pages, Limit: limit})
		if e != nil {
			return serviceError(c, f, e)
		}
		return serviceOutput(c, f, out)
	}
	c.Flags().StringVar(&query, "query", "", "Japanese keyword passed to the native source search")
	c.Flags().StringVar(&kind, "kind", "guides", "Native source category: guides or experiences")
	c.Flags().IntVar(&pages, "max-pages", 1, "Maximum listing pages inspected, separate from output, 1..5")
	c.Flags().IntVar(&limit, "limit", 10, "Maximum returned candidates, separate from page coverage, 1..50")
	return c
}
