// Sprints, assets, meetings, notifications alias, and sql command.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/monday/internal/store"
	"github.com/spf13/cobra"
)

// ──────────────────────────────────────────────────────────────────────────
// sprints (monday-dev)
// ──────────────────────────────────────────────────────────────────────────

const querySprintsBoards = `query {
  boards(board_kind: public, limit: 100) {
    id
    name
    workspace_id
    items_count
    state
    description
  }
}`

const querySprintsForBoard = `query($id: [ID!]) {
  boards(ids: $id) {
    id
    name
    items_count
    columns(types: [status]) { id title settings_str }
  }
}`

func newSprintsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sprints",
		Short: "List sprint boards and per-board sprint metadata (monday-dev).",
	}
	cmd.AddCommand(newSprintsBoardsCmd(flags))
	cmd.AddCommand(newSprintsMetadataCmd(flags))
	return cmd
}

func newSprintsBoardsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "boards",
		Short:   "List monday-dev sprint boards (public boards in the account).",
		Example: "  monday-pp-cli sprints boards --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(querySprintsBoards, nil)
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
	return cmd
}

func newSprintsMetadataCmd(flags *rootFlags) *cobra.Command {
	var boardID string
	cmd := &cobra.Command{
		Use:     "metadata --board <id>",
		Short:   "Fetch a board's status-column metadata (the sprint stages).",
		Example: "  monday-pp-cli sprints metadata --board 12345 --json",
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
			data, err := c.GraphQL(querySprintsForBoard, map[string]any{"id": []string{boardID}})
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
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// assets
// ──────────────────────────────────────────────────────────────────────────

const queryAssetsForItem = `query($ids: [ID!]) {
  items(ids: $ids) {
    id
    assets {
      id
      name
      url
      file_extension
      file_size
      created_at
      uploaded_by { id name }
    }
  }
}`

func newAssetsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "List file assets on monday.com items.",
	}
	cmd.AddCommand(newAssetsListCmd(flags))
	return cmd
}

func newAssetsListCmd(flags *rootFlags) *cobra.Command {
	var itemID string
	cmd := &cobra.Command{
		Use:     "list --item <id>",
		Short:   "List assets attached to one item.",
		Example: "  monday-pp-cli assets list --item 12345 --json --select id,name,url",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if itemID == "" {
				return usageErr(fmt.Errorf("--item is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryAssetsForItem, map[string]any{"ids": []string{itemID}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			items, err := pluckJSONField(data, "items")
			if err != nil {
				return err
			}
			var arr []map[string]json.RawMessage
			_ = json.Unmarshal(items, &arr)
			if len(arr) == 0 {
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), arr[0]["assets"], flags)
		},
	}
	cmd.Flags().StringVar(&itemID, "item", "", "Item ID (required).")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// meetings (notetaker)
// ──────────────────────────────────────────────────────────────────────────

const queryMeetingsList = `query($limit: Int!, $cursor: String) {
  notetaker {
    meetings(limit: $limit, cursor: $cursor) {
      meetings {
        id
        title
        start_time
        end_time
        summary
      }
      page_info { has_next_page cursor }
    }
  }
}`

func newMeetingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meetings",
		Short: "List notetaker meetings (titles, start/end time, AI-generated summaries).",
	}
	cmd.AddCommand(newMeetingsListCmd(flags))
	return cmd
}

func newMeetingsListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List notetaker meetings with title, start/end time, and AI-generated summary, paginated by --limit and a --cursor.",
		Example: "  monday-pp-cli meetings list --limit 25 --json --select id,title,start_time,summary",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"limit": limit}
			if cursor != "" {
				vars["cursor"] = cursor
			}
			data, err := c.GraphQL(queryMeetingsList, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			// Response shape: notetaker.meetings is a MeetingsResponse object
			// ({ meetings: [...], page_info }), so the list is nested one level
			// below the notetaker.meetings field.
			notetaker, err := pluckJSONField(data, "notetaker")
			if err != nil {
				return err
			}
			view := json.RawMessage("[]")
			var nextCursor string
			var hasNext bool
			var nt map[string]json.RawMessage
			if json.Unmarshal(notetaker, &nt) == nil {
				if resp, ok := nt["meetings"]; ok {
					var mr map[string]json.RawMessage
					if json.Unmarshal(resp, &mr) == nil {
						if list, ok := mr["meetings"]; ok && len(list) > 0 && string(list) != "null" {
							view = list
						}
						if pi, ok := mr["page_info"]; ok {
							var pageInfo struct {
								HasNextPage bool   `json:"has_next_page"`
								Cursor      string `json:"cursor"`
							}
							if json.Unmarshal(pi, &pageInfo) == nil {
								hasNext = pageInfo.HasNextPage
								nextCursor = pageInfo.Cursor
							}
						}
					}
				}
			}
			if err := printOutputWithFlags(cmd.OutOrStdout(), view, flags); err != nil {
				return err
			}
			// Surface the pagination cursor so --cursor is usable for the next
			// page. Goes to stderr to keep stdout a clean meetings list.
			if hasNext && nextCursor != "" {
				fmt.Fprintf(os.Stderr, "next page: --cursor %s\n", nextCursor)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Page size.")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous page's page_info.cursor (omit for the first page).")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// notifications: alias for `notify` to satisfy the SKILL doc reference
// ──────────────────────────────────────────────────────────────────────────

func newNotificationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "Send notifications to monday.com users (alias for `notify`).",
	}
	cmd.AddCommand(newNotificationsCreateCmd(flags))
	return cmd
}

func newNotificationsCreateCmd(flags *rootFlags) *cobra.Command {
	var userID, targetID, text, targetType string
	cmd := &cobra.Command{
		Use:     "create --user <id> --target-id <id> --text <body>",
		Short:   "Send a notification to a user (same shape as `monday-pp-cli notify`).",
		Example: "  monday-pp-cli notifications create --user 12345 --target-id 99999 --text \"Standup\" --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if userID == "" || targetID == "" || text == "" {
				return usageErr(fmt.Errorf("--user, --target-id, and --text are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationCreateNotification, map[string]any{
				"user_id":     userID,
				"target_id":   targetID,
				"text":        text,
				"target_type": targetType,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "create_notification")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&userID, "user", "", "Recipient user ID (required).")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target object ID — item or update (required).")
	cmd.Flags().StringVar(&text, "text", "", "Notification body (required).")
	cmd.Flags().StringVar(&targetType, "target-type", "Project", "Target type: Project (item) or Post (update).")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// sql: read-only message until the local store is populated
// ──────────────────────────────────────────────────────────────────────────

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "sql <query>",
		Short:   "Run a read-only SELECT against the local SQLite store.",
		Long:    "Runs a read-only SELECT/WITH query against monday-pp-cli's local SQLite store populated by sync. Raw monday objects are available in the generic resources table as id, resource_type, data.",
		Example: "  monday-pp-cli sql \"SELECT resource_type, COUNT(*) FROM resources GROUP BY resource_type\" --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if err := validateMondayReadOnlySQL(query); err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("monday-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			rows, err := db.Query(query)
			if err != nil {
				return err
			}
			defer rows.Close()
			out, err := sqlRowsToObjects(rows)
			if err != nil {
				return err
			}
			data, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/monday-pp-cli/data.db)")
	return cmd
}

func validateMondayReadOnlySQL(query string) error {
	lower := strings.ToLower(strings.TrimSpace(query))
	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")) {
		return fmt.Errorf("only read-only SELECT/WITH queries are allowed")
	}
	blocked := []string{";", " insert ", " update ", " delete ", " drop ", " alter ", " create ", " replace ", " attach ", " detach ", " pragma ", " vacuum "}
	padded := " " + lower + " "
	for _, token := range blocked {
		if strings.Contains(padded, token) {
			return fmt.Errorf("query contains blocked SQL token %q", strings.TrimSpace(token))
		}
	}
	return nil
}

func sqlRowsToObjects(rows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(...any) error
	Err() error
}) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
