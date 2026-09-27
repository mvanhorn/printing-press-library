// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

func newStayOffersCmd(flags *rootFlags) *cobra.Command {
	var f stayFlags
	cmd := &cobra.Command{
		Use: "offers <property-id>", Short: "List observed room and plan relationships for one dated stay",
		Long:        "Inspect dated room/plan inventory for one property. Distinct plan/room pairs remain separate. Source listings may not establish exact room facts; use stay plan for those. Defaults: 5 results, one night, two adults per room; at most two source pages.",
		Example:     "  jalan-pp-cli stay offers 385995 --check-in 2026-11-10 --adults 2 --limit 5",
		Annotations: stayAnnotations("live", "<property-id>=385995;--check-in=2026-11-10;--adults=2;--limit=3;--max-age=5m"), SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, f.query)
			}
			id, err := stayPropertyID(args)
			if err != nil {
				return stayReportError(cmd, err)
			}
			q, err := stayValidateQuery(cmd, f.query, false)
			if err != nil {
				return stayReportError(cmd, err)
			}
			if _, err := jalan.ValidateOffersQuery(q); err != nil {
				return stayReportError(cmd, err)
			}
			f.query = q
			return stayCall(cmd, flags, &f, func(ctx context.Context, c stayService) (jalan.Response, error) { return c.Offers(ctx, id, q) })
		},
	}
	addStayPartyFlags(cmd, &f)
	addStayOfferFilterFlags(cmd, &f)
	addStayPageFlags(cmd, &f)
	addStayCacheFlags(cmd, &f)
	return cmd
}
