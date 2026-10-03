// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
// pp:client-call shared RunE uses internal/carstay public Directory/Detail/DateCandidates; see carstay_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelSpotsCompareCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "compare <first> <second> [third] [fourth] [fifth]"}
	configureCarstayCommand(cmd, flags, "compare", &o)
	cmd.Flags().IntVar(&o.textLimit, "text-limit", 2000, "Maximum Unicode characters per source text field; larger rules stay on the canonical page")
	cmd.Flags().StringSliceVar(&o.facilities, "facilities", o.facilities, "Source facility keys to align, such as restroom,electricity,nonFreeShower")
	return cmd
}
