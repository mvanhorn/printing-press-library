// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: blueprint apply.

// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clay/internal/client"
	"github.com/spf13/cobra"
)

type appliedColumn struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	FieldID string `json:"fieldId,omitempty"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

type applyResult struct {
	TableID   string          `json:"tableId"`
	TableName string          `json:"tableName"`
	ViewID    string          `json:"viewId,omitempty"`
	Created   []appliedColumn `json:"columns"`
	Skipped   []string        `json:"skippedUnresolved,omitempty"`
	Failures  int             `json:"failures"`
}

func newNovelBlueprintApplyCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	var flagWorkbook string
	var flagName string
	var flagTable string

	cmd := &cobra.Command{
		Use:   "apply <blueprint.json>",
		Short: "Rebuild a captured table design, remapping formula references to the new column ids.",
		Long: "Use this command to clone a proven table design into a new workbook or vertical.\n" +
			"Columns are created in dependency order so formulas resolve against columns that already exist.\n" +
			"Do NOT use it to edit an existing table's columns one at a time; use 'columns update'.",
		Example: "  clay-pp-cli blueprint apply ./plumbing-austin.json --workbook wb_abc --name 'Plumbing Dallas' --workspace 1234567",
		Annotations: map[string]string{
			"mcp:read-only":  "false",
			"pp:happy-args":  "<blueprint>=./blueprint.json;--workbook=wb_example;--dry-run",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "blueprint apply")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<blueprint.json> is required"))
			}
			if flagTable == "" && flagWorkbook == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("one of --workbook (create a new table) or --table (apply into an existing table) is required"))
			}
			ws, err := resolveWorkspace(flagWorkspace)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading blueprint: %w", err)
			}
			var bp tableBlueprint
			if err := json.Unmarshal(raw, &bp); err != nil {
				return fmt.Errorf("parsing blueprint %s: %w", args[0], err)
			}
			if len(bp.Columns) == 0 {
				return fmt.Errorf("blueprint %s declares no columns", args[0])
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			res := applyResult{TableName: bp.Name}
			if flagName != "" {
				res.TableName = flagName
			}

			if flagTable != "" {
				tbl, fErr := fetchTable(ctx, c, ws, flagTable)
				if fErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), fErr, flags)
				}
				res.TableID = tbl.ID
				res.TableName = tbl.Name
				res.ViewID = tbl.FirstViewID
			} else {
				created, cErr := createBlueprintTable(ctx, c, ws, flagWorkbook, bp.Type)
				if cErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), cErr, flags)
				}
				res.TableID = created.ID
				res.ViewID = created.FirstViewID
				if res.TableName != "" && res.TableName != created.Name {
					if rErr := renameTable(ctx, c, ws, created.ID, res.TableName); rErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: table created but rename failed: %v\n", rErr)
					}
				}
			}

			// Create columns in dependency order so {{Name}} refs resolve.
			ordered, unresolved := orderByDependency(bp.Columns)
			res.Skipped = unresolved
			nameToID := map[string]string{}
			if tbl, fErr := fetchTable(ctx, c, ws, res.TableID); fErr == nil {
				for _, f := range tbl.Fields {
					nameToID[strings.ToLower(f.Name)] = f.ID
				}
			}

			for _, col := range ordered {
				applied := appliedColumn{Name: col.Name, Type: col.Type}
				body, bErr := blueprintColumnBody(col, res.ViewID, nameToID)
				if bErr != nil {
					applied.Status = "skipped"
					applied.Error = bErr.Error()
					res.Failures++
					res.Created = append(res.Created, applied)
					continue
				}
				out, status, pErr := c.Post(ctx, fmt.Sprintf("/workspaces/%s/tables/%s/fields", ws, res.TableID), body)
				if pErr != nil || status >= 300 {
					applied.Status = "error"
					if pErr != nil {
						applied.Error = pErr.Error()
					} else {
						applied.Error = fmt.Sprintf("HTTP %d", status)
					}
					res.Failures++
					res.Created = append(res.Created, applied)
					continue
				}
				var createdField struct {
					ID    string `json:"id"`
					Field struct {
						ID string `json:"id"`
					} `json:"field"`
				}
				_ = json.Unmarshal(out, &createdField)
				id := createdField.ID
				if id == "" {
					id = createdField.Field.ID
				}
				applied.FieldID = id
				applied.Status = "created"
				if id != "" {
					nameToID[strings.ToLower(col.Name)] = id
				}
				res.Created = append(res.Created, applied)
			}

			if res.Failures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d columns failed; table %s was still created\n",
					res.Failures, len(ordered), res.TableID)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "table %s (%s)\n", res.TableID, res.TableName)
			for _, col := range res.Created {
				if col.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-8s %-30s %s\n", col.Status, col.Name, col.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-8s %-30s %s\n", col.Status, col.Name, col.FieldID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().StringVar(&flagWorkbook, "workbook", "", "Workbook to create the new table in (wb_...)")
	cmd.Flags().StringVar(&flagTable, "table", "", "Apply columns into this existing table instead of creating one")
	cmd.Flags().StringVar(&flagName, "name", "", "Name for the newly created table")
	return cmd
}

func createBlueprintTable(ctx context.Context, c *client.Client, ws, workbookID, tableType string) (*clayTable, error) {
	if tableType == "" {
		tableType = "spreadsheet"
	}
	body := map[string]any{
		"workspaceId":    ws,
		"workbookId":     workbookID,
		"type":           tableType,
		"template":       "basic",
		"sourceSettings": map[string]any{},
	}
	raw, status, err := c.Post(ctx, fmt.Sprintf("/workspaces/%s/tables", ws), body)
	if err != nil {
		return nil, fmt.Errorf("creating table: %w", err)
	}
	if status >= 300 {
		return nil, fmt.Errorf("creating table: HTTP %d", status)
	}
	var wrapper struct {
		Table *clayTable `json:"table"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Table != nil && wrapper.Table.ID != "" {
		return wrapper.Table, nil
	}
	var t clayTable
	if err := json.Unmarshal(raw, &t); err != nil || t.ID == "" {
		return nil, fmt.Errorf("creating table: unexpected response shape")
	}
	return &t, nil
}

func renameTable(ctx context.Context, c *client.Client, ws, tableID, name string) error {
	body := map[string]any{"name": name, "tableSettings": map[string]any{}, "fieldGroupMap": map[string]any{}, "sourceSettings": map[string]any{}}
	_, status, err := c.Patch(ctx, fmt.Sprintf("/workspaces/%s/tables/%s", ws, tableID), body)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("HTTP %d", status)
	}
	return nil
}

// blueprintColumnBody rebuilds a create-field body, rewriting {{Name}} refs to ids.
func blueprintColumnBody(col blueprintColumn, viewID string, nameToID map[string]string) (map[string]any, error) {
	body := map[string]any{"type": col.Type, "name": col.Name}
	if viewID != "" {
		body["activeViewId"] = viewID
	}
	var ts map[string]any
	if len(col.TypeSettings) > 0 {
		if err := json.Unmarshal(col.TypeSettings, &ts); err != nil {
			return nil, fmt.Errorf("bad typeSettings: %w", err)
		}
	}
	if ts == nil {
		ts = map[string]any{}
	}
	if col.FormulaNamed != "" {
		remapped, unknown := remapNamedRefs(col.FormulaNamed, nameToID)
		if len(unknown) > 0 {
			return nil, fmt.Errorf("unresolved column reference(s): %s", strings.Join(unknown, ", "))
		}
		if _, ok := ts["formulaText"]; ok {
			ts["formulaText"] = remapped
		} else {
			ts["formula"] = remapped
		}
		delete(ts, "formulaPrompt")
	}
	if len(ts) > 0 {
		body["typeSettings"] = ts
	}
	return body, nil
}

// remapNamedRefs rewrites {{Column Name}} to {{f_id}} using the live table's names.
func remapNamedRefs(formula string, nameToID map[string]string) (string, []string) {
	var unknown []string
	out := namedRefPattern.ReplaceAllStringFunc(formula, func(m string) string {
		sub := namedRefPattern.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		token := strings.TrimSpace(sub[1])
		if strings.HasPrefix(token, "f_") {
			return m
		}
		if id, ok := nameToID[strings.ToLower(token)]; ok {
			return "{{" + id + "}}"
		}
		unknown = append(unknown, token)
		return m
	})
	return out, unknown
}

// orderByDependency returns columns sorted so dependencies come first.
// Columns in an unresolvable cycle are returned in the second slice.
func orderByDependency(cols []blueprintColumn) ([]blueprintColumn, []string) {
	byName := map[string]blueprintColumn{}
	for _, c := range cols {
		byName[strings.ToLower(c.Name)] = c
	}
	var ordered []blueprintColumn
	placed := map[string]bool{}
	// Iterate until no progress; anything left has a cycle or a missing dep.
	for progress := true; progress; {
		progress = false
		for _, c := range cols {
			key := strings.ToLower(c.Name)
			if placed[key] {
				continue
			}
			ready := true
			for _, dep := range c.DependsOn {
				depKey := strings.ToLower(dep)
				if _, known := byName[depKey]; known && !placed[depKey] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			ordered = append(ordered, c)
			placed[key] = true
			progress = true
		}
	}
	var leftover []string
	for _, c := range cols {
		if !placed[strings.ToLower(c.Name)] {
			leftover = append(leftover, c.Name)
		}
	}
	return ordered, leftover
}
