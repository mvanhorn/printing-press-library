// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
// pp:client-call shared RunE uses internal/carstay public Directory/Detail/DateCandidates; see carstay_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelSpotsFitCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "fit <id> [second] [third] [fourth] [fifth]"}
	configureCarstayCommand(cmd, flags, "fit", &o)
	cmd.Flags().Float64Var(&o.length, "length-m", 0, "Vehicle length in meters for parking-space dimensional comparison")
	cmd.Flags().Float64Var(&o.width, "width-m", 0, "Vehicle width in meters for parking-space dimensional comparison")
	cmd.Flags().Float64Var(&o.height, "height-m", 0, "Vehicle height in meters; absent source height remains unknown")
	cmd.Flags().StringSliceVar(&o.required, "require", nil, "Required source facility keys; qualified and missing flags need confirmation")
	return cmd
}
