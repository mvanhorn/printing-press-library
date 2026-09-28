// pp:data-source live
package cli

import (
	"fmt"
	"net/url"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/source"
	"github.com/spf13/cobra"
)

// Preserve the explicitly callable advanced source endpoint with the same
// public AJAX headers as the domain lookup, without editing generated files.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		cmd, _, e := root.Find([]string{"catalogs", "suggest"})
		if e != nil || cmd == root || cmd.Name() != "suggest" {
			return
		}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			keyword, e := cmd.Flags().GetString("keyword")
			if e != nil {
				return e
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch public source suggestions")
			}
			if keyword == "" {
				return usageErr(fmt.Errorf("catalogs suggest requires --keyword; use areas QUERY for typed choices"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, e := tripClient(flags)
			if e != nil {
				return tripError(e)
			}
			raw := source.Origin + "/en/suggest/keyword_suggest?keyword=" + url.QueryEscape(keyword)
			body, _, e := c.Fetch(ctx, raw, 15*time.Minute)
			if e != nil {
				return tripError(e)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), body, flags)
		}
	})
}
