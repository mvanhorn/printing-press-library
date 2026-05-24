// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newFormulaTestCmd(flags *rootFlags) *cobra.Command {
	var sampleSize int
	var recordID string

	cmd := &cobra.Command{
		Use:         "test <baseId> <tableId> <formula>",
		Short:       "Test an Airtable formula against a sample of records",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fetches a small sample of records (or a single record by --record-id),
runs the formula server-side against each via filterByFormula, and reports
which records would match.`,
		Example: strings.Trim(`
  # Test a formula against 25 sample records
  airtable-pp-cli formula test appXXX tblYYY "AND({Status}='Open', NOT({Owner}=''))"

  # Test against a specific record
  airtable-pp-cli formula test appXXX tblYYY "{Priority}='High'" --record-id recZZZ
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 3 {
				return usageErr(fmt.Errorf("baseId, tableId, and formula are required\nUsage: %s <baseId> <tableId> <formula>", cmd.CommandPath()))
			}
			baseID, tableID, formula := args[0], args[1], args[2]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := replacePathParam("/{baseId}/{tableIdOrName}", "baseId", baseID)
			path = replacePathParam(path, "tableIdOrName", tableID)
			params := map[string]string{
				"filterByFormula": formula,
				"maxRecords":      fmt.Sprintf("%d", sampleSize),
			}
			if recordID != "" {
				// Narrow to a single record by RECORD_ID() guard.
				params["filterByFormula"] = fmt.Sprintf("AND(RECORD_ID()='%s', %s)", recordID, formula)
				params["maxRecords"] = "1"
			}
			data, err := c.Get(cmd.Context(), path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var env struct {
				Records []json.RawMessage `json:"records"`
			}
			if err := json.Unmarshal(data, &env); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}
			report := map[string]any{
				"formula":     formula,
				"sample_size": sampleSize,
				"matched":     len(env.Records),
				"records":     env.Records,
			}
			return flags.printJSON(cmd, report)
		},
	}
	cmd.Flags().IntVar(&sampleSize, "sample-size", 25, "Max records to test against")
	cmd.Flags().StringVar(&recordID, "record-id", "", "Test against a specific record only")
	return cmd
}
