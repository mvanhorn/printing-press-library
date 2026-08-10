// Boards commands for monday.com.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const queryBoardsList = `query($limit: Int!, $page: Int!, $workspace_ids: [ID], $state: State) {
  boards(limit: $limit, page: $page, workspace_ids: $workspace_ids, state: $state) {
    id
    name
    state
    board_kind
    description
    workspace_id
    updated_at
    items_count
    owners { id }
  }
}`

const queryBoardGet = `query($ids: [ID!]) {
  boards(ids: $ids) {
    id
    name
    state
    board_kind
    description
    workspace_id
    updated_at
    items_count
    owners { id name }
    columns { id title type description }
    groups { id title color position }
    tags { id name color }
  }
}`

const queryBoardActivity = `query($id: [ID!], $limit: Int!, $page: Int!, $from: ISO8601DateTime, $to: ISO8601DateTime, $user_ids: [ID!], $column_ids: [String!]) {
  boards(ids: $id) {
    activity_logs(limit: $limit, page: $page, from: $from, to: $to, user_ids: $user_ids, column_ids: $column_ids) {
      id
      event
      data
      entity
      created_at
      user_id
    }
  }
}`

const mutationCreateBoard = `mutation($name: String!, $kind: BoardKind!, $workspace_id: ID, $template_id: ID, $description: String) {
  create_board(board_name: $name, board_kind: $kind, workspace_id: $workspace_id, template_id: $template_id, description: $description) {
    id
    name
    state
  }
}`

func newBoardsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "boards",
		Short: "List, get, create, and inspect monday.com boards.",
	}
	cmd.AddCommand(newBoardsListCmd(flags))
	cmd.AddCommand(newBoardsGetCmd(flags))
	cmd.AddCommand(newBoardsActivityCmd(flags))
	cmd.AddCommand(newBoardsCreateCmd(flags))
	cmd.AddCommand(newBoardsHealthCmd(flags))
	return cmd
}

func newBoardsListCmd(flags *rootFlags) *cobra.Command {
	var limit, page int
	var workspaceIDs, state string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List boards the API token can see.",
		Example: "  monday-pp-cli boards list --limit 50 --json --select id,name,workspace_id",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"limit": limit, "page": page}
			if ids := splitCSV(workspaceIDs); len(ids) > 0 {
				vars["workspace_ids"] = ids
			}
			if state != "" {
				vars["state"] = state
			}
			data, err := c.GraphQL(queryBoardsList, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "boards")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Page size.")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-based).")
	cmd.Flags().StringVar(&workspaceIDs, "workspace-ids", "", "Comma-separated workspace IDs to filter.")
	cmd.Flags().StringVar(&state, "state", "", "Filter by state: active, archived, deleted, all.")
	return cmd
}

func newBoardsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Fetch one board (with columns, groups, tags) by ID.",
		Example: "  monday-pp-cli boards get 12345 --json --select id,name,columns",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("board id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryBoardGet, map[string]any{"ids": []string{args[0]}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckFirstFromArrayField(data, "boards")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func newBoardsActivityCmd(flags *rootFlags) *cobra.Command {
	var limit, page int
	var fromISO, toISO, userIDs, columnIDs string
	cmd := &cobra.Command{
		Use:     "activity <board-id>",
		Short:   "List activity log entries for a board (column changes, item moves, etc.).",
		Long:    "Fetch the audit trail of column-value changes, item creations, deletions, and moves on a board. Filter by time window, user, or column.",
		Example: "  monday-pp-cli boards activity 12345 --from 2026-01-01T00:00:00Z --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("board id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{
				"id":    []string{args[0]},
				"limit": limit,
				"page":  page,
			}
			if fromISO != "" {
				vars["from"] = fromISO
			}
			if toISO != "" {
				vars["to"] = toISO
			}
			if ids := splitCSV(userIDs); len(ids) > 0 {
				vars["user_ids"] = ids
			}
			if cols := splitCSV(columnIDs); len(cols) > 0 {
				vars["column_ids"] = cols
			}
			data, err := c.GraphQL(queryBoardActivity, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			boards, err := pluckJSONField(data, "boards")
			if err != nil {
				return err
			}
			first, err := pluckFirstFromArrayObject(boards, "activity_logs")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), first, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Page size (max 10000).")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-based).")
	cmd.Flags().StringVar(&fromISO, "from", "", "ISO 8601 lower bound (created_at >= from).")
	cmd.Flags().StringVar(&toISO, "to", "", "ISO 8601 upper bound (created_at <= to).")
	cmd.Flags().StringVar(&userIDs, "user-ids", "", "Comma-separated user IDs to filter.")
	cmd.Flags().StringVar(&columnIDs, "column-ids", "", "Comma-separated column IDs to filter.")
	return cmd
}

func newBoardsCreateCmd(flags *rootFlags) *cobra.Command {
	var name, kind, workspaceID, templateID, description string
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new monday.com board with name, kind (public/private/share), and optional workspace, template, and description.",
		Example: "  monday-pp-cli boards create --name \"Q3 Launch\" --kind public --workspace-id 12345 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || kind == "" {
				return usageErr(fmt.Errorf("--name and --kind are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"name": name, "kind": kind}
			if workspaceID != "" {
				vars["workspace_id"] = workspaceID
			}
			if templateID != "" {
				vars["template_id"] = templateID
			}
			if description != "" {
				vars["description"] = description
			}
			data, err := c.GraphQL(mutationCreateBoard, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "boards.create", vars)
			}
			view, err := pluckJSONField(data, "create_board")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Board name (required).")
	cmd.Flags().StringVar(&kind, "kind", "", "Board kind: public, private, share (required).")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Workspace ID to create the board in.")
	cmd.Flags().StringVar(&templateID, "template-id", "", "Template board ID to create from.")
	cmd.Flags().StringVar(&description, "description", "", "Optional description.")
	return cmd
}
