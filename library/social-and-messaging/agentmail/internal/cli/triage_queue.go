// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelTriageQueueCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, inbox string
	cmd := &cobra.Command{
		Use: "queue", Short: "Rank unresolved inbound conversations across inboxes with age, direction, labels, and pending drafts.",
		Example:     "  agentmail-pp-cli triage queue --db /tmp/agentmail.db --inbox inbox_support --since 7d --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "triage queue")
			}
			sinceAt, err := workflowSince(since)
			if err != nil {
				return usageErr(err)
			}
			db, err := openWorkflowStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return workflowOutput(cmd.OutOrStdout(), map[string]any{"items": []map[string]any{}, "count": 0, "scanned": 0, "reason": "local mirror not found; run sync first"}, flags, nil)
			}
			defer closeWorkflowStore(db)
			hintIfUnsynced(cmd, db, "messages")
			hintIfStale(cmd, db, "messages", 24*time.Hour)
			resources, err := loadWorkflowResources(cmd.Context(), db)
			if err != nil {
				return err
			}
			type threadState struct {
				inbound        workflowResource
				latest         time.Time
				outboundLatest time.Time
				inbox, subject string
				labels         []string
				draft          bool
			}
			groups := map[string]*threadState{}
			for _, r := range resources {
				if workflowFamily(r, "drafts") {
					tid := workflowDraftThread(r, resources)
					if tid != "" {
						if g := groups[tid]; g != nil && workflowDraftPending(r) {
							g.draft = true
						}
					}
					continue
				}
				if !workflowFamily(r, "messages") {
					continue
				}
				if inbox != "" && !strings.EqualFold(workflowInbox(r), inbox) {
					continue
				}
				at := workflowMessageTime(r)
				if !sinceAt.IsZero() && !at.IsZero() && at.Before(sinceAt) {
					continue
				}
				tid := workflowThread(r)
				if tid == "" {
					tid = workflowMessageID(r)
				}
				if tid == "" {
					continue
				}
				g := groups[tid]
				if g == nil {
					g = &threadState{inbox: workflowInbox(r), labels: []string{}}
					groups[tid] = g
				}
				if g.inbox == "" {
					g.inbox = workflowInbox(r)
				}
				if g.subject == "" {
					g.subject = wfString(r.Data, "subject", "title")
				}
				g.labels = uniqueStrings(append(g.labels, workflowLabels(r.Data)...))
				direction := workflowDirectionFor(r.Data, workflowInboxAddress(workflowInbox(r), resources))
				if direction == "outbound" {
					if !at.IsZero() && (g.outboundLatest.IsZero() || at.After(g.outboundLatest)) {
						g.outboundLatest = at
					}
					continue
				}
				if direction != "inbound" {
					continue
				}
				if g.inbound.Data == nil || at.After(workflowMessageTime(g.inbound)) {
					g.inbound = r
					g.latest = at
				}
			}
			// A second pass joins drafts after message groups have been created.
			for _, r := range resources {
				if workflowFamily(r, "drafts") && workflowDraftPending(r) {
					tid := workflowDraftThread(r, resources)
					if g := groups[tid]; g != nil {
						g.draft = true
					}
				}
			}
			items := []map[string]any{}
			for tid, g := range groups {
				if g.inbound.Data == nil || (!g.outboundLatest.IsZero() && g.outboundLatest.After(workflowMessageTime(g.inbound))) {
					continue
				}
				at := workflowMessageTime(g.inbound)
				items = append(items, map[string]any{"thread_id": tid, "inbox_id": g.inbox, "message_id": workflowMessageID(g.inbound), "subject": g.subject, "direction": "inbound", "latest_at": at.UTC().Format(time.RFC3339), "age": durationAge(at), "labels": g.labels, "pending_draft": g.draft, "priority": int(time.Since(at).Hours()) + 1})
			}
			sort.SliceStable(items, func(i, j int) bool { return items[i]["latest_at"].(string) < items[j]["latest_at"].(string) })
			result := map[string]any{"items": items, "count": len(items), "scanned": len(resources), "scan_limit": workflowScanLimit, "reason": workflowReason(len(items), "unresolved inbound conversations")}
			return workflowOutput(cmd.OutOrStdout(), result, flags, items)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: local AgentMail data.db)")
	cmd.Flags().StringVar(&inbox, "inbox", "", "Only include messages from this inbox ID")
	cmd.Flags().StringVar(&since, "since", "", "Only include activity since 7d, 24h, or an RFC3339 timestamp")
	return cmd
}
