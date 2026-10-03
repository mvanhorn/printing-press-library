// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newCarstayShowCmd(flags *rootFlags) *cobra.Command {
	o := defaultCarstayOptions()
	cmd := &cobra.Command{Use: "show <id>"}
	configureCarstayCommand(cmd, flags, "show", &o)
	cmd.Flags().IntVar(&o.textLimit, "text-limit", 2000, "Maximum Unicode characters per source text field; larger rules stay on the canonical page")
	return cmd
}
