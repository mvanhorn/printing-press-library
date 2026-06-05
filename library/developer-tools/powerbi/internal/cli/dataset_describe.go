// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newDatasetDescribeCmd(flags *rootFlags) *cobra.Command {
	var group string
	cmd := &cobra.Command{
		Use:   "describe <dataset-id>",
		Short: "Best-effort schema introspection: list tables, columns, and measures in a dataset",
		Long: `Probe a dataset's structure via DAX INFO functions and fall back to dataset
metadata when those functions are not available.

INFO.TABLES(), INFO.COLUMNS(), and INFO.MEASURES() are only available on
Power BI Premium / Microsoft Fabric capacities (per the executeQueries
documentation). On Pro-only datasets, this command surfaces the dataset's
configuration block and datasources instead, with a clear note about what
could not be introspected.`,
		Example: `  powerbi-pp-cli datasets describe DATASET_ID --group W
  powerbi-pp-cli datasets describe DATASET_ID --group W --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			datasetID := args[0]
			if group == "" {
				return usageErr(fmt.Errorf("--group is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result := map[string]any{
				"dataset_id": datasetID,
				"group_id":   group,
			}

			// Best-effort: INFO.TABLES + INFO.COLUMNS + INFO.MEASURES via executeQueries.
			// All three may return 400 "INFO functions not supported" on Pro datasets — we
			// catch that and fall back gracefully.
			tables, tablesErr := runInfoDAX(c, group, datasetID, "SELECTCOLUMNS(INFO.TABLES(), \"name\", [Name], \"description\", [Description])")
			if tablesErr == nil {
				result["tables"] = tables
			}
			columns, columnsErr := runInfoDAX(c, group, datasetID, "SELECTCOLUMNS(INFO.COLUMNS(), \"table_id\", [TableID], \"name\", [ExplicitName], \"data_type\", [ExplicitDataType])")
			if columnsErr == nil {
				result["columns"] = columns
			}
			measures, measuresErr := runInfoDAX(c, group, datasetID, "SELECTCOLUMNS(INFO.MEASURES(), \"name\", [Name], \"expression\", [Expression])")
			if measuresErr == nil {
				result["measures"] = measures
			}

			if tablesErr != nil && columnsErr != nil && measuresErr != nil {
				result["info_functions"] = "unavailable"
				result["info_functions_reason"] = "INFO.TABLES/COLUMNS/MEASURES require Power BI Premium or Microsoft Fabric. This dataset appears to be on a Pro-only capacity."
				// Fall back: pull dataset metadata + datasources.
				dsRaw, err := c.Get(fmt.Sprintf("/groups/%s/datasets/%s", group, datasetID), nil)
				if err == nil {
					var meta map[string]any
					_ = json.Unmarshal(dsRaw, &meta)
					result["dataset"] = meta
				}
				srcRaw, err := c.Get(fmt.Sprintf("/groups/%s/datasets/%s/datasources", group, datasetID), nil)
				if err == nil {
					var src map[string]any
					_ = json.Unmarshal(srcRaw, &src)
					result["datasources"] = src
				}
			} else {
				result["info_functions"] = "available"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Human-friendly output.
			fmt.Fprintf(cmd.OutOrStdout(), "Dataset %s (workspace %s)\n", datasetID, group)
			if result["info_functions"] == "unavailable" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s INFO.TABLES/COLUMNS/MEASURES unavailable (Pro-only capacity).\n", yellow("[note]"))
				if ds, ok := result["dataset"].(map[string]any); ok {
					if name, _ := ds["name"].(string); name != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  name:       %s\n", name)
					}
					if cb, _ := ds["configuredBy"].(string); cb != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  owner:      %s\n", cb)
					}
				}
				return nil
			}
			if v, ok := result["tables"].([]map[string]any); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Tables (%d):\n", len(v))
				for _, t := range v {
					name := stringOrEmpty(t["[name]"])
					if name == "" {
						name = stringOrEmpty(t["name"])
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", name)
				}
			}
			if v, ok := result["measures"].([]map[string]any); ok && len(v) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Measures (%d):\n", len(v))
				for _, m := range v {
					name := stringOrEmpty(m["[name]"])
					if name == "" {
						name = stringOrEmpty(m["name"])
					}
					expr := stringOrEmpty(m["[expression]"])
					if expr == "" {
						expr = stringOrEmpty(m["expression"])
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s  =  %s\n", name, truncate(strings.ReplaceAll(expr, "\n", " "), 80))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&group, "group", "g", "", "Workspace (group) ID (required)")
	return cmd
}

// runInfoDAX executes a DAX EVALUATE wrapping the supplied expression and
// returns the rows from the first result table, or an error if anything went
// wrong (network, HTTP non-2xx, DAX error, missing result).
func runInfoDAX(c clientLike, group, datasetID, expr string) ([]map[string]any, error) {
	body := daxRequest{
		Queries: []daxQuery{{Query: "EVALUATE " + expr}},
	}
	raw, status, err := c.Post(fmt.Sprintf("/groups/%s/datasets/%s/executeQueries", group, datasetID), body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", status, string(raw))
	}
	var resp daxResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("DAX error: %s", resp.Error.Message)
	}
	if len(resp.Results) == 0 || len(resp.Results[0].Tables) == 0 {
		return nil, fmt.Errorf("no result table")
	}
	if resp.Results[0].Error != nil {
		return nil, fmt.Errorf("DAX query error: %s", resp.Results[0].Error.Message)
	}
	return resp.Results[0].Tables[0].Rows, nil
}

// clientLike narrows the dependency surface so this helper can be tested
// without a full client.
type clientLike interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

func stringOrEmpty(v any) string {
	s, _ := v.(string)
	return s
}
