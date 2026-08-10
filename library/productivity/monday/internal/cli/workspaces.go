// Workspaces commands for monday.com.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const queryWorkspacesList = `query($limit: Int!, $page: Int!, $kind: WorkspaceKind, $state: State) {
  workspaces(limit: $limit, page: $page, kind: $kind, state: $state) {
    id
    name
    kind
    description
    state
    created_at
  }
}`

const queryWorkspaceGet = `query($ids: [ID!]) {
  workspaces(ids: $ids) {
    id
    name
    kind
    description
    state
    created_at
  }
}`

const mutationCreateWorkspace = `mutation($name: String!, $kind: WorkspaceKind!, $description: String) {
  create_workspace(name: $name, workspace_kind: $kind, description: $description) {
    id
    name
    kind
    description
    state
  }
}`

const mutationUpdateWorkspace = `mutation($id: ID!, $attributes: UpdateWorkspaceAttributesInput!) {
  update_workspace(id: $id, attributes: $attributes) {
    id
    name
    description
    state
  }
}`

func newWorkspacesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspaces",
		Short: "List, get, create, and update monday.com workspaces.",
	}
	cmd.AddCommand(newWorkspacesListCmd(flags))
	cmd.AddCommand(newWorkspacesGetCmd(flags))
	cmd.AddCommand(newWorkspacesCreateCmd(flags))
	cmd.AddCommand(newWorkspacesUpdateCmd(flags))
	return cmd
}

func newWorkspacesListCmd(flags *rootFlags) *cobra.Command {
	var limit, page int
	var kind, state string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List workspaces the API token can see.",
		Example: "  monday-pp-cli workspaces list --json --select id,name,kind",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"limit": limit, "page": page}
			if kind != "" {
				vars["kind"] = kind
			}
			if state != "" {
				vars["state"] = state
			}
			data, err := c.GraphQL(queryWorkspacesList, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "workspaces")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Page size.")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-based).")
	cmd.Flags().StringVar(&kind, "kind", "", "Filter: open, closed.")
	cmd.Flags().StringVar(&state, "state", "", "Filter: active, archived, deleted, all.")
	return cmd
}

func newWorkspacesGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Fetch one workspace by ID.",
		Example: "  monday-pp-cli workspaces get 12345 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("workspace id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryWorkspaceGet, map[string]any{"ids": []string{args[0]}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckFirstFromArrayField(data, "workspaces")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func newWorkspacesCreateCmd(flags *rootFlags) *cobra.Command {
	var name, kind, description string
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new monday.com workspace with name, kind (open/closed), and optional description.",
		Example: "  monday-pp-cli workspaces create --name \"Q3 Launch\" --kind open --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || kind == "" {
				return usageErr(fmt.Errorf("--name and --kind are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"name": name, "kind": kind}
			if description != "" {
				vars["description"] = description
			}
			data, err := c.GraphQL(mutationCreateWorkspace, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "workspaces.create", vars)
			}
			view, err := pluckJSONField(data, "create_workspace")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Workspace name (required).")
	cmd.Flags().StringVar(&kind, "kind", "", "Workspace kind: open or closed (required).")
	cmd.Flags().StringVar(&description, "description", "", "Optional description.")
	return cmd
}

func newWorkspacesUpdateCmd(flags *rootFlags) *cobra.Command {
	var name, description, state string
	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update a workspace's name, description, or state.",
		Example: "  monday-pp-cli workspaces update 12345 --description \"Q3 launch board\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("workspace id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			attrs := map[string]any{}
			if name != "" {
				attrs["name"] = name
			}
			if description != "" {
				attrs["description"] = description
			}
			if state != "" {
				attrs["state"] = state
			}
			if len(attrs) == 0 {
				return usageErr(fmt.Errorf("at least one of --name, --description, --state must be set"))
			}
			updateVars := map[string]any{"id": args[0], "attributes": attrs}
			data, err := c.GraphQL(mutationUpdateWorkspace, updateVars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "workspaces.update", updateVars)
			}
			view, err := pluckJSONField(data, "update_workspace")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name.")
	cmd.Flags().StringVar(&description, "description", "", "New description.")
	cmd.Flags().StringVar(&state, "state", "", "New state: active, archived, deleted.")
	return cmd
}
