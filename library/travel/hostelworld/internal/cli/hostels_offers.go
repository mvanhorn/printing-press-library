// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/spf13/cobra"
)

func newNovelHostelsOffersCmd(flags *rootFlags) *cobra.Command {
	var o stayOptions
	cmd := &cobra.Command{Use: "offers [property-id]", Short: "Fetch dated dorm and private plans with explicit price units and source terms", Example: "  " + "hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent", Annotations: map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "pp:live-happy-path": "true", "pp:data-source": "live", "pp:happy-args": "id=67481;--check-in=2026-10-04;--check-out=2026-10-07;--guests=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || !hw.ValidID(args[0]) {
			return usageErr(fmt.Errorf("offers requires one numeric property ID"))
		}
		q, err := o.query()
		if err != nil {
			return usageErr(err)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		p, err := propertyObject(ctx, c, args[0])
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		v, err := offersObject(ctx, c, args[0], p, q, o)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		limitPlans(v, o.limit)
		return outputPlanning(cmd, flags, v, o.save)
	}}
	decoratePlanning(cmd, flags)
	stayFlags(cmd, &o)
	return cmd
}
