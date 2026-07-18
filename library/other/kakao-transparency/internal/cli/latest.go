// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the Kakao Transparency CLI. Carried across
// regen via the novel-command merge path; the generated stub was replaced.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelLatestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "latest",
		Short: "Fetch the newest published half-year report without knowing which period it is",
		Long: "Probes the archive backward from the upcoming half-year until the API answers with a " +
			"published report, then returns that report in full (statistics rows, narrative summary, " +
			"workbook links). Removes the guess-the-period failure mode: the endpoint requires an exact " +
			"year/half-year and answers out-of-range requests with an HTML page instead of a JSON error.",
		Example:     "  github.com/mvanhorn/printing-press-library/library/other/kakao-transparency latest --agent\n  github.com/mvanhorn/printing-press-library/library/other/kakao-transparency latest",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Seed on a period old enough to be certainly published (reports
			// publish with about a half-year lag), then follow each report's
			// own nextYn cursor forward. The walk only ever requests published
			// periods, so it never trips the API's unpublished-period error
			// page or the client's server-error retry pass.
			year, half := kakaoSeedPeriod()
			var newest kakaoReportData
			found := false
			err = walkKakaoArchive(ctx, c, year, half, func(data kakaoReportData) bool {
				newest = data
				found = true
				return true
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !found {
				return apiErr(fmt.Errorf("no published report found from %d onward; the API may have changed", year))
			}
			return printJSONFiltered(cmd.OutOrStdout(), newest, flags)
		},
	}
	return cmd
}
