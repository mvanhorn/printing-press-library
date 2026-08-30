// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelScheduleAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath, dueWithin string
	cmd := &cobra.Command{
		Use: "audit", Short: "Find scheduled drafts that are overdue, orphaned, duplicated, or missing review state.",
		Example:     "  agentmail-pp-cli schedule audit --db /tmp/agentmail.db --due-within 24h --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "schedule audit")
			}
			var horizon time.Time
			if strings.TrimSpace(dueWithin) != "" {
				d, err := time.ParseDuration(dueWithin)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --due-within %q: %w", dueWithin, err))
				}
				horizon = time.Now().UTC().Add(d)
			}
			db, err := openWorkflowStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return workflowOutput(cmd.OutOrStdout(), map[string]any{"items": []map[string]any{}, "count": 0, "scanned": 0, "reason": "local mirror not found; run sync first"}, flags, nil)
			}
			defer closeWorkflowStore(db)
			hintIfUnsynced(cmd, db, "drafts")
			hintIfStale(cmd, db, "drafts", 24*time.Hour)
			resources, err := loadWorkflowResources(cmd.Context(), db)
			if err != nil {
				return err
			}
			threads, inboxes := map[string]bool{}, map[string]bool{}
			draftsByID := map[string]workflowResource{}
			for _, r := range resources {
				id := wfID(r)
				if workflowFamily(r, "threads") {
					threads[id] = true
				}
				if workflowFamily(r, "inboxes") {
					inboxes[id] = true
				}
				if workflowFamily(r, "drafts") && wfString(r.Data, "send_at", "scheduled_at", "schedule_at") != "" {
					if key := wfID(r); key != "" {
						if current, ok := draftsByID[key]; !ok || len(current.Data) < len(r.Data) {
							draftsByID[key] = r
						}
					}
				}
			}
			drafts := make([]workflowResource, 0, len(draftsByID))
			for _, draft := range draftsByID {
				drafts = append(drafts, draft)
			}
			duplicate := map[string]int{}
			for _, d := range drafts {
				tid := workflowDraftThread(d, resources)
				key := tid
				if key == "" {
					key = workflowInbox(d)
				}
				duplicate[strings.ToLower(key+"|"+wfString(d.Data, "subject", "title"))]++
			}
			items := []map[string]any{}
			now := time.Now().UTC()
			for _, d := range drafts {
				rawSchedule := wfString(d.Data, "send_at", "scheduled_at", "schedule_at")
				at, scheduleErr := parseWorkflowTimeStrict(rawSchedule)
				if !horizon.IsZero() && scheduleErr == nil && at.After(horizon) {
					continue
				}
				issues := []string{}
				if scheduleErr != nil {
					issues = append(issues, "malformed_schedule")
				} else if at.Before(now) {
					issues = append(issues, "overdue")
				}
				tid := workflowDraftThread(d, resources)
				if tid != "" && !threads[tid] {
					issues = append(issues, "orphaned_thread")
				}
				iid := workflowInbox(d)
				if iid != "" && !inboxes[iid] {
					issues = append(issues, "orphaned_inbox")
				}
				key := tid
				if key == "" {
					key = iid
				}
				key = strings.ToLower(key + "|" + wfString(d.Data, "subject", "title"))
				if duplicate[key] > 1 {
					issues = append(issues, "duplicate")
				}
				review := strings.ToLower(wfString(d.Data, "review_state", "review_status", "approval_status"))
				if review == "" || review == "pending" || review == "unreviewed" {
					issues = append(issues, "unreviewed")
				}
				if len(issues) == 0 {
					continue
				}
				scheduledAt := rawSchedule
				if scheduleErr == nil {
					scheduledAt = at.UTC().Format(time.RFC3339)
				}
				items = append(items, map[string]any{"draft_id": wfID(d), "inbox_id": iid, "thread_id": tid, "subject": wfString(d.Data, "subject", "title"), "scheduled_at": scheduledAt, "issues": issues, "status": wfString(d.Data, "status", "state")})
			}
			sortWorkflowByTime(items, "scheduled_at")
			result := map[string]any{"items": items, "count": len(items), "scheduled_count": len(drafts), "scanned": len(resources), "scan_limit": workflowScanLimit, "reason": workflowReason(len(items), "schedule audit findings")}
			return workflowOutput(cmd.OutOrStdout(), result, flags, items)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: local AgentMail data.db)")
	cmd.Flags().StringVar(&dueWithin, "due-within", "", "Only inspect drafts due within a duration such as 24h")
	return cmd
}
