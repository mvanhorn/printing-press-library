// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelThreadRollupCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use: "rollup <thread-id> [<thread-id>...]", Short: "Render compact conversation handoff context with participants, counts, latest direction, age, labels, and extracted reply content.",
		Example:     "  agentmail-pp-cli thread rollup thread_demo --db /tmp/agentmail.db --json --agent --select thread_id,latest_direction,message_count,pending_draft",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "thread-id=thread_demo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(cmd.Help())
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "thread rollup")
			}
			db, err := openWorkflowStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return workflowOutput(cmd.OutOrStdout(), map[string]any{"items": []map[string]any{}, "count": 0, "scanned": 0, "reason": "local mirror not found for requested thread(s) \"" + strings.Join(args, ", ") + "\"; run sync first"}, flags, nil)
			}
			defer closeWorkflowStore(db)
			hintIfUnsynced(cmd, db, "threads")
			hintIfStale(cmd, db, "threads", 24*time.Hour)
			resources, err := loadWorkflowResources(cmd.Context(), db)
			if err != nil {
				return err
			}
			wanted := map[string]bool{}
			for _, id := range args {
				wanted[strings.TrimSpace(id)] = true
			}
			type state struct {
				id, inbox, subject, latestDirection, content string
				messages, inbound, outbound                  int
				pending                                      bool
				latest, first                                time.Time
				participants, labels                         []string
			}
			states := map[string]*state{}
			for _, r := range resources {
				tid := workflowThread(r)
				rid := wfID(r)
				if workflowFamily(r, "threads") && tid == "" {
					tid = rid
				}
				if tid == "" || !wanted[tid] && !wanted[rid] {
					continue
				}
				if _, ok := states[tid]; !ok {
					states[tid] = &state{id: tid, participants: []string{}, labels: []string{}}
				}
				s := states[tid]
				if s.inbox == "" {
					s.inbox = workflowInbox(r)
				}
				if s.subject == "" {
					s.subject = wfString(r.Data, "subject", "title")
				}
				s.participants = uniqueStrings(append(s.participants, workflowParticipants(r.Data)...))
				s.labels = uniqueStrings(append(s.labels, workflowLabels(r.Data)...))
				if workflowFamily(r, "drafts") {
					if workflowDraftPending(r) {
						s.pending = true
					}
					continue
				}
				if !workflowFamily(r, "messages") {
					continue
				}
				s.messages++
				d := workflowDirectionFor(r.Data, workflowInboxAddress(s.inbox, resources))
				if d == "inbound" {
					s.inbound++
				}
				if d == "outbound" {
					s.outbound++
				}
				at := workflowMessageTime(r)
				if s.first.IsZero() || (!at.IsZero() && at.Before(s.first)) {
					s.first = at
				}
				if s.latest.IsZero() || at.After(s.latest) {
					s.latest = at
					s.latestDirection = d
					s.content = wfString(r.Data, "text", "body", "content", "extracted_text", "extracted_html", "snippet", "preview")
				}
			}
			// Attach drafts in a separate pass so resource ordering cannot hide a pending draft.
			for _, r := range resources {
				if workflowFamily(r, "drafts") && workflowDraftPending(r) {
					if s := states[workflowDraftThread(r, resources)]; s != nil {
						s.pending = true
					}
				}
			}
			items := []map[string]any{}
			for _, s := range states {
				if s.messages == 0 && s.subject == "" {
					continue
				}
				latest := ""
				if !s.latest.IsZero() {
					latest = s.latest.UTC().Format(time.RFC3339)
				}
				items = append(items, map[string]any{"thread_id": s.id, "inbox_id": s.inbox, "subject": s.subject, "message_count": s.messages, "inbound_count": s.inbound, "outbound_count": s.outbound, "participants": s.participants, "latest_direction": s.latestDirection, "latest_at": latest, "age": durationAge(s.first), "labels": s.labels, "pending_draft": s.pending, "latest_content": s.content})
			}
			sortWorkflowByTime(items, "latest_at")
			reason := workflowReason(len(items), "requested threads")
			if len(items) == 0 {
				reason = "no requested threads \"" + strings.Join(args, ", ") + "\" found in the local mirror"
			}
			result := map[string]any{"items": items, "count": len(items), "scanned": len(resources), "scan_limit": workflowScanLimit, "reason": reason}
			return workflowOutput(cmd.OutOrStdout(), result, flags, items)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: local AgentMail data.db)")
	return cmd
}
