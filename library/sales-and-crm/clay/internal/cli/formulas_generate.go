// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: formulas generate.
//
// Clay's AI formula endpoint needs more than a prompt. It requires the calling
// user's id, the table's column-name -> field-id map, and three array fields
// that must be present even when empty. Without the column map it returns a
// name-shaped placeholder that will not resolve against a real table; with it,
// the returned formula references actual field ids.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type formulaSuggestion struct {
	TableID   string `json:"tableId,omitempty"`
	Prompt    string `json:"prompt"`
	Formula   string `json:"formula"`
	Readable  string `json:"readable,omitempty"`
	DataType  string `json:"dataType,omitempty"`
	ColumnsIn int    `json:"columnsConsidered"`
}

func newNovelFormulasGenerateCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace, flagTable, flagPrompt, flagMode string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Turn a natural-language prompt into a Clay formula for a specific table.",
		Long: "Resolves the table's columns so the generated formula references real field ids,\n" +
			"and prints a readable form with ids rendered back as column names.\n" +
			"Pass the result to 'columns set-formula' to apply it. This command never writes.",
		Example: "  clay-pp-cli formulas generate --table t_example123 --prompt \"combine company and city\"",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--table=t_example;--prompt=uppercase the company name;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "formulas generate")
			}
			if strings.TrimSpace(flagPrompt) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--prompt is required"))
			}
			ws, err := resolveWorkspace(flagWorkspace)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// The endpoint requires the calling user's id.
			meRaw, err := c.Get(ctx, "/me", nil)
			if err != nil {
				return fmt.Errorf("resolving current user: %w", err)
			}
			var me struct {
				ID json.Number `json:"id"`
			}
			if err := json.Unmarshal(meRaw, &me); err != nil || me.ID.String() == "" {
				return fmt.Errorf("current user id unavailable; is the browser session still valid?")
			}

			// Column map: without it Clay returns a placeholder that will not
			// resolve against a real table.
			columnNamesToIds := map[string]string{}
			var byID map[string]clayField
			if flagTable != "" {
				tbl, tErr := fetchTable(ctx, c, ws, flagTable)
				if tErr != nil {
					return tErr
				}
				byID = indexByID(tbl.Fields)
				for _, f := range tbl.Fields {
					if f.Name != "" {
						columnNamesToIds[f.Name] = f.ID
					}
				}
			}

			mode := flagMode
			if mode == "" {
				mode = "basic"
			}
			body := map[string]any{
				"id":                            jsonNumberOrString(me.ID),
				"userPromptInput":               flagPrompt,
				"mode":                          mode,
				"columnNamesToIds":              columnNamesToIds,
				"userProvidedCorrectedExamples": []any{},
				"rawExampleTableData":           []any{},
				"formattedExampleTableData":     []any{},
			}
			raw, status, err := c.Post(ctx, fmt.Sprintf("/workspaces/%s/ai-generation/formula", ws), body)
			if err != nil {
				return fmt.Errorf("generating formula: %w", err)
			}
			if status >= 300 {
				return fmt.Errorf("generating formula: HTTP %d", status)
			}
			var out struct {
				Formula  string `json:"formula"`
				DataType string `json:"dataType"`
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				return fmt.Errorf("parsing formula response: %w", err)
			}

			view := formulaSuggestion{
				TableID: flagTable, Prompt: flagPrompt,
				Formula: out.Formula, DataType: out.DataType,
				ColumnsIn: len(columnNamesToIds),
			}
			if byID != nil {
				view.Readable = resolveRefsToNames(out.Formula, byID)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "formula:  %s\n", view.Formula)
			if view.Readable != "" && view.Readable != view.Formula {
				fmt.Fprintf(cmd.OutOrStdout(), "readable: %s\n", view.Readable)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "type:     %s (considered %d column(s))\n", view.DataType, view.ColumnsIn)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().StringVar(&flagTable, "table", "", "Table id (t_...) whose columns the formula may reference")
	cmd.Flags().StringVar(&flagPrompt, "prompt", "", "Natural-language description of the formula")
	cmd.Flags().StringVar(&flagMode, "mode", "basic", "Generation mode")
	return cmd
}

// jsonNumberOrString sends the user id as a number when it is one, matching the
// endpoint's union type, and falls back to the string form otherwise.
func jsonNumberOrString(n json.Number) any {
	if i, err := n.Int64(); err == nil {
		return i
	}
	return n.String()
}
