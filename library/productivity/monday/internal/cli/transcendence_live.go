// Transcendence commands that operate against the live monday.com API
// (no local store dependency): since, complexity-budget, cross-ref, context,
// bottleneck (computed from API activity_logs), column-drift (single-call diff
// against cached schema), mentions (single-call cross-resource search via the
// items_page + updates queries).

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ──────────────────────────────────────────────────────────────────────────
// since: cross-board activity tail
// ──────────────────────────────────────────────────────────────────────────

const querySinceActivity = `query($id: [ID!], $limit: Int!, $page: Int!, $from: ISO8601DateTime, $user_ids: [ID!]) {
  boards(ids: $id) {
    id
    name
    activity_logs(limit: $limit, page: $page, from: $from, user_ids: $user_ids) {
      id
      event
      data
      entity
      created_at
      user_id
    }
  }
}`

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var boardsCSV, userIDs, fromISO string
	var limit int
	cmd := &cobra.Command{
		Use:     "since <window>",
		Short:   "Replay activity log changes across boards within a window (e.g. 2h, 1d, 1w) or since an ISO timestamp.",
		Long:    "Pulls recent column-value changes, item moves, and creations across one or more boards. Use --board to scope to specific boards (otherwise the command refuses to fetch every board the token can see, which is slow).",
		Example: "  monday-pp-cli since 2h --board 12345,67890 --json --select event,user_id,data.column_id\n  monday-pp-cli since 2026-04-01T00:00:00Z --board 12345 --user-ids 9999",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			window := args[0]
			boardIDs := splitCSV(boardsCSV)
			// --from overrides the positional window with an explicit lower
			// bound. Route both through parseSinceWindow so the value is
			// validated and normalized to RFC3339 before it reaches the API;
			// previously --from was discarded and the raw window string (e.g.
			// "2h") was sent as an ISO8601DateTime, which the API rejects.
			src := window
			if fromISO != "" {
				src = fromISO
			}
			t, err := parseSinceWindow(src)
			if err != nil {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(err)
			}
			from := t.UTC().Format(time.RFC3339)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{
				"id":    boardIDs,
				"limit": limit,
				"from":  from,
			}
			if uids := splitCSV(userIDs); len(uids) > 0 {
				vars["user_ids"] = uids
			}
			// activity_logs paginates per board, so a single page-1 call drops
			// every event past --limit on a busy board, contradicting the
			// "tail every change since a window" promise. Walk pages and
			// accumulate per board until every board returns a short (< limit)
			// page, i.e. no board has more activity. maxSincePages is a runaway
			// guard, not a functional limit; hitting it is surfaced on stderr
			// rather than silently truncating.
			const maxSincePages = 200
			type boardAccum struct {
				name    string
				entries []json.RawMessage
			}
			order := make([]string, 0, len(boardIDs))
			accum := make(map[string]*boardAccum)
			truncated := false
			for page := 1; ; page++ {
				vars["page"] = page
				data, err := c.GraphQL(querySinceActivity, vars)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if dryRunOK(flags) {
					return nil
				}
				rawBoards, err := pluckJSONField(data, "boards")
				if err != nil {
					return err
				}
				var boards []map[string]json.RawMessage
				if err := json.Unmarshal(rawBoards, &boards); err != nil {
					return fmt.Errorf("parsing boards: %w", err)
				}
				maxRows := 0
				for _, b := range boards {
					var id, name string
					_ = json.Unmarshal(b["id"], &id)
					_ = json.Unmarshal(b["name"], &name)
					var logs []json.RawMessage
					if raw := b["activity_logs"]; len(raw) > 0 {
						_ = json.Unmarshal(raw, &logs)
					}
					ba, ok := accum[id]
					if !ok {
						ba = &boardAccum{name: name}
						accum[id] = ba
						order = append(order, id)
					}
					ba.entries = append(ba.entries, logs...)
					if len(logs) > maxRows {
						maxRows = len(logs)
					}
				}
				if maxRows < limit {
					break
				}
				if page >= maxSincePages {
					truncated = true
					break
				}
			}
			if truncated {
				fmt.Fprintf(os.Stderr, "warning: stopped after %d pages (limit %d); results may be incomplete. Narrow --board or the window for full coverage.\n", maxSincePages, limit)
			}
			type entry struct {
				BoardID   string          `json:"board_id"`
				BoardName string          `json:"board_name"`
				Activity  json.RawMessage `json:"activity"`
			}
			merged := make([]entry, 0, len(order))
			for _, id := range order {
				ba := accum[id]
				activity := json.RawMessage("[]")
				if len(ba.entries) > 0 {
					if encoded, err := json.Marshal(ba.entries); err == nil {
						activity = encoded
					}
				}
				merged = append(merged, entry{BoardID: id, BoardName: ba.name, Activity: activity})
			}
			return printJSONFiltered(cmd.OutOrStdout(), merged, flags)
		},
	}
	cmd.Flags().StringVar(&boardsCSV, "board", "", "Comma-separated board IDs to scan (required).")
	cmd.Flags().StringVar(&userIDs, "user-ids", "", "Comma-separated user IDs to filter activity by.")
	cmd.Flags().StringVar(&fromISO, "from", "", "Override window with an explicit ISO 8601 lower bound.")
	cmd.Flags().IntVar(&limit, "limit", 100, "Per-board activity log page size.")
	_ = cmd.MarkFlagRequired("board")
	return cmd
}

// parseSinceWindow accepts "2h", "1d", "30m", "1w" or an ISO 8601 timestamp.
func parseSinceWindow(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("expected window like 2h, 1d, 30m, 1w or ISO timestamp")
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return time.Time{}, fmt.Errorf("bad duration %q: %w", s, err)
	}
	now := time.Now().UTC()
	switch unit {
	case 'm':
		return now.Add(-time.Duration(n) * time.Minute), nil
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'd':
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	case 'w':
		return now.Add(-time.Duration(n) * 7 * 24 * time.Hour), nil
	}
	return time.Time{}, fmt.Errorf("unknown unit %q in %q", unit, s)
}

// ──────────────────────────────────────────────────────────────────────────
// complexity-budget: pre-flight cost probe
// ──────────────────────────────────────────────────────────────────────────

func newComplexityBudgetCmd(flags *rootFlags) *cobra.Command {
	var queryFile, varsJSON string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:     "complexity-budget",
		Short:   "Predict the GraphQL complexity-points cost of a query and the remaining account-minute budget.",
		Long:    "Wraps the supplied query in a complexity { before after reset_in_x_seconds } selection so monday.com returns the budget without you needing to execute and debug a 429.",
		Example: "  monday-pp-cli complexity-budget --query-file ./bulk-items.graphql --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var inner string
			switch {
			case queryFile != "":
				b, err := os.ReadFile(queryFile)
				if err != nil {
					return fmt.Errorf("reading %s: %w", queryFile, err)
				}
				inner = string(b)
			case fromStdin:
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				inner = string(b)
			case len(args) > 0:
				inner = args[0]
			default:
				inner = `query { me { id } }`
			}
			wrapped := wrapWithComplexity(inner)
			vars := map[string]any{}
			if varsJSON != "" {
				if err := json.Unmarshal([]byte(varsJSON), &vars); err != nil {
					return usageErr(fmt.Errorf("parsing --vars: %w", err))
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(wrapped, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "complexity")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&queryFile, "query-file", "", "Path to a file containing a GraphQL query to probe.")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read the query from stdin.")
	cmd.Flags().StringVar(&varsJSON, "vars", "", "Variables as a JSON object.")
	return cmd
}

// wrapWithComplexity inserts a `complexity { before after query reset_in_x_seconds }`
// selection into the supplied query. Looks for the first `{` after a `query`
// or `mutation` keyword and inserts the complexity field at the head of the
// top-level selection set. Falls back to a passthrough probe-only wrapper.
func wrapWithComplexity(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return `query { complexity { before after query reset_in_x_seconds } }`
	}
	open := strings.Index(q, "{")
	if open < 0 {
		return `query { complexity { before after query reset_in_x_seconds } }`
	}
	return q[:open+1] + " complexity { before after query reset_in_x_seconds } " + q[open+1:]
}

// ──────────────────────────────────────────────────────────────────────────
// cross-ref: export items joined to a cross-system ID column as JSON
// ──────────────────────────────────────────────────────────────────────────

const queryCrossRef = `query($board_id: ID!, $limit: Int!, $cursor: String) {
  boards(ids: [$board_id]) {
    id
    name
    items_page(limit: $limit, cursor: $cursor) {
      cursor
      items {
        id
        name
        state
        updated_at
        group { id title }
        column_values {
          id
          text
          value
          column { id title type }
        }
      }
    }
  }
}`

func newCrossRefCmd(flags *rootFlags) *cobra.Command {
	var boardID, linkColumn, statusColumn, ownerColumn string
	var limit int
	var includeNull bool
	cmd := &cobra.Command{
		Use:     "cross-ref",
		Short:   "Export items joined to their cross-system ID column (Linear id, Notion page-id, Slack thread-ts) as JSON ready to pipe.",
		Long:    "Walks every item on the board, finds the column whose id or title matches --link-column, and emits one JSON row per item with: monday_id, item_name, linker (the cross-system id), status, owner, group, board, updated_at. Pipe straight into linear-pp-cli, notion-pp-cli, etc.",
		Example: "  monday-pp-cli cross-ref --board 12345 --link-column linear_id --json | jq '.[] | select(.linker != null)'",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cursor := ""
			type row struct {
				MondayID  string          `json:"monday_id"`
				ItemName  string          `json:"item_name"`
				Linker    *string         `json:"linker"`
				Status    *string         `json:"status,omitempty"`
				Owner     *string         `json:"owner,omitempty"`
				Group     string          `json:"group,omitempty"`
				BoardID   string          `json:"board_id"`
				BoardName string          `json:"board_name"`
				UpdatedAt string          `json:"updated_at,omitempty"`
				Raw       json.RawMessage `json:"-"`
			}
			out := make([]row, 0, 256)
			var boardName string
			for {
				vars := map[string]any{"board_id": boardID, "limit": limit}
				if cursor != "" {
					vars["cursor"] = cursor
				}
				data, err := c.GraphQL(queryCrossRef, vars)
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
				_ = json.Unmarshal(bs[0]["name"], &boardName)
				page := bs[0]["items_page"]
				var pg struct {
					Cursor string            `json:"cursor"`
					Items  []json.RawMessage `json:"items"`
				}
				if err := json.Unmarshal(page, &pg); err != nil {
					return fmt.Errorf("parsing items_page: %w", err)
				}
				for _, it := range pg.Items {
					var item struct {
						ID        string `json:"id"`
						Name      string `json:"name"`
						UpdatedAt string `json:"updated_at"`
						Group     struct {
							Title string `json:"title"`
						} `json:"group"`
						ColumnValues []struct {
							ID     string `json:"id"`
							Text   string `json:"text"`
							Column struct {
								ID    string `json:"id"`
								Title string `json:"title"`
								Type  string `json:"type"`
							} `json:"column"`
						} `json:"column_values"`
					}
					if err := json.Unmarshal(it, &item); err != nil {
						continue
					}
					var linker, status, owner *string
					for _, cv := range item.ColumnValues {
						switch {
						case cv.Column.ID == linkColumn || cv.Column.Title == linkColumn:
							v := cv.Text
							linker = &v
						case statusColumn != "" && (cv.Column.ID == statusColumn || cv.Column.Title == statusColumn):
							v := cv.Text
							status = &v
						case ownerColumn != "" && (cv.Column.ID == ownerColumn || cv.Column.Title == ownerColumn):
							v := cv.Text
							owner = &v
						}
					}
					if !includeNull && (linker == nil || *linker == "") {
						continue
					}
					out = append(out, row{
						MondayID:  item.ID,
						ItemName:  item.Name,
						Linker:    linker,
						Status:    status,
						Owner:     owner,
						Group:     item.Group.Title,
						BoardID:   boardID,
						BoardName: boardName,
						UpdatedAt: item.UpdatedAt,
					})
				}
				if pg.Cursor == "" || len(pg.Items) == 0 {
					break
				}
				cursor = pg.Cursor
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&linkColumn, "link-column", "", "Column ID or title that holds the cross-system ID (required).")
	cmd.Flags().StringVar(&statusColumn, "status-column", "status", "Column ID or title to project as 'status' on each row.")
	cmd.Flags().StringVar(&ownerColumn, "owner-column", "person", "Column ID or title to project as 'owner' on each row.")
	cmd.Flags().IntVar(&limit, "limit", 100, "Per-page item count.")
	cmd.Flags().BoolVar(&includeNull, "include-null", false, "Include items where the link column is empty.")
	_ = cmd.MarkFlagRequired("board")
	_ = cmd.MarkFlagRequired("link-column")
	return cmd
}

// ──────────────────────────────────────────────────────────────────────────
// context: dump everything about one item
// ──────────────────────────────────────────────────────────────────────────

const queryContextItem = `query($ids: [ID!]) {
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
    subitems { id name state }
    column_values {
      id
      text
      value
      type
      column { id title type }
    }
    updates(limit: 25) {
      id
      body
      text_body
      created_at
      creator_id
      replies { id body text_body created_at creator_id }
    }
    assets {
      id
      name
      url
      file_extension
      file_size
    }
  }
}`

func newContextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "context <item-id>",
		Short:   "Dump everything about one item (column values, updates, replies, subitems, assets, board, group) as one JSON blob.",
		Long:    "Designed for piping to an LLM as task context. One single GraphQL call returns the item, every typed column value, recent updates and replies, subitems, and asset metadata.",
		Example: "  monday-pp-cli context 1234567890 --json",
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
			data, err := c.GraphQL(queryContextItem, map[string]any{"ids": []string{args[0]}})
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

// ──────────────────────────────────────────────────────────────────────────
// bottleneck: per-status dwell time computed from API activity logs
// ──────────────────────────────────────────────────────────────────────────

const queryBottleneckActivity = `query($id: [ID!], $limit: Int!, $page: Int!, $column_ids: [String!]) {
  boards(ids: $id) {
    activity_logs(limit: $limit, page: $page, column_ids: $column_ids) {
      id
      event
      data
      entity
      created_at
    }
  }
}`

func newBottleneckCmd(flags *rootFlags) *cobra.Command {
	var boardID, columnID string
	var maxPages int
	cmd := &cobra.Command{
		Use:     "bottleneck",
		Short:   "Per-status dwell-time analyzer: for each status value, median and p90 time spent before transitioning out.",
		Long:    "Reads board activity_logs for status-column transitions, reconstructs per-item dwell times, and reports median + p90 per status value. The status column defaults to 'status'; pass --column to target a specific column id.",
		Example: "  monday-pp-cli bottleneck --board 12345 --column status --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type evt struct {
				ItemID  string
				Created time.Time
				PrevVal string
				NewVal  string
			}
			events := make([]evt, 0, 1024)
			vars := map[string]any{
				"id":    []string{boardID},
				"limit": 5000,
				"page":  1,
			}
			if columnID != "" {
				vars["column_ids"] = []string{columnID}
			}
			for page := 1; page <= maxPages; page++ {
				vars["page"] = page
				data, err := c.GraphQL(queryBottleneckActivity, vars)
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
				logs, err := pluckFirstFromArrayObject(boards, "activity_logs")
				if err != nil {
					return err
				}
				var rows []map[string]json.RawMessage
				if err := json.Unmarshal(logs, &rows); err != nil {
					return fmt.Errorf("parsing activity_logs: %w", err)
				}
				if len(rows) == 0 {
					break
				}
				for _, r := range rows {
					var event string
					_ = json.Unmarshal(r["event"], &event)
					if event != "update_column_value" && event != "create_pulse" {
						continue
					}
					var dataStr string
					_ = json.Unmarshal(r["data"], &dataStr)
					var inner map[string]any
					_ = json.Unmarshal([]byte(dataStr), &inner)
					var createdISO string
					_ = json.Unmarshal(r["created_at"], &createdISO)
					t := parseMondayTime(createdISO)
					itemID := jsonGetString(inner, "pulse_id")
					prev, newv := jsonGetString(inner, "previous_value"), jsonGetString(inner, "value")
					events = append(events, evt{ItemID: itemID, Created: t, PrevVal: prev, NewVal: newv})
				}
				if len(rows) < 5000 {
					break
				}
			}
			sort.Slice(events, func(i, j int) bool { return events[i].Created.Before(events[j].Created) })
			// Compute dwell times per (item, status_value) — when a value was held.
			type key struct {
				Status string
			}
			holds := make(map[key][]time.Duration)
			lastEnter := make(map[string]struct {
				Status string
				At     time.Time
			})
			for _, e := range events {
				prevHold, ok := lastEnter[e.ItemID]
				if ok && prevHold.Status != "" {
					dwell := e.Created.Sub(prevHold.At)
					if dwell > 0 {
						holds[key{prevHold.Status}] = append(holds[key{prevHold.Status}], dwell)
					}
				}
				lastEnter[e.ItemID] = struct {
					Status string
					At     time.Time
				}{Status: shortStatusLabel(e.NewVal), At: e.Created}
			}
			type out struct {
				Status      string  `json:"status"`
				Count       int     `json:"transitions_observed"`
				MedianHours float64 `json:"median_hours"`
				P90Hours    float64 `json:"p90_hours"`
			}
			result := make([]out, 0, len(holds))
			for k, ds := range holds {
				sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
				med := percentile(ds, 50)
				p90 := percentile(ds, 90)
				result = append(result, out{
					Status:      k.Status,
					Count:       len(ds),
					MedianHours: round2(med.Hours()),
					P90Hours:    round2(p90.Hours()),
				})
			}
			sort.Slice(result, func(i, j int) bool { return result[i].P90Hours > result[j].P90Hours })
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&columnID, "column", "status", "Status-column ID to compute dwell time on.")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum activity-log pages to scan (5000 events each).")
	_ = cmd.MarkFlagRequired("board")
	return cmd
}

func percentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	idx := (p * (len(d) - 1)) / 100
	return d[idx]
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

func parseMondayTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05 UTC", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func jsonGetString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		if f, ok := v.(float64); ok {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	return ""
}

func shortStatusLabel(s string) string {
	if s == "" {
		return ""
	}
	var inner map[string]any
	if err := json.Unmarshal([]byte(s), &inner); err == nil {
		if v, ok := inner["label"]; ok {
			if str, ok := v.(string); ok && str != "" {
				return str
			}
		}
		if v, ok := inner["text"]; ok {
			if str, ok := v.(string); ok && str != "" {
				return str
			}
		}
	}
	return s
}

// ──────────────────────────────────────────────────────────────────────────
// column-drift: diff cached vs live boards.columns
// ──────────────────────────────────────────────────────────────────────────

const queryColumnDrift = `query($ids: [ID!]) {
  boards(ids: $ids) {
    id
    columns { id title type description }
  }
}`

func newColumnDriftCmd(flags *rootFlags) *cobra.Command {
	var boardID, snapshotPath string
	cmd := &cobra.Command{
		Use:     "column-drift",
		Short:   "Detect added, removed, renamed, or retyped columns since the last snapshot.",
		Long:    "Compares the current board column schema against a snapshot file. If the snapshot does not exist, the current schema is written to it and 'no drift' is reported. Each subsequent run computes the diff and updates the snapshot.",
		Example: "  monday-pp-cli column-drift --board 12345 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if snapshotPath == "" {
				home, _ := os.UserHomeDir()
				snapshotPath = filepath.Join(home, ".cache", "monday-pp-cli", "column-snapshots", "board-"+boardID+".json")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryColumnDrift, map[string]any{"ids": []string{boardID}})
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
			currCols, err := pluckFirstFromArrayObject(boards, "columns")
			if err != nil {
				return err
			}
			type col struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}
			var current []col
			if err := json.Unmarshal(currCols, &current); err != nil {
				return fmt.Errorf("parsing current columns: %w", err)
			}
			currMap := make(map[string]col, len(current))
			for _, c := range current {
				currMap[c.ID] = c
			}
			var prev []col
			if b, err := os.ReadFile(snapshotPath); err == nil {
				_ = json.Unmarshal(b, &prev)
			}
			prevMap := make(map[string]col, len(prev))
			for _, c := range prev {
				prevMap[c.ID] = c
			}
			type diff struct {
				Change    string `json:"change"`
				ID        string `json:"column_id"`
				Title     string `json:"title,omitempty"`
				Type      string `json:"type,omitempty"`
				PrevTitle string `json:"prev_title,omitempty"`
				PrevType  string `json:"prev_type,omitempty"`
			}
			diffs := make([]diff, 0)
			counts := map[string]int{"added": 0, "removed": 0, "renamed": 0, "retyped": 0}
			for id, p := range prevMap {
				if c, ok := currMap[id]; !ok {
					diffs = append(diffs, diff{Change: "removed", ID: id, PrevTitle: p.Title, PrevType: p.Type})
					counts["removed"]++
				} else {
					if p.Title != c.Title {
						diffs = append(diffs, diff{Change: "renamed", ID: id, Title: c.Title, PrevTitle: p.Title})
						counts["renamed"]++
					}
					if p.Type != c.Type {
						diffs = append(diffs, diff{Change: "retyped", ID: id, Type: c.Type, PrevType: p.Type})
						counts["retyped"]++
					}
				}
			}
			for id, c := range currMap {
				if _, ok := prevMap[id]; !ok {
					diffs = append(diffs, diff{Change: "added", ID: id, Title: c.Title, Type: c.Type})
					counts["added"]++
				}
			}
			// Persist snapshot
			_ = os.MkdirAll(filepath.Dir(snapshotPath), 0o755)
			if b, err := json.MarshalIndent(current, "", "  "); err == nil {
				_ = os.WriteFile(snapshotPath, b, 0o600)
			}
			result := map[string]any{
				"board_id": boardID,
				"snapshot": snapshotPath,
				"changes":  diffs,
				"summary":  counts,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&boardID, "board", "", "Board ID (required).")
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "", "Override snapshot file path.")
	_ = cmd.MarkFlagRequired("board")
	return cmd
}
