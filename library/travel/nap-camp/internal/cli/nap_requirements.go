// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/napcamp"
	"github.com/spf13/cobra"
	"math"
)

func napRequirements(cmd *cobra.Command, r *napcamp.Requirements) {
	cmd.Flags().IntVar(&r.People, "people", 0, "Required group size including children; 0 means unspecified")
	cmd.Flags().BoolVar(&r.Power, "power", false, "Require AC power at this specific pitch")
	cmd.Flags().BoolVar(&r.Pets, "pets", false, "Require pets at this specific pitch; source restrictions still apply")
	cmd.Flags().Float64Var(&r.Length, "length-m", 0, "Vehicle length in metres; 0 means unspecified, clearance remains unknown")
	cmd.Flags().Float64Var(&r.Width, "width-m", 0, "Vehicle width in metres; 0 means unspecified, clearance remains unknown")
	cmd.Flags().Float64Var(&r.Height, "height-m", 0, "Vehicle height in metres; 0 means unspecified, clearance remains unknown")
}
func napRequirementError(r napcamp.Requirements) error {
	if r.People < 0 || r.People > 50 {
		return usageErr(fmt.Errorf("--people must be 0..50"))
	}
	for _, n := range []float64{r.Length, r.Width, r.Height} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 30 {
			return usageErr(fmt.Errorf("--length-m, --width-m and --height-m must be finite 0..30 metres"))
		}
	}
	return nil
}
