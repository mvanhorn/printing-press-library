// More transcendence commands: mentions, bulk-edit, whoami-load, search,
// whoami, boards-health.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ──────────────────────────────────────────────────────────────────────────
// whoami: thin alias for account.me
// ──────────────────────────────────────────────────────────────────────────

func newWhoamiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "whoami",
		Short:   "Print the authenticated monday.com user (alias for `account me`).",
		Example: "  monday-pp-cli whoami --json --select name,email,account.name",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryAccountMe, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "me")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// search: cross-resource live search using monday's `items_page_by_column_values`
// ──────────────────────────────────────────────────────────────────────────

const querySearchItems = `query($board_id: ID!, $columns: [ItemsPageByColumnValuesQuery!]!) {
  items_page_by_column_values(board_id: $board_id, columns: $columns, limit: 100) {
    items {
      id
      name
      board { id name }
      group { id title }
      column_values { id text type column { id title } }
    }
  }
}`

const querySearchByName = `query($board_id: ID!, $rules: [ItemsQueryRule!], $term: CompareValue) {
  boards(ids: [$board_id]) {
    items_page(limit: 100, query_params: { rules: $rules, operator: and }) {
      items {
        id
        name
        group { id title }
        column_values { id text type column { id title } }
      }
    }
  }
}`

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var boardID, columnID, term string
	cmd := &cobra.Command{
		Use:     "search <term>",
		Short:   "Search items on a board by exact column-value match (uses monday's items_page_by_column_values).",
		Long:    "monday.com's items_page_by_column_values queries items whose <column-id> equals <term>. Provide --column to search a specific column. Without --column, search defaults to text columns.",
		Example: "  monday-pp-cli search \"Acme\" --board 12345 --column status1 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				term = args[0]
			}
			if boardID == "" || term == "" {
				return usageErr(fmt.Errorf("usage: search <term> --board <id> [--column <column-id>]"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if columnID == "" {
				columnID = "name"
			}
			columns := []map[string]any{
				{"column_id": columnID, "column_values": []string{term}},
			}
			data, err := c.GraphQL(querySearchItems, map[string]any{"board_id": boardID, "columns": columns})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			page, err := pluckJSONField(data, "items_page_by_column_values")
			if err != nil {
				return err
			}
			var pg map[string]json.RawMessage
			_ = json.Unmarshal(page, &pg)
			items, ok := pg["items"]
			if !ok {
				items = json.RawMessage("[]")
			}
			return printOutputWithFlags(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&columnID, "column", "", "Column ID to search (default: name).")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// mentions: hydrated cross-board search across items + updates
// ──────────────────────────────────────────────────────────────────────────

const queryMentionsItems = `query($board_id: ID!, $cursor: String) {
  boards(ids: [$board_id]) {
    id
    name
    items_page(limit: 200, cursor: $cursor) {
      cursor
      items {
        id
        name
        group { id title }
        column_values { id text type column { id title type } }
        updates(limit: 5) { id body text_body creator_id }
      }
    }
  }
}`

func newMentionsCmd(flags *rootFlags) *cobra.Command {
	var boardsCSV string
	var includeUpdates bool
	cmd := &cobra.Command{
		Use:     "mentions <text>",
		Short:   "Search across item names, text columns, and (optionally) update bodies for a substring; results hydrated with board + group context.",
		Long:    "Walks every item on the supplied boards (cursor-paginated), case-insensitively matches the search term against item name, every text/long_text column value, and (with --updates) each item's recent update bodies. Returns one JSON row per match with full context.",
		Example: "  monday-pp-cli mentions \"Acme Corp\" --board 12345,67890 --updates --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			term := strings.ToLower(args[0])
			boardIDs := splitCSV(boardsCSV)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type hit struct {
				BoardID    string `json:"board_id"`
				BoardName  string `json:"board_name"`
				GroupTitle string `json:"group_title,omitempty"`
				ItemID     string `json:"item_id"`
				ItemName   string `json:"item_name"`
				Field      string `json:"field"`
				ColumnID   string `json:"column_id,omitempty"`
				ColumnType string `json:"column_type,omitempty"`
				Snippet    string `json:"snippet"`
			}
			results := make([]hit, 0, 64)
			for _, bid := range boardIDs {
				cursor := ""
				for {
					vars := map[string]any{"board_id": bid}
					if cursor != "" {
						vars["cursor"] = cursor
					}
					data, err := c.GraphQL(queryMentionsItems, vars)
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
						return fmt.Errorf("parsing boards: %w", err)
					}
					if len(bs) == 0 {
						break
					}
					var boardName string
					_ = json.Unmarshal(bs[0]["name"], &boardName)
					page := bs[0]["items_page"]
					var pg struct {
						Cursor string            `json:"cursor"`
						Items  []json.RawMessage `json:"items"`
					}
					if err := json.Unmarshal(page, &pg); err != nil {
						return fmt.Errorf("parsing items_page: %w", err)
					}
					for _, raw := range pg.Items {
						var it struct {
							ID    string `json:"id"`
							Name  string `json:"name"`
							Group struct {
								Title string `json:"title"`
							} `json:"group"`
							ColumnValues []struct {
								ID     string `json:"id"`
								Text   string `json:"text"`
								Type   string `json:"type"`
								Column struct {
									Title string `json:"title"`
									Type  string `json:"type"`
								} `json:"column"`
							} `json:"column_values"`
							Updates []struct {
								ID       string `json:"id"`
								Body     string `json:"body"`
								TextBody string `json:"text_body"`
							} `json:"updates"`
						}
						if err := json.Unmarshal(raw, &it); err != nil {
							continue
						}
						if strings.Contains(strings.ToLower(it.Name), term) {
							results = append(results, hit{
								BoardID: bid, BoardName: boardName, GroupTitle: it.Group.Title,
								ItemID: it.ID, ItemName: it.Name, Field: "name", Snippet: it.Name,
							})
						}
						for _, cv := range it.ColumnValues {
							if cv.Text == "" {
								continue
							}
							if strings.Contains(strings.ToLower(cv.Text), term) {
								results = append(results, hit{
									BoardID: bid, BoardName: boardName, GroupTitle: it.Group.Title,
									ItemID: it.ID, ItemName: it.Name,
									Field:      "column_value",
									ColumnID:   cv.ID,
									ColumnType: cv.Column.Type,
									Snippet:    cv.Text,
								})
							}
						}
						if includeUpdates {
							for _, u := range it.Updates {
								body := u.TextBody
								if body == "" {
									body = u.Body
								}
								if body == "" {
									continue
								}
								if strings.Contains(strings.ToLower(body), term) {
									results = append(results, hit{
										BoardID: bid, BoardName: boardName, GroupTitle: it.Group.Title,
										ItemID: it.ID, ItemName: it.Name,
										Field:   "update",
										Snippet: snippet(body, term, 80),
									})
								}
							}
						}
					}
					if pg.Cursor == "" || len(pg.Items) == 0 {
						break
					}
					cursor = pg.Cursor
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&boardsCSV, "board", "", "Comma-separated board IDs to search (required).")
	cmd.Flags().BoolVar(&includeUpdates, "updates", false, "Also search update (comment) bodies.")
	_ = cmd.MarkFlagRequired("board")
	return cmd
}

func snippet(body, term string, width int) string {
	lower := strings.ToLower(body)
	idx := strings.Index(lower, term)
	if idx < 0 {
		return truncate(body, width)
	}
	// Work in runes to avoid slicing mid-character on multi-byte UTF-8.
	runes := []rune(body)
	// Find the rune index corresponding to byte offset idx in the original body.
	runeIdx := len([]rune(body[:idx]))
	half := width / 2
	start := runeIdx - half
	if start < 0 {
		start = 0
	}
	termRunes := len([]rune(term))
	end := runeIdx + termRunes + half
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

// ──────────────────────────────────────────────────────────────────────────
// whoami-load: per-person workload across boards
// ──────────────────────────────────────────────────────────────────────────

const queryWhoamiLoad = `query($board_id: ID!, $cursor: String) {
  boards(ids: [$board_id]) {
    id
    name
    items_page(limit: 200, cursor: $cursor) {
      cursor
      items {
        id
        name
        state
        column_values {
          id
          text
          value
          type
          column { title type }
        }
      }
    }
  }
}`

func newWhoamiLoadCmd(flags *rootFlags) *cobra.Command {
	var boardsCSV, statusColumn, personColumn string
	cmd := &cobra.Command{
		Use:     "whoami-load",
		Short:   "Per-person workload across the supplied boards: open-item count weighted by status.",
		Long:    "Walks every item on the supplied boards, finds the person column (default: 'person') and status column (default: 'status'), and reports per-person counts in each status. Returns one JSON row per (person, board) pair.",
		Example: "  monday-pp-cli whoami-load --board 12345,67890 --json --select person.name,total,by_status",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			boardIDs := splitCSV(boardsCSV)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type bucket struct {
				Person   string         `json:"person"`
				BoardID  string         `json:"board_id"`
				Total    int            `json:"total"`
				ByStatus map[string]int `json:"by_status"`
			}
			loads := make(map[string]*bucket)
			for _, bid := range boardIDs {
				cursor := ""
				for {
					vars := map[string]any{"board_id": bid}
					if cursor != "" {
						vars["cursor"] = cursor
					}
					data, err := c.GraphQL(queryWhoamiLoad, vars)
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
							ColumnValues []struct {
								ID     string `json:"id"`
								Text   string `json:"text"`
								Type   string `json:"type"`
								Column struct {
									Title string `json:"title"`
									Type  string `json:"type"`
								} `json:"column"`
							} `json:"column_values"`
						}
						_ = json.Unmarshal(raw, &it)
						personText, statusText := "", ""
						for _, cv := range it.ColumnValues {
							lid := strings.ToLower(cv.ID)
							ltitle := strings.ToLower(cv.Column.Title)
							if cv.Type == "people" || cv.Column.Type == "people" || lid == personColumn || ltitle == personColumn {
								if personText == "" {
									personText = cv.Text
								}
							}
							if cv.Type == "status" || cv.Column.Type == "status" || lid == statusColumn || ltitle == statusColumn {
								if statusText == "" {
									statusText = cv.Text
								}
							}
						}
						persons := strings.Split(personText, ",")
						for _, p := range persons {
							p = strings.TrimSpace(p)
							if p == "" {
								continue
							}
							key := bid + "/" + p
							b, ok := loads[key]
							if !ok {
								b = &bucket{Person: p, BoardID: bid, ByStatus: make(map[string]int)}
								loads[key] = b
							}
							b.Total++
							if statusText != "" {
								b.ByStatus[statusText]++
							}
						}
					}
					if pg.Cursor == "" || len(pg.Items) == 0 {
						break
					}
					cursor = pg.Cursor
				}
			}
			results := make([]*bucket, 0, len(loads))
			for _, b := range loads {
				results = append(results, b)
			}
			sort.Slice(results, func(i, j int) bool { return results[i].Total > results[j].Total })
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&boardsCSV, "board", "", "Comma-separated board IDs (required).")
	cmd.Flags().StringVar(&statusColumn, "status-column", "status", "Column ID or title that holds status.")
	cmd.Flags().StringVar(&personColumn, "person-column", "person", "Column ID or title that holds the assignee.")
	_ = cmd.MarkFlagRequired("board")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// bulk-edit: CSV-driven typed column-value edits
// ──────────────────────────────────────────────────────────────────────────

func newBulkEditCmd(flags *rootFlags) *cobra.Command {
	var fromCSV, boardID string
	var columns []string
	var apply, createLabels bool
	cmd := &cobra.Command{
		Use:     "bulk-edit",
		Short:   "Apply CSV-driven typed column-value edits with a unified diff in dry-run.",
		Long:    "Read a CSV with at minimum an 'id' column. Every other column whose name matches a Monday column id (or is named in --column) is staged as a column-value change. By default the command prints what would change; pass --apply to send the mutations.",
		Example: "  monday-pp-cli bulk-edit --from updates.csv --board 12345 --column status,owner",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if _, err := os.Stat(fromCSV); err != nil {
					// Dry-run with no input file: emit a valid JSON preview
					// (empty plan) instead of returning empty stdout, so
					// `--dry-run --json` is machine-parseable.
					return emitDryRun(cmd, flags, "bulk-edit", map[string]any{
						"from":        fromCSV,
						"board_id":    boardID,
						"plan_count":  0,
						"would_apply": !apply,
						"note":        "input CSV not found; nothing to preview",
					})
				}
			}
			f, err := os.Open(fromCSV)
			if err != nil {
				return fmt.Errorf("opening %s: %w", fromCSV, err)
			}
			defer f.Close()
			reader := csv.NewReader(f)
			rows, err := reader.ReadAll()
			if err != nil {
				return fmt.Errorf("parsing CSV: %w", err)
			}
			if len(rows) < 2 {
				return fmt.Errorf("CSV must have a header row + at least one data row")
			}
			header := rows[0]
			idIdx := -1
			for i, h := range header {
				if strings.EqualFold(h, "id") {
					idIdx = i
					break
				}
			}
			if idIdx < 0 {
				return fmt.Errorf("CSV must include an 'id' column")
			}
			whitelist := map[string]bool{}
			for _, c := range columns {
				whitelist[c] = true
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type plan struct {
				ItemID string         `json:"item_id"`
				Values map[string]any `json:"values"`
			}
			plans := make([]plan, 0, len(rows)-1)
			for _, row := range rows[1:] {
				if len(row) != len(header) {
					continue
				}
				id := strings.TrimSpace(row[idIdx])
				if id == "" {
					continue
				}
				values := map[string]any{}
				for i, h := range header {
					if i == idIdx {
						continue
					}
					if len(whitelist) > 0 && !whitelist[h] {
						continue
					}
					raw := row[i]
					var parsed any
					if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
						values[h] = parsed
					} else {
						values[h] = raw
					}
				}
				if len(values) == 0 {
					continue
				}
				plans = append(plans, plan{ItemID: id, Values: values})
			}
			summary := map[string]any{
				"board_id":    boardID,
				"plans":       plans,
				"plan_count":  len(plans),
				"applied":     0,
				"failed":      0,
				"errors":      []string{},
				"would_apply": !apply,
			}
			if !apply {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			applied, failed, errMsgs := 0, 0, []string{}
			for _, p := range plans {
				cv, _ := json.Marshal(p.Values)
				_, err := c.GraphQL(mutationItemsSet, map[string]any{
					"item_id":       p.ItemID,
					"board_id":      boardID,
					"column_values": string(cv),
					"create_labels": createLabels,
				})
				if err != nil {
					failed++
					errMsgs = append(errMsgs, fmt.Sprintf("item %s: %v", p.ItemID, err))
					continue
				}
				applied++
			}
			summary["applied"] = applied
			summary["failed"] = failed
			summary["errors"] = errMsgs
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	cmd.Flags().StringVar(&fromCSV, "from", "", "Path to a CSV with an 'id' column plus per-column-value columns (required).")
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID the items belong to (required).")
	cmd.Flags().StringSliceVar(&columns, "column", nil, "Whitelist of CSV column names to apply (default: all non-id).")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually run the mutations (default: dry-run + print plan).")
	cmd.Flags().BoolVar(&createLabels, "create-labels", false, "Create new status/dropdown labels if they don't exist yet.")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("board")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// boards-health: per-board scorecard (live, single board)
// ──────────────────────────────────────────────────────────────────────────

const queryBoardHealth = `query($board_id: ID!, $cursor: String) {
  boards(ids: [$board_id]) {
    id
    name
    items_count
    items_page(limit: 200, cursor: $cursor) {
      cursor
      items {
        id
        updated_at
        column_values {
          id
          text
          type
          column { id title type }
        }
      }
    }
  }
}`

func newBoardsHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "health <board-id>",
		Short:   "Per-board scorecard: % owner-set, % due-set, % overdue, % updated-7d, count empty-status, count broken-mirror.",
		Example: "  monday-pp-cli boards health 12345 --json",
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
			boardID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cursor := ""
			total := 0
			ownerSet, dueSet, overdue, updated7d := 0, 0, 0, 0
			emptyStatus, brokenMirror := 0, 0
			now := time.Now().UTC()
			cutoff := now.Add(-7 * 24 * time.Hour)
			var boardName string
			for {
				vars := map[string]any{"board_id": boardID}
				if cursor != "" {
					vars["cursor"] = cursor
				}
				data, err := c.GraphQL(queryBoardHealth, vars)
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
				_ = json.Unmarshal(bs[0]["name"], &boardName)
				page := bs[0]["items_page"]
				var pg struct {
					Cursor string            `json:"cursor"`
					Items  []json.RawMessage `json:"items"`
				}
				_ = json.Unmarshal(page, &pg)
				for _, raw := range pg.Items {
					total++
					var it struct {
						UpdatedAt    string `json:"updated_at"`
						ColumnValues []struct {
							ID     string `json:"id"`
							Text   string `json:"text"`
							Type   string `json:"type"`
							Column struct {
								Type string `json:"type"`
							} `json:"column"`
						} `json:"column_values"`
					}
					if err := json.Unmarshal(raw, &it); err != nil {
						continue
					}
					if t := parseMondayTime(it.UpdatedAt); !t.IsZero() && t.After(cutoff) {
						updated7d++
					}
					hasOwner, hasDue, isOverdue, hasStatus := false, false, false, false
					for _, cv := range it.ColumnValues {
						switch cv.Column.Type {
						case "people":
							if strings.TrimSpace(cv.Text) != "" {
								hasOwner = true
							}
						case "date", "timeline":
							if strings.TrimSpace(cv.Text) != "" {
								hasDue = true
								if t := parseMondayTime(cv.Text); !t.IsZero() && t.Before(now) {
									isOverdue = true
								}
							}
						case "status":
							if strings.TrimSpace(cv.Text) != "" {
								hasStatus = true
							}
						case "mirror":
							if strings.Contains(cv.Text, "broken") || cv.Text == "Item not found" {
								brokenMirror++
							}
						}
					}
					if hasOwner {
						ownerSet++
					}
					if hasDue {
						dueSet++
					}
					if isOverdue {
						overdue++
					}
					if !hasStatus {
						emptyStatus++
					}
				}
				if pg.Cursor == "" || len(pg.Items) == 0 {
					break
				}
				cursor = pg.Cursor
			}
			pct := func(n, d int) float64 {
				if d == 0 {
					return 0
				}
				return round2(float64(n) * 100 / float64(d))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"board_id":            boardID,
				"board_name":          boardName,
				"total_items":         total,
				"pct_owner_set":       pct(ownerSet, total),
				"pct_due_set":         pct(dueSet, total),
				"pct_overdue":         pct(overdue, total),
				"pct_updated_7d":      pct(updated7d, total),
				"empty_status_count":  emptyStatus,
				"broken_mirror_count": brokenMirror,
			}, flags)
		},
	}
	return cmd
}
