// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built support code for the Printing Press transcendence commands
// (timesheet, recap, audit, team, billable, project burn, backfill).

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/clockify/internal/store"
)

// timeEntry is the subset of a Clockify time entry the hand-built timesheet,
// recap, audit, and billable commands operate on.
type timeEntry struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Billable    bool          `json:"billable"`
	ProjectID   string        `json:"project_id"`
	TaskID      string        `json:"task_id"`
	UserID      string        `json:"user_id"`
	WorkspaceID string        `json:"workspace_id"`
	TagIDs      []string      `json:"tag_ids"`
	Start       time.Time     `json:"start"`
	End         time.Time     `json:"end"`
	Running     bool          `json:"running"`
	Locked      bool          `json:"locked"`
	Duration    time.Duration `json:"-"`
	Hours       float64       `json:"hours"`
}

var iso8601DurRE = regexp.MustCompile(`^P(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

// parseISO8601Duration parses a Clockify duration string (the PT8H30M form).
// It returns 0 for an empty string or any value it cannot parse.
func parseISO8601Duration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	m := iso8601DurRE.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	var d time.Duration
	if m[1] != "" {
		if h, err := strconv.Atoi(m[1]); err == nil {
			d += time.Duration(h) * time.Hour
		}
	}
	if m[2] != "" {
		if mn, err := strconv.Atoi(m[2]); err == nil {
			d += time.Duration(mn) * time.Minute
		}
	}
	if m[3] != "" {
		if sec, err := strconv.ParseFloat(m[3], 64); err == nil {
			d += time.Duration(sec * float64(time.Second))
		}
	}
	return d
}

// rawEntry mirrors the wire shape of a Clockify time entry.
type rawEntry struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	Billable     bool     `json:"billable"`
	ProjectID    string   `json:"projectId"`
	TaskID       string   `json:"taskId"`
	UserID       string   `json:"userId"`
	WorkspaceID  string   `json:"workspaceId"`
	TagIDs       []string `json:"tagIds"`
	IsLocked     bool     `json:"isLocked"`
	TimeInterval struct {
		Start    string `json:"start"`
		End      string `json:"end"`
		Duration string `json:"duration"`
	} `json:"timeInterval"`
}

// parseEntry decodes one synced time-entry row into a timeEntry. The bool is
// false for rows that are not valid entries (missing id).
func parseEntry(raw json.RawMessage) (timeEntry, bool) {
	var r rawEntry
	if err := json.Unmarshal(raw, &r); err != nil || r.ID == "" {
		return timeEntry{}, false
	}
	te := timeEntry{
		ID: r.ID, Description: r.Description, Billable: r.Billable,
		ProjectID: r.ProjectID, TaskID: r.TaskID, UserID: r.UserID,
		WorkspaceID: r.WorkspaceID, TagIDs: r.TagIDs, Locked: r.IsLocked,
	}
	if r.TimeInterval.Start != "" {
		if t, err := time.Parse(time.RFC3339, r.TimeInterval.Start); err == nil {
			te.Start = t.Local()
		}
	}
	if r.TimeInterval.End != "" {
		if t, err := time.Parse(time.RFC3339, r.TimeInterval.End); err == nil {
			te.End = t.Local()
		}
	}
	switch {
	case !te.Start.IsZero() && !te.End.IsZero():
		te.Duration = te.End.Sub(te.Start)
	case r.TimeInterval.Duration != "":
		te.Duration = parseISO8601Duration(r.TimeInterval.Duration)
	}
	te.Running = !te.Start.IsZero() && te.End.IsZero()
	te.Hours = te.Duration.Hours()
	return te, true
}

// loadRaw reads JSON object rows for an entity, trying the deterministic
// typed tables first and falling back to the generic resources table keyed
// by any of the given resource_type guesses. Both the typed-table `data`
// column and the resources `data` column are NOT NULL, so a bare string
// scan is safe.
func loadRaw(db *store.Store, typedTables, resourceTypeGuesses []string) ([]json.RawMessage, error) {
	var out []json.RawMessage
	for _, t := range typedTables {
		if !identRE.MatchString(t) {
			continue
		}
		rows, err := db.Query(`SELECT data FROM "` + t + `"`)
		if err != nil {
			continue // table absent in this CLI's schema
		}
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				continue
			}
			out = append(out, json.RawMessage(data))
		}
		rowsErr := rows.Err()
		rows.Close()
		if rowsErr != nil {
			return nil, fmt.Errorf("scanning %q: %w", t, rowsErr)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	for _, rt := range resourceTypeGuesses {
		rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = ?`, rt)
		if err != nil {
			continue
		}
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				continue
			}
			out = append(out, json.RawMessage(data))
		}
		rowsErr := rows.Err()
		rows.Close()
		if rowsErr != nil {
			return nil, fmt.Errorf("scanning resources %q: %w", rt, rowsErr)
		}
	}
	return out, nil
}

var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// loadTimeEntries reads every synced time entry from the local store.
func loadTimeEntries(db *store.Store) ([]timeEntry, error) {
	raws, err := loadRaw(db,
		[]string{"user_time_entries", "time_entries"},
		[]string{"time-entries", "user-time-entries", "time_entries", "user_time_entries"})
	if err != nil {
		return nil, err
	}
	entries := make([]timeEntry, 0, len(raws))
	seen := make(map[string]bool, len(raws))
	for _, r := range raws {
		te, ok := parseEntry(r)
		if !ok || seen[te.ID] {
			continue
		}
		seen[te.ID] = true
		entries = append(entries, te)
	}
	return entries, nil
}

// nameIndex maps an entity id to a display name from synced rows. It reads
// "name" with a fallback to "displayName".
func nameIndex(db *store.Store, typedTables, resourceTypeGuesses []string) map[string]json.RawMessage {
	raws, _ := loadRaw(db, typedTables, resourceTypeGuesses)
	idx := make(map[string]json.RawMessage, len(raws))
	for _, r := range raws {
		var obj struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(r, &obj) == nil && obj.ID != "" {
			idx[obj.ID] = r
		}
	}
	return idx
}

// projectInfo is the project fields the novel commands need.
type projectInfo struct {
	Name     string `json:"name"`
	ClientID string `json:"clientId"`
	Billable bool   `json:"billable"`
	Archived bool   `json:"archived"`
	Estimate struct {
		Estimate string `json:"estimate"`
		Type     string `json:"type"`
	} `json:"timeEstimate"`
}

// loadProjects returns a projectId -> projectInfo index from the local store.
func loadProjects(db *store.Store) map[string]projectInfo {
	idx := nameIndex(db, []string{"projects"}, []string{"projects", "project"})
	out := make(map[string]projectInfo, len(idx))
	for id, raw := range idx {
		var p projectInfo
		if json.Unmarshal(raw, &p) == nil {
			out[id] = p
		}
	}
	return out
}

// loadClientNames returns a clientId -> client name index.
func loadClientNames(db *store.Store) map[string]string {
	idx := nameIndex(db, []string{"clients"}, []string{"clients", "client"})
	out := make(map[string]string, len(idx))
	for id, raw := range idx {
		var c struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &c) == nil {
			out[id] = c.Name
		}
	}
	return out
}

// weekStart returns midnight of the Monday on or before ref, in ref's location.
func weekStart(ref time.Time) time.Time {
	y, m, d := ref.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, ref.Location())
	offset := (int(day.Weekday()) + 6) % 7 // Monday = 0
	return day.AddDate(0, 0, -offset)
}

// dayStart returns midnight of ref in its location.
func dayStart(ref time.Time) time.Time {
	y, m, d := ref.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, ref.Location())
}

var nDaysRE = regexp.MustCompile(`^(\d+)d$`)

// resolveRange turns a range spec into a [start,end) window plus a label.
// Accepts: today, yesterday, this-week, last-week, this-month, last-month,
// Nd (e.g. 7d, 30d), all, or "" (defaults to this-month).
func resolveRange(spec string, now time.Time) (start, end time.Time, label string, err error) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if spec == "" {
		spec = "this-month"
	}
	today := dayStart(now)
	switch spec {
	case "today":
		return today, today.AddDate(0, 0, 1), "today", nil
	case "yesterday":
		return today.AddDate(0, 0, -1), today, "yesterday", nil
	case "this-week", "week":
		ws := weekStart(now)
		return ws, ws.AddDate(0, 0, 7), "this week", nil
	case "last-week":
		ws := weekStart(now).AddDate(0, 0, -7)
		return ws, ws.AddDate(0, 0, 7), "last week", nil
	case "this-month", "month":
		ms := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return ms, ms.AddDate(0, 1, 0), "this month", nil
	case "last-month":
		ms := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0)
		return ms, ms.AddDate(0, 1, 0), "last month", nil
	case "all":
		return time.Time{}, now.AddDate(100, 0, 0), "all time", nil
	}
	if m := nDaysRE.FindStringSubmatch(spec); m != nil {
		n, _ := strconv.Atoi(m[1])
		return today.AddDate(0, 0, -n+1), today.AddDate(0, 0, 1),
			fmt.Sprintf("last %d days", n), nil
	}
	return time.Time{}, time.Time{}, "",
		fmt.Errorf("unknown --range %q (use today, this-week, last-week, this-month, last-month, all, or Nd like 7d)", spec)
}

// parseDateFlag parses a YYYY-MM-DD date flag, defaulting to now when empty.
func parseDateFlag(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return now, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --date %q (want YYYY-MM-DD)", s)
	}
	return t, nil
}

// rawUser is the user fields resolveWorkspaceUser extracts.
type rawUser struct {
	ID               string `json:"id"`
	ActiveWorkspace  string `json:"activeWorkspace"`
	DefaultWorkspace string `json:"defaultWorkspace"`
	Email            string `json:"email"`
	Name             string `json:"name"`
}

// resolveWorkspaceUser calls GET /v1/user and returns the caller's workspace
// id and user id. The override is used in place of the active workspace when
// non-empty (the --workspace flag).
func resolveWorkspaceUser(c *client.Client, override string) (workspaceID, userID string, err error) {
	data, err := c.Get("/v1/user", nil)
	if err != nil {
		return "", "", fmt.Errorf("resolving current user: %w", err)
	}
	var u rawUser
	if err := json.Unmarshal(data, &u); err != nil {
		return "", "", fmt.Errorf("parsing current user: %w", err)
	}
	ws := override
	if ws == "" {
		ws = u.ActiveWorkspace
	}
	if ws == "" {
		ws = u.DefaultWorkspace
	}
	if ws == "" || u.ID == "" {
		return "", "", fmt.Errorf("could not determine workspace/user from /v1/user; pass --workspace")
	}
	return ws, u.ID, nil
}

// emptyStoreHint is the standard message when a novel command finds no
// synced data to work with.
const emptyStoreHint = "no synced data found — run 'clockify-pp-cli sync' first"

// inWindow reports whether an entry starts within [start,end). A zero start
// means -infinity; a zero end means +infinity.
func inWindow(e timeEntry, start, end time.Time) bool {
	if e.Start.IsZero() {
		return false
	}
	if !start.IsZero() && e.Start.Before(start) {
		return false
	}
	if !end.IsZero() && !e.Start.Before(end) {
		return false
	}
	return true
}

// mergeEntries concatenates two entry slices, de-duplicating by id.
func mergeEntries(a, b []timeEntry) []timeEntry {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]timeEntry, 0, len(a)+len(b))
	for _, src := range [][]timeEntry{a, b} {
		for _, e := range src {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			out = append(out, e)
		}
	}
	return out
}

// fetchTimeEntriesLive pulls the authenticated user's time entries for the
// window from the Clockify API and caches each one into the local store under
// resource_type "time-entries" so later commands read it offline. This is the
// path that makes the offline timesheet/recap/audit features work: the
// generator's bulk sync cannot fetch time entries (the nested /user/{userId}
// endpoint needs a runtime-resolved user id), so the novel commands hydrate
// the store themselves on first use.
func fetchTimeEntriesLive(db *store.Store, c *client.Client, workspace string, start, end time.Time) ([]timeEntry, error) {
	wsID, userID, err := resolveWorkspaceUser(c, workspace)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/v1/workspaces/%s/user/%s/time-entries", wsID, userID)
	var entries []timeEntry
	seen := map[string]bool{}
	for page := 1; page <= 50; page++ {
		params := map[string]string{
			"page":      strconv.Itoa(page),
			"page-size": "200",
		}
		if !start.IsZero() {
			params["start"] = start.UTC().Format("2006-01-02T15:04:05Z")
		}
		if !end.IsZero() && end.Year() < 3000 {
			params["end"] = end.UTC().Format("2006-01-02T15:04:05Z")
		}
		data, err := c.Get(path, params)
		if err != nil {
			return nil, fmt.Errorf("fetching time entries: %w", err)
		}
		var raws []json.RawMessage
		if err := json.Unmarshal(data, &raws); err != nil {
			return nil, fmt.Errorf("parsing time entries: %w", err)
		}
		for _, r := range raws {
			te, ok := parseEntry(r)
			if !ok || seen[te.ID] {
				continue
			}
			seen[te.ID] = true
			entries = append(entries, te)
			_ = db.Upsert("time-entries", te.ID, r)
		}
		if len(raws) < 200 {
			break
		}
	}
	return entries, nil
}

// ensureTimeEntries returns time entries for the [winStart,winEnd) window,
// honoring --data-source: local reads the store only; live always fetches;
// auto (default) reads the store, fetching the window live and caching it
// when the store has nothing in that window. Under --dry-run it never makes a
// live call (verify probes commands with --dry-run), so it returns whatever
// the local store already holds.
func ensureTimeEntries(db *store.Store, flags *rootFlags, winStart, winEnd time.Time, workspace string) ([]timeEntry, error) {
	stored, err := loadTimeEntries(db)
	if err != nil {
		return nil, err
	}
	if flags.dataSource == "local" || flags.dryRun {
		return stored, nil
	}
	if flags.dataSource != "live" {
		for _, e := range stored {
			if inWindow(e, winStart, winEnd) {
				return stored, nil // window already cached
			}
		}
	}
	c, err := flags.newClient()
	if err != nil {
		if len(stored) > 0 {
			return stored, nil
		}
		return nil, err
	}
	fetched, err := fetchTimeEntriesLive(db, c, workspace, winStart, winEnd)
	if err != nil {
		if len(stored) > 0 {
			return stored, nil // degrade to whatever is cached
		}
		return nil, err
	}
	return mergeEntries(stored, fetched), nil
}

// ftsSanitizeQuery makes a user search string safe for SQLite FTS5 MATCH.
// FTS5 treats characters like '-', ':', '*', and '[' as operators or column
// syntax, so an unquoted term such as "pp-test" raises a SQL error ("no such
// column: test"). Wrapping each whitespace-separated token as an FTS5 string
// literal keeps multi-word AND semantics while neutralizing the special
// characters. Embedded double quotes are doubled per FTS5 escaping.
func ftsSanitizeQuery(q string) string {
	fields := strings.Fields(q)
	if len(fields) == 0 {
		return q
	}
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " ")
}
