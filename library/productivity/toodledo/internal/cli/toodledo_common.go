// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for the Toodledo novel commands: GTD enum
// decoders, local-mirror access, name<->id resolution, and a typed task-row
// query against the local SQLite store. Kept in its own file (no generated
// header) so it survives regeneration as a whole hand-authored unit.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/toodledo/internal/store"
	"github.com/spf13/cobra"
)

// stripTaskMetaElement removes Toodledo's leading {num,total} metadata element
// from a tasks/get.php (or notes/get.php) array response so callers see only
// real objects. The sync path drops it automatically via id-extraction; live
// reads need this explicit strip.
func stripTaskMetaElement(data json.RawMessage) json.RawMessage {
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) != nil || len(arr) == 0 {
		return data
	}
	var first map[string]json.RawMessage
	if json.Unmarshal(arr[0], &first) == nil {
		_, hasNum := first["num"]
		_, hasTotal := first["total"]
		_, hasID := first["id"]
		if hasNum && hasTotal && !hasID {
			if out, err := json.Marshal(arr[1:]); err == nil {
				return out
			}
		}
	}
	return data
}

// --- Toodledo enum decoders (source: Toodledo v3 API + toodledo-mcp types) ---

var taskStatusLabels = map[int]string{
	0: "None", 1: "Next Action", 2: "Active", 3: "Planning", 4: "Delegated",
	5: "Waiting", 6: "Hold", 7: "Postponed", 8: "Someday", 9: "Canceled", 10: "Reference",
}

var taskPriorityLabels = map[int]string{
	-1: "Negative", 0: "Low", 1: "Medium", 2: "High", 3: "Top",
}

var goalLevelLabels = map[int]string{0: "Lifetime", 1: "Long-term", 2: "Short-term"}

// GTD status numeric constants used by the novel commands.
const (
	statusNextAction = 1
	statusWaiting    = 5
	statusSomeday    = 8
)

func normalizeEnumKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func reverseEnum(labels map[int]string) map[string]int {
	m := make(map[string]int, len(labels))
	for k, v := range labels {
		m[normalizeEnumKey(v)] = k
	}
	return m
}

var taskStatusByName = reverseEnum(taskStatusLabels)
var taskPriorityByName = reverseEnum(taskPriorityLabels)
var goalLevelByName = reverseEnum(goalLevelLabels)

func statusLabel(n int) string {
	if l, ok := taskStatusLabels[n]; ok {
		return l
	}
	return fmt.Sprintf("status:%d", n)
}

func priorityLabel(n int) string {
	if l, ok := taskPriorityLabels[n]; ok {
		return l
	}
	return fmt.Sprintf("priority:%d", n)
}

func goalLevelLabel(n int) string {
	if l, ok := goalLevelLabels[n]; ok {
		return l
	}
	return fmt.Sprintf("level:%d", n)
}

// goalLevelAliases maps short user-facing spellings to the numeric goal level,
// in addition to the canonical labels handled by goalLevelByName.
var goalLevelAliases = map[string]int{
	"lifetime": 0, "life": 0,
	"long": 1, "longterm": 1,
	"short": 2, "shortterm": 2,
}

// parseGoalLevel accepts a level name ("lifetime"/"long"/"short", the full
// labels, hyphen/underscore variants) or a number (0/1/2). Returns (value, true)
// when recognized.
func parseGoalLevel(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		_, ok := goalLevelLabels[n]
		return n, ok
	}
	key := normalizeEnumKey(s)
	if v, ok := goalLevelByName[key]; ok {
		return v, true
	}
	if v, ok := goalLevelAliases[strings.ReplaceAll(key, "_", "")]; ok {
		return v, true
	}
	return 0, false
}

// parseStatus accepts a status name ("next_action") or a number. Returns
// (value, true) when recognized.
func parseStatus(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		_, ok := taskStatusLabels[n]
		return n, ok
	}
	v, ok := taskStatusByName[normalizeEnumKey(s)]
	return v, ok
}

func parsePriority(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		_, ok := taskPriorityLabels[n]
		return n, ok
	}
	v, ok := taskPriorityByName[normalizeEnumKey(s)]
	return v, ok
}

// syncResourceDefaultParams returns per-resource query params that the syncer
// must send on every request beyond pagination/since. Toodledo's tasks endpoint
// returns only id/title/modified/completed unless `fields` requests the optional
// columns the local mirror and GTD commands depend on (folder, context, status,
// priority, duedate, star, …). Called from the generated sync loop.
func syncResourceDefaultParams(resource string) map[string]string {
	switch resource {
	case "tasks":
		return map[string]string{
			"fields": "folder,context,goal,location,tag,startdate,duedate,duedatemod,starttime,duetime,remind,repeat,status,star,priority,length,added,note,parent,children,order",
		}
	}
	return nil
}

// --- Local-mirror access ---

// toodledoDBPath resolves the SQLite mirror path, honoring an explicit --db override.
func toodledoDBPath(dbFlag string) string {
	if strings.TrimSpace(dbFlag) != "" {
		return dbFlag
	}
	return defaultDBPath("toodledo-pp-cli")
}

// openLocalMirror opens the local mirror for read queries. Returns
// (store, true, nil) when present, or (nil, false, nil) when the mirror file
// does not exist yet so the caller can emit the standard sync hint.
func openLocalMirror(cmd *cobra.Command, dbPath string) (*store.Store, bool, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, false, nil
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening local mirror: %w", err)
	}
	return db, true, nil
}

// emitMissingMirror prints the standard "no local mirror" hint and an empty
// JSON result for machine consumers. A missing mirror is an empty-cache state,
// not a usage or API failure, so the caller returns nil after this.
func emitMissingMirror(cmd *cobra.Command, flags *rootFlags, dbPath string) {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: toodledo-pp-cli sync --db %s\n", dbPath, dbPath)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
}

// loadNameMap loads an id->name map from a typed reference table
// (folders/contexts/goals). The typed tables store id as TEXT; values are
// parsed back to int to match the integer foreign keys on tasks.
func loadNameMap(db *store.Store, table string) map[int]string {
	m := map[int]string{}
	rows, err := db.DB().Query(fmt.Sprintf(`SELECT "id", "name" FROM "%s"`, table))
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var idStr, name sql.NullString
		if err := rows.Scan(&idStr, &name); err != nil {
			continue
		}
		if id, err := strconv.Atoi(strings.TrimSpace(idStr.String)); err == nil {
			m[id] = name.String
		}
	}
	return m
}

// resolveRefID resolves a folder/context/goal NAME to its integer id using the
// local mirror (case-insensitive, exact then substring). Returns
// (id, true, nil) on a hit, (0, false, nil) when not found. A purely numeric
// input is accepted as an id directly.
func resolveRefID(db *store.Store, table, name string) (int, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, nil
	}
	if n, err := strconv.Atoi(name); err == nil {
		return n, true, nil
	}
	names := loadNameMap(db, table)
	lower := strings.ToLower(name)
	// exact
	for id, nm := range names {
		if strings.ToLower(nm) == lower {
			return id, true, nil
		}
	}
	// substring (both directions) — pick deterministically: shortest name, then
	// lexically first, so a fuzzy match is stable across runs (map order is not).
	best, bestID, found := "", 0, false
	for id, nm := range names {
		ln := strings.ToLower(nm)
		if strings.Contains(ln, lower) || strings.Contains(lower, ln) {
			if !found || len(nm) < len(best) || (len(nm) == len(best) && nm < best) {
				best, bestID, found = nm, id, true
			}
		}
	}
	if found {
		return bestID, true, nil
	}
	return 0, false, nil
}

// availableNames returns the sorted display names in a reference table, for
// "did you mean / available:" error messages.
func availableNames(db *store.Store, table string) []string {
	names := loadNameMap(db, table)
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// --- Typed task rows ---

// taskRow is the projection the GTD commands return. Reference ids are decorated
// with their resolved names where a name map is supplied.
type taskRow struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	StatusLabel   string `json:"status_label"`
	Priority      int    `json:"priority"`
	PriorityLabel string `json:"priority_label"`
	Folder        int    `json:"folder"`
	FolderName    string `json:"folder_name,omitempty"`
	Context       int    `json:"context"`
	ContextName   string `json:"context_name,omitempty"`
	Goal          int    `json:"goal"`
	GoalName      string `json:"goal_name,omitempty"`
	Star          int    `json:"star"`
	Due           int64  `json:"duedate"`
	DueISO        string `json:"due,omitempty"`
	Completed     int64  `json:"completed"`
}

// scanTaskRows runs a SELECT against the typed tasks table and returns decorated
// rows. The query MUST select, in order:
//
//	id, title, status, priority, folder, context, goal, star, duedate, completed
//
// All numeric columns are scanned NULL-safe (the API omits unset optional
// fields, so the synced column can be NULL).
func scanTaskRows(db *store.Store, query string, args []any, folders, contexts, goals map[int]string) ([]taskRow, error) {
	rows, err := db.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying tasks: %w", err)
	}
	defer rows.Close()
	out := make([]taskRow, 0)
	for rows.Next() {
		var id, title sql.NullString
		var status, priority, folder, contextID, goal, star sql.NullInt64
		var due, completed sql.NullInt64
		if err := rows.Scan(&id, &title, &status, &priority, &folder, &contextID, &goal, &star, &due, &completed); err != nil {
			continue
		}
		r := taskRow{
			ID:        id.String,
			Title:     title.String,
			Status:    int(status.Int64),
			Priority:  int(priority.Int64),
			Folder:    int(folder.Int64),
			Context:   int(contextID.Int64),
			Goal:      int(goal.Int64),
			Star:      int(star.Int64),
			Due:       due.Int64,
			Completed: completed.Int64,
		}
		r.StatusLabel = statusLabel(r.Status)
		r.PriorityLabel = priorityLabel(r.Priority)
		if r.Folder != 0 && folders != nil {
			r.FolderName = folders[r.Folder]
		}
		if r.Context != 0 && contexts != nil {
			r.ContextName = contexts[r.Context]
		}
		if r.Goal != 0 && goals != nil {
			r.GoalName = goals[r.Goal]
		}
		if r.Due > 0 {
			r.DueISO = time.Unix(r.Due, 0).UTC().Format("2006-01-02")
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// taskSelectColumns is the canonical column list scanTaskRows expects.
const taskSelectColumns = `"id", "title", "status", "priority", "folder", "context", "goal", "star", "duedate", "completed"`

// renderTaskList writes a taskRow slice as JSON (machine) or a compact human table.
func renderTaskList(cmd *cobra.Command, flags *rootFlags, header string, rows []taskRow) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	w := cmd.OutOrStdout()
	if header != "" {
		fmt.Fprintf(w, "%s (%d)\n", header, len(rows))
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "  (none)")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("ID\tPRIORITY\tDUE\tCONTEXT\tTITLE"))
	for _, r := range rows {
		due := r.DueISO
		if due == "" {
			due = "-"
		}
		ctx := r.ContextName
		if ctx == "" && r.Context != 0 {
			ctx = strconv.Itoa(r.Context)
		}
		if ctx == "" {
			ctx = "-"
		}
		star := ""
		if r.Star == 1 {
			star = "★ "
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s%s\n", r.ID, priorityLabel(r.Priority), due, ctx, star, truncate(r.Title, 60))
	}
	return tw.Flush()
}

// startOfTodayUnix bounds "due today" against the UTC calendar day. Toodledo
// stores YYYY-MM-DD due dates at noon UTC (see parseDueDate), so the day window
// must also be UTC or noon-UTC due dates fall outside a local-time window for
// far-east/west timezones.
func startOfTodayUnix() int64 {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix()
}
