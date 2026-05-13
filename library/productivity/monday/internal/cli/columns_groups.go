// Columns and groups commands for monday.com.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const queryColumnsList = `query($board_id: [ID!]) {
  boards(ids: $board_id) {
    columns { id title type description settings_str archived width }
  }
}`

const mutationColumnsCreate = `mutation($board_id: ID!, $title: String!, $column_type: ColumnType!, $description: String, $defaults: JSON) {
  create_column(board_id: $board_id, title: $title, column_type: $column_type, description: $description, defaults: $defaults) {
    id
    title
    type
  }
}`

const mutationColumnsUpdate = `mutation($board_id: ID!, $column_id: String!, $title: String) {
  change_column_title(board_id: $board_id, column_id: $column_id, title: $title) {
    id
    title
  }
}`

const mutationColumnsDelete = `mutation($board_id: ID!, $column_id: String!) {
  delete_column(board_id: $board_id, column_id: $column_id) { id }
}`

const queryGroupsList = `query($board_id: [ID!]) {
  boards(ids: $board_id) {
    groups { id title color position archived deleted }
  }
}`

const mutationGroupsCreate = `mutation($board_id: ID!, $name: String!, $color: String) {
  create_group(board_id: $board_id, group_name: $name, group_color: $color) {
    id
    title
    color
  }
}`

func newColumnsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "columns",
		Short: "List, create, update, and delete columns on a board, plus column-type schema.",
	}
	cmd.AddCommand(newColumnsListCmd(flags))
	cmd.AddCommand(newColumnsCreateCmd(flags))
	cmd.AddCommand(newColumnsUpdateCmd(flags))
	cmd.AddCommand(newColumnsDeleteCmd(flags))
	cmd.AddCommand(newColumnsTypesCmd(flags))
	return cmd
}

func newColumnsListCmd(flags *rootFlags) *cobra.Command {
	var boardID string
	cmd := &cobra.Command{
		Use:     "list --board <id>",
		Short:   "List columns on a board.",
		Example: "  monday-pp-cli columns list --board 12345 --json",
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
			data, err := c.GraphQL(queryColumnsList, map[string]any{"board_id": []string{boardID}})
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
			view, err := pluckFirstFromArrayObject(boards, "columns")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	return cmd
}

func newColumnsCreateCmd(flags *rootFlags) *cobra.Command {
	var boardID, title, colType, description string
	cmd := &cobra.Command{
		Use:     "create --board <id> --title <name> --type <type>",
		Short:   "Create a column on a board.",
		Long:    "Valid types include: status, color, text, long_text, numbers, date, timeline, dropdown, people, world_clock, location, country, link, file, board_relation, mirror, formula, dependency, last_updated, creation_log, item_id, hour, week, button, vote, rating, tags, email, phone, checkbox, integration.",
		Example: "  monday-pp-cli columns create --board 12345 --title \"Owner\" --type people --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if boardID == "" || title == "" || colType == "" {
				return usageErr(fmt.Errorf("--board, --title, and --type are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{
				"board_id":    boardID,
				"title":       title,
				"column_type": colType,
			}
			if description != "" {
				vars["description"] = description
			}
			data, err := c.GraphQL(mutationColumnsCreate, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "create_column")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&title, "title", "", "Column title (required).")
	cmd.Flags().StringVar(&colType, "type", "", "Column type (required) — see --help for the full list.")
	cmd.Flags().StringVar(&description, "description", "", "Optional column description.")
	return cmd
}

func newColumnsUpdateCmd(flags *rootFlags) *cobra.Command {
	var boardID, columnID, title string
	cmd := &cobra.Command{
		Use:     "update --board <id> --column <column-id> --title <new-title>",
		Short:   "Rename a column on a board.",
		Example: "  monday-pp-cli columns update --board 12345 --column status1 --title \"Stage\" --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if boardID == "" || columnID == "" || title == "" {
				return usageErr(fmt.Errorf("--board, --column, and --title are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationColumnsUpdate, map[string]any{
				"board_id":  boardID,
				"column_id": columnID,
				"title":     title,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "change_column_title")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&columnID, "column", "", "Column ID (required).")
	cmd.Flags().StringVar(&title, "title", "", "New column title (required).")
	return cmd
}

func newColumnsDeleteCmd(flags *rootFlags) *cobra.Command {
	var boardID, columnID string
	cmd := &cobra.Command{
		Use:     "delete --board <id> --column <column-id>",
		Short:   "Delete a column from a board.",
		Example: "  monday-pp-cli columns delete --board 12345 --column dropdown1 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if boardID == "" || columnID == "" {
				return usageErr(fmt.Errorf("--board and --column are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationColumnsDelete, map[string]any{
				"board_id":  boardID,
				"column_id": columnID,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "delete_column")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&columnID, "column", "", "Column ID (required).")
	return cmd
}

// columnTypes is the static reference of monday.com column types and their
// JSON shapes for `change_column_value`. Sourced from monday's API reference;
// the most useful subset that an agent typically needs.
var columnTypes = []map[string]string{
	{"type": "status", "value_shape": `{"label":"Done"}`, "notes": "Pass label name as plain string also works for default status columns."},
	{"type": "text", "value_shape": `"plain string"`, "notes": "Plain text column."},
	{"type": "long_text", "value_shape": `{"text":"long body..."}`, "notes": "Multi-line text."},
	{"type": "numbers", "value_shape": `"42"`, "notes": "Numeric column; pass as string."},
	{"type": "date", "value_shape": `{"date":"2026-12-31","time":"15:00:00"}`, "notes": "Date or date+time."},
	{"type": "timeline", "value_shape": `{"from":"2026-01-01","to":"2026-01-31"}`, "notes": "Two-date span."},
	{"type": "dropdown", "value_shape": `{"labels":["Backend","API"]}`, "notes": "Multiple labels by name."},
	{"type": "people", "value_shape": `{"personsAndTeams":[{"id":12345,"kind":"person"}]}`, "notes": "Persons or teams; kind is person|team."},
	{"type": "checkbox", "value_shape": `{"checked":"true"}`, "notes": "True/false."},
	{"type": "link", "value_shape": `{"url":"https://example.com","text":"Link text"}`, "notes": "URL with display text."},
	{"type": "email", "value_shape": `{"email":"a@b.com","text":"Display"}`, "notes": "Email with display."},
	{"type": "phone", "value_shape": `{"phone":"+15551234567","countryShortName":"US"}`, "notes": "Phone with country."},
	{"type": "rating", "value_shape": `{"rating":4}`, "notes": "1-5 rating."},
	{"type": "world_clock", "value_shape": `{"timezone":"America/New_York"}`, "notes": "Timezone."},
	{"type": "location", "value_shape": `{"address":"...","lat":"...","lng":"..."}`, "notes": "Geographic location."},
	{"type": "country", "value_shape": `{"countryCode":"US","countryName":"United States"}`, "notes": "Country code + name."},
	{"type": "tags", "value_shape": `{"tag_ids":[12345,67890]}`, "notes": "Pre-existing tag IDs."},
	{"type": "vote", "value_shape": `(read-only via mutation; users vote in the UI)`, "notes": "Read-only via API."},
	{"type": "mirror", "value_shape": `(read-only; reflects another column)`, "notes": "Read-only mirror column."},
	{"type": "formula", "value_shape": `(read-only; computed from other columns)`, "notes": "Read-only formula column."},
	{"type": "dependency", "value_shape": `{"item_ids":[12345]}`, "notes": "Item dependencies."},
	{"type": "board_relation", "value_shape": `{"item_ids":[12345]}`, "notes": "Cross-board item links."},
}

func newColumnsTypesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "types",
		Short:   "Print the static reference of monday.com column types and their JSON value shapes.",
		Long:    "Each column type expects a different JSON shape when sending column_values to create_item / change_multiple_column_values. This is the cheat-sheet — no API call.",
		Example: "  monday-pp-cli columns types --json | jq '.[] | select(.type==\"people\")'",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSONFiltered(cmd.OutOrStdout(), columnTypes, flags)
		},
	}
	return cmd
}

func newGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List and create groups on a board.",
	}
	cmd.AddCommand(newGroupsListCmd(flags))
	cmd.AddCommand(newGroupsCreateCmd(flags))
	return cmd
}

func newGroupsListCmd(flags *rootFlags) *cobra.Command {
	var boardID string
	cmd := &cobra.Command{
		Use:     "list --board <id>",
		Short:   "List groups on a board.",
		Example: "  monday-pp-cli groups list --board 12345 --json",
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
			data, err := c.GraphQL(queryGroupsList, map[string]any{"board_id": []string{boardID}})
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
			view, err := pluckFirstFromArrayObject(boards, "groups")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	return cmd
}

func newGroupsCreateCmd(flags *rootFlags) *cobra.Command {
	var boardID, name, color string
	cmd := &cobra.Command{
		Use:     "create --board <id> --name <name>",
		Short:   "Create a group on a board.",
		Example: "  monday-pp-cli groups create --board 12345 --name \"In Review\" --color #784BD1 --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if boardID == "" || name == "" {
				return usageErr(fmt.Errorf("--board and --name are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"board_id": boardID, "name": name}
			if color != "" {
				vars["color"] = color
			}
			data, err := c.GraphQL(mutationGroupsCreate, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "create_group")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&name, "name", "", "Group name (required).")
	cmd.Flags().StringVar(&color, "color", "", "Hex color (optional).")
	return cmd
}
