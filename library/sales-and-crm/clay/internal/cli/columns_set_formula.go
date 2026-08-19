// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type formulaView struct {
	TableID   string `json:"tableId"`
	FieldID   string `json:"fieldId"`
	Column    string `json:"column"`
	Formula   string `json:"formula"`
	Raw       string `json:"rawFormula,omitempty"`
	Updated   bool   `json:"updated"`
	Unchanged bool   `json:"unchanged,omitempty"`
}

func newNovelColumnsSetFormulaCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	var flagFormula string

	cmd := &cobra.Command{
		Use:   "set-formula <tableId> <fieldId>",
		Short: "Read a column's formula with references resolved to column names, or write a new one.",
		Long: "Called without --formula this prints the current formula with {{f_id}} rewritten to {{Column Name}}.\n" +
			"Called with --formula it converts column names back to ids and PATCHes the column.\n" +
			"Do NOT use it to create a column; use 'columns create'.",
		Example: "  clay-pp-cli columns set-formula t_abc f_xyz --formula \"{{Company}}\" --workspace 1234567",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:happy-args": "<tableId>=t_example;<fieldId>=f_example;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> and <fieldId> are both required"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "columns set-formula")
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
			tbl, err := fetchTable(ctx, c, ws, args[0])
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			byID := indexByID(tbl.Fields)
			target, ok := byID[args[1]]
			if !ok {
				return notFoundErr(fmt.Errorf("column %s not found in table %s", args[1], args[0]))
			}
			ts := target.settings()
			view := formulaView{
				TableID: tbl.ID, FieldID: target.ID, Column: target.Name,
				Raw:     ts.formulaBody(),
				Formula: resolveRefsToNames(ts.formulaBody(), byID),
			}

			if flagFormula == "" {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				if view.Formula == "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s has no formula\n", target.Name)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", target.Name, view.Formula)
				return nil
			}

			byName := indexByName(tbl.Fields)
			remapped, unknown, ambiguous := resolveNamesToRefs(flagFormula, byName, byID)
			if len(unknown) > 0 {
				return usageErr(fmt.Errorf("unknown column reference(s): %s", strings.Join(unknown, ", ")))
			}
			for _, tok := range ambiguous {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %q is both a real field id and the name of a different column; "+
						"binding it as the explicit field id\n", tok)
			}
			if remapped == view.Raw {
				view.Unchanged = true
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: formula unchanged\n", target.Name)
				return nil
			}

			var settings map[string]any
			if len(target.TypeSettings) > 0 {
				_ = json.Unmarshal(target.TypeSettings, &settings)
			}
			if settings == nil {
				settings = map[string]any{}
			}
			if _, has := settings["formulaText"]; has || ts.FormulaText != "" {
				settings["formulaText"] = remapped
			} else {
				settings["formula"] = remapped
			}
			body := map[string]any{"typeSettings": settings}
			_, status, err := c.Patch(ctx, fmt.Sprintf("/workspaces/%s/tables/%s/fields/%s", ws, tbl.ID, target.ID), body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("updating formula: %w", err), flags)
			}
			if status >= 300 {
				return fmt.Errorf("updating formula: HTTP %d", status)
			}
			view.Updated = true
			view.Formula = resolveRefsToNames(remapped, byID)
			view.Raw = remapped
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s: %s\n", target.Name, view.Formula)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().StringVar(&flagFormula, "formula", "", "New formula, using {{Column Name}} references")
	return cmd
}
