// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live -- re-runs the saved report definition against the GAM
// API and streams fresh rows; nothing is read from the local mirror.
func newNovelReportRerunCmd(flags *rootFlags) *cobra.Command {
	var flagDateRange string
	var flagNetwork string
	var flagLimit int
	var flagReportTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "rerun <report-id>",
		Short: "Re-execute an existing saved report definition by ID and stream fresh rows.",
		Long: `Re-run a previously created report by its numeric ID and fetch the fresh rows,
without re-specifying its definition. Skips the create step: it :runs the saved
report "networks/{code}/reports/{report-id}", polls to completion, and fetches
rows. Emits the same {report_id, operation, row_count, rows} envelope as
"report run".

Scope note: the Ad Manager API exposes no update verb for saved reports, so a
saved report's date range cannot be patched here. --date-range is therefore
rejected; to report a different window, create a fresh report with "report run".`,
		Example:     "  google-ad-manager-pp-cli report rerun 1234567",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rerun report <report-id> and fetch rows")
				return nil
			}
			// Bound by the report-timeout (async report runs take minutes), NOT
			// the root --timeout, whose 60s default would cut off polling.
			ctx, cancel := context.WithTimeout(cmd.Context(), flagReportTimeout+30*time.Second)
			defer cancel()

			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("report id required: rerun <report-id>"))
			}
			if strings.TrimSpace(flagDateRange) != "" {
				return usageErr(fmt.Errorf("--date-range cannot be applied to a saved report (no update verb in the API); use \"report run\" to report a different window"))
			}
			code, err := resolveNetworkCode(flagNetwork)
			if err != nil {
				return err
			}

			limit := flagLimit
			if cliutil.IsDogfoodEnv() && limit > 25 {
				limit = 25
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			reportName := networkParent(code) + "/reports/" + strings.TrimSpace(args[0])

			rows, completed, err := runReportAndFetch(ctx, c, reportName, limit, flagReportTimeout)
			if err != nil {
				return err
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"report_id": strings.TrimSpace(args[0]),
				"operation": completed,
				"row_count": len(rows),
				"rows":      rows,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagDateRange, "date-range", "", "Date-range override (not supported for saved reports; see Long help).")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "GAM network code (else $GOOGLE_AD_MANAGER_NETWORK_CODE).")
	cmd.Flags().IntVar(&flagLimit, "limit", 1000, "Maximum number of rows to fetch.")
	cmd.Flags().DurationVar(&flagReportTimeout, "report-timeout", 300*time.Second, "Max time to poll the async report run before giving up. Governs the whole run/fetch and is NOT bounded by --timeout; large reports may need a higher value.")
	return cmd
}
