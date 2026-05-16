// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

// Novel feature commands for zoho-projects-pp-cli. These are LOCAL commands
// that query the synced SQLite store; they do not hit the API.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/zoho-projects/internal/store"
)

// typedExit signals cobra to exit with a non-zero code while preserving the
// error semantic.
type typedExit struct {
	code int
	msg  string
}

func (e *typedExit) Error() string { return e.msg }
func (e *typedExit) ExitCode() int { return e.code }

func stringContainsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// dbInit returns the store + path, or an error with a hint to sync.
func openLocalDB(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("zoho-projects-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'zoho-projects-pp-cli sync' first.", err)
	}
	return db, nil
}

// ---------- overdue ----------

type overdueRow struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"` // task | issue
	Name        string  `json:"name"`
	Project     string  `json:"project,omitempty"`
	Owner       string  `json:"owner,omitempty"`
	EndDate     string  `json:"end_date"`
	DaysOverdue float64 `json:"days_overdue"`
}

func newOverdueCmd(flags *rootFlags) *cobra.Command {
	var dbPath, project string
	var includeIssues bool
	cmd := &cobra.Command{
		Use:   "overdue",
		Short: "Tasks (and optionally issues) past their end_date but not closed",
		Long: `Joins locally-synced tasks + issues with their projects to compute days-overdue
counts. LOCAL command: requires 'sync' first.`,
		Example: `  zoho-projects-pp-cli overdue --json --select id,name,project,owner,days_overdue
  zoho-projects-pp-cli overdue --project "Q3 Launch" --include-issues --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []overdueRow{})
			}
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeOverdue(cmd, db, project, includeIssues)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project name")
	cmd.Flags().BoolVar(&includeIssues, "include-issues", true, "Also include overdue issues")
	return cmd
}

func computeOverdue(cmd *cobra.Command, db *store.Store, projectFilter string, includeIssues bool) ([]overdueRow, error) {
	today := time.Now().UTC().Format("2006-01-02")
	out := []overdueRow{}
	// Tasks
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_tasks
		WHERE IFNULL(json_extract(data, '$.end_date'), '') != ''
		  AND IFNULL(json_extract(data, '$.end_date'), '') < ?
		  AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')`, today)
	if err == nil {
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			r := overdueRow{Kind: "task"}
			r.ID, _ = obj["id"].(string)
			r.Name, _ = obj["name"].(string)
			r.EndDate, _ = obj["end_date"].(string)
			if owners, ok := obj["owners"].([]any); ok && len(owners) > 0 {
				if o, ok := owners[0].(map[string]any); ok {
					r.Owner, _ = o["name"].(string)
				}
			}
			if proj, ok := obj["project"].(map[string]any); ok {
				r.Project, _ = proj["name"].(string)
			}
			if projectFilter != "" && !stringContainsFold(r.Project, projectFilter) {
				continue
			}
			if t, err := time.Parse("2006-01-02", r.EndDate); err == nil {
				r.DaysOverdue = math.Round(time.Since(t).Hours()/24*10) / 10
			}
			out = append(out, r)
		}
		rows.Close()
	}
	// Issues
	if includeIssues {
		irows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_issues
			WHERE IFNULL(json_extract(data, '$.due_date'), '') != ''
			  AND IFNULL(json_extract(data, '$.due_date'), '') < ?
			  AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')`, today)
		if err == nil {
			for irows.Next() {
				var raw []byte
				if err := irows.Scan(&raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				r := overdueRow{Kind: "issue"}
				r.ID, _ = obj["id"].(string)
				r.Name, _ = obj["title"].(string)
				r.EndDate, _ = obj["due_date"].(string)
				if proj, ok := obj["project"].(map[string]any); ok {
					r.Project, _ = proj["name"].(string)
				}
				if projectFilter != "" && !stringContainsFold(r.Project, projectFilter) {
					continue
				}
				if t, err := time.Parse("2006-01-02", r.EndDate); err == nil {
					r.DaysOverdue = math.Round(time.Since(t).Hours()/24*10) / 10
				}
				out = append(out, r)
			}
			irows.Close()
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DaysOverdue > out[j].DaysOverdue })
	return out, nil
}

// ---------- stale-projects ----------

type staleProjectRow struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Owner        string  `json:"owner,omitempty"`
	DaysInactive float64 `json:"days_inactive"`
	LastActivity string  `json:"last_activity,omitempty"`
	TaskCount    int     `json:"task_count"`
}

func newStaleProjectsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	cmd := &cobra.Command{
		Use:         "stale-projects",
		Short:       "Active projects with no task activity in N days",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []staleProjectRow{})
			}
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeStaleProjects(cmd, db, days)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 14, "Inactivity threshold")
	return cmd
}

func computeStaleProjects(cmd *cobra.Command, db *store.Store, days int) ([]staleProjectRow, error) {
	threshold := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data,
		(SELECT MAX(IFNULL(json_extract(t.data, '$.last_modified_time'), '')) FROM projects_tasks t WHERE json_extract(t.data, '$.project.id') = projects.id) AS last_activity,
		(SELECT COUNT(*) FROM projects_tasks t WHERE json_extract(t.data, '$.project.id') = projects.id) AS task_count
		FROM projects
		WHERE LOWER(IFNULL(json_extract(data, '$.status'), 'active')) = 'active'`)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []staleProjectRow
	for rows.Next() {
		var raw []byte
		var lastAct *string
		var taskCount int
		if err := rows.Scan(&raw, &lastAct, &taskCount); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		r := staleProjectRow{TaskCount: taskCount}
		r.ID, _ = obj["id"].(string)
		r.Name, _ = obj["name"].(string)
		r.Owner, _ = obj["owner_name"].(string)
		if lastAct != nil && *lastAct != "" {
			r.LastActivity = *lastAct
			if t, err := time.Parse(time.RFC3339, *lastAct); err == nil {
				r.DaysInactive = math.Round(time.Since(t).Hours()/24*10) / 10
			}
		} else {
			r.DaysInactive = -1 // never had activity
		}
		if r.LastActivity == "" || r.LastActivity < threshold {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DaysInactive > out[j].DaysInactive })
	return out, nil
}

// ---------- workload ----------

type workloadRow struct {
	Owner      string  `json:"owner"`
	OwnerID    string  `json:"owner_id,omitempty"`
	OpenTasks  int     `json:"open_tasks"`
	OpenIssues int     `json:"open_issues"`
	TotalOpen  int     `json:"total_open"`
	SharePct   float64 `json:"share_pct"`
}

type workloadStats struct {
	Mean   float64       `json:"mean"`
	Stddev float64       `json:"stddev"`
	Spread float64       `json:"spread"`
	Rows   []workloadRow `json:"rows"`
}

func newWorkloadCmd(flags *rootFlags) *cobra.Command {
	var dbPath, sortBy string
	cmd := &cobra.Command{
		Use:         "workload",
		Short:       "Open tasks + issues per owner with team-level dispersion",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, &workloadStats{Rows: []workloadRow{}})
			}
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			stats, err := computeWorkload(cmd, db, sortBy)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, stats)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&sortBy, "sort", "count", "Sort: count | spread | name")
	return cmd
}

func computeWorkload(cmd *cobra.Command, db *store.Store, sortBy string) (*workloadStats, error) {
	agg := map[string]*workloadRow{}
	// Tasks open
	taskRows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_tasks
		WHERE LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')`)
	if err == nil {
		for taskRows.Next() {
			var raw []byte
			if err := taskRows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			if owners, ok := obj["owners"].([]any); ok {
				for _, o := range owners {
					om, ok := o.(map[string]any)
					if !ok {
						continue
					}
					name, _ := om["name"].(string)
					id, _ := om["id"].(string)
					if name == "" && id == "" {
						continue
					}
					key := id
					if key == "" {
						key = name
					}
					r := agg[key]
					if r == nil {
						r = &workloadRow{Owner: name, OwnerID: id}
						agg[key] = r
					}
					r.OpenTasks++
				}
			}
		}
		taskRows.Close()
	}
	// Issues open
	issueRows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_issues
		WHERE LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')`)
	if err == nil {
		for issueRows.Next() {
			var raw []byte
			if err := issueRows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			id, _ := obj["assignee_id"].(string)
			if id == "" {
				continue
			}
			r := agg[id]
			if r == nil {
				r = &workloadRow{OwnerID: id}
				agg[id] = r
			}
			r.OpenIssues++
		}
		issueRows.Close()
	}

	var rowsOut []workloadRow
	total := 0
	for _, r := range agg {
		r.TotalOpen = r.OpenTasks + r.OpenIssues
		total += r.TotalOpen
		rowsOut = append(rowsOut, *r)
	}
	if total > 0 {
		for i := range rowsOut {
			rowsOut[i].SharePct = math.Round(float64(rowsOut[i].TotalOpen)/float64(total)*1000) / 10
		}
	}
	stats := &workloadStats{Rows: rowsOut}
	if len(rowsOut) > 0 {
		mean := float64(total) / float64(len(rowsOut))
		var sumSq float64
		minC, maxC := rowsOut[0].TotalOpen, rowsOut[0].TotalOpen
		for _, r := range rowsOut {
			sumSq += (float64(r.TotalOpen) - mean) * (float64(r.TotalOpen) - mean)
			if r.TotalOpen < minC {
				minC = r.TotalOpen
			}
			if r.TotalOpen > maxC {
				maxC = r.TotalOpen
			}
		}
		stats.Mean = math.Round(mean*100) / 100
		stats.Stddev = math.Round(math.Sqrt(sumSq/float64(len(rowsOut)))*100) / 100
		stats.Spread = float64(maxC - minC)
	}
	switch sortBy {
	case "name":
		sort.Slice(stats.Rows, func(i, j int) bool { return stats.Rows[i].Owner < stats.Rows[j].Owner })
	default:
		sort.Slice(stats.Rows, func(i, j int) bool { return stats.Rows[i].TotalOpen > stats.Rows[j].TotalOpen })
	}
	return stats, nil
}

// ---------- grep ----------

type grepHit struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Project string `json:"project,omitempty"`
	Snippet string `json:"snippet"`
}

func newGrepCmd(flags *rootFlags) *cobra.Command {
	var dbPath, in string
	var limit int
	cmd := &cobra.Command{
		Use:         "grep [query]",
		Short:       "Search tasks, issues, and projects locally",
		Example:     "  zoho-projects-pp-cli grep 'payment gateway' --in tasks,issues --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []grepHit{})
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			sources := strings.Split(strings.ReplaceAll(in, " ", ""), ",")
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			hits, err := runGrep(cmd, db, query, sources, limit)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, hits)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&in, "in", "tasks,issues,projects", "Sources: tasks,issues,projects")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum results")
	return cmd
}

func runGrep(cmd *cobra.Command, db *store.Store, query string, sources []string, limit int) ([]grepHit, error) {
	q := "%" + query + "%"
	enabled := map[string]bool{}
	for _, s := range sources {
		enabled[strings.ToLower(strings.TrimSpace(s))] = true
	}
	out := []grepHit{}
	if enabled["tasks"] {
		rows, err := db.DB().QueryContext(cmd.Context(), `SELECT
			IFNULL(json_extract(data, '$.id'), ''),
			IFNULL(json_extract(data, '$.name'), ''),
			IFNULL(json_extract(data, '$.details.description'), ''),
			IFNULL(json_extract(data, '$.project.name'), '')
		FROM projects_tasks
		WHERE LOWER(IFNULL(json_extract(data, '$.name'), '')) LIKE LOWER(?)
		   OR LOWER(IFNULL(json_extract(data, '$.details.description'), '')) LIKE LOWER(?)
		LIMIT ?`, q, q, limit)
		if err == nil {
			for rows.Next() {
				var id, name, desc, proj string
				if err := rows.Scan(&id, &name, &desc, &proj); err == nil {
					out = append(out, grepHit{ID: id, Kind: "task", Name: name, Project: proj, Snippet: snip(coalesceStr(name, desc), query)})
				}
			}
			rows.Close()
		}
	}
	if enabled["issues"] {
		rows, err := db.DB().QueryContext(cmd.Context(), `SELECT
			IFNULL(json_extract(data, '$.id'), ''),
			IFNULL(json_extract(data, '$.title'), ''),
			IFNULL(json_extract(data, '$.description'), ''),
			IFNULL(json_extract(data, '$.project.name'), '')
		FROM projects_issues
		WHERE LOWER(IFNULL(json_extract(data, '$.title'), '')) LIKE LOWER(?)
		   OR LOWER(IFNULL(json_extract(data, '$.description'), '')) LIKE LOWER(?)
		LIMIT ?`, q, q, limit)
		if err == nil {
			for rows.Next() {
				var id, title, desc, proj string
				if err := rows.Scan(&id, &title, &desc, &proj); err == nil {
					out = append(out, grepHit{ID: id, Kind: "issue", Name: title, Project: proj, Snippet: snip(coalesceStr(title, desc), query)})
				}
			}
			rows.Close()
		}
	}
	if enabled["projects"] {
		rows, err := db.DB().QueryContext(cmd.Context(), `SELECT
			IFNULL(json_extract(data, '$.id'), ''),
			IFNULL(json_extract(data, '$.name'), ''),
			IFNULL(json_extract(data, '$.description'), '')
		FROM projects
		WHERE LOWER(IFNULL(json_extract(data, '$.name'), '')) LIKE LOWER(?)
		   OR LOWER(IFNULL(json_extract(data, '$.description'), '')) LIKE LOWER(?)
		LIMIT ?`, q, q, limit)
		if err == nil {
			for rows.Next() {
				var id, name, desc string
				if err := rows.Scan(&id, &name, &desc); err == nil {
					out = append(out, grepHit{ID: id, Kind: "project", Name: name, Snippet: snip(coalesceStr(name, desc), query)})
				}
			}
			rows.Close()
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func snip(text, query string) string {
	if text == "" {
		return text
	}
	lt := strings.ToLower(text)
	lq := strings.ToLower(query)
	idx := strings.Index(lt, lq)
	if idx == -1 {
		if len(text) > 180 {
			return text[:180] + "…"
		}
		return text
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 80
	if end > len(text) {
		end = len(text)
	}
	prefix := ""
	if start > 0 {
		prefix = "…"
	}
	suffix := ""
	if end < len(text) {
		suffix = "…"
	}
	return prefix + text[start:end] + suffix
}

func coalesceStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------- issue-heat ----------

type issueHeatRow struct {
	ProjectID string `json:"project_id"`
	Project   string `json:"project"`
	Critical  int    `json:"critical"`
	High      int    `json:"high"`
	Medium    int    `json:"medium"`
	Low       int    `json:"low"`
	Total     int    `json:"total"`
}

func newIssueHeatCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "issue-heat",
		Short:       "Per-project open-issue counts by severity",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []issueHeatRow{})
			}
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeIssueHeat(cmd, db)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeIssueHeat(cmd *cobra.Command, db *store.Store) ([]issueHeatRow, error) {
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT
		IFNULL(json_extract(data, '$.project.id'), '') AS pid,
		IFNULL(json_extract(data, '$.project.name'), '') AS pname,
		LOWER(IFNULL(json_extract(data, '$.severity.type'), IFNULL(json_extract(data, '$.severity.name'), ''))) AS sev
	FROM projects_issues
	WHERE LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')`)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	agg := map[string]*issueHeatRow{}
	for rows.Next() {
		var pid, pname, sev string
		if err := rows.Scan(&pid, &pname, &sev); err != nil {
			continue
		}
		key := pid
		if key == "" {
			key = pname
		}
		r := agg[key]
		if r == nil {
			r = &issueHeatRow{ProjectID: pid, Project: pname}
			agg[key] = r
		}
		r.Total++
		switch {
		case strings.Contains(sev, "critical"):
			r.Critical++
		case strings.Contains(sev, "high"):
			r.High++
		case strings.Contains(sev, "medium"):
			r.Medium++
		case strings.Contains(sev, "low"):
			r.Low++
		}
	}
	out := []issueHeatRow{}
	for _, r := range agg {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Critical*4+out[i].High*2+out[i].Medium > out[j].Critical*4+out[j].High*2+out[j].Medium
	})
	return out, nil
}

// ---------- project-burn ----------

type projectBurnRow struct {
	ID               string  `json:"id"`
	Project          string  `json:"project"`
	TaskCount        int     `json:"task_count"`
	CompletedTasks   int     `json:"completed_tasks"`
	PctComplete      float64 `json:"pct_complete"`
	Velocity4w       float64 `json:"velocity_4w_per_week,omitempty"`
	ProjectedEndDate string  `json:"projected_end_date,omitempty"`
}

func newProjectBurnCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "project-burn",
		Short:       "Per-project completion ratio + projected end date",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []projectBurnRow{})
			}
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeProjectBurn(cmd, db)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeProjectBurn(cmd *cobra.Command, db *store.Store) ([]projectBurnRow, error) {
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT
		IFNULL(json_extract(data, '$.id'), ''),
		IFNULL(json_extract(data, '$.name'), '')
	FROM projects
	WHERE LOWER(IFNULL(json_extract(data, '$.status'), 'active')) = 'active'`)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []projectBurnRow
	now := time.Now().UTC()
	fourWeeksAgo := now.AddDate(0, 0, -28).Format(time.RFC3339)
	for rows.Next() {
		var pid, pname string
		if err := rows.Scan(&pid, &pname); err != nil {
			continue
		}
		var total, completed, recent int
		_ = db.DB().QueryRow(`SELECT COUNT(*) FROM projects_tasks WHERE json_extract(data, '$.project.id') = ?`, pid).Scan(&total)
		_ = db.DB().QueryRow(`SELECT COUNT(*) FROM projects_tasks WHERE json_extract(data, '$.project.id') = ?
			AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) IN ('closed', 'completed')`, pid).Scan(&completed)
		_ = db.DB().QueryRow(`SELECT COUNT(*) FROM projects_tasks WHERE json_extract(data, '$.project.id') = ?
			AND IFNULL(json_extract(data, '$.last_modified_time'), '') > ?
			AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) IN ('closed', 'completed')`, pid, fourWeeksAgo).Scan(&recent)
		r := projectBurnRow{ID: pid, Project: pname, TaskCount: total, CompletedTasks: completed}
		if total > 0 {
			r.PctComplete = math.Round(float64(completed)/float64(total)*1000) / 10
		}
		if recent > 0 && completed < total {
			velocity := float64(recent) / 4.0 // per week
			r.Velocity4w = math.Round(velocity*10) / 10
			remaining := total - completed
			weeks := float64(remaining) / velocity
			eta := now.AddDate(0, 0, int(weeks*7))
			r.ProjectedEndDate = eta.Format("2006-01-02")
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PctComplete < out[j].PctComplete })
	return out, nil
}

// ---------- unassigned ----------

type unassignedRow struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Project string `json:"project,omitempty"`
	Status  string `json:"status,omitempty"`
	EndDate string `json:"end_date,omitempty"`
}

func newUnassignedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "unassigned",
		Short:       "Open tasks and issues with no owner across active projects",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []unassignedRow{})
			}
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeUnassigned(cmd, db)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeUnassigned(cmd *cobra.Command, db *store.Store) ([]unassignedRow, error) {
	out := []unassignedRow{}
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_tasks
		WHERE (json_extract(data, '$.owners') IS NULL OR json_array_length(json_extract(data, '$.owners')) = 0)
		  AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')`)
	if err == nil {
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			r := unassignedRow{Kind: "task"}
			r.ID, _ = obj["id"].(string)
			r.Name, _ = obj["name"].(string)
			r.EndDate, _ = obj["end_date"].(string)
			if proj, ok := obj["project"].(map[string]any); ok {
				r.Project, _ = proj["name"].(string)
			}
			if st, ok := obj["status"].(map[string]any); ok {
				r.Status, _ = st["name"].(string)
			}
			out = append(out, r)
		}
		rows.Close()
	}
	irows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_issues
		WHERE (json_extract(data, '$.assignee_id') IS NULL OR json_extract(data, '$.assignee_id') = '')
		  AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')`)
	if err == nil {
		for irows.Next() {
			var raw []byte
			if err := irows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			r := unassignedRow{Kind: "issue"}
			r.ID, _ = obj["id"].(string)
			r.Name, _ = obj["title"].(string)
			r.EndDate, _ = obj["due_date"].(string)
			if proj, ok := obj["project"].(map[string]any); ok {
				r.Project, _ = proj["name"].(string)
			}
			if st, ok := obj["status"].(map[string]any); ok {
				r.Status, _ = st["name"].(string)
			}
			out = append(out, r)
		}
		irows.Close()
	}
	return out, nil
}

// ---------- my-focus ----------

type focusRow struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Project  string `json:"project,omitempty"`
	EndDate  string `json:"end_date,omitempty"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
}

func newMyFocusCmd(flags *rootFlags) *cobra.Command {
	var dbPath, ownerID string
	var limit int
	cmd := &cobra.Command{
		Use:   "my-focus",
		Short: "Top N items assigned to the authenticated user, ranked by due date",
		Long: `Joins locally-synced tasks + issues filtered by owner, ranks by end_date / due_date.
Defaults to the authenticated user (resolved from local users table).`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []focusRow{})
			}
			db, err := openLocalDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if ownerID == "" {
				// Try to resolve from users table (first row with zuid set)
				_ = db.DB().QueryRow(`SELECT IFNULL(json_extract(data, '$.id'), '') FROM users LIMIT 1`).Scan(&ownerID)
			}
			if ownerID == "" {
				return fmt.Errorf("provide --owner <id> or sync users first")
			}
			rows, err := computeMyFocus(cmd, db, ownerID, limit)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&ownerID, "owner", "", "User ID (defaults to first synced user)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum items")
	return cmd
}

func computeMyFocus(cmd *cobra.Command, db *store.Store, ownerID string, limit int) ([]focusRow, error) {
	out := []focusRow{}
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_tasks
		WHERE json_extract(data, '$.owners') LIKE '%' || ? || '%'
		  AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')
		ORDER BY IFNULL(json_extract(data, '$.end_date'), '9999-12-31') ASC
		LIMIT ?`, ownerID, limit)
	if err == nil {
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			r := focusRow{Kind: "task"}
			r.ID, _ = obj["id"].(string)
			r.Name, _ = obj["name"].(string)
			r.EndDate, _ = obj["end_date"].(string)
			r.Priority, _ = obj["priority"].(string)
			if proj, ok := obj["project"].(map[string]any); ok {
				r.Project, _ = proj["name"].(string)
			}
			if st, ok := obj["status"].(map[string]any); ok {
				r.Status, _ = st["name"].(string)
			}
			out = append(out, r)
		}
		rows.Close()
	}
	irows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM projects_issues
		WHERE IFNULL(json_extract(data, '$.assignee_id'), '') = ?
		  AND LOWER(IFNULL(json_extract(data, '$.status.type'), IFNULL(json_extract(data, '$.status.name'), ''))) NOT IN ('closed', 'completed')
		ORDER BY IFNULL(json_extract(data, '$.due_date'), '9999-12-31') ASC
		LIMIT ?`, ownerID, limit)
	if err == nil {
		for irows.Next() {
			var raw []byte
			if err := irows.Scan(&raw); err != nil {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				continue
			}
			r := focusRow{Kind: "issue"}
			r.ID, _ = obj["id"].(string)
			r.Name, _ = obj["title"].(string)
			r.EndDate, _ = obj["due_date"].(string)
			if proj, ok := obj["project"].(map[string]any); ok {
				r.Project, _ = proj["name"].(string)
			}
			if st, ok := obj["status"].(map[string]any); ok {
				r.Status, _ = st["name"].(string)
			}
			out = append(out, r)
		}
		irows.Close()
	}
	// Final sort by end date ascending, limit
	sort.Slice(out, func(i, j int) bool {
		a := out[i].EndDate
		b := out[j].EndDate
		if a == "" {
			a = "9999-12-31"
		}
		if b == "" {
			b = "9999-12-31"
		}
		return a < b
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
