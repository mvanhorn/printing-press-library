// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newCarstayDirectoryCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "directory"}
	configureCarstayCommand(cmd, flags, "directory", &o)
	cmd.Flags().IntVar(&o.limit, "limit", 10, "Maximum result or coverage-gap rows, from 1 to 50")
	cmd.Flags().IntVar(&o.maxScan, "max-scan-records", 500, "Maximum directory records to examine independently of returned rows")
	return cmd
}
