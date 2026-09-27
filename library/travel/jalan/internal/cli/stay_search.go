// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

func newStaySearchCmd(flags *rootFlags) *cobra.Command {
	var f stayFlags
	cmd := &cobra.Command{
		Use: "search", Short: "Search dated stays in a supported Japanese or English destination",
		Long:    "Search dated public accommodation inventory with identical occupancy in every room. Destination aliases resolve explicit source areas. Limit defaults to 5 (max 30); at most two source pages per call. Inventory is freshly observed by default; returned prices retain their source units and extra-fee notes.",
		Example: "  jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --adults 2 --limit 5\n  jalan-pp-cli stay search --area-code 136200 --check-in 2026-11-10 --lodging-type hotel --select id,name_ja,price.amount",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--destination=Hakone;--check-in=2026-11-10;--adults=2;--limit=3;--max-age=5m",
		}, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, f.query)
			}
			if len(args) != 0 {
				return stayReportError(cmd, &jalan.Error{Code: "usage", Message: "stay search does not accept positional arguments", Hint: "Use --destination Hakone or --area-code 136200."})
			}
			q, err := stayValidateQuery(cmd, f.query, true)
			if err != nil {
				return stayReportError(cmd, err)
			}
			f.query = q
			return stayCall(cmd, flags, &f, func(ctx context.Context, c stayService) (jalan.Response, error) { return c.Search(ctx, q) })
		},
	}
	cmd.Flags().StringVar(&f.query.Destination, "destination", "", "Supported Japanese/English area alias; see stay locations")
	cmd.Flags().StringVar(&f.query.AreaCode, "area-code", "", "Explicit six-digit Jalan prefecture/large-area code; use instead of --destination")
	addStayPartyFlags(cmd, &f)
	addStayFilterFlags(cmd, &f)
	addStayPageFlags(cmd, &f)
	addStayCacheFlags(cmd, &f)
	return cmd
}
