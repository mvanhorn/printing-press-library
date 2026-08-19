// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: blueprint export.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// blueprintColumn is one portable column definition.
type blueprintColumn struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	TypeSettings json.RawMessage `json:"typeSettings,omitempty"`
	// FormulaNamed is the formula with {{f_id}} rewritten to {{Column Name}},
	// which is what makes a blueprint portable across tables.
	FormulaNamed string   `json:"formulaNamed,omitempty"`
	DependsOn    []string `json:"dependsOn,omitempty"`
}

// tableBlueprint is a portable, committable description of a table's column graph.
type tableBlueprint struct {
	Blueprint  string            `json:"blueprint"`
	SourceID   string            `json:"sourceTableId"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	WorkbookID string            `json:"sourceWorkbookId,omitempty"`
	Columns    []blueprintColumn `json:"columns"`
}

func newNovelBlueprintExportCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	var flagOut string
	var flagIncludeSystem bool

	cmd := &cobra.Command{
		Use:   "export <tableId>",
		Short: "Snapshot a table's entire column graph to a portable JSON blueprint you can commit to git.",
		Long: "Use this command to capture a proven table design as a file you can review, diff, and replay.\n" +
			"Formula references are rewritten from generated field ids to column names so the blueprint is portable.\n" +
			"Do NOT use it to export row data; use 'tables records' for rows.",
		Example: "  clay-pp-cli blueprint export t_abc123 --workspace 1234567 --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "<tableId>=t_example;--workspace=1;--dry-run",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "blueprint export")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> is required"))
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

			bp := tableBlueprint{
				Blueprint:  "clay/v1",
				SourceID:   tbl.ID,
				Name:       tbl.Name,
				Type:       tbl.Type,
				WorkbookID: tbl.WorkbookID,
				Columns:    make([]blueprintColumn, 0, len(tbl.Fields)),
			}
			for _, f := range tbl.Fields {
				if !flagIncludeSystem && isSystemField(f.ID) {
					continue
				}
				ts := f.settings()
				col := blueprintColumn{Name: f.Name, Type: f.Type, TypeSettings: f.TypeSettings}
				if body := ts.formulaBody(); body != "" {
					col.FormulaNamed = resolveRefsToNames(body, byID)
					for _, ref := range formulaRefs(body) {
						if dep, ok := byID[ref]; ok {
							col.DependsOn = append(col.DependsOn, dep.Name)
						}
					}
				}
				bp.Columns = append(bp.Columns, col)
			}

			if flagOut != "" {
				data, mErr := json.MarshalIndent(bp, "", "  ")
				if mErr != nil {
					return fmt.Errorf("encoding blueprint: %w", mErr)
				}
				if wErr := os.WriteFile(flagOut, append(data, '\n'), 0o600); wErr != nil {
					return fmt.Errorf("writing %s: %w", flagOut, wErr)
				}
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "wrote %d columns to %s\n", len(bp.Columns), flagOut)
					return nil
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), bp, flags)
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().StringVar(&flagOut, "out", "", "Write the blueprint to this file instead of stdout")
	cmd.Flags().BoolVar(&flagIncludeSystem, "include-system", false, "Include Clay system columns such as Created At")
	return cmd
}

// isSystemField reports Clay's built-in columns, which must not be recreated.
func isSystemField(id string) bool {
	switch id {
	case "f_created_at", "f_updated_at", "f_deleted_at":
		return true
	}
	return false
}
