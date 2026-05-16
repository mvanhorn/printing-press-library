// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

// Novel feature commands for zoho-desk-pp-cli. These are LOCAL commands that
// query the synced SQLite store; they do not hit the API.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
)

// ---------- at-risk ----------

type atRiskRow struct {
	TicketID      string  `json:"ticket_id"`
	Number        int64   `json:"ticket_number,omitempty"`
	Subject       string  `json:"subject"`
	Status        string  `json:"status"`
	Priority      string  `json:"priority,omitempty"`
	Assignee      string  `json:"assignee,omitempty"`
	AssigneeID    string  `json:"assignee_id,omitempty"`
	Department    string  `json:"department,omitempty"`
	DueDate       string  `json:"due_date"`
	HoursToBreach float64 `json:"hours_to_breach"`
}

func newAtRiskCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath     string
		within     string
		unassigned bool
		department string
	)
	cmd := &cobra.Command{
		Use:   "at-risk",
		Short: "Tickets approaching SLA breach within a time window",
		Long: `Locally joins tickets.due_date with status and assignee to surface tickets
about to breach. Filter by unassigned-only to find tickets needing owners.

LOCAL command: requires 'sync' with tickets populated.`,
		Example: `  # Unassigned tickets breaching within 4 hours
  zoho-desk-pp-cli at-risk --within 4h --unassigned --json

  # Anything breaching in the next day
  zoho-desk-pp-cli at-risk --within 24h --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []atRiskRow{})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			dur, err := time.ParseDuration(within)
			if err != nil {
				return fmt.Errorf("parse --within: %w", err)
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			rows, err := computeAtRisk(cmd, db, dur, unassigned, department)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&within, "within", "4h", "Breach window (Go duration, e.g. 4h, 24h)")
	cmd.Flags().BoolVar(&unassigned, "unassigned", false, "Filter to tickets with no assignee")
	cmd.Flags().StringVar(&department, "department", "", "Filter by department name (case-insensitive contains)")
	return cmd
}

func computeAtRisk(cmd *cobra.Command, db *store.Store, within time.Duration, unassigned bool, deptFilter string) ([]atRiskRow, error) {
	cutoff := time.Now().UTC().Add(within).Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)

	q := `SELECT data FROM tickets
	WHERE IFNULL(json_extract(data, '$.dueDate'), '') > ?
	  AND IFNULL(json_extract(data, '$.dueDate'), '') < ?
	  AND IFNULL(json_extract(data, '$.status'), '') NOT IN ('Closed', 'closed', 'Resolved')`
	params := []any{now, cutoff}
	if unassigned {
		q += ` AND (json_extract(data, '$.assigneeId') IS NULL OR json_extract(data, '$.assigneeId') = '')`
	}
	q += ` ORDER BY json_extract(data, '$.dueDate') ASC LIMIT 200`
	rows, err := db.DB().QueryContext(cmd.Context(), q, params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []atRiskRow
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		row := atRiskRowFromTicket(obj)
		if deptFilter != "" && !stringContainsFold(row.Department, deptFilter) {
			continue
		}
		if row.DueDate != "" {
			if t, err := time.Parse(time.RFC3339, row.DueDate); err == nil {
				row.HoursToBreach = math.Round(time.Until(t).Hours()*10) / 10
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func atRiskRowFromTicket(obj map[string]any) atRiskRow {
	r := atRiskRow{}
	r.TicketID, _ = obj["id"].(string)
	if n, ok := obj["ticketNumber"].(float64); ok {
		r.Number = int64(n)
	}
	r.Subject, _ = obj["subject"].(string)
	r.Status, _ = obj["status"].(string)
	r.Priority, _ = obj["priority"].(string)
	r.AssigneeID, _ = obj["assigneeId"].(string)
	r.DueDate, _ = obj["dueDate"].(string)
	if dept, ok := obj["departmentId"].(string); ok {
		r.Department = dept
	}
	if assignee, ok := obj["assignee"].(map[string]any); ok {
		if name, ok := assignee["name"].(string); ok {
			r.Assignee = name
		}
	}
	return r
}

// ---------- aging triage ----------

type agingRow struct {
	TicketID         string  `json:"ticket_id"`
	Number           int64   `json:"ticket_number,omitempty"`
	Subject          string  `json:"subject"`
	Status           string  `json:"status"`
	Assignee         string  `json:"assignee,omitempty"`
	LastAgentReplyAt string  `json:"last_agent_reply_at,omitempty"`
	DaysSinceReply   float64 `json:"days_since_reply"`
}

func newAgingCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		days   int
		status string
	)
	cmd := &cobra.Command{
		Use:   "aging",
		Short: "Open tickets where the last agent reply is more than N days old",
		Long: `Joins tickets with threads locally to compute the age of the most recent
agent reply per ticket. Surfaces tickets the customer is waiting on.`,
		Example: `  # Open tickets stale for 5+ days
  zoho-desk-pp-cli aging --days 5 --status open --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []agingRow{})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			rows, err := computeAging(cmd, db, days, status)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 5, "Minimum days since last agent reply")
	cmd.Flags().StringVar(&status, "status", "open", "Filter to status (default 'open')")
	return cmd
}

func computeAging(cmd *cobra.Command, db *store.Store, days int, status string) ([]agingRow, error) {
	threshold := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	q := `SELECT t.data,
		(SELECT MAX(IFNULL(json_extract(th.data, '$.createdTime'), '')) FROM threads th
		 WHERE json_extract(th.data, '$.ticketId') = t.id
		   AND IFNULL(json_extract(th.data, '$.author.type'), 'AGENT') = 'AGENT') AS last_reply
	FROM tickets t
	WHERE 1=1`
	params := []any{}
	if status != "" {
		q += ` AND LOWER(IFNULL(json_extract(t.data, '$.status'), '')) LIKE LOWER(?)`
		params = append(params, "%"+status+"%")
	}
	q += ` HAVING (last_reply IS NULL OR last_reply < ?)`
	params = append(params, threshold)
	q += ` ORDER BY last_reply ASC LIMIT 200`

	rows, err := db.DB().QueryContext(cmd.Context(), q, params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []agingRow
	for rows.Next() {
		var raw []byte
		var lastReply *string
		if err := rows.Scan(&raw, &lastReply); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		row := agingRow{}
		row.TicketID, _ = obj["id"].(string)
		if n, ok := obj["ticketNumber"].(float64); ok {
			row.Number = int64(n)
		}
		row.Subject, _ = obj["subject"].(string)
		row.Status, _ = obj["status"].(string)
		if assignee, ok := obj["assignee"].(map[string]any); ok {
			row.Assignee, _ = assignee["name"].(string)
		}
		if lastReply != nil {
			row.LastAgentReplyAt = *lastReply
			if t, err := time.Parse(time.RFC3339, *lastReply); err == nil {
				row.DaysSinceReply = math.Round(time.Since(t).Hours()/24*10) / 10
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// ---------- grep (conversational FTS) ----------

type grepHit struct {
	TicketID    string `json:"ticket_id,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Source      string `json:"source"` // "ticket" | "thread" | "comment"
	SnippetID   string `json:"snippet_id,omitempty"`
	Snippet     string `json:"snippet"`
	CreatedTime string `json:"created_time,omitempty"`
}

func newGrepCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		in     string
		from   string
		to     string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "grep [query]",
		Short: "Search across ticket subject, description, threads, and comments",
		Long: `Full-text search across locally-synced conversation content. Use --in to
restrict source (tickets, threads, comments). Case-insensitive substring match.

This complements Zoho's /search endpoint (subject + description only, keyword-only)
with deep search into thread and comment content.`,
		Example: `  zoho-desk-pp-cli grep "TLS handshake" --in comments,threads --json
  zoho-desk-pp-cli grep "billing question" --from 2026-01-01 --json`,
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
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			hits, err := runGrep(cmd, db, query, sources, from, to, limit)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, hits)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&in, "in", "tickets,threads,comments", "Comma-separated sources: tickets,threads,comments")
	cmd.Flags().StringVar(&from, "from", "", "Earliest createdTime (ISO 8601)")
	cmd.Flags().StringVar(&to, "to", "", "Latest createdTime (ISO 8601)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum hits")
	return cmd
}

func runGrep(cmd *cobra.Command, db *store.Store, query string, sources []string, from, to string, limit int) ([]grepHit, error) {
	q := "%" + query + "%"
	out := []grepHit{}

	enabled := map[string]bool{}
	for _, s := range sources {
		enabled[strings.ToLower(s)] = true
	}

	if enabled["tickets"] {
		sql := `SELECT IFNULL(json_extract(data, '$.id'), ''), IFNULL(json_extract(data, '$.subject'), ''),
				IFNULL(json_extract(data, '$.description'), ''), IFNULL(json_extract(data, '$.createdTime'), '')
			FROM tickets
			WHERE LOWER(IFNULL(json_extract(data, '$.subject'), '')) LIKE LOWER(?)
			   OR LOWER(IFNULL(json_extract(data, '$.description'), '')) LIKE LOWER(?)`
		params := []any{q, q}
		if from != "" {
			sql += " AND IFNULL(json_extract(data, '$.createdTime'), '') >= ?"
			params = append(params, from)
		}
		if to != "" {
			sql += " AND IFNULL(json_extract(data, '$.createdTime'), '') <= ?"
			params = append(params, to)
		}
		sql += " LIMIT ?"
		params = append(params, limit)
		rows, err := db.DB().QueryContext(cmd.Context(), sql, params...)
		if err == nil {
			for rows.Next() {
				var id, subj, desc, ct string
				if err := rows.Scan(&id, &subj, &desc, &ct); err == nil {
					snippet := truncateSnippet(coalesceStr(extractMatch(subj, query), extractMatch(desc, query)), 200)
					out = append(out, grepHit{TicketID: id, Subject: subj, Source: "ticket", SnippetID: id, Snippet: snippet, CreatedTime: ct})
				}
			}
			rows.Close()
		}
	}

	if enabled["threads"] {
		sql := `SELECT IFNULL(json_extract(data, '$.id'), ''), IFNULL(json_extract(data, '$.ticketId'), ''),
				IFNULL(json_extract(data, '$.content'), ''), IFNULL(json_extract(data, '$.createdTime'), '')
			FROM threads
			WHERE LOWER(IFNULL(json_extract(data, '$.content'), '')) LIKE LOWER(?)`
		params := []any{q}
		if from != "" {
			sql += " AND IFNULL(json_extract(data, '$.createdTime'), '') >= ?"
			params = append(params, from)
		}
		if to != "" {
			sql += " AND IFNULL(json_extract(data, '$.createdTime'), '') <= ?"
			params = append(params, to)
		}
		sql += " LIMIT ?"
		params = append(params, limit)
		rows, err := db.DB().QueryContext(cmd.Context(), sql, params...)
		if err == nil {
			for rows.Next() {
				var threadID, ticketID, content, ct string
				if err := rows.Scan(&threadID, &ticketID, &content, &ct); err == nil {
					out = append(out, grepHit{TicketID: ticketID, Source: "thread", SnippetID: threadID, Snippet: truncateSnippet(extractMatch(content, query), 300), CreatedTime: ct})
				}
			}
			rows.Close()
		}
	}

	if enabled["comments"] {
		sql := `SELECT IFNULL(json_extract(data, '$.id'), ''), IFNULL(json_extract(data, '$.ticketId'), ''),
				IFNULL(json_extract(data, '$.content'), ''), IFNULL(json_extract(data, '$.createdTime'), '')
			FROM ticket_comments
			WHERE LOWER(IFNULL(json_extract(data, '$.content'), '')) LIKE LOWER(?)`
		params := []any{q}
		if from != "" {
			sql += " AND IFNULL(json_extract(data, '$.createdTime'), '') >= ?"
			params = append(params, from)
		}
		if to != "" {
			sql += " AND IFNULL(json_extract(data, '$.createdTime'), '') <= ?"
			params = append(params, to)
		}
		sql += " LIMIT ?"
		params = append(params, limit)
		rows, err := db.DB().QueryContext(cmd.Context(), sql, params...)
		if err == nil {
			for rows.Next() {
				var commentID, ticketID, content, ct string
				if err := rows.Scan(&commentID, &ticketID, &content, &ct); err == nil {
					out = append(out, grepHit{TicketID: ticketID, Source: "comment", SnippetID: commentID, Snippet: truncateSnippet(extractMatch(content, query), 300), CreatedTime: ct})
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

func extractMatch(text, query string) string {
	if text == "" || query == "" {
		return text
	}
	lt := strings.ToLower(text)
	lq := strings.ToLower(query)
	idx := strings.Index(lt, lq)
	if idx == -1 {
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

func truncateSnippet(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func coalesceStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------- workload ----------

type workloadRow struct {
	AssigneeID string  `json:"assignee_id"`
	Assignee   string  `json:"assignee"`
	Department string  `json:"department,omitempty"`
	OpenCount  int     `json:"open_count"`
	SharePct   float64 `json:"share_pct"`
}

type workloadStats struct {
	Department string        `json:"department,omitempty"`
	Mean       float64       `json:"mean"`
	Stddev     float64       `json:"stddev"`
	Gini       float64       `json:"gini"`
	Spread     float64       `json:"spread"`
	Rows       []workloadRow `json:"rows"`
}

func newWorkloadCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath     string
		department string
		sortBy     string
	)
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Per-agent open-ticket counts with team dispersion stats",
		Long: `Locally groups tickets by assigneeId, computes mean / stddev / Gini /
spread across agents in a department. Reveals load imbalance.`,
		Example: `  # Cross-team workload spread
  zoho-desk-pp-cli workload --department Support --sort spread --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, &workloadStats{Rows: []workloadRow{}})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			stats, err := computeWorkload(cmd, db, department, sortBy)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, stats)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&department, "department", "", "Filter by department name")
	cmd.Flags().StringVar(&sortBy, "sort", "count", "Sort by: count, spread, name")
	return cmd
}

func computeWorkload(cmd *cobra.Command, db *store.Store, department, sortBy string) (*workloadStats, error) {
	q := `SELECT IFNULL(json_extract(data, '$.assigneeId'), '') AS aid,
		IFNULL(json_extract(data, '$.departmentId'), '') AS did,
		COUNT(*) AS n
	FROM tickets
	WHERE IFNULL(json_extract(data, '$.status'), '') NOT IN ('Closed', 'closed', 'Resolved', 'resolved')
	  AND json_extract(data, '$.assigneeId') IS NOT NULL
	GROUP BY aid, did`
	rows, err := db.DB().QueryContext(cmd.Context(), q)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	type agg struct {
		aid, did string
		count    int
	}
	var data []agg
	for rows.Next() {
		var a agg
		if err := rows.Scan(&a.aid, &a.did, &a.count); err != nil {
			return nil, err
		}
		if a.aid == "" {
			continue
		}
		data = append(data, a)
	}

	// Resolve names from agents/departments tables
	agentNames := map[string]string{}
	if r, err := db.DB().QueryContext(cmd.Context(), `SELECT IFNULL(json_extract(data, '$.id'), ''), IFNULL(json_extract(data, '$.firstName') || ' ' || json_extract(data, '$.lastName'), json_extract(data, '$.name'), '') FROM agents`); err == nil {
		for r.Next() {
			var id, name string
			if err := r.Scan(&id, &name); err == nil {
				agentNames[id] = strings.TrimSpace(name)
			}
		}
		r.Close()
	}
	deptNames := map[string]string{}
	if r, err := db.DB().QueryContext(cmd.Context(), `SELECT IFNULL(json_extract(data, '$.id'), ''), IFNULL(json_extract(data, '$.name'), '') FROM departments`); err == nil {
		for r.Next() {
			var id, name string
			if err := r.Scan(&id, &name); err == nil {
				deptNames[id] = name
			}
		}
		r.Close()
	}

	rowsOut := []workloadRow{}
	total := 0
	for _, a := range data {
		dn := deptNames[a.did]
		if department != "" && !stringContainsFold(dn, department) {
			continue
		}
		rowsOut = append(rowsOut, workloadRow{
			AssigneeID: a.aid,
			Assignee:   agentNames[a.aid],
			Department: dn,
			OpenCount:  a.count,
		})
		total += a.count
	}
	if total > 0 {
		for i := range rowsOut {
			rowsOut[i].SharePct = math.Round(float64(rowsOut[i].OpenCount)/float64(total)*1000) / 10
		}
	}

	// Stats
	stats := &workloadStats{Department: department, Rows: rowsOut}
	if len(rowsOut) > 0 {
		mean := float64(total) / float64(len(rowsOut))
		var sumSq float64
		for _, r := range rowsOut {
			sumSq += (float64(r.OpenCount) - mean) * (float64(r.OpenCount) - mean)
		}
		stats.Mean = math.Round(mean*100) / 100
		stats.Stddev = math.Round(math.Sqrt(sumSq/float64(len(rowsOut)))*100) / 100
		// Gini coefficient (sorted)
		counts := make([]int, len(rowsOut))
		for i, r := range rowsOut {
			counts[i] = r.OpenCount
		}
		sort.Ints(counts)
		n := len(counts)
		sum := 0
		weighted := 0
		for i, c := range counts {
			sum += c
			weighted += (i + 1) * c
		}
		if sum > 0 {
			stats.Gini = math.Round((float64(2*weighted)/(float64(n)*float64(sum))-float64(n+1)/float64(n))*1000) / 1000
		}
		if len(counts) > 0 {
			stats.Spread = float64(counts[len(counts)-1] - counts[0])
		}
	}

	switch sortBy {
	case "spread":
		sort.Slice(stats.Rows, func(i, j int) bool { return stats.Rows[i].OpenCount > stats.Rows[j].OpenCount })
	case "name":
		sort.Slice(stats.Rows, func(i, j int) bool { return stats.Rows[i].Assignee < stats.Rows[j].Assignee })
	default:
		sort.Slice(stats.Rows, func(i, j int) bool { return stats.Rows[i].OpenCount > stats.Rows[j].OpenCount })
	}
	return stats, nil
}

// ---------- reopens ----------

type reopenRow struct {
	TicketID     string `json:"ticket_id"`
	Subject      string `json:"subject,omitempty"`
	ReopenCount  int    `json:"reopen_count"`
	LastReopened string `json:"last_reopened,omitempty"`
}

func newReopensCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		minCount int
		window   string
	)
	cmd := &cobra.Command{
		Use:   "reopens",
		Short: "Tickets reopened at least N times within a window",
		Long: `Scans local ticket_history for status transitions closed→open, groups
by ticketId, and reports counts. Signals unresolved root causes.`,
		Example:     `  zoho-desk-pp-cli reopens --min-count 2 --window 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []reopenRow{})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			cutoff := ""
			if window != "" {
				if dur, err := time.ParseDuration(window); err == nil {
					cutoff = time.Now().UTC().Add(-dur).Format(time.RFC3339)
				}
			}
			rows, err := computeReopens(cmd, db, minCount, cutoff)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&minCount, "min-count", 2, "Minimum reopen count")
	cmd.Flags().StringVar(&window, "window", "720h", "Lookback window (Go duration, default 30d)")
	return cmd
}

func computeReopens(cmd *cobra.Command, db *store.Store, minCount int, cutoff string) ([]reopenRow, error) {
	// Look at ticket history events where event_type indicates a status reopen.
	// Different tables in the generated store may carry history; try both.
	for _, table := range []string{"ticket_history", "tickets_history"} {
		q := `SELECT IFNULL(json_extract(data, '$.ticketId'), '') AS tid,
			IFNULL(json_extract(data, '$.eventType'), '') AS et,
			IFNULL(json_extract(data, '$.eventTime'), '') AS et_time
		FROM ` + table + `
		WHERE LOWER(IFNULL(json_extract(data, '$.eventType'), '')) LIKE '%reopen%'
		   OR (LOWER(IFNULL(json_extract(data, '$.newStatus'), '')) = 'open'
			   AND LOWER(IFNULL(json_extract(data, '$.oldStatus'), '')) IN ('closed', 'resolved'))`
		if cutoff != "" {
			q += " AND IFNULL(json_extract(data, '$.eventTime'), '') >= ?"
		}
		params := []any{}
		if cutoff != "" {
			params = append(params, cutoff)
		}
		rows, err := db.DB().QueryContext(cmd.Context(), q, params...)
		if err != nil {
			continue // table may not exist
		}
		counts := map[string]*reopenRow{}
		for rows.Next() {
			var tid, et, etTime string
			if err := rows.Scan(&tid, &et, &etTime); err != nil {
				continue
			}
			if tid == "" {
				continue
			}
			r, ok := counts[tid]
			if !ok {
				r = &reopenRow{TicketID: tid}
				counts[tid] = r
			}
			r.ReopenCount++
			if etTime > r.LastReopened {
				r.LastReopened = etTime
			}
		}
		rows.Close()

		var out []reopenRow
		for _, r := range counts {
			if r.ReopenCount >= minCount {
				// Resolve subject
				var subj string
				if err := db.DB().QueryRow(`SELECT IFNULL(json_extract(data, '$.subject'), '') FROM tickets WHERE IFNULL(json_extract(data, '$.id'), '') = ?`, r.TicketID).Scan(&subj); err == nil {
					r.Subject = subj
				}
				out = append(out, *r)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ReopenCount > out[j].ReopenCount })
		return out, nil
	}
	return []reopenRow{}, nil
}

// ---------- suggest-agent ----------

type agentSuggestion struct {
	AgentID    string `json:"agent_id"`
	Name       string `json:"name"`
	OpenCount  int    `json:"open_count"`
	Department string `json:"department,omitempty"`
}

func newSuggestAgentCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "suggest-agent [ticketId]",
		Short: "Suggest the least-loaded active agent in a ticket's department",
		Long: `Looks up the ticket's department, lists active agents in that department,
ranks by ascending open-ticket count. Read-only.`,
		Example:     `  zoho-desk-pp-cli suggest-agent 12345 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []agentSuggestion{})
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			ticketID := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			sug, err := suggestAgents(cmd, db, ticketID)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, sug)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func suggestAgents(cmd *cobra.Command, db *store.Store, ticketID string) ([]agentSuggestion, error) {
	var deptID string
	if err := db.DB().QueryRow(`SELECT IFNULL(json_extract(data, '$.departmentId'), '') FROM tickets WHERE IFNULL(json_extract(data, '$.id'), '') = ?`, ticketID).Scan(&deptID); err != nil {
		return nil, fmt.Errorf("ticket %s not found locally; sync first: %w", ticketID, err)
	}
	if deptID == "" {
		return nil, fmt.Errorf("ticket %s has no departmentId", ticketID)
	}
	// All active agents in department
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT
		IFNULL(json_extract(data, '$.id'), '') AS aid,
		IFNULL(json_extract(data, '$.firstName') || ' ' || json_extract(data, '$.lastName'), json_extract(data, '$.name'), '') AS name
	FROM agents
	WHERE IFNULL(json_extract(data, '$.status'), '') IN ('ACTIVE', 'active', 'AVAILABLE')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sug []agentSuggestion
	for rows.Next() {
		var aid, name string
		if err := rows.Scan(&aid, &name); err != nil {
			continue
		}
		var count int
		_ = db.DB().QueryRow(`SELECT COUNT(*) FROM tickets WHERE IFNULL(json_extract(data, '$.assigneeId'), '') = ?
			AND IFNULL(json_extract(data, '$.status'), '') NOT IN ('Closed', 'closed', 'Resolved')`, aid).Scan(&count)
		sug = append(sug, agentSuggestion{AgentID: aid, Name: strings.TrimSpace(name), OpenCount: count, Department: deptID})
	}
	sort.Slice(sug, func(i, j int) bool { return sug[i].OpenCount < sug[j].OpenCount })
	if len(sug) > 10 {
		sug = sug[:10]
	}
	return sug, nil
}

// ---------- contact-history ----------

type contactHistory struct {
	ContactID     string        `json:"contact_id"`
	Name          string        `json:"name"`
	Email         string        `json:"email,omitempty"`
	AccountID     string        `json:"account_id,omitempty"`
	AccountName   string        `json:"account_name,omitempty"`
	TicketCount   int           `json:"ticket_count"`
	OpenTickets   int           `json:"open_tickets"`
	RecentTickets []ticketBrief `json:"recent_tickets,omitempty"`
}

type ticketBrief struct {
	ID          string `json:"id"`
	Number      int64  `json:"ticket_number,omitempty"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	CreatedTime string `json:"created_time"`
}

func newContactHistoryCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "contact-history [email]",
		Short: "All tickets, account, and stats for a contact",
		Long: `Joins contacts × tickets × accounts locally to surface a contact's full
support history in one query.`,
		Example:     `  zoho-desk-pp-cli contact-history jane@example.com --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, &contactHistory{})
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			ident := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			h, err := buildContactHistory(cmd, db, ident, limit)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, h)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 10, "Recent tickets to include")
	return cmd
}

func buildContactHistory(cmd *cobra.Command, db *store.Store, ident string, limit int) (*contactHistory, error) {
	var raw []byte
	q := `SELECT data FROM contacts
		WHERE LOWER(IFNULL(json_extract(data, '$.email'), '')) = LOWER(?)
		   OR IFNULL(json_extract(data, '$.id'), '') = ?
		LIMIT 1`
	if err := db.DB().QueryRowContext(cmd.Context(), q, ident, ident).Scan(&raw); err != nil {
		return nil, fmt.Errorf("contact %q not found locally — sync first", ident)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	h := &contactHistory{}
	h.ContactID, _ = obj["id"].(string)
	if first, ok := obj["firstName"].(string); ok {
		h.Name = first
	}
	if last, ok := obj["lastName"].(string); ok {
		h.Name = strings.TrimSpace(h.Name + " " + last)
	}
	h.Email, _ = obj["email"].(string)
	h.AccountID, _ = obj["accountId"].(string)
	// Account name
	if h.AccountID != "" {
		var aname string
		_ = db.DB().QueryRow(`SELECT IFNULL(json_extract(data, '$.accountName'), '') FROM accounts WHERE IFNULL(json_extract(data, '$.id'), '') = ?`, h.AccountID).Scan(&aname)
		h.AccountName = aname
	}
	// Ticket counts
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM tickets WHERE IFNULL(json_extract(data, '$.contactId'), '') = ?`, h.ContactID).Scan(&h.TicketCount)
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM tickets
		WHERE IFNULL(json_extract(data, '$.contactId'), '') = ?
		AND IFNULL(json_extract(data, '$.status'), '') NOT IN ('Closed', 'closed', 'Resolved')`, h.ContactID).Scan(&h.OpenTickets)
	// Recent tickets
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM tickets
		WHERE IFNULL(json_extract(data, '$.contactId'), '') = ?
		ORDER BY IFNULL(json_extract(data, '$.createdTime'), '') DESC
		LIMIT ?`, h.ContactID, limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var traw []byte
			if err := rows.Scan(&traw); err != nil {
				continue
			}
			var t map[string]any
			if err := json.Unmarshal(traw, &t); err != nil {
				continue
			}
			b := ticketBrief{}
			b.ID, _ = t["id"].(string)
			if n, ok := t["ticketNumber"].(float64); ok {
				b.Number = int64(n)
			}
			b.Subject, _ = t["subject"].(string)
			b.Status, _ = t["status"].(string)
			b.CreatedTime, _ = t["createdTime"].(string)
			h.RecentTickets = append(h.RecentTickets, b)
		}
	}
	return h, nil
}

// ---------- escalations ----------

type escalationRow struct {
	TicketID      string   `json:"ticket_id"`
	Subject       string   `json:"subject,omitempty"`
	Assignees     []string `json:"distinct_assignees"`
	ReassignCount int      `json:"reassign_count"`
	LastChange    string   `json:"last_change,omitempty"`
}

func newEscalationsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath       string
		minReassigns int
	)
	cmd := &cobra.Command{
		Use:   "escalations",
		Short: "Tickets reassigned to N or more distinct agents",
		Long: `Scans local ticket_history for assignee-change events, counts distinct
assignees per ticket. Surfaces ownership churn / unresolvable tickets.`,
		Example:     `  zoho-desk-pp-cli escalations --min-reassigns 3 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []escalationRow{})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()
			rows, err := computeEscalations(cmd, db, minReassigns)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&minReassigns, "min-reassigns", 3, "Minimum distinct assignees")
	return cmd
}

func computeEscalations(cmd *cobra.Command, db *store.Store, minReassigns int) ([]escalationRow, error) {
	for _, table := range []string{"ticket_history", "tickets_history"} {
		rows, err := db.DB().QueryContext(cmd.Context(), `SELECT
			IFNULL(json_extract(data, '$.ticketId'), '') AS tid,
			IFNULL(json_extract(data, '$.newValue'), '') AS new_assignee,
			IFNULL(json_extract(data, '$.eventTime'), '') AS et
		FROM `+table+`
		WHERE LOWER(IFNULL(json_extract(data, '$.fieldName'), '')) IN ('assigneeid', 'assignee')`)
		if err != nil {
			continue
		}
		assignees := map[string]map[string]bool{}
		lastChange := map[string]string{}
		for rows.Next() {
			var tid, na, et string
			if err := rows.Scan(&tid, &na, &et); err != nil {
				continue
			}
			if tid == "" || na == "" {
				continue
			}
			if _, ok := assignees[tid]; !ok {
				assignees[tid] = map[string]bool{}
			}
			assignees[tid][na] = true
			if et > lastChange[tid] {
				lastChange[tid] = et
			}
		}
		rows.Close()

		var out []escalationRow
		for tid, distinct := range assignees {
			if len(distinct) < minReassigns {
				continue
			}
			r := escalationRow{TicketID: tid, ReassignCount: len(distinct), LastChange: lastChange[tid]}
			for a := range distinct {
				r.Assignees = append(r.Assignees, a)
			}
			sort.Strings(r.Assignees)
			var subj string
			_ = db.DB().QueryRow(`SELECT IFNULL(json_extract(data, '$.subject'), '') FROM tickets WHERE IFNULL(json_extract(data, '$.id'), '') = ?`, tid).Scan(&subj)
			r.Subject = subj
			out = append(out, r)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ReassignCount > out[j].ReassignCount })
		return out, nil
	}
	return []escalationRow{}, nil
}

// unused for now, but referenced if we need numeric parsing
var _ = strconv.Itoa
