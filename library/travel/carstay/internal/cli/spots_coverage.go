// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
// pp:client-call shared RunE uses internal/carstay public Directory/Detail/DateCandidates; see carstay_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelSpotsCoverageCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "coverage"}
	configureCarstayCommand(cmd, flags, "coverage", &o)
	cmd.Flags().IntVar(&o.limit, "limit", 10, "Maximum result or coverage-gap rows, from 1 to 50")
	cmd.Flags().IntVar(&o.maxScan, "max-scan-records", 500, "Maximum directory records to examine independently of returned rows")
	cmd.Flags().StringVar(&o.prefecture, "prefecture", "", "Japanese prefecture or English name, such as Yamanashi")
	return cmd
}
