// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelRegistrationFirmCmd(flags *rootFlags) *cobra.Command {
	var flagCrd string
	var flagGroup string
	var flagName string

	cmd := &cobra.Command{
		Use:   "firm",
		Short: "Firm profile lookup by organization CRD",
		Long: "Firm profile lookup by --crd (the firm's organization CRD/ID).\n\n" +
			"A POST query can't guarantee exactly one row without a confirmed unique-key field, so the\n" +
			"full match set is returned under 'records'; when exactly one record matches, it is also\n" +
			"surfaced under 'profile' for convenience.\n\n" +
			"Requires a FINRA credential entitled for firm data — a basic-tier credential will receive a\n" +
			"403 with a clear permission-denied message.",
		Example:     "--crd 1234567 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--crd=1000001", "pp:requires-tier": "entitled"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s/%s firm profile records for CRD %s\n", flagGroup, flagName, flagCrd)
				return nil
			}
			if strings.TrimSpace(flagCrd) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--crd is required"))
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
				// dataset.
				"compareFilters": []map[string]any{
					{"fieldName": "firmCrdNumber", "fieldValue": flagCrd, "compareType": "EQUAL"},
				},
			}
			data, _, err := c.PostQueryWithParams(ctx, path, nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			records, err := parseDatasetRecords(data)
			if err != nil {
				return apiErr(fmt.Errorf("parsing %s/%s response: %w", flagGroup, flagName, err))
			}

			view := registrationFirmView{CRD: flagCrd, Records: records, Count: len(records)}
			if len(records) == 1 {
				view.Profile = records[0]
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "CRD %s: %d matching record(s)\n", view.CRD, view.Count)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCrd, "crd", "", "Firm organization CRD number to look up (required)")
	cmd.Flags().StringVar(&flagGroup, "group", "FIRM", "Dataset group for Firm Profile (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	cmd.Flags().StringVar(&flagName, "name", "FIRMPROFILE", "Dataset name for Firm Profile (confirmed). Run 'finra-pp-cli catalog' to inspect all datasets.")
	return cmd
}

type registrationFirmView struct {
	CRD     string           `json:"crd"`
	Records []map[string]any `json:"records"`
	Count   int              `json:"count"`
	Profile map[string]any   `json:"profile,omitempty"`
}
