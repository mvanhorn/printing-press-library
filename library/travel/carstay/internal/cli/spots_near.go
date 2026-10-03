// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
// pp:client-call shared RunE uses internal/carstay public Directory/Detail/DateCandidates; see carstay_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelSpotsNearCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "near"}
	configureCarstayCommand(cmd, flags, "near", &o)
	cmd.Flags().StringVar(&o.language, "lang", "ja", "Published discovery subset or handoff language: ja (fuller) or en (English-approved only)")
	cmd.Flags().IntVar(&o.limit, "limit", 10, "Maximum result or coverage-gap rows, from 1 to 50")
	cmd.Flags().IntVar(&o.maxScan, "max-scan-records", 500, "Maximum directory records to examine independently of returned rows")
	cmd.Flags().StringVar(&o.prefecture, "prefecture", "", "Japanese prefecture or English name, such as Yamanashi")
	cmd.Flags().Float64Var(&o.lat, "lat", 0, "Itinerary point latitude in decimal degrees, required")
	cmd.Flags().Float64Var(&o.lon, "lon", 0, "Itinerary point longitude in decimal degrees, required")
	cmd.Flags().Float64Var(&o.radius, "radius-km", 50, "Straight-line search radius in kilometers, greater than zero up to 2000")
	_ = cmd.MarkFlagRequired("lat")
	_ = cmd.MarkFlagRequired("lon")
	return cmd
}
