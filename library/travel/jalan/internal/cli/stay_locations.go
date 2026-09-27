// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

func newNovelStayLocationsCmd(flags *rootFlags) *cobra.Command {
	var query string
	cmd := &cobra.Command{
		Use: "locations", Short: "Resolve supported Japanese and English destination aliases explicitly",
		Long:        "Read a curated source-linked area catalogue without a network request. English aliases map explicit Japanese areas; this is not machine translation or exhaustive place search. Omit --query to list supported areas; use a source area code for other destinations.",
		Example:     "  jalan-pp-cli stay locations --query Hakone\n  jalan-pp-cli stay locations --query Tokyo",
		Annotations: stayAnnotations("computed", "--query=Hakone"), SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, jalan.Query{})
			}
			if len(args) != 0 {
				return stayReportError(cmd, &jalan.Error{Code: "usage", Message: "stay locations does not accept positional arguments", Hint: "Use --query Hakone."})
			}
			response, err := jalan.Locations(query)
			if err != nil {
				return stayReportError(cmd, err)
			}
			if err := stayPrintResponse(cmd, flags, response); err != nil {
				return stayReportError(cmd, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Japanese or English alias (omit to list source-linked supported areas)")
	return cmd
}
