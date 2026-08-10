// Items commands for monday.com (the rows on a board).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const queryItemsList = `query($board_id: ID!, $limit: Int!, $cursor: String) {
  boards(ids: [$board_id]) {
    items_page(limit: $limit, cursor: $cursor) {
      cursor
      items {
        id
        name
        state
        created_at
        updated_at
        creator_id
        group { id title }
        column_values {
          id
          text
          value
          type
          column { id title type }
        }
      }
    }
  }
}`

const queryItemGet = `query($ids: [ID!]) {
  items(ids: $ids) {
    id
    name
    state
    created_at
    updated_at
    creator_id
    board { id name }
    group { id title color }
    parent_item { id name }
    column_values {
      id
      text
      value
      type
      column { id title type }
    }
    updates(limit: 5) {
      id
      body
      text_body
      created_at
      creator_id
    }
    assets {
      id
      name
      url
      file_extension
    }
  }
}`

const mutationItemsCreate = `mutation($board_id: ID!, $group_id: String, $name: String!, $column_values: JSON, $create_labels: Boolean) {
  create_item(board_id: $board_id, group_id: $group_id, item_name: $name, column_values: $column_values, create_labels_if_missing: $create_labels) {
    id
    name
    board { id }
    group { id title }
  }
}`

const mutationItemsDelete = `mutation($id: ID!) {
  delete_item(item_id: $id) { id }
}`

const mutationItemsMove = `mutation($id: ID!, $group_id: String!) {
  move_item_to_group(item_id: $id, group_id: $group_id) { id group { id title } }
}`

const mutationItemsSet = `mutation($board_id: ID!, $item_id: ID!, $column_values: JSON!, $create_labels: Boolean) {
  change_multiple_column_values(board_id: $board_id, item_id: $item_id, column_values: $column_values, create_labels_if_missing: $create_labels) {
    id
    column_values { id text value type }
  }
}`

func newItemsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "items",
		Short: "List, get, create, update, delete, and move items on monday.com boards.",
	}
	cmd.AddCommand(newItemsListCmd(flags))
	cmd.AddCommand(newItemsGetCmd(flags))
	cmd.AddCommand(newItemsCreateCmd(flags))
	cmd.AddCommand(newItemsDeleteCmd(flags))
	cmd.AddCommand(newItemsMoveCmd(flags))
	cmd.AddCommand(newItemsSetCmd(flags))
	return cmd
}

func newItemsListCmd(flags *rootFlags) *cobra.Command {
	var boardID, cursor string
	var limit int
	var all bool
	cmd := &cobra.Command{
		Use:     "list --board <id>",
		Short:   "List items on a board (cursor-paginated; --all to fetch every page).",
		Example: "  monday-pp-cli items list --board 12345 --limit 100 --all --json --select items.id,items.name",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if boardID == "" {
				return usageErr(fmt.Errorf("--board is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var allItems []json.RawMessage
			currentCursor := cursor
			for {
				vars := map[string]any{"board_id": boardID, "limit": limit}
				if currentCursor != "" {
					vars["cursor"] = currentCursor
				}
				data, err := c.GraphQL(queryItemsList, vars)
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
				page, err := pluckFirstFromArrayObject(boards, "items_page")
				if err != nil {
					return err
				}
				var pageObj struct {
					Cursor string            `json:"cursor"`
					Items  []json.RawMessage `json:"items"`
				}
				if err := json.Unmarshal(page, &pageObj); err != nil {
					return fmt.Errorf("parsing items_page: %w", err)
				}
				allItems = append(allItems, pageObj.Items...)
				if !all || pageObj.Cursor == "" || len(pageObj.Items) == 0 {
					break
				}
				currentCursor = pageObj.Cursor
			}
			out, err := json.Marshal(allItems)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().IntVar(&limit, "limit", 100, "Page size (1-500).")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Cursor for resuming a page.")
	cmd.Flags().BoolVar(&all, "all", false, "Fetch every page (loops on the cursor).")
	return cmd
}

func newItemsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Fetch one item by ID, including column values, updates, and assets.",
		Example: "  monday-pp-cli items get 1234567890 --json --select id,name,column_values",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("item id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryItemGet, map[string]any{"ids": []string{args[0]}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckFirstFromArrayField(data, "items")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func newItemsCreateCmd(flags *rootFlags) *cobra.Command {
	var boardID, groupID, name string
	var columnValues []string
	var createLabels bool
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create an item on a board.",
		Long:    "Create a new item. --column accepts repeated key=value pairs; values that look like JSON are passed as JSON, otherwise as strings.",
		Example: "  monday-pp-cli items create --board 12345 --group topics --name \"Launch checklist\" --column status=Working --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if boardID == "" || name == "" {
				return usageErr(fmt.Errorf("--board and --name are required"))
			}
			cv, err := buildColumnValuesJSON(columnValues)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{
				"board_id":      boardID,
				"name":          name,
				"create_labels": createLabels,
			}
			if groupID != "" {
				vars["group_id"] = groupID
			}
			if cv != "" {
				vars["column_values"] = cv
			}
			data, err := c.GraphQL(mutationItemsCreate, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "items.create", vars)
			}
			view, err := pluckJSONField(data, "create_item")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&groupID, "group", "", "Group ID. Defaults to the board's first group.")
	cmd.Flags().StringVar(&name, "name", "", "Item name (required).")
	cmd.Flags().StringSliceVar(&columnValues, "column", nil, "Repeatable: column-id=value (e.g. --column status=Done --column owner=12345).")
	cmd.Flags().BoolVar(&createLabels, "create-labels", false, "Create new status/dropdown labels if they don't exist yet.")
	return cmd
}

func newItemsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete an item by ID.",
		Example: "  monday-pp-cli items delete 1234567890 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("item id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationItemsDelete, map[string]any{"id": args[0]})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "delete_item")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func newItemsMoveCmd(flags *rootFlags) *cobra.Command {
	var groupID string
	cmd := &cobra.Command{
		Use:     "move <id> --group <group-id>",
		Short:   "Move an item to another group on the same board.",
		Example: "  monday-pp-cli items move 1234567890 --group done",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || groupID == "" {
				return usageErr(fmt.Errorf("item id and --group are required"))
			}
			if err := requireNumericID("item id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationItemsMove, map[string]any{"id": args[0], "group_id": groupID})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "move_item_to_group")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&groupID, "group", "", "Destination group ID (required).")
	return cmd
}

func newItemsSetCmd(flags *rootFlags) *cobra.Command {
	var boardID string
	var columnValues []string
	var createLabels bool
	cmd := &cobra.Command{
		Use:     "set <item-id>",
		Short:   "Change one or more typed column values on an item.",
		Long:    "monday.com requires --board because change_multiple_column_values is scoped per board. --column accepts repeated key=value pairs; JSON-shaped values are passed as JSON, others as strings.",
		Example: "  monday-pp-cli items set 1234567890 --board 99999 --column status=Done --column owner='{\"personsAndTeams\":[{\"id\":42,\"kind\":\"person\"}]}' --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || boardID == "" {
				return usageErr(fmt.Errorf("item id and --board are required"))
			}
			if len(columnValues) == 0 {
				return usageErr(fmt.Errorf("at least one --column is required"))
			}
			cv, err := buildColumnValuesJSON(columnValues)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationItemsSet, map[string]any{
				"item_id":       args[0],
				"board_id":      boardID,
				"column_values": cv,
				"create_labels": createLabels,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "change_multiple_column_values")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID the item belongs to (required).")
	cmd.Flags().StringSliceVar(&columnValues, "column", nil, "Repeatable: column-id=value.")
	cmd.Flags().BoolVar(&createLabels, "create-labels", false, "Create new status/dropdown labels if they don't exist yet.")
	return cmd
}

// buildColumnValuesJSON converts repeated --column key=value pairs into the
// JSON-encoded string monday.com expects. Values that parse as JSON are
// embedded as JSON; everything else becomes a quoted string. Status/text
// columns can usually pass plain strings; person/dropdown/timeline need JSON.
func buildColumnValuesJSON(pairs []string) (string, error) {
	if len(pairs) == 0 {
		return "", nil
	}
	out := make(map[string]any, len(pairs))
	for _, p := range pairs {
		idx := strings.IndexByte(p, '=')
		if idx <= 0 {
			return "", fmt.Errorf("--column %q must be of the form key=value", p)
		}
		key := strings.TrimSpace(p[:idx])
		val := p[idx+1:]
		var parsed any
		if err := json.Unmarshal([]byte(val), &parsed); err == nil {
			out[key] = parsed
		} else {
			out[key] = val
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encoding column values: %w", err)
	}
	return string(b), nil
}
