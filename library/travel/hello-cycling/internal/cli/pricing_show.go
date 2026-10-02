// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelPricingShowCmd(f *rootFlags) *cobra.Command {
	var area string
	c := &cobra.Command{Use: "show", Short: "Read published area/model price bands and municipality exceptions.", Example: "  hello-cycling-pp-cli pricing show --area tokyo --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}
	c.Flags().StringVar(&area, "area", "", "Advertised price area slug from pricing areas, for example tokyo")
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcLiveOnly(f); e != nil {
			return e
		}
		if area == "" || len(area) > 40 || strings.ContainsAny(area, "/?#.") {
			return usageErr(fmt.Errorf("--area must be an advertised slug; use pricing areas"))
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "pricing show")
		}
		ctx, cancel := hcContext(c, f)
		defer cancel()
		out, e := cycling.NewClient().Prices(ctx, area)
		if e != nil {
			return hcErr(e)
		}
		return hcPrint(c, f, out)
	}
	return c
}
