// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// PATCH(novel1219-hours-audit): ported from the public library entry (Greptile PR #2450 round 2 P1).
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"github.com/spf13/cobra"
)

type hoursAuditStep struct {
	AutomationID   string `json:"automation_id"`
	AutomationName string `json:"automation_name"`
	IsActive       bool   `json:"is_active"`
	NodeID         string `json:"node_id"`
	ActionType     string `json:"action_type"`
	HasWindow      bool   `json:"has_window"`
	Window         string `json:"window,omitempty"`
}
type hoursAuditSummary struct {
	Automations   int `json:"automations"`
	Steps         int `json:"steps"`
	WithWindow    int `json:"with_window"`
	WithoutWindow int `json:"without_window"`
	// Unpublished counts automations that --published-only excluded because they
	// have no published audited actions; they still count toward Automations.
	Unpublished int `json:"unpublished,omitempty"`
}
type hoursAuditView struct {
	Steps    []hoursAuditStep  `json:"steps"`
	Summary  hoursAuditSummary `json:"summary"`
	Synced   bool              `json:"synced"`
	Hint     string            `json:"hint,omitempty"`
	Partial  bool              `json:"partial"`
	Checked  int               `json:"checked"`
	Total    *int              `json:"total,omitempty"`
	Failures []string          `json:"failures,omitempty"`
}
type automationAuditDoc struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	IsActive         bool   `json:"isActive"`
	GraphVersion     int    `json:"graphVersion"`
	PublishedActions struct {
		Nodes []automationAuditNode `json:"nodes"`
	} `json:"publishedActions"`
	Actions struct {
		Nodes []automationAuditNode `json:"nodes"`
	} `json:"actions"`
}
type automationAuditNode struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Action struct {
		Type   string `json:"type"`
		Config struct {
			RunWindow *automationRunWindow `json:"runWindow"`
		} `json:"config"`
	} `json:"action"`
}
type automationRunWindow struct {
	Days      []string `json:"days"`
	StartHour any      `json:"startHour"`
	EndHour   any      `json:"endHour"`
}

type hoursAuditRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

var queryHoursAuditRows = func(db *store.Store) (hoursAuditRows, error) {
	return db.Query(`SELECT id, data FROM resources WHERE resource_type = 'automations' ORDER BY id`)
}

// Keep sql imported in the production signature behind the test seam above.
var _ hoursAuditRows = (*sql.Rows)(nil)

func newNovelAutomationsHoursAuditCmd(flags *rootFlags) *cobra.Command {
	var publishedOnly bool
	var dbPath string
	cmd := &cobra.Command{
		Use: "hours-audit", Short: "Every Send SMS, Send Email and Assign step across synced automations, listed with or without an operating-hours window.",
		Long:        "Audit operating-hours windows on Send SMS, Send Email and Assign steps.\n\nWith --published-only, summary.unpublished counts automations with no published audited actions. summary.automations equals the distinct automations represented by rows plus summary.unpublished, and summary.steps equals the number of rows.",
		Example:     "  conduyt-crm-pp-cli automations hours-audit --published-only --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--json=true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "automations hours-audit")
			}
			if len(args) == 0 && !novelInvocationHasFlags(cmd, flags) {
				return cmd.Help()
			}
			view := hoursAuditView{Steps: make([]hoursAuditStep, 0)}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if flags.dataSource != "local" {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				// PATCH(novel1219-hours-audit): the reprint's paginatedGet reports truncation on the context instead of
				// the public entry's typed error, so the pages are walked here with the shared progress guard.
				items, reported, pageFailures, err := fetchHoursAuditAutomations(ctx, c)
				if err == nil {
					flags.agentSource = "live"
					view.Synced = true
					total := len(items)
					if reported != nil {
						total = *reported
					}
					view.Total = &total
					for i, item := range items {
						encoded, marshalErr := json.Marshal(item)
						if marshalErr != nil {
							return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("decoding automation %d: %w", i+1, marshalErr))
						}
						var doc automationAuditDoc
						if decodeErr := json.Unmarshal(encoded, &doc); decodeErr != nil {
							return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("decoding automation %d: %w", i+1, decodeErr))
						}
						view.Checked++
						if appendErr := appendHoursAuditDoc(&view, doc, publishedOnly); appendErr != nil {
							return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("decoding automation %s run window: %w", doc.ID, appendErr))
						}
					}
					if len(pageFailures) > 0 {
						view.Partial = true
						view.Failures = append(view.Failures, pageFailures...)
					}
					if view.Checked != total {
						view.Partial = true
						view.Failures = append(view.Failures, fmt.Sprintf("checked %d of %d automations", view.Checked, total))
					}
					if view.Partial {
						return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("live automation audit is incomplete: %s", strings.Join(view.Failures, "; ")))
					}
					return outputHoursAudit(cmd, flags, view)
				}
				if flags.dataSource == "live" || !isNetworkError(err) {
					return outputHoursAuditFailure(cmd, flags, view, err)
				}
			}
			path := dbPath
			if path == "" {
				path = defaultDBPath("conduyt-crm-pp-cli")
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: conduyt-crm-pp-cli sync --resources automations --db %s\n", path, path)
				view.Hint = "run: conduyt-crm-pp-cli sync --resources automations --db " + path
				return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("local automations mirror is missing"))
			}
			db, err := store.OpenWithContext(ctx, path)
			if err != nil {
				return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("opening local mirror: %w", err))
			}
			// PATCH(novel1219-hours-audit): sync state, freshness and attempt completion come from one read through
			// the shared mirror reader, as for the contacts mirror (novel1219 r3-r5).
			state, err := readResourceSyncState(ctx, newMirrorReader(db.DB()), "automations")
			if err != nil {
				return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("checking automations sync state: %w", errorsWithClose(err, db.Close())))
			}
			syncedCount := state.Total
			view.Synced = !state.SyncedAt.IsZero()
			if !view.Synced {
				view.Hint = "run: conduyt-crm-pp-cli sync --resources automations --db " + path
				closeErr := db.Close()
				cause := errorsWithClose(fmt.Errorf("local automations mirror has not been synced"), closeErr)
				return outputHoursAuditFailure(cmd, flags, view, cause)
			}
			view.Total = &syncedCount
			if !state.AttemptComplete {
				view.Partial = true
				view.Failures = append(view.Failures, "the last automations sync attempt did not finish")
			}
			if resourceMirrorOlderThan(cmd, "automations", state.SyncedAt, flags.maxAge) {
				view.Partial = true
				view.Failures = append(view.Failures, "local automations mirror is older than --max-age")
			}
			rows, err := queryHoursAuditRows(db)
			if err != nil {
				return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("reading automations: %w", errorsWithClose(err, db.Close())))
			}
			for rows.Next() {
				var id string
				var raw string
				if err := rows.Scan(&id, &raw); err != nil {
					return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("scanning automation: %w", errorsWithClose(err, rows.Close(), db.Close())))
				}
				var doc automationAuditDoc
				var shape map[string]any
				if err := json.Unmarshal([]byte(raw), &shape); err != nil {
					return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("decoding automation %s: %w", id, errorsWithClose(err, rows.Close(), db.Close())))
				}
				if _, bad := shape["error"]; bad || shape["message"] != nil || shape["data"] != nil {
					cause := errorsWithClose(fmt.Errorf("decoding automation %s: unexpected object shape", id), rows.Close(), db.Close())
					return outputHoursAuditFailure(cmd, flags, view, cause)
				}
				if err := json.Unmarshal([]byte(raw), &doc); err != nil {
					return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("decoding automation %s: %w", id, errorsWithClose(err, rows.Close(), db.Close())))
				}
				view.Checked++
				if err := appendHoursAuditDoc(&view, doc, publishedOnly); err != nil {
					return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("decoding automation %s run window: %w", id, errorsWithClose(err, rows.Close(), db.Close())))
				}
			}
			if err := rows.Err(); err != nil {
				return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("reading automations: %w", errorsWithClose(err, rows.Close(), db.Close())))
			}
			if err := rows.Close(); err != nil {
				return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("closing automation rows: %w", errorsWithClose(err, db.Close())))
			}
			if view.Total != nil && view.Checked != *view.Total {
				view.Partial = true
				view.Failures = append(view.Failures, fmt.Sprintf("local automations mirror contains %d decoded rows but sync state reports total %d", view.Checked, *view.Total))
			}
			if err := db.Close(); err != nil {
				return outputHoursAuditFailure(cmd, flags, view, fmt.Errorf("closing local mirror: %w", err))
			}
			if err := outputHoursAudit(cmd, flags, view); err != nil {
				return err
			}
			if view.Partial {
				return apiErr(fmt.Errorf("hours audit is incomplete: %s", strings.Join(view.Failures, "; ")))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&publishedOnly, "published-only", false, "Inspect published automation actions only")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}

func appendHoursAuditDoc(view *hoursAuditView, doc automationAuditDoc, publishedOnly bool) error {
	view.Summary.Automations++
	nodes := make([]automationAuditNode, 0)
	if doc.GraphVersion == 2 || len(doc.PublishedActions.Nodes) > 0 {
		nodes = append(nodes, doc.PublishedActions.Nodes...)
	}
	if !publishedOnly {
		nodes = append(nodes, doc.Actions.Nodes...)
	}
	auditedPublishedActions := 0
	if publishedOnly {
		for _, node := range nodes {
			if node.Type == "action" && auditedAction(node.Action.Type) {
				auditedPublishedActions++
			}
		}
		if auditedPublishedActions == 0 {
			view.Summary.Unpublished++
		}
	}
	for _, node := range nodes {
		if node.Type != "action" || !auditedAction(node.Action.Type) {
			continue
		}
		step := hoursAuditStep{AutomationID: doc.ID, AutomationName: doc.Name, IsActive: doc.IsActive, NodeID: node.ID, ActionType: node.Action.Type}
		if node.Action.Config.RunWindow != nil {
			step.HasWindow = true
			window, err := formatRunWindow(*node.Action.Config.RunWindow)
			if err != nil {
				return err
			}
			step.Window = window
			view.Summary.WithWindow++
		} else {
			view.Summary.WithoutWindow++
		}
		view.Steps = append(view.Steps, step)
	}
	view.Summary.Steps = len(view.Steps)
	return nil
}

func errorsWithClose(primary error, closeErrs ...error) error {
	for _, closeErr := range closeErrs {
		if closeErr != nil {
			if primary == nil {
				primary = closeErr
			} else {
				primary = fmt.Errorf("%v; closing resource: %w", primary, closeErr)
			}
		}
	}
	return primary
}

func auditedAction(v string) bool {
	return v == "send_sms" || v == "send_email" || v == "assign_to_user"
}
func formatRunWindow(w automationRunWindow) (string, error) {
	start, err := formatHour(w.StartHour)
	if err != nil {
		return "", fmt.Errorf("startHour: %w", err)
	}
	end, err := formatHour(w.EndHour)
	if err != nil {
		return "", fmt.Errorf("endHour: %w", err)
	}
	return formatDays(w.Days) + " · " + start + "–" + end, nil
}
func formatDays(days []string) string {
	if len(days) == 5 && strings.EqualFold(days[0], "Mon") && strings.EqualFold(days[4], "Fri") {
		return "Mon–Fri"
	}
	return strings.Join(days, ", ")
}
func formatHour(v any) (string, error) {
	var h float64
	switch x := v.(type) {
	case float64:
		h = x
	case string:
		parsed, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return "", err
		}
		h = parsed
	default:
		return "", fmt.Errorf("unsupported hour value %T", v)
	}
	hour := int(h)
	minute := int((h-float64(hour))*60 + 0.5)
	suffix := "AM"
	if hour >= 12 {
		suffix = "PM"
	}
	shown := hour % 12
	if shown == 0 {
		shown = 12
	}
	return fmt.Sprintf("%d:%02d %s", shown, minute, suffix), nil
}
func outputHoursAudit(cmd *cobra.Command, flags *rootFlags, view hoursAuditView) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if view.Partial {
		fmt.Fprintln(cmd.OutOrStdout(), "WARNING: hours audit is incomplete: "+strings.Join(view.Failures, "; "))
	}
	current := ""
	for _, step := range view.Steps {
		if step.AutomationName != current {
			current = step.AutomationName
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", current)
		}
		window := "no window"
		if step.HasWindow {
			window = step.Window
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", step.NodeID, step.ActionType, window)
	}
	return nil
}

func outputHoursAuditFailure(cmd *cobra.Command, flags *rootFlags, view hoursAuditView, cause error) error {
	view.Partial = true
	view.Failures = append(view.Failures, cause.Error())
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "WARNING: hours audit is incomplete: %v\n", cause)
	}
	return classifyAPIErrorOnly(cause)
}

// fetchHoursAuditAutomations reads every page of GET /automations (per_page 50). A first-page fetch or any decode
// failure is an error (auto falls back to the mirror only on a network error); a later page that fails, a stalled page
// sequence, the page cap or a total that changes between pages is a failure that makes the audit partial. PATCH(novel1219-hours-audit)
func fetchHoursAuditAutomations(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}) ([]map[string]any, *int, []string, error) {
	const perPage = 50
	items := make([]map[string]any, 0)
	var reported *int
	var totalPresent, totalValid bool
	var failures []string
	progress := newPaginationProgressGuard()
	seen := make(map[string]struct{})
	for page := 1; ; page++ {
		raw, err := c.Get(ctx, "/automations", map[string]string{"page": strconv.Itoa(page), "per_page": strconv.Itoa(perPage)})
		if err != nil {
			if page > 1 { // as in the public entry: a later page that fails leaves a partial live answer
				failures = append(failures, fmt.Sprintf("fetching automations page %d: %v", page, err))
				break
			}
			return nil, nil, nil, fmt.Errorf("fetching automations: %w", err)
		}
		if err := rejectResponseErrorEnvelope(raw); err != nil {
			return nil, nil, nil, fmt.Errorf("decoding automations page %d: %w", page, err)
		}
		pageItems, err := objectItems(raw, false)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("decoding automations page %d: %w", page, err)
		}
		// As in the public entry's paginatedGet: the first page fixes the total's state (missing, invalid or a value)
		// and any later page whose state differs, a total that appears, disappears, turns malformed or changes, is
		// truncation. PATCH(novel1219-hours-audit) r15
		n, present, valid := hoursAuditPageTotal(raw)
		if page == 1 {
			totalPresent, totalValid = present, valid
			if present && valid {
				first := n
				reported = &first
			}
		} else if present != totalPresent || valid != totalValid || (present && valid && reported != nil && n != *reported) {
			prior := 0
			if reported != nil {
				prior = *reported
			}
			failures = append(failures, fmt.Sprintf("response metadata total changed from %s to %s", formatHoursAuditTotal(prior, totalPresent, totalValid), formatHoursAuditTotal(n, present, valid)))
			break
		}
		repeated := ""
		for _, id := range objectPaginationIDs(pageItems) {
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				repeated = id
				break
			}
			seen[id] = struct{}{}
		}
		if repeated != "" { // a repeated automation means the pages overlap: the live answer is not a clean set
			failures = append(failures, fmt.Sprintf("automations page %d repeated automation %s", page, repeated))
			break
		}
		more := len(pageItems) == perPage && (reported == nil || len(items)+len(pageItems) < *reported)
		if err := progress.observe(objectPaginationIDs(pageItems), "", more); err != nil {
			failures = append(failures, err.Error())
			break
		}
		items = append(items, pageItems...)
		if !more {
			break
		}
	}
	return items, reported, failures, nil
}

// hoursAuditPageTotal reads a page's total the way the public entry's reportedCollectionTotal does: total,
// total_count, totalCount, TotalCount, totalResults or Total at the root, or inside meta, pagination, paging or
// metadata (two levels at most), after hoisting a {data:{data,meta}} envelope. present reports the key exists; valid
// that it is a non-negative integer. PATCH(novel1219-hours-audit) r15
func hoursAuditPageTotal(raw json.RawMessage) (n int, present, valid bool) {
	if hoisted, ok := hoistPaginatedEnvelope(raw); ok {
		raw = hoisted
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return 0, false, false
	}
	return hoursAuditTotalAtDepth(obj, 0)
}

func hoursAuditTotalAtDepth(obj map[string]json.RawMessage, depth int) (int, bool, bool) {
	if obj == nil || depth > 2 {
		return 0, false, false
	}
	for _, key := range []string{"total", "total_count", "totalCount", "TotalCount", "totalResults", "Total"} {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		var n json.Number
		if json.Unmarshal(raw, &n) != nil {
			return 0, true, false
		}
		if v, err := n.Int64(); err == nil && v >= 0 {
			return int(v), true, true
		}
		return 0, true, false
	}
	for _, wrap := range []string{"meta", "pagination", "paging", "metadata"} {
		raw, ok := obj[wrap]
		if !ok {
			continue
		}
		var inner map[string]json.RawMessage
		if json.Unmarshal(raw, &inner) != nil {
			continue
		}
		if n, present, valid := hoursAuditTotalAtDepth(inner, depth+1); present {
			return n, present, valid
		}
	}
	return 0, false, false
}

func formatHoursAuditTotal(total int, present, valid bool) string {
	if !present {
		return "missing"
	}
	if !valid {
		return "invalid"
	}
	return strconv.Itoa(total)
}
