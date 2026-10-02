// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/yamato"
	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelParcelCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "parcel"}
	var l, w, h, kg float64
	var upright bool
	cmd = luggageCmd(cmd, "Determine luggage category from dimensions and weight; retain acceptance unknowns.", "--length 70 --width 45 --height 30 --weight 23 --agent", "computed", f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parcel")
		}
		if e := validateDataSourceStrategy(f, cmd.Annotations["pp:data-source"]); e != nil {
			return usageErr(e)
		}
		if cmd.Annotations["pp:data-source"] == "computed" && f.dataSource == "live" {
			return usageErr(fmt.Errorf("parcel uses verified local rules; choose --data-source auto or local"))
		}
		if e := noLuggageArgs(args); e != nil {
			return e
		}
		p, e := yamato.Classify(l, w, h, kg, upright)
		if e != nil {
			return usageErr(e)
		}
		return f.printJSON(cmd, map[string]any{"meta": map[string]any{"source": "computed", "timezone": "Asia/Tokyo", "upstream_requests": 0}, "results": p})
	}
	cmd.Flags().Float64Var(&l, "length", 0, "Measured parcel length including handles and wheels, in cm")
	cmd.Flags().Float64Var(&w, "width", 0, "Measured parcel width including protruding parts, in cm")
	cmd.Flags().Float64Var(&h, "height", 0, "Measured parcel height including protruding parts, in cm")
	cmd.Flags().Float64Var(&kg, "weight", 0, "Total packed parcel weight measured in kilograms")
	cmd.Flags().BoolVar(&upright, "upright", false, "Parcel must stay upright; apply 100cm longest-side limit")
	return cmd
}
