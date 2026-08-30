// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelDeliveryReconcileCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, inbox string
	cmd := &cobra.Command{
		Use: "reconcile", Short: "Reconcile outbound messages with status, thread placement, timestamps, and later inbound activity.",
		Example:     "  agentmail-pp-cli delivery reconcile --db /tmp/agentmail.db --inbox inbox_support --since 7d --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "delivery reconcile")
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
			messages := []workflowResource{}
			inbound := map[string][]workflowResource{}
			for _, r := range resources {
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
				messages = append(messages, r)
				direction := workflowDirectionFor(r.Data, workflowInboxAddress(workflowInbox(r), resources))
				if direction == "inbound" {
					inbound[workflowThread(r)] = append(inbound[workflowThread(r)], r)
				}
			}
			items := []map[string]any{}
			for _, out := range messages {
				if workflowDirectionFor(out.Data, workflowInboxAddress(workflowInbox(out), resources)) != "outbound" {
					continue
				}
				at := workflowMessageTime(out)
				issues := []string{}
				tid := workflowThread(out)
				if tid == "" {
					issues = append(issues, "thread_missing")
				}
				state := wfString(out.Data, "delivery_status", "delivery_state", "status", "state")
				if strings.TrimSpace(state) == "" {
					issues = append(issues, "delivery_state_missing")
				}
				laterInbound := false
				for _, in := range inbound[tid] {
					if workflowMessageTime(in).After(at) {
						laterInbound = true
						break
					}
				}
				if !laterInbound {
					issues = append(issues, "no_later_inbound")
				}
				if len(issues) == 0 {
					continue
				}
				items = append(items, map[string]any{"message_id": workflowMessageID(out), "thread_id": tid, "inbox_id": workflowInbox(out), "subject": wfString(out.Data, "subject", "title"), "sent_at": at.UTC().Format(time.RFC3339), "delivery_state": state, "issues": issues, "later_inbound": laterInbound})
			}
			sortWorkflowByTime(items, "sent_at")
			result := map[string]any{"items": items, "count": len(items), "outbound_count": workflowOutboundCount(messages, resources), "scanned": len(resources), "scan_limit": workflowScanLimit, "reason": workflowReason(len(items), "delivery reconciliation findings")}
			return workflowOutput(cmd.OutOrStdout(), result, flags, items)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: local AgentMail data.db)")
	cmd.Flags().StringVar(&inbox, "inbox", "", "Only inspect outbound messages from this inbox ID")
	cmd.Flags().StringVar(&since, "since", "", "Only include activity since 7d, 24h, or an RFC3339 timestamp")
	return cmd
}
