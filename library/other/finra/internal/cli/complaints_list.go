// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/finra/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelComplaintsListCmd(flags *rootFlags) *cobra.Command {
	var flagFirm string
	var flagSince string
	var flagGroup string
	var flagName string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Full 4530 customer complaint filing history for a firm",
		Long: "Full 4530 customer complaint filing history for --firm.\n\n" +
			"--since optionally narrows to a trailing window (e.g. 24h, 7d, 1w); omit it for the full\n" +
			"history. Use this for the full history; use 'complaints new' to see only filings that\n" +
			"appeared since your last sync instead.\n\n" +
			"Requires a FINRA credential entitled for firm data — a basic-tier credential will receive a\n" +
			"403 with a clear permission-denied message.",
		Example:     "--firm 19847 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--firm=19847", "pp:requires-tier": "entitled"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s filings for firm %s\n", flagGroup, flagName, flagFirm)
				return nil
			}
			if strings.TrimSpace(flagFirm) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--firm is required"))
			}

			var window time.Duration
			var hasWindow bool
			if strings.TrimSpace(flagSince) != "" {
				w, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				window, hasWindow = w, true
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := replacePathParam(replacePathParam("/data/group/{group}/name/{name}", "group", flagGroup), "name", flagName)
			body := map[string]any{
				// firmCrdNumber is the confirmed identifier field for this
				// dataset. No date filter is sent server-side: the
				// filing-date field name is unconfirmed, so --since (when
				// provided) is applied client-side below instead via
				// findRecordDate/filterRecordsByDateWindow.
				"compareFilters": []map[string]any{
					{"fieldName": "firmCrdNumber", "fieldValue": flagFirm, "compareType": "EQUAL"},
				},
			}
			data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			filings, err := parseDatasetRecords(data)
			if err != nil {
				return apiErr(fmt.Errorf("parsing %s/%s response: %w", flagGroup, flagName, err))
			}

			if hasWindow {
				now := time.Now().UTC()
				// submissionDate is the confirmed filing-date field for
				// 4530FILINGS (per /metadata); preferred over the generic
				// date-key scan since a filing record also carries
				// discoveryDate, and picking the wrong one alphabetically
				// would filter by the wrong semantic date.
				filings = filterRecordsByDateWindowPreferField(filings, "submissionDate", now.Add(-window), now)
			}

			view := complaintsListView{FirmCRD: flagFirm, Filings: filings, Count: len(filings)}
			if hasWindow {
				view.Since = flagSince
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%d filing(s) for firm %s\n", view.Count, view.FirmCRD)
				for _, f := range filings {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", summarizeFiling(f))
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagFirm, "firm", "", "Firm CRD number to list 4530 customer complaint filings for (required)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Optionally narrow to a trailing window (e.g. 24h, 7d, 1w); omit for full history")
	cmd.Flags().StringVar(&flagGroup, "group", "FIRM", "Dataset group for 4530 Customer Complaints (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "4530FILINGS", "Dataset name for 4530 Customer Complaints (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	return cmd
}

type complaintsListView struct {
	FirmCRD string           `json:"firm_crd"`
	Since   string           `json:"since,omitempty"`
	Filings []map[string]any `json:"filings"`
	Count   int              `json:"count"`
}
