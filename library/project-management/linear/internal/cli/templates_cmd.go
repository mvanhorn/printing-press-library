package cli

// templates_cmd.go closes GAP-044: templateCreate, templateUpdate, and
// templateDelete had no CLI surface, and the only read was the promoted
// `templates` leaf, which dumps every template in the workspace with no
// by-id form and no team scope. Query.templates takes zero arguments and
// returns a plain list rather than a connection, so --team filters
// client-side on Template.team, which is the only option the schema offers.
//
// The leaves are registered on the existing promoted `templates` command in
// promoted_templates.go, so `templates` on its own keeps its current
// behaviour.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// templateRow is the projection `templates list` renders. templateData is
// omitted from the row: it is the template's whole payload and belongs to
// `templates get`, where the caller asked for one template.
type templateRow struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description,omitempty"`
	SortOrder   float64 `json:"sortOrder"`
	CreatedAt   string  `json:"createdAt,omitempty"`
	UpdatedAt   string  `json:"updatedAt,omitempty"`
	LastApplied string  `json:"lastAppliedAt,omitempty"`
	Team        *struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"team,omitempty"`
}

func newTemplatesListCmd(flags *rootFlags) *cobra.Command {
	var team, templateType string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List workspace templates, optionally scoped by team or type",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List every template the token can see.

Query.templates accepts no arguments, so --team and --type are applied to the
returned list rather than pushed into the query. --team matches a team key, a
team name, or a team UUID, and templates with no team are workspace-level
templates that only appear when --team is omitted.`,
		Example: `  linear-pp-cli templates list --agent
  linear-pp-cli templates list --team ENG --agent --select id,name,type`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var resp struct {
				Templates []templateRow `json:"templates"`
			}
			if err := c.QueryInto(client.TemplatesQuery, nil, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			rows := make([]templateRow, 0, len(resp.Templates))
			for _, row := range resp.Templates {
				if !templateMatchesTeam(row, team) {
					continue
				}
				if templateType != "" && !strings.EqualFold(row.Type, templateType) {
					continue
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Type != rows[j].Type {
					return rows[i].Type < rows[j].Type
				}
				if rows[i].SortOrder != rows[j].SortOrder {
					return rows[i].SortOrder < rows[j].SortOrder
				}
				return rows[i].Name < rows[j].Name
			})
			out, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "templates", true)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Keep only templates owned by this team key, name, or UUID")
	cmd.Flags().StringVar(&templateType, "type", "", "Keep only templates of this type")
	return cmd
}

// templateMatchesTeam reports whether a template belongs to the requested
// team. An empty needle keeps everything, including workspace-level templates
// that have no team at all.
func templateMatchesTeam(row templateRow, needle string) bool {
	if needle == "" {
		return true
	}
	if row.Team == nil {
		return false
	}
	return strings.EqualFold(row.Team.Key, needle) ||
		strings.EqualFold(row.Team.Name, needle) ||
		row.Team.ID == needle
}

func newTemplatesGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <template-id>",
		Short:       "Get one template including its templateData payload",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Read one template by UUID. The response carries templateData, the JSON payload
Linear applies when the template is used, which is the shape 'templates create'
and 'templates update' expect back.`,
		Example: `  linear-pp-cli templates get <template-uuid> --agent
  linear-pp-cli templates get <template-uuid> --agent --select templateData`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<template-id> is required"))
			}
			templateID := args[0]
			if !store.IsUUID(templateID) {
				return usageErr(fmt.Errorf("<template-id> expects a template UUID, got %q; run 'templates list' to find it", templateID))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var resp struct {
				Template json.RawMessage `json:"template"`
			}
			if err := c.QueryInto(client.TemplateQuery, map[string]any{"id": templateID}, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			if len(resp.Template) == 0 || string(resp.Template) == "null" {
				return notFoundErr(fmt.Errorf("template %s not found", templateID))
			}
			return renderLiveObject(cmd, flags, resp.Template, "templates")
		},
	}
	return cmd
}

func newTemplatesCreateCmd(flags *rootFlags) *cobra.Command {
	var name, description, templateType, icon, color string
	var data, dataFile string
	var team string
	var sortOrder float64
	var dbPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a template",
		Long: `Create a template via the templateCreate mutation. TemplateCreateInput requires
name, type, and templateData.

--type is passed through verbatim because Linear types it as a free-form
String, not an enum: read an existing template with 'templates get' to see the
type string the workspace uses for the kind of template you want. --data-file
is the ergonomic path for templateData, which is a full JSON payload.

--team scopes the template to one team. Omitting it creates a workspace-level
template.`,
		Example: `  linear-pp-cli templates create --name "Bug report" --type issue --data-file /tmp/template.json --team ENG --agent
  linear-pp-cli templates create --name "x" --type issue --data '{}' --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, payloadSet, err := wbJSONObject("data", data, "data-file", dataFile, "templateData")
			if err != nil {
				return err
			}
			if name == "" || templateType == "" || !payloadSet {
				if dryRunOK(flags) {
					return nil
				}
				switch {
				case name == "":
					return usageErr(fmt.Errorf("--name is required"))
				case templateType == "":
					return usageErr(fmt.Errorf("--type is required (read an existing template with 'templates get' to see the type strings this workspace uses)"))
				default:
					return usageErr(fmt.Errorf("--data or --data-file is required (templateData is a JSON object)"))
				}
			}
			input := map[string]any{
				"name":         name,
				"type":         templateType,
				"templateData": payload,
			}
			if cmd.Flags().Changed("description") {
				input["description"] = description
			}
			if icon != "" {
				input["icon"] = icon
			}
			if color != "" {
				input["color"] = color
			}
			if cmd.Flags().Changed("sort-order") {
				input["sortOrder"] = sortOrder
			}
			var unresolved []string
			if team != "" {
				teamIDs, pending := wbResolveTeamsLocal(dbPath, []string{team})
				input["teamId"] = teamIDs[0]
				unresolved = pending
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_template", "templateCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if len(unresolved) > 0 {
				resolved, err := wbResolveTeamsLive(c, []string{input["teamId"].(string)})
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["teamId"] = resolved[0]
			}
			resp, err := c.Mutate(client.TemplateCreateMutation, map[string]any{"input": input})
			if err != nil {
				return wbClassifyCreateError("templateCreate", err, flags)
			}
			template, err := extractMutationObject(resp, "templateCreate", "template")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, template, "templates")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Template name (required)")
	cmd.Flags().StringVar(&templateType, "type", "", "Template type string (required)")
	cmd.Flags().StringVar(&description, "description", "", "Template description")
	cmd.Flags().StringVar(&data, "data", "", "templateData as an inline JSON object")
	cmd.Flags().StringVar(&dataFile, "data-file", "", "Read templateData JSON from a file")
	cmd.Flags().StringVar(&icon, "icon", "", "Template icon name")
	cmd.Flags().StringVar(&color, "color", "", "Template color as a hex string")
	cmd.Flags().Float64Var(&sortOrder, "sort-order", 0, "Explicit sort order")
	cmd.Flags().StringVar(&team, "team", "", "Scope the template to this team key or UUID")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}

func newTemplatesUpdateCmd(flags *rootFlags) *cobra.Command {
	var name, description, icon, color string
	var data, dataFile string
	var team string
	var sortOrder float64
	var dbPath string
	cmd := &cobra.Command{
		Use:     "update <template-id>",
		Aliases: []string{"edit"},
		Short:   "Update a template",
		Long: `Edit a template via the templateUpdate mutation. At least one field flag is
required.

TemplateUpdateInput has no type field, so a template's type is fixed at
creation. --data replaces templateData wholesale, so read the current payload
with 'templates get <id> --select templateData' before editing it.`,
		Example: `  linear-pp-cli templates update <template-uuid> --name "Bug report v2" --agent
  linear-pp-cli templates update <template-uuid> --data-file /tmp/template.json --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<template-id> is required"))
			}
			templateID := args[0]
			if !store.IsUUID(templateID) {
				return usageErr(fmt.Errorf("<template-id> expects a template UUID, got %q", templateID))
			}
			payload, payloadSet, err := wbJSONObject("data", data, "data-file", dataFile, "templateData")
			if err != nil {
				return err
			}
			input := map[string]any{}
			if name != "" {
				input["name"] = name
			}
			if cmd.Flags().Changed("description") {
				input["description"] = description
			}
			if icon != "" {
				input["icon"] = icon
			}
			if color != "" {
				input["color"] = color
			}
			if cmd.Flags().Changed("sort-order") {
				input["sortOrder"] = sortOrder
			}
			if payloadSet {
				input["templateData"] = payload
			}
			var unresolved []string
			if team != "" {
				teamIDs, pending := wbResolveTeamsLocal(dbPath, []string{team})
				input["teamId"] = teamIDs[0]
				unresolved = pending
			}
			if len(input) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass at least one field to update (--name, --description, --data, --data-file, --icon, --color, --sort-order, --team)"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_template", "templateUpdate", map[string]any{"id": templateID, "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if len(unresolved) > 0 {
				resolved, err := wbResolveTeamsLive(c, []string{input["teamId"].(string)})
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["teamId"] = resolved[0]
			}
			resp, err := c.Mutate(client.TemplateUpdateMutation, map[string]any{"id": templateID, "input": input})
			if err != nil {
				return classifyMutationError("templateUpdate", err, flags, nil)
			}
			template, err := extractMutationObject(resp, "templateUpdate", "template")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, template, "templates")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Template name")
	cmd.Flags().StringVar(&description, "description", "", "Template description")
	cmd.Flags().StringVar(&data, "data", "", "Replace templateData with this inline JSON object")
	cmd.Flags().StringVar(&dataFile, "data-file", "", "Replace templateData with JSON read from a file")
	cmd.Flags().StringVar(&icon, "icon", "", "Template icon name")
	cmd.Flags().StringVar(&color, "color", "", "Template color as a hex string")
	cmd.Flags().Float64Var(&sortOrder, "sort-order", 0, "Explicit sort order")
	cmd.Flags().StringVar(&team, "team", "", "Move the template to this team key or UUID")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}

func newTemplatesDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <template-id>",
		Short: "Delete a template",
		Long: `Delete a template via the templateDelete mutation.

Deletion is confirmed interactively unless --yes is passed. --agent implies
--yes. With --ignore-missing an already-deleted template exits 0 as a no-op.`,
		Example: `  linear-pp-cli templates delete <template-uuid> --yes --agent
  linear-pp-cli templates delete <template-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<template-id> is required"))
			}
			templateID := args[0]
			if !store.IsUUID(templateID) {
				return usageErr(fmt.Errorf("<template-id> expects a template UUID, got %q", templateID))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_template", "templateDelete", map[string]any{"id": templateID})
			}
			if err := wbConfirm(cmd, flags, fmt.Sprintf("Delete template %s", templateID)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.TemplateDeleteMutation, map[string]any{"id": templateID})
			if err != nil {
				return wbClassifyDeleteError("templateDelete", err, flags)
			}
			id, err := wbDecodeDeletePayload(resp, "templateDelete")
			if err != nil {
				return err
			}
			return wbRenderMutationEvent(cmd, flags, "template_deleted", map[string]any{"id": firstNonEmpty(id, templateID)},
				fmt.Sprintf("Deleted template %s", firstNonEmpty(id, templateID)))
		},
	}
	return cmd
}
