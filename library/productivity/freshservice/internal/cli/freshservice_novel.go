// Copyright 2026 mark-van-de-ven. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(freshservice-novel-commands): hand-authored transcendence commands
// (breach-risk, my-queue, workload, change-collisions, recurrence, kb-gaps,
// orphan-assets, dept-sla, oncall-gap) and shared helpers; the OpenAPI spec
// describes the underlying endpoints but not these cross-resource aggregations.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/freshservice/internal/store"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// PATCH(freshservice-filter-query-quoting): wrap query value in literal double
// quotes for /tickets/filter and /changes/filter; generator emits the raw user
// string, which Freshservice rejects with HTTP 500.
func wrapFreshserviceFilterQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	for strings.HasPrefix(q, `"`) && strings.HasSuffix(q, `"`) && len(q) >= 2 {
		q = q[1 : len(q)-1]
	}
	return `"` + q + `"`
}

func openLocalStore(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("freshservice-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'freshservice-pp-cli sync' first.", err)
	}
	return db, nil
}

func loadResources(db *store.Store, resourceType string) ([]map[string]any, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = ?`, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

func anyString(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func anyInt(obj map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		v, ok := obj[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case float64:
			return int(t), true
		case int:
			return t, true
		case int64:
			return int(t), true
		case string:
			if n, err := strconv.Atoi(t); err == nil {
				return n, true
			}
		case json.Number:
			if n, err := t.Int64(); err == nil {
				return int(n), true
			}
		}
	}
	return 0, false
}

func anyTime(obj map[string]any, keys ...string) (time.Time, bool) {
	s := anyString(obj, keys...)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func ticketStatusName(code int) string {
	switch code {
	case 2:
		return "open"
	case 3:
		return "pending"
	case 4:
		return "resolved"
	case 5:
		return "closed"
	}
	return strconv.Itoa(code)
}

func priorityName(code int) string {
	switch code {
	case 1:
		return "low"
	case 2:
		return "medium"
	case 3:
		return "high"
	case 4:
		return "urgent"
	}
	return strconv.Itoa(code)
}

func changeStatusName(code int) string {
	switch code {
	case 1:
		return "open"
	case 2:
		return "planning"
	case 3:
		return "awaiting_approval"
	case 4:
		return "pending_release"
	case 5:
		return "pending_review"
	case 6:
		return "closed"
	}
	return strconv.Itoa(code)
}

// isOpenTicket treats any ticket that is not Resolved (4) or Closed (5) as
// still open. Freshservice tenants commonly define custom statuses (status
// codes 6, 9, 10, …) that represent intermediate workflow states; restricting
// to the built-in 2/3 codes would silently drop every ticket sitting in a
// custom status from breach-risk, my-queue, and workload. Resolved/Closed
// are the only reserved "done" codes — everything else is open.
func isOpenTicket(status int) bool {
	return status != 4 && status != 5 && status != 0
}

// ticketResolvedAt pulls the canonical resolved/closed timestamp from a
// Freshservice ticket. The ticket-detail response writes the actual values
// into the `stats` sub-object (`stats.resolved_at`, `stats.closed_at`); list
// responses typically omit `stats` entirely, so callers must fall back to
// `updated_at` when this returns false.
func ticketResolvedAt(t map[string]any) (time.Time, bool) {
	stats, ok := t["stats"].(map[string]any)
	if !ok {
		return time.Time{}, false
	}
	if v, ok := anyTime(stats, "resolved_at", "closed_at"); ok {
		return v, true
	}
	return time.Time{}, false
}

// extractAssetDisplayIDs pulls every asset display_id referenced by a ticket
// or contract object. Freshservice's canonical shape on a ticket detail
// response is `assets: [{display_id, ...}, ...]`; contracts use
// `asset_associations: [{display_id, ...}, ...]` or sometimes `assets`.
// Both shapes plus a legacy scalar `asset_id` are accepted so callers cope
// with whatever sync happened to land.
func extractAssetDisplayIDs(obj map[string]any) []string {
	var out []string
	for _, key := range []string{"assets", "asset_associations"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if did := anyString(m, "display_id", "id", "asset_id"); did != "" {
				out = append(out, did)
			}
		}
	}
	if aid := anyString(obj, "asset_id"); aid != "" {
		out = append(out, aid)
	}
	return out
}

// ---------------------------------------------------------------------------
// breach-risk
// ---------------------------------------------------------------------------

func newBreachRiskCmd(flags *rootFlags) *cobra.Command {
	var hours float64
	var group string
	var assignee string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "breach-risk",
		Short: "List open tickets projected to breach SLA within N hours",
		Long: `Scan the local ticket corpus for open tickets whose due_by is within the
specified time horizon. Results are sorted by minutes remaining (smallest first).

Requires a prior 'freshservice-pp-cli sync' run to populate tickets locally.`,
		Example: strings.Trim(`
  # Tickets at risk of breaching in the next 4 hours
  freshservice-pp-cli breach-risk --hours 4

  # Scope to a group and emit agent-friendly JSON
  freshservice-pp-cli breach-risk --hours 8 --group Infrastructure --agent

  # Already-breached tickets
  freshservice-pp-cli breach-risk --hours 0
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := loadResources(db, "tickets")
			if err != nil {
				return fmt.Errorf("loading tickets: %w", err)
			}
			meta, _ := LoadTenantMeta(db)

			now := time.Now()
			horizon := now.Add(time.Duration(hours * float64(time.Hour)))

			type row struct {
				ID        string  `json:"id"`
				Subject   string  `json:"subject"`
				Priority  string  `json:"priority"`
				Status    string  `json:"status"`
				DueBy     string  `json:"due_by"`
				MinutesTo float64 `json:"minutes_remaining"`
				GroupID   int     `json:"group_id,omitempty"`
				Responder int     `json:"responder_id,omitempty"`
				Requester int     `json:"requester_id,omitempty"`
				Overdue   bool    `json:"overdue"`
			}

			results := make([]row, 0)
			for _, t := range tickets {
				st, _ := anyInt(t, "status")
				if !isOpenTicket(st) {
					continue
				}
				due, ok := anyTime(t, "due_by", "fr_due_by")
				if !ok {
					continue
				}
				if due.After(horizon) {
					continue
				}
				gid, _ := anyInt(t, "group_id")
				if group != "" && fmt.Sprintf("%d", gid) != group && !strings.EqualFold(anyString(t, "group_name"), group) {
					continue
				}
				rid, _ := anyInt(t, "responder_id")
				if assignee != "" && fmt.Sprintf("%d", rid) != assignee && !strings.EqualFold(anyString(t, "responder_name"), assignee) {
					continue
				}
				prio, _ := anyInt(t, "priority")
				results = append(results, row{
					ID:        anyString(t, "id"),
					Subject:   anyString(t, "subject"),
					Priority:  meta.PriorityLabel(prio),
					Status:    meta.StatusLabel(st),
					DueBy:     due.Format(time.RFC3339),
					MinutesTo: due.Sub(now).Minutes(),
					GroupID:   gid,
					Responder: rid,
					Requester: func() int { x, _ := anyInt(t, "requester_id"); return x }(),
					Overdue:   due.Before(now),
				})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].MinutesTo < results[j].MinutesTo })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"hours_horizon": hours,
					"count":         len(results),
					"tickets":       results,
				})
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tickets at risk in the requested window.")
				return nil
			}
			headers := []string{"ID", "PRIORITY", "STATUS", "DUE_BY", "MINUTES", "SUBJECT"}
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				rows = append(rows, []string{
					r.ID, r.Priority, r.Status, r.DueBy,
					fmt.Sprintf("%.0f", r.MinutesTo),
					truncate(r.Subject, 60),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().Float64Var(&hours, "hours", 4, "Hours of horizon ahead; 0 means already overdue")
	cmd.Flags().StringVar(&group, "group", "", "Filter by group ID or group name")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by responder ID or responder name")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// my-queue
// ---------------------------------------------------------------------------

func newMyQueueCmd(flags *rootFlags) *cobra.Command {
	var agent string
	var includeApprovals bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "my-queue [agent-id-or-email]",
		Short: "Open tickets + pending change approvals for an agent",
		Long: `Combines all open tickets assigned to a specific agent with any pending
change approvals where the agent is approver, ranked by SLA proximity.

Provide the agent as a positional argument (ID or email) or via --agent-id.

Requires a prior 'freshservice-pp-cli sync' run.`,
		Example: strings.Trim(`
  # Positional form
  freshservice-pp-cli my-queue ops@example.com --agent

  # Or via flag
  freshservice-pp-cli my-queue --agent-id 42 --approvals=false --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if agent == "" && len(args) > 0 {
				agent = args[0]
			}
			if agent == "" {
				return cmd.Help()
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := loadResources(db, "tickets")
			if err != nil {
				return fmt.Errorf("loading tickets: %w", err)
			}
			meta, _ := LoadTenantMeta(db)

			now := time.Now()

			type ticketRow struct {
				ID        string  `json:"id"`
				Subject   string  `json:"subject"`
				Priority  string  `json:"priority"`
				Status    string  `json:"status"`
				DueBy     string  `json:"due_by,omitempty"`
				MinutesTo float64 `json:"minutes_remaining,omitempty"`
			}
			type approvalRow struct {
				ChangeID string `json:"change_id"`
				Subject  string `json:"subject"`
				Status   string `json:"status"`
			}

			queue := make([]ticketRow, 0)
			for _, t := range tickets {
				st, _ := anyInt(t, "status")
				if !isOpenTicket(st) {
					continue
				}
				rid, _ := anyInt(t, "responder_id")
				rname := anyString(t, "responder_name")
				if fmt.Sprintf("%d", rid) != agent && !strings.EqualFold(rname, agent) {
					// also try lookup by responder email if present
					if remail := anyString(t, "responder_email"); !strings.EqualFold(remail, agent) {
						continue
					}
				}
				prio, _ := anyInt(t, "priority")
				row := ticketRow{
					ID:       anyString(t, "id"),
					Subject:  anyString(t, "subject"),
					Priority: meta.PriorityLabel(prio),
					Status:   meta.StatusLabel(st),
				}
				if due, ok := anyTime(t, "due_by", "fr_due_by"); ok {
					row.DueBy = due.Format(time.RFC3339)
					row.MinutesTo = due.Sub(now).Minutes()
				}
				queue = append(queue, row)
			}
			// PATCH(freshservice-myqueue-no-due-sort): tickets without due_by
			// have no SLA clock; previously they kept MinutesTo at its float64
			// zero value and floated to the top as if due-right-now. Sort
			// SLA-tracked tickets first (by minutes remaining), then the
			// no-SLA tickets after them in their natural store order.
			sort.SliceStable(queue, func(i, j int) bool {
				iHasDue := queue[i].DueBy != ""
				jHasDue := queue[j].DueBy != ""
				if iHasDue != jHasDue {
					return iHasDue
				}
				return queue[i].MinutesTo < queue[j].MinutesTo
			})

			approvals := make([]approvalRow, 0)
			var approvalsNote string
			if includeApprovals {
				changes, err := loadResources(db, "changes")
				if err != nil {
					return fmt.Errorf("loading changes: %w", err)
				}
				// Surface every change that is currently awaiting approval
				// (status 3). Freshservice exposes approver identity only on
				// the per-change /changes/{id}/approvals endpoint, which sync
				// does not currently traverse, so we cannot filter to "for
				// this specific agent". List every awaiting-approval change
				// and document the limitation in the JSON envelope.
				for _, c := range changes {
					st, _ := anyInt(c, "status")
					if st != 3 {
						continue
					}
					approvals = append(approvals, approvalRow{
						ChangeID: anyString(c, "id"),
						Subject:  anyString(c, "subject"),
						Status:   changeStatusName(st),
					})
				}
				if len(approvals) > 0 {
					approvalsNote = "showing all awaiting-approval changes; per-agent filtering requires hydrating /changes/{id}/approvals which sync does not currently fetch"
				}
			}

			payload := map[string]any{
				"agent":     agent,
				"tickets":   queue,
				"approvals": approvals,
				"counts": map[string]int{
					"tickets":   len(queue),
					"approvals": len(approvals),
				},
			}
			if approvalsNote != "" {
				payload["approvals_note"] = approvalsNote
			}
			if flags.asJSON {
				return flags.printJSON(cmd, payload)
			}
			if len(queue) == 0 && len(approvals) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Queue is empty.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Open tickets for %s: %d\n", agent, len(queue))
			tRows := make([][]string, 0, len(queue))
			for _, r := range queue {
				// PATCH(freshservice-myqueue-no-due-sort): tickets without an
				// SLA clock render MINUTES as "—" instead of "0".
				minutes := "—"
				if r.DueBy != "" {
					minutes = fmt.Sprintf("%.0f", r.MinutesTo)
				}
				tRows = append(tRows, []string{r.ID, r.Priority, r.Status, minutes, truncate(r.Subject, 60)})
			}
			if err := flags.printTable(cmd, []string{"ID", "PRIORITY", "STATUS", "MINUTES", "SUBJECT"}, tRows); err != nil {
				return err
			}
			if includeApprovals {
				fmt.Fprintf(cmd.OutOrStdout(), "\nPending change approvals: %d\n", len(approvals))
				aRows := make([][]string, 0, len(approvals))
				for _, a := range approvals {
					aRows = append(aRows, []string{a.ChangeID, a.Status, truncate(a.Subject, 60)})
				}
				return flags.printTable(cmd, []string{"CHANGE_ID", "STATUS", "SUBJECT"}, aRows)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent-id", "", "Agent identifier (numeric ID, name, or email)")
	cmd.Flags().BoolVar(&includeApprovals, "approvals", true, "Include pending change approvals")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// workload
// ---------------------------------------------------------------------------

func newWorkloadCmd(flags *rootFlags) *cobra.Command {
	var group string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Per-agent open-ticket distribution with load score",
		Long: `Aggregate open tickets across agents to expose workload imbalance.
Outputs per-agent: open count, average age in hours, P1+P2 count, and a
normalized load score (0-100 across the cohort).

Requires a prior 'freshservice-pp-cli sync' run.`,
		Example: strings.Trim(`
  # All agents, table view
  freshservice-pp-cli workload

  # Scope to a group, agent JSON
  freshservice-pp-cli workload --group Infrastructure --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := loadResources(db, "tickets")
			if err != nil {
				return fmt.Errorf("loading tickets: %w", err)
			}

			type agg struct {
				ResponderID int     `json:"responder_id"`
				Open        int     `json:"open_count"`
				HighPri     int     `json:"p1_p2_count"`
				AgeSumHours float64 `json:"-"`
				AvgAgeHours float64 `json:"avg_age_hours"`
				LoadScore   float64 `json:"load_score"`
			}
			byAgent := map[int]*agg{}
			now := time.Now()
			for _, t := range tickets {
				st, _ := anyInt(t, "status")
				if !isOpenTicket(st) {
					continue
				}
				gid, _ := anyInt(t, "group_id")
				if group != "" && fmt.Sprintf("%d", gid) != group && !strings.EqualFold(anyString(t, "group_name"), group) {
					continue
				}
				rid, _ := anyInt(t, "responder_id")
				if rid == 0 {
					continue
				}
				a, ok := byAgent[rid]
				if !ok {
					a = &agg{ResponderID: rid}
					byAgent[rid] = a
				}
				a.Open++
				prio, _ := anyInt(t, "priority")
				if prio >= 3 {
					a.HighPri++
				}
				if created, ok := anyTime(t, "created_at"); ok {
					a.AgeSumHours += now.Sub(created).Hours()
				}
			}

			rows := make([]*agg, 0, len(byAgent))
			maxLoad := 0.0
			for _, a := range byAgent {
				if a.Open > 0 {
					a.AvgAgeHours = a.AgeSumHours / float64(a.Open)
				}
				raw := float64(a.Open) + 2.0*float64(a.HighPri)
				if raw > maxLoad {
					maxLoad = raw
				}
				a.LoadScore = raw
				rows = append(rows, a)
			}
			if maxLoad > 0 {
				for _, a := range rows {
					a.LoadScore = 100.0 * a.LoadScore / maxLoad
				}
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].LoadScore > rows[j].LoadScore })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"agent_count": len(rows),
					"agents":      rows,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open tickets assigned to any agent in the local store.")
				return nil
			}
			headers := []string{"RESPONDER_ID", "OPEN", "P1_P2", "AVG_AGE_H", "LOAD"}
			tbl := make([][]string, 0, len(rows))
			for _, a := range rows {
				tbl = append(tbl, []string{
					strconv.Itoa(a.ResponderID),
					strconv.Itoa(a.Open),
					strconv.Itoa(a.HighPri),
					fmt.Sprintf("%.1f", a.AvgAgeHours),
					fmt.Sprintf("%.1f", a.LoadScore),
				})
			}
			return flags.printTable(cmd, headers, tbl)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Filter by group ID or name")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// change-collisions
// ---------------------------------------------------------------------------

func newChangeCollisionsCmd(flags *rootFlags) *cobra.Command {
	var windowStr string
	var ci string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "change-collisions",
		Short: "Find change records whose planned windows overlap",
		Long: `Detect overlapping planned maintenance windows across changes. Use
--window to widen the lookahead horizon, --ci to narrow to a configuration
item present in change descriptions.

The --window flag accepts standard Go duration units (s, m, h) plus shorthand
"d" (days) and "w" (weeks), e.g. 48h, 7d, 2w.

Requires a prior 'freshservice-pp-cli sync' run.`,
		Example: strings.Trim(`
  # All overlaps in the next 48 hours
  freshservice-pp-cli change-collisions --window 48h

  # Only changes mentioning prod-db-01
  freshservice-pp-cli change-collisions --window 7d --ci prod-db-01 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window, err := parsePeriod(windowStr)
			if err != nil {
				return fmt.Errorf("invalid --window %q: %w", windowStr, err)
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			changes, err := loadResources(db, "changes")
			if err != nil {
				return fmt.Errorf("loading changes: %w", err)
			}

			now := time.Now()
			end := now.Add(window)

			type chg struct {
				ID      string    `json:"id"`
				Subject string    `json:"subject"`
				Start   time.Time `json:"planned_start_date"`
				End     time.Time `json:"planned_end_date"`
			}
			var inWindow []chg
			for _, c := range changes {
				start, sok := anyTime(c, "planned_start_date")
				stop, eok := anyTime(c, "planned_end_date")
				if !sok || !eok {
					continue
				}
				if stop.Before(now) || start.After(end) {
					continue
				}
				if ci != "" {
					hay := strings.ToLower(anyString(c, "subject") + " " + anyString(c, "description") + " " + anyString(c, "description_text"))
					if !strings.Contains(hay, strings.ToLower(ci)) {
						continue
					}
				}
				inWindow = append(inWindow, chg{
					ID: anyString(c, "id"), Subject: anyString(c, "subject"),
					Start: start, End: stop,
				})
			}
			type collision struct {
				A          string `json:"change_a"`
				B          string `json:"change_b"`
				OverlapMin int    `json:"overlap_minutes"`
				SubjectA   string `json:"subject_a"`
				SubjectB   string `json:"subject_b"`
			}
			collisions := make([]collision, 0)
			for i := 0; i < len(inWindow); i++ {
				for j := i + 1; j < len(inWindow); j++ {
					a, b := inWindow[i], inWindow[j]
					start := a.Start
					if b.Start.After(start) {
						start = b.Start
					}
					stop := a.End
					if b.End.Before(stop) {
						stop = b.End
					}
					if stop.After(start) {
						collisions = append(collisions, collision{
							A: a.ID, B: b.ID,
							OverlapMin: int(stop.Sub(start).Minutes()),
							SubjectA:   a.Subject, SubjectB: b.Subject,
						})
					}
				}
			}
			sort.Slice(collisions, func(i, j int) bool { return collisions[i].OverlapMin > collisions[j].OverlapMin })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"window":     windowStr,
					"count":      len(collisions),
					"collisions": collisions,
				})
			}
			if len(collisions) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No collisions in the next %s.\n", windowStr)
				return nil
			}
			headers := []string{"CHANGE_A", "CHANGE_B", "OVERLAP_MIN", "SUBJECT_A → SUBJECT_B"}
			rows := make([][]string, 0, len(collisions))
			for _, c := range collisions {
				rows = append(rows, []string{
					c.A, c.B, strconv.Itoa(c.OverlapMin),
					truncate(c.SubjectA, 30) + " → " + truncate(c.SubjectB, 30),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&windowStr, "window", "48h", "Lookahead horizon (e.g., 48h, 7d, 2w)")
	cmd.Flags().StringVar(&ci, "ci", "", "Filter to changes mentioning this configuration item")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// recurrence
// ---------------------------------------------------------------------------

func newRecurrenceCmd(flags *rootFlags) *cobra.Command {
	var asset string
	var requester string
	var days int
	var minCount int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "recurrence",
		Short: "Surface repeated symptom patterns in historical tickets",
		Long: `Groups locally synced tickets by subject keyword signature to surface
problems that keep coming back. Optionally scope to a single asset or requester.

Requires a prior 'freshservice-pp-cli sync' run.`,
		Example: strings.Trim(`
  # All recurring patterns in the last 90 days
  freshservice-pp-cli recurrence --days 90

  # Pattern recurrence for a single asset
  freshservice-pp-cli recurrence --asset FS-2301 --days 90 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := loadResources(db, "tickets")
			if err != nil {
				return fmt.Errorf("loading tickets: %w", err)
			}
			cutoff := time.Now().AddDate(0, 0, -days)

			type pattern struct {
				Signature string   `json:"signature"`
				Count     int      `json:"count"`
				Tickets   []string `json:"ticket_ids"`
			}
			buckets := map[string]*pattern{}
			for _, t := range tickets {
				if created, ok := anyTime(t, "created_at"); ok && created.Before(cutoff) {
					continue
				}
				if asset != "" {
					if !strings.EqualFold(anyString(t, "display_id"), asset) && !strings.Contains(anyString(t, "subject"), asset) {
						hay := anyString(t, "description_text") + " " + anyString(t, "description")
						if !strings.Contains(strings.ToLower(hay), strings.ToLower(asset)) {
							continue
						}
					}
				}
				if requester != "" {
					rid, _ := anyInt(t, "requester_id")
					if fmt.Sprintf("%d", rid) != requester && !strings.EqualFold(anyString(t, "requester_email"), requester) {
						continue
					}
				}
				sig := signatureFor(anyString(t, "subject"))
				if sig == "" {
					continue
				}
				p, ok := buckets[sig]
				if !ok {
					p = &pattern{Signature: sig}
					buckets[sig] = p
				}
				p.Count++
				p.Tickets = append(p.Tickets, anyString(t, "id"))
			}
			patterns := make([]*pattern, 0)
			for _, p := range buckets {
				if p.Count >= minCount {
					patterns = append(patterns, p)
				}
			}
			sort.Slice(patterns, func(i, j int) bool { return patterns[i].Count > patterns[j].Count })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"window_days":   days,
					"pattern_count": len(patterns),
					"patterns":      patterns,
				})
			}
			if len(patterns) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recurring patterns above threshold.")
				return nil
			}
			headers := []string{"COUNT", "SIGNATURE", "TICKETS"}
			rows := make([][]string, 0, len(patterns))
			for _, p := range patterns {
				rows = append(rows, []string{strconv.Itoa(p.Count), p.Signature, strings.Join(p.Tickets, ",")})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&asset, "asset", "", "Filter to tickets referencing this asset display_id or name")
	cmd.Flags().StringVar(&requester, "requester", "", "Filter to a single requester ID or email")
	cmd.Flags().IntVar(&days, "days", 90, "How far back to look")
	cmd.Flags().IntVar(&minCount, "min-count", 2, "Minimum occurrence count to report")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

var noiseWords = map[string]bool{
	"a": true, "an": true, "and": true, "the": true, "for": true, "to": true,
	"of": true, "in": true, "on": true, "is": true, "with": true, "by": true,
	"re": true, "fwd": true, "issue": true, "problem": true, "error": true,
}

func signatureFor(subject string) string {
	subject = strings.ToLower(subject)
	subject = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			return r
		}
		return ' '
	}, subject)
	words := strings.Fields(subject)
	var keep []string
	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		if noiseWords[w] {
			continue
		}
		keep = append(keep, w)
		if len(keep) == 4 {
			break
		}
	}
	sort.Strings(keep)
	return strings.Join(keep, "-")
}

// ---------------------------------------------------------------------------
// kb-gaps
// ---------------------------------------------------------------------------

func newKBGapsCmd(flags *rootFlags) *cobra.Command {
	var group string
	var days int
	var minTickets int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "kb-gaps",
		Short: "Topic clusters in recent tickets with no matching KB article",
		Long: `For each recurring ticket-subject signature, check whether any solution
article in the local store mentions the signature words. Clusters with no
matching article and at least --min-tickets occurrences are returned.

Requires a prior 'freshservice-pp-cli sync' run with solutions enabled.`,
		Example: strings.Trim(`
  freshservice-pp-cli kb-gaps --days 30 --min-tickets 3

  freshservice-pp-cli kb-gaps --group SRE --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := loadResources(db, "tickets")
			if err != nil {
				return fmt.Errorf("loading tickets: %w", err)
			}
			solutions, _ := loadResources(db, "solutions")

			cutoff := time.Now().AddDate(0, 0, -days)
			buckets := map[string]int{}
			ticketsBySig := map[string][]string{}
			for _, t := range tickets {
				if created, ok := anyTime(t, "created_at"); ok && created.Before(cutoff) {
					continue
				}
				if group != "" {
					gid, _ := anyInt(t, "group_id")
					if fmt.Sprintf("%d", gid) != group && !strings.EqualFold(anyString(t, "group_name"), group) {
						continue
					}
				}
				sig := signatureFor(anyString(t, "subject"))
				if sig == "" {
					continue
				}
				buckets[sig]++
				ticketsBySig[sig] = append(ticketsBySig[sig], anyString(t, "id"))
			}

			kbCorpus := strings.Builder{}
			for _, s := range solutions {
				kbCorpus.WriteString(strings.ToLower(anyString(s, "name") + " " + anyString(s, "description")))
				kbCorpus.WriteByte(' ')
			}
			corpus := kbCorpus.String()

			type gap struct {
				Signature string   `json:"signature"`
				Count     int      `json:"count"`
				Tickets   []string `json:"ticket_ids"`
			}
			gaps := make([]gap, 0)
			for sig, count := range buckets {
				if count < minTickets {
					continue
				}
				words := strings.Split(sig, "-")
				covered := true
				for _, w := range words {
					if !strings.Contains(corpus, w) {
						covered = false
						break
					}
				}
				if covered {
					continue
				}
				gaps = append(gaps, gap{Signature: sig, Count: count, Tickets: ticketsBySig[sig]})
			}
			sort.Slice(gaps, func(i, j int) bool { return gaps[i].Count > gaps[j].Count })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"window_days": days,
					"min_tickets": minTickets,
					"count":       len(gaps),
					"gaps":        gaps,
				})
			}
			if len(gaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No knowledge-base gaps detected.")
				return nil
			}
			headers := []string{"COUNT", "SIGNATURE", "TICKETS"}
			rows := make([][]string, 0, len(gaps))
			for _, g := range gaps {
				rows = append(rows, []string{strconv.Itoa(g.Count), g.Signature, strings.Join(g.Tickets, ",")})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Filter by group ID or name")
	cmd.Flags().IntVar(&days, "days", 30, "How far back to look")
	cmd.Flags().IntVar(&minTickets, "min-tickets", 3, "Minimum tickets per signature to report")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// orphan-assets
// ---------------------------------------------------------------------------

func newOrphanAssetsCmd(flags *rootFlags) *cobra.Command {
	var assetType string
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "orphan-assets",
		Short: "Assets with no recent ticket activity and no active contract",
		Long: `Cross-references the local assets, tickets, and contracts corpora to
identify assets that have no associated tickets in the last N days and no
active contract linked. Surfaces hardware you may be paying for but nobody uses.

Requires a prior 'freshservice-pp-cli sync' run that covered assets, tickets,
and contracts.`,
		Example: strings.Trim(`
  freshservice-pp-cli orphan-assets --days 60

  freshservice-pp-cli orphan-assets --type laptop --days 90 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			assets, err := loadResources(db, "assets")
			if err != nil {
				return fmt.Errorf("loading assets: %w", err)
			}
			tickets, _ := loadResources(db, "tickets")
			contracts, _ := loadResources(db, "contracts")

			cutoff := time.Now().AddDate(0, 0, -days)
			recentAssetTouch := map[string]bool{}
			for _, t := range tickets {
				if updated, ok := anyTime(t, "updated_at", "created_at"); ok && updated.Before(cutoff) {
					continue
				}
				// Freshservice tickets reference assets via an "assets" array
				// of {display_id, ...} entries (not a flat asset_id field).
				// Iterate the array if present; fall back to legacy scalar
				// fields for other shapes.
				for _, did := range extractAssetDisplayIDs(t) {
					if did != "" {
						recentAssetTouch[did] = true
					}
				}
			}
			activeContractAsset := map[string]bool{}
			for _, c := range contracts {
				status := anyString(c, "status", "contract_status")
				if status != "" && !strings.EqualFold(status, "active") && !strings.EqualFold(status, "approved") {
					continue
				}
				// Contracts can attach to assets through either an explicit
				// asset_id field or a nested asset_associations / assets list.
				for _, did := range extractAssetDisplayIDs(c) {
					if did != "" {
						activeContractAsset[did] = true
					}
				}
			}

			type row struct {
				DisplayID   string `json:"display_id"`
				Name        string `json:"name"`
				AssetTypeID int    `json:"asset_type_id,omitempty"`
				Department  int    `json:"department_id,omitempty"`
				UpdatedAt   string `json:"updated_at,omitempty"`
			}
			orphans := make([]row, 0)
			for _, a := range assets {
				if assetType != "" {
					at := anyString(a, "asset_type_name")
					tid, _ := anyInt(a, "asset_type_id")
					if !strings.EqualFold(at, assetType) && fmt.Sprintf("%d", tid) != assetType {
						continue
					}
				}
				disp := anyString(a, "display_id", "id")
				if recentAssetTouch[disp] || activeContractAsset[disp] {
					continue
				}
				atid, _ := anyInt(a, "asset_type_id")
				dept, _ := anyInt(a, "department_id")
				upd := ""
				if t, ok := anyTime(a, "updated_at"); ok {
					upd = t.Format(time.RFC3339)
				}
				orphans = append(orphans, row{
					DisplayID:   disp,
					Name:        anyString(a, "name"),
					AssetTypeID: atid,
					Department:  dept,
					UpdatedAt:   upd,
				})
			}
			sort.Slice(orphans, func(i, j int) bool { return orphans[i].DisplayID < orphans[j].DisplayID })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"window_days": days,
					"count":       len(orphans),
					"orphans":     orphans,
				})
			}
			if len(orphans) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No orphan assets detected.")
				return nil
			}
			headers := []string{"DISPLAY_ID", "TYPE_ID", "DEPT_ID", "UPDATED_AT", "NAME"}
			rows := make([][]string, 0, len(orphans))
			for _, o := range orphans {
				rows = append(rows, []string{o.DisplayID, strconv.Itoa(o.AssetTypeID), strconv.Itoa(o.Department), o.UpdatedAt, truncate(o.Name, 40)})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&assetType, "type", "", "Filter by asset type name or asset_type_id")
	cmd.Flags().IntVar(&days, "days", 60, "Days without ticket activity to consider an asset orphaned")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// dept-sla
// ---------------------------------------------------------------------------

func newDeptSLACmd(flags *rootFlags) *cobra.Command {
	var period string
	var sortBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dept-sla",
		Short: "SLA compliance, breach count, and mean-time-to-resolve by department",
		Long: `Aggregates ticket SLA outcomes grouped by requester's department over the
specified rolling window. Compliance is computed as (1 - breaches/resolved).

Requires a prior 'freshservice-pp-cli sync' run with tickets, requesters,
and departments.`,
		Example: strings.Trim(`
  freshservice-pp-cli dept-sla --period 30

  freshservice-pp-cli dept-sla --period 90 --sort breach-rate --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := loadResources(db, "tickets")
			if err != nil {
				return fmt.Errorf("loading tickets: %w", err)
			}
			requesters, _ := loadResources(db, "requesters")
			departments, _ := loadResources(db, "departments")

			reqDept := map[int]int{}
			for _, r := range requesters {
				rid, _ := anyInt(r, "id")
				dept, _ := anyInt(r, "department_id")
				if rid > 0 {
					reqDept[rid] = dept
				}
			}
			deptName := map[int]string{}
			for _, d := range departments {
				did, _ := anyInt(d, "id")
				deptName[did] = anyString(d, "name")
			}
			d, err := parsePeriod(period)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-d)
			periodDays := int(d.Hours() / 24)

			type agg struct {
				DepartmentID int     `json:"department_id"`
				Name         string  `json:"department_name"`
				Resolved     int     `json:"resolved_count"`
				Breaches     int     `json:"breach_count"`
				BreachRate   float64 `json:"breach_rate"`
				MeanResolveH float64 `json:"mean_resolve_hours"`
				totalHours   float64
			}
			byDept := map[int]*agg{}
			for _, t := range tickets {
				st, _ := anyInt(t, "status")
				if st != 4 && st != 5 {
					continue
				}
				// Freshservice writes the actual resolved/closed timestamp into
				// stats.resolved_at / stats.closed_at on ticket detail responses.
				// updated_at is a poor substitute — any post-resolution comment
				// bumps it and skews mean-time-to-resolve.
				closed, ok := ticketResolvedAt(t)
				if !ok {
					closed, ok = anyTime(t, "updated_at")
				}
				if !ok || closed.Before(cutoff) {
					continue
				}
				rid, _ := anyInt(t, "requester_id")
				dept := reqDept[rid]
				a, ok := byDept[dept]
				if !ok {
					a = &agg{DepartmentID: dept, Name: deptName[dept]}
					if a.Name == "" {
						a.Name = "unknown"
					}
					byDept[dept] = a
				}
				a.Resolved++
				if created, ok := anyTime(t, "created_at"); ok {
					a.totalHours += closed.Sub(created).Hours()
				}
				// Breach detection: compare actual resolved time to due_by. We
				// previously checked a top-level `sla_breached` boolean, but
				// that field is not part of the documented Freshservice ticket
				// schema; relying on it produced silent zero-breach reports.
				if due, ok := anyTime(t, "due_by"); ok && closed.After(due) {
					a.Breaches++
				}
			}
			results := make([]*agg, 0, len(byDept))
			for _, a := range byDept {
				if a.Resolved > 0 {
					a.BreachRate = float64(a.Breaches) / float64(a.Resolved)
					a.MeanResolveH = a.totalHours / float64(a.Resolved)
				}
				results = append(results, a)
			}
			switch sortBy {
			case "breach-rate":
				sort.Slice(results, func(i, j int) bool { return results[i].BreachRate > results[j].BreachRate })
			case "resolve-time":
				sort.Slice(results, func(i, j int) bool { return results[i].MeanResolveH > results[j].MeanResolveH })
			case "resolved":
				sort.Slice(results, func(i, j int) bool { return results[i].Resolved > results[j].Resolved })
			default:
				sort.Slice(results, func(i, j int) bool { return results[i].BreachRate > results[j].BreachRate })
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"period_days":      periodDays,
					"department_count": len(results),
					"departments":      results,
				})
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No resolved tickets in the requested window.")
				return nil
			}
			headers := []string{"DEPT_ID", "DEPARTMENT", "RESOLVED", "BREACHES", "BREACH_RATE", "MEAN_RESOLVE_H"}
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				rows = append(rows, []string{
					strconv.Itoa(r.DepartmentID), truncate(r.Name, 30),
					strconv.Itoa(r.Resolved), strconv.Itoa(r.Breaches),
					fmt.Sprintf("%.1f%%", r.BreachRate*100),
					fmt.Sprintf("%.1f", r.MeanResolveH),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&period, "period", "30d", "Rolling window (e.g., 24h, 30d, 4w)")
	cmd.Flags().StringVar(&sortBy, "sort", "breach-rate", "Sort by breach-rate, resolve-time, or resolved")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// oncall-gap
// ---------------------------------------------------------------------------

func newOnCallGapCmd(flags *rootFlags) *cobra.Command {
	var group string
	var period string
	var severity string
	var ackMinutes int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "oncall-gap",
		Short: "High-severity tickets that arrived without a timely first response",
		Long: `Scans recent high-severity tickets and reports those whose first response
exceeded --ack-minutes — surfacing on-call rotation coverage gaps.

Requires a prior 'freshservice-pp-cli sync' run.`,
		Example: strings.Trim(`
  freshservice-pp-cli oncall-gap --period 4w --severity P1,P2

  freshservice-pp-cli oncall-gap --group SRE --ack-minutes 30 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := loadResources(db, "tickets")
			if err != nil {
				return fmt.Errorf("loading tickets: %w", err)
			}
			meta, _ := LoadTenantMeta(db)
			cutoff := time.Now()
			if d, err := parsePeriod(period); err == nil {
				cutoff = cutoff.Add(-d)
			} else {
				return usageErr(err)
			}

			sevLevels := map[int]bool{}
			for _, s := range strings.Split(severity, ",") {
				s = strings.TrimSpace(strings.ToUpper(s))
				switch s {
				case "P1", "URGENT":
					sevLevels[4] = true
				case "P2", "HIGH":
					sevLevels[3] = true
				case "P3", "MEDIUM":
					sevLevels[2] = true
				case "P4", "LOW":
					sevLevels[1] = true
				}
			}

			type row struct {
				ID         string  `json:"id"`
				Priority   string  `json:"priority"`
				CreatedAt  string  `json:"created_at"`
				AckMinutes float64 `json:"ack_minutes,omitempty"`
				Subject    string  `json:"subject"`
			}
			gaps := make([]row, 0)
			for _, t := range tickets {
				created, ok := anyTime(t, "created_at")
				if !ok || created.Before(cutoff) {
					continue
				}
				prio, _ := anyInt(t, "priority")
				if !sevLevels[prio] {
					continue
				}
				if group != "" {
					gid, _ := anyInt(t, "group_id")
					if fmt.Sprintf("%d", gid) != group && !strings.EqualFold(anyString(t, "group_name"), group) {
						continue
					}
				}
				// PATCH(freshservice-oncall-gap-frt-fallback-removed): drop the
				// `frt_escalated_at` fallback. That field marks when the
				// First Response Time *SLA escalation timer fired* — i.e.,
				// the SLA was breached — not when an agent actually responded.
				// Using it as a fallback let tickets that escalated without
				// ever being responded to look as if they had been acked at
				// escalation time, silently excluding them from the gap report
				// whenever escalation fell within --ack-minutes.
				firstResp, hasResp := anyTime(t, "first_responded_at")
				var ackMin float64
				if hasResp {
					ackMin = firstResp.Sub(created).Minutes()
					if ackMin <= float64(ackMinutes) {
						continue
					}
				} else {
					// no first response yet; treat as effectively past ack
					ackMin = time.Since(created).Minutes()
					if ackMin <= float64(ackMinutes) {
						continue
					}
				}
				gaps = append(gaps, row{
					ID:         anyString(t, "id"),
					Priority:   meta.PriorityLabel(prio),
					CreatedAt:  created.Format(time.RFC3339),
					AckMinutes: ackMin,
					Subject:    anyString(t, "subject"),
				})
			}
			sort.Slice(gaps, func(i, j int) bool { return gaps[i].AckMinutes > gaps[j].AckMinutes })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"period":      period,
					"severity":    severity,
					"ack_minutes": ackMinutes,
					"count":       len(gaps),
					"gaps":        gaps,
				})
			}
			if len(gaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No on-call gaps detected.")
				return nil
			}
			headers := []string{"ID", "PRIORITY", "CREATED_AT", "ACK_MIN", "SUBJECT"}
			rows := make([][]string, 0, len(gaps))
			for _, g := range gaps {
				rows = append(rows, []string{g.ID, g.Priority, g.CreatedAt, fmt.Sprintf("%.0f", g.AckMinutes), truncate(g.Subject, 50)})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "Filter by group ID or name")
	cmd.Flags().StringVar(&period, "period", "4w", "Rolling window (e.g., 24h, 7d, 4w)")
	cmd.Flags().StringVar(&severity, "severity", "P1,P2", "Comma-separated severities to include (P1=urgent, P2=high)")
	cmd.Flags().IntVar(&ackMinutes, "ack-minutes", 15, "Acknowledge threshold in minutes")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func parsePeriod(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty period")
	}
	if strings.HasSuffix(s, "w") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, fmt.Errorf("invalid period %q", s)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid period %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
