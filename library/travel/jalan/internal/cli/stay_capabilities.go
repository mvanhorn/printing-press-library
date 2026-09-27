// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

func newStayCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "capabilities", Short: "Discover source coverage, supported filters and request limits",
		Long:        "Describe the public accommodation source contract, explicit unsupported actions and observation limits. This discovery command performs no network requests.",
		Example:     "  jalan-pp-cli stay capabilities",
		Annotations: stayAnnotations("computed", ""), SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, jalan.Query{})
			}
			if len(args) != 0 {
				return stayReportError(cmd, &jalan.Error{Code: "usage", Message: "stay capabilities accepts no arguments", Hint: "Run jalan-pp-cli stay capabilities."})
			}
			if err := stayPrintResponse(cmd, flags, jalan.Capabilities()); err != nil {
				return stayReportError(cmd, err)
			}
			return nil
		},
	}
}
