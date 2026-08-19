// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// Clay's built-in cross-table lookup action.
const (
	lookupActionPackageID = "4299091f-3cd3-4d68-b198-0143575f471d"
	lookupSingleActionKey = "lookup-row-in-other-table"
	lookupMultiActionKey  = "lookup-multiple-rows-in-other-table"
)

type linkResult struct {
	TableID     string `json:"tableId"`
	FieldID     string `json:"fieldId,omitempty"`
	Column      string `json:"column"`
	FromTable   string `json:"fromTableId"`
	MatchColumn string `json:"matchColumn"`
	ActionKey   string `json:"actionKey"`
	Created     bool   `json:"created"`
}

func newNovelColumnsLinkCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace, flagFrom, flagOn, flagName string
	var flagMultiple bool

	cmd := &cobra.Command{
		Use:   "link <tableId>",
		Short: "Connect two tables by creating a lookup column that pulls values from another table.",
		Long: "Use this command to wire one table's data into another on a join key.\n" +
			"Do NOT use it for references inside the same table; use a formula column instead.",
		Example: "  clay-pp-cli columns link t_prospects --from t_citycache --on City --workspace 1234567",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:happy-args": "<tableId>=t_example;--from=t_other;--on=City;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "columns link")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> is required"))
			}
			if flagFrom == "" || flagOn == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --on are both required"))
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
			target, err := fetchTable(ctx, c, ws, args[0])
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if _, err := fetchTable(ctx, c, ws, flagFrom); err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("source table %s is not readable: %w", flagFrom, err), flags)
			}
			matchField, ok := indexByName(target.Fields)[lowerTrim(flagOn)]
			if !ok {
				return notFoundErr(fmt.Errorf("column %q not found in table %s", flagOn, target.ID))
			}

			actionKey := lookupSingleActionKey
			if flagMultiple {
				actionKey = lookupMultiActionKey
			}
			name := flagName
			if name == "" {
				name = "Lookup from " + flagFrom
			}
			body := map[string]any{
				"type": "action",
				"name": name,
				"typeSettings": map[string]any{
					"actionKey":       actionKey,
					"actionPackageId": lookupActionPackageID,
					"actionVersion":   1,
					"inputsBinding": []map[string]any{
						{"name": "tableId", "formulaText": fmt.Sprintf("%q", flagFrom), "optional": false},
						{"name": "matchValue", "formulaText": "{{" + matchField.ID + "}}", "optional": false},
					},
					"runAsButton":      false,
					"useStaticIP":      false,
					"dataTypeSettings": map[string]any{"type": "json"},
				},
			}
			if target.FirstViewID != "" {
				body["activeViewId"] = target.FirstViewID
			}
			raw, status, err := c.Post(ctx, fmt.Sprintf("/workspaces/%s/tables/%s/fields", ws, target.ID), body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("creating lookup column: %w", err), flags)
			}
			if status >= 300 {
				return fmt.Errorf("creating lookup column: HTTP %d", status)
			}
			var created struct {
				ID    string `json:"id"`
				Field struct {
					ID string `json:"id"`
				} `json:"field"`
			}
			_ = json.Unmarshal(raw, &created)
			id := created.ID
			if id == "" {
				id = created.Field.ID
			}
			res := linkResult{
				TableID: target.ID, FieldID: id, Column: name,
				FromTable: flagFrom, MatchColumn: matchField.Name,
				ActionKey: actionKey, Created: true,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s (%s) linking %s on %s\n", res.Column, res.FieldID, res.FromTable, res.MatchColumn)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Table id to look values up in (t_...)")
	cmd.Flags().StringVar(&flagOn, "on", "", "Column in the target table whose value is the join key")
	cmd.Flags().StringVar(&flagName, "name", "", "Name for the new lookup column")
	cmd.Flags().BoolVar(&flagMultiple, "multiple", false, "Look up multiple matching rows instead of one")
	return cmd
}
