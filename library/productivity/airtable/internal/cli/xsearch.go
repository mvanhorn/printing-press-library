// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/cliutil"
	"github.com/spf13/cobra"
)

// newXsearchCmd implements cross-table search across all tables in a
// base. The framework `search` command exists and is bound to the local
// FTS path; `xsearch` is the live-API fan-out that the absorb manifest
// promises. Named distinctly so neither collides with the other.
func newXsearchCmd(flags *rootFlags) *cobra.Command {
	var sampleSize int
	var fieldType string

	cmd := &cobra.Command{
		Use:         "xsearch <baseId> <query>",
		Short:       "Cross-table search across every table in a base via the live API",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fans out across every table in the base (via schema cache), parallelizes
with rate-limit awareness, and surfaces matches with table+field context.`,
		Example: strings.Trim(`
  # Search every table in a base for "hello"
  airtable-pp-cli xsearch appXXX "hello"

  # JSON output
  airtable-pp-cli xsearch appXXX "hello" --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("baseId and query are required\nUsage: %s <baseId> <query>", cmd.CommandPath()))
			}
			baseID, query := args[0], args[1]
			_ = fieldType

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: fetch the schema for the base.
			schemaPath := replacePathParam("/meta/bases/{baseId}/tables", "baseId", baseID)
			schemaRaw, err := c.Get(cmd.Context(), schemaPath, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			tables, err := parseSchema(schemaRaw)
			if err != nil {
				return fmt.Errorf("parse schema: %w", err)
			}

			// Step 2: fan out a filterByFormula search across each table.
			type match struct {
				Table   string          `json:"table"`
				TableID string          `json:"table_id"`
				Record  json.RawMessage `json:"record"`
			}
			var matches []match
			results, errs := cliutil.FanoutRun(cmd.Context(), tables, func(t schemaTable) string { return t.ID }, func(ctx context.Context, t schemaTable) ([]match, error) {
				formula := fmt.Sprintf("FIND(LOWER(%q), LOWER(CONCATENATE(ARRAYJOIN(VALUES(),' '))))>0", query)
				path := replacePathParam("/{baseId}/{tableIdOrName}", "baseId", baseID)
				path = replacePathParam(path, "tableIdOrName", t.ID)
				params := map[string]string{
					"filterByFormula": formula,
					"maxRecords":      fmt.Sprintf("%d", sampleSize),
				}
				data, err := c.Get(ctx, path, params)
				if err != nil {
					return nil, err
				}
				var env struct {
					Records []json.RawMessage `json:"records"`
				}
				if err := json.Unmarshal(data, &env); err != nil {
					return nil, err
				}
				out := make([]match, 0, len(env.Records))
				for _, r := range env.Records {
					out = append(out, match{Table: t.Name, TableID: t.ID, Record: r})
				}
				return out, nil
			})
			for _, partial := range results {
				matches = append(matches, partial.Value...)
			}
			if len(errs) > 0 && len(matches) == 0 {
				return classifyAPIError(errs[0].Err, flags)
			}
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			return flags.printJSON(cmd, matches)
		},
	}
	cmd.Flags().IntVar(&sampleSize, "sample-size", 25, "Per-table max records to return")
	cmd.Flags().StringVar(&fieldType, "field-type-filter", "", "Restrict to fields of this type (reserved)")
	return cmd
}

// unused-import guard
var _ = client.BinaryResponseHeader
