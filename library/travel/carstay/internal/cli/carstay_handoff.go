// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newCarstayHandoffCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "handoff <id>"}
	configureCarstayCommand(cmd, flags, "handoff", &o)
	cmd.Flags().StringVar(&o.language, "lang", "ja", "Published discovery subset or handoff language: ja (fuller) or en (English-approved only)")
	cmd.Flags().StringVar(&o.checkIn, "check-in", "", "Japan-local check-in date in YYYY-MM-DD; provide check-out too")
	cmd.Flags().StringVar(&o.checkOut, "check-out", "", "Japan-local check-out date later than check-in in YYYY-MM-DD")
	return cmd
}
