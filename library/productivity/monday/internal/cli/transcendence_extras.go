// Velocity, resolve, and reconcile transcendence commands.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ──────────────────────────────────────────────────────────────────────────
// velocity: sprint slip / committed / done report from activity log
// ──────────────────────────────────────────────────────────────────────────

const queryVelocityActivity = `query($id: [ID!], $limit: Int!, $page: Int!, $column_ids: [String!]) {
  boards(ids: $id) {
    name
    activity_logs(limit: $limit, page: $page, column_ids: $column_ids) {
      id
      event
      data
      created_at
    }
  }
}`

const queryVelocityItems = `query($id: [ID!], $cursor: String) {
  boards(ids: $id) {
    items_page(limit: 200, cursor: $cursor) {
      cursor
      items {
        id
        name
        created_at
        column_values { id text type column { title type } }
      }
    }
  }
}`

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var boardID, statusColumn, doneStatus string
	var maxPages int
	cmd := &cobra.Command{
		Use:     "velocity",
		Short:   "Per-status velocity report on a sprint board: total items, in each status today, and rows that transitioned to done in the activity-log window.",
		Long:    "Combines current item state (per-status counts) with activity-log transitions (entered each status, exited each status) so you can see committed-vs-done and stuck-vs-flowing at a glance. Defaults assume a 'status' column with a 'Done' label; override via --status-column / --done-status.",
		Example: "  monday-pp-cli velocity --board 12345 --json",
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
			// Current state — per-status counts.
			currentByStatus := make(map[string]int)
			cursor := ""
			total := 0
			for {
				vars := map[string]any{"id": []string{boardID}}
				if cursor != "" {
					vars["cursor"] = cursor
				}
				data, err := c.GraphQL(queryVelocityItems, vars)
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
				var bs []map[string]json.RawMessage
				if err := json.Unmarshal(boards, &bs); err != nil {
					return err
				}
				if len(bs) == 0 {
					break
				}
				page := bs[0]["items_page"]
				var pg struct {
					Cursor string            `json:"cursor"`
					Items  []json.RawMessage `json:"items"`
				}
				_ = json.Unmarshal(page, &pg)
				for _, raw := range pg.Items {
					total++
					var it struct {
						ColumnValues []struct {
							ID     string `json:"id"`
							Text   string `json:"text"`
							Column struct {
								Title string `json:"title"`
								Type  string `json:"type"`
							} `json:"column"`
						} `json:"column_values"`
					}
					_ = json.Unmarshal(raw, &it)
					for _, cv := range it.ColumnValues {
						if cv.Column.Type == "status" || cv.ID == statusColumn || strings.EqualFold(cv.Column.Title, statusColumn) {
							s := cv.Text
							if s == "" {
								s = "(empty)"
							}
							currentByStatus[s]++
							break
						}
					}
				}
				if pg.Cursor == "" || len(pg.Items) == 0 {
					break
				}
				cursor = pg.Cursor
			}

			// Activity log — transitions into the done state.
			enteredDone := 0
			leftDone := 0
			for page := 1; page <= maxPages; page++ {
				vars := map[string]any{
					"id":         []string{boardID},
					"limit":      5000,
					"page":       page,
					"column_ids": []string{statusColumn},
				}
				data, err := c.GraphQL(queryVelocityActivity, vars)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				boards, err := pluckJSONField(data, "boards")
				if err != nil {
					return err
				}
				logs, err := pluckFirstFromArrayObject(boards, "activity_logs")
				if err != nil {
					return err
				}
				var rows []map[string]json.RawMessage
				if err := json.Unmarshal(logs, &rows); err != nil {
					break
				}
				if len(rows) == 0 {
					break
				}
				for _, r := range rows {
					var event, dataStr string
					_ = json.Unmarshal(r["event"], &event)
					_ = json.Unmarshal(r["data"], &dataStr)
					var inner map[string]any
					_ = json.Unmarshal([]byte(dataStr), &inner)
					prev := shortStatusLabel(jsonGetString(inner, "previous_value"))
					newv := shortStatusLabel(jsonGetString(inner, "value"))
					if event == "update_column_value" {
						if newv == doneStatus && prev != doneStatus {
							enteredDone++
						}
						if prev == doneStatus && newv != doneStatus {
							leftDone++
						}
					}
				}
				if len(rows) < 5000 {
					break
				}
			}

			result := map[string]any{
				"board_id":                boardID,
				"total_items":             total,
				"current_items_by_status": currentByStatus,
				"transitions_into_done":   enteredDone,
				"transitions_out_of_done": leftDone,
				"status_column":           statusColumn,
				"done_status":             doneStatus,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&statusColumn, "status-column", "status", "Column ID or title that holds status.")
	cmd.Flags().StringVar(&doneStatus, "done-status", "Done", "Status label that means \"completed\" (used to count entries into done).")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum activity-log pages to scan (5000 events each).")
	_ = cmd.MarkFlagRequired("board")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// resolve: walk mirror/formula columns to find their source value
// ──────────────────────────────────────────────────────────────────────────

const queryResolveItem = `query($ids: [ID!]) {
  items(ids: $ids) {
    id
    name
    board { id name }
    column_values {
      id
      text
      value
      type
      column { id title type settings_str }
      ... on MirrorValue {
        display_value
        mirrored_items { linked_item { id name } linked_board { id name } }
      }
      ... on FormulaValue {
        display_value
      }
    }
  }
}`

func newResolveCmd(flags *rootFlags) *cobra.Command {
	var itemID, columnID string
	cmd := &cobra.Command{
		Use:     "resolve",
		Short:   "Walk mirror and formula columns on an item to show source board/item and resolved value.",
		Long:    "For mirror columns: shows the source board → source item → linked column and the resolved display value. For formula columns: shows the resolved display value (the formula expression itself is in column.settings_str).",
		Example: "  monday-pp-cli resolve --item 1234567890 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireNumericID("item id", itemID); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryResolveItem, map[string]any{"ids": []string{itemID}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			it, err := pluckFirstFromArrayField(data, "items")
			if err != nil {
				return err
			}
			var item struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Board struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"board"`
				ColumnValues []json.RawMessage `json:"column_values"`
			}
			if err := json.Unmarshal(it, &item); err != nil {
				return fmt.Errorf("parsing item: %w", err)
			}
			type chainEntry struct {
				ColumnID    string          `json:"column_id"`
				ColumnTitle string          `json:"column_title"`
				ColumnType  string          `json:"column_type"`
				Value       string          `json:"value,omitempty"`
				Display     string          `json:"display_value,omitempty"`
				MirrorOf    json.RawMessage `json:"mirror_of,omitempty"`
				Settings    string          `json:"settings_str,omitempty"`
			}
			out := make([]chainEntry, 0)
			for _, raw := range item.ColumnValues {
				var cv struct {
					ID     string `json:"id"`
					Text   string `json:"text"`
					Type   string `json:"type"`
					Column struct {
						ID       string `json:"id"`
						Title    string `json:"title"`
						Type     string `json:"type"`
						Settings string `json:"settings_str"`
					} `json:"column"`
					DisplayValue  string          `json:"display_value"`
					MirroredItems json.RawMessage `json:"mirrored_items"`
				}
				_ = json.Unmarshal(raw, &cv)
				if columnID != "" && cv.Column.ID != columnID {
					continue
				}
				if cv.Column.Type != "mirror" && cv.Column.Type != "formula" && columnID == "" {
					continue
				}
				e := chainEntry{
					ColumnID:    cv.Column.ID,
					ColumnTitle: cv.Column.Title,
					ColumnType:  cv.Column.Type,
					Value:       cv.Text,
					Display:     cv.DisplayValue,
				}
				if cv.Column.Type == "mirror" && len(cv.MirroredItems) > 0 {
					e.MirrorOf = cv.MirroredItems
				}
				if cv.Column.Type == "formula" {
					e.Settings = cv.Column.Settings
				}
				out = append(out, e)
			}
			result := map[string]any{
				"item_id":        item.ID,
				"item_name":      item.Name,
				"board_id":       item.Board.ID,
				"board_name":     item.Board.Name,
				"resolved_chain": out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&itemID, "item", "", "Item ID to resolve (required).")
	cmd.Flags().StringVar(&columnID, "column", "", "Optional: resolve only this column id (defaults: every mirror/formula column on the item).")
	_ = cmd.MarkFlagRequired("item")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// reconcile: join a board to an external CSV
// ──────────────────────────────────────────────────────────────────────────

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var boardID, csvPath, keyColumn, mondayColumn string
	cmd := &cobra.Command{
		Use:     "reconcile",
		Short:   "Compare a Monday board against an external CSV; emit only-in-monday, only-in-csv, and diff sets.",
		Long:    "Joins Monday items to a CSV by matching the CSV's --key column against the Monday column id named by --monday-column (defaults to item name). Emits three lists: only_in_monday, only_in_csv, both. Useful for syncing Monday with Salesforce, Stripe, or any other system that owns the same identifiers.",
		Example: "  monday-pp-cli reconcile --board 12345 --against-csv salesforce-export.csv --key sf_account_id --monday-column salesforce_id --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if _, err := os.Stat(csvPath); err != nil {
					// Dry-run with no input file: emit a valid JSON preview
					// instead of empty stdout, mirroring bulk-edit, so
					// `--dry-run --json` is machine-parseable.
					return emitDryRun(cmd, flags, "reconcile", map[string]any{
						"board":         boardID,
						"against_csv":   csvPath,
						"key":           keyColumn,
						"monday_column": mondayColumn,
						"note":          "input CSV not found; nothing to preview",
					})
				}
			}
			f, err := os.Open(csvPath)
			if err != nil {
				return fmt.Errorf("opening %s: %w", csvPath, err)
			}
			defer f.Close()
			rows, err := csv.NewReader(f).ReadAll()
			if err != nil {
				return fmt.Errorf("parsing CSV: %w", err)
			}
			if len(rows) < 2 {
				return fmt.Errorf("CSV must have a header row + at least one data row")
			}
			header := rows[0]
			keyIdx := -1
			for i, h := range header {
				if strings.EqualFold(h, keyColumn) {
					keyIdx = i
					break
				}
			}
			if keyIdx < 0 {
				return fmt.Errorf("CSV is missing the --key column %q", keyColumn)
			}
			csvByKey := make(map[string]map[string]string, len(rows)-1)
			for _, r := range rows[1:] {
				if len(r) != len(header) {
					continue
				}
				k := strings.TrimSpace(r[keyIdx])
				if k == "" {
					continue
				}
				m := make(map[string]string, len(header))
				for i, h := range header {
					m[h] = r[i]
				}
				csvByKey[k] = m
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cursor := ""
			mondayByKey := make(map[string]map[string]any)
			for {
				vars := map[string]any{"board_id": boardID, "limit": 200}
				if cursor != "" {
					vars["cursor"] = cursor
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
				var bs []map[string]json.RawMessage
				if err := json.Unmarshal(boards, &bs); err != nil {
					return err
				}
				if len(bs) == 0 {
					break
				}
				page := bs[0]["items_page"]
				var pg struct {
					Cursor string            `json:"cursor"`
					Items  []json.RawMessage `json:"items"`
				}
				_ = json.Unmarshal(page, &pg)
				for _, raw := range pg.Items {
					var it struct {
						ID           string `json:"id"`
						Name         string `json:"name"`
						State        string `json:"state"`
						ColumnValues []struct {
							ID     string `json:"id"`
							Text   string `json:"text"`
							Column struct {
								Title string `json:"title"`
							} `json:"column"`
						} `json:"column_values"`
					}
					if err := json.Unmarshal(raw, &it); err != nil {
						continue
					}
					var key string
					if mondayColumn == "" || strings.EqualFold(mondayColumn, "name") {
						key = it.Name
					} else {
						for _, cv := range it.ColumnValues {
							if cv.ID == mondayColumn || strings.EqualFold(cv.Column.Title, mondayColumn) {
								key = cv.Text
								break
							}
						}
					}
					if key == "" {
						continue
					}
					mondayByKey[key] = map[string]any{
						"id":    it.ID,
						"name":  it.Name,
						"state": it.State,
					}
				}
				if pg.Cursor == "" || len(pg.Items) == 0 {
					break
				}
				cursor = pg.Cursor
			}

			onlyInMonday := make([]map[string]any, 0)
			onlyInCSV := make([]map[string]string, 0)
			both := make([]map[string]any, 0)
			for k, m := range mondayByKey {
				if csvRow, ok := csvByKey[k]; ok {
					both = append(both, map[string]any{
						"key":    k,
						"monday": m,
						"csv":    csvRow,
					})
				} else {
					m["__key"] = k
					onlyInMonday = append(onlyInMonday, m)
				}
			}
			for k, csvRow := range csvByKey {
				if _, ok := mondayByKey[k]; !ok {
					csvRow["__key"] = k
					onlyInCSV = append(onlyInCSV, csvRow)
				}
			}
			sort.Slice(onlyInMonday, func(i, j int) bool {
				return fmt.Sprint(onlyInMonday[i]["__key"]) < fmt.Sprint(onlyInMonday[j]["__key"])
			})
			sort.Slice(onlyInCSV, func(i, j int) bool { return onlyInCSV[i]["__key"] < onlyInCSV[j]["__key"] })
			sort.Slice(both, func(i, j int) bool {
				return fmt.Sprint(both[i]["key"]) < fmt.Sprint(both[j]["key"])
			})
			result := map[string]any{
				"board_id":       boardID,
				"key_column":     keyColumn,
				"monday_column":  mondayColumn,
				"only_in_monday": onlyInMonday,
				"only_in_csv":    onlyInCSV,
				"both":           both,
				"summary": map[string]int{
					"only_in_monday": len(onlyInMonday),
					"only_in_csv":    len(onlyInCSV),
					"both":           len(both),
				},
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Monday board ID (required).")
	cmd.Flags().StringVar(&csvPath, "against-csv", "", "Path to the external CSV (required).")
	cmd.Flags().StringVar(&keyColumn, "key", "", "Name of the CSV column that holds the join key (required).")
	cmd.Flags().StringVar(&mondayColumn, "monday-column", "name", "Monday column id or title to match (default: name).")
	_ = cmd.MarkFlagRequired("board")
	_ = cmd.MarkFlagRequired("against-csv")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}
