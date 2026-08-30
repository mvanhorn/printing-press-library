// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type sendCheckResult struct {
	DraftID     string           `json:"draft_id"`
	Found       bool             `json:"found"`
	Safe        bool             `json:"safe"`
	InboxID     string           `json:"inbox_id,omitempty"`
	ThreadID    string           `json:"thread_id,omitempty"`
	Subject     string           `json:"subject,omitempty"`
	Recipients  []string         `json:"recipients"`
	ScheduledAt string           `json:"scheduled_at,omitempty"`
	Risks       []map[string]any `json:"risks"`
	Evidence    map[string]any   `json:"evidence"`
	Reason      string           `json:"reason"`
}

func newNovelSendCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var live bool
	cmd := &cobra.Command{
		Use: "check <draft-id>", Short: "Review a draft for deterministic recipient, attachment, schedule, duplicate, and idempotency risks before sending.",
		Example:     "  agentmail-pp-cli send check draft_demo --db /tmp/agentmail.db --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "draft-id=draft_demo"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(cmd.Help())
			}
			if live {
				return usageErr(fmt.Errorf("send check is local-only; --live is not supported"))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "send check")
			}
			draftID := strings.TrimSpace(args[0])
			db, err := openWorkflowStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return workflowOutput(cmd.OutOrStdout(), sendCheckResult{DraftID: draftID, Safe: false, Recipients: []string{}, Risks: []map[string]any{{"code": "mirror_missing", "severity": "error", "message": "local mirror not found; draft safety cannot be established"}}, Evidence: map[string]any{"scanned": 0, "scan_limit": workflowScanLimit}, Reason: "local mirror not found; run sync first"}, flags, nil)
			}
			defer closeWorkflowStore(db)
			hintIfUnsynced(cmd, db, "drafts")
			hintIfStale(cmd, db, "drafts", 24*time.Hour)
			resources, err := loadWorkflowResources(cmd.Context(), db)
			if err != nil {
				return err
			}
			var draft *workflowResource
			attachments := []workflowResource{}
			messages := []workflowResource{}
			for i := range resources {
				r := resources[i]
				if workflowFamily(r, "drafts") && strings.EqualFold(wfID(r), draftID) {
					draft = &resources[i]
				}
				if workflowAttachmentBelongs(r, draftID) {
					attachments = append(attachments, r)
				}
				if workflowFamily(r, "messages") {
					messages = append(messages, r)
				}
			}
			result := sendCheckResult{DraftID: draftID, Recipients: []string{}, Risks: []map[string]any{}, Evidence: map[string]any{"scanned": len(resources), "scan_limit": workflowScanLimit}, Reason: "draft passed deterministic local checks"}
			if draft == nil {
				result.Reason = "draft not found in the local mirror; safety cannot be established"
				result.Risks = append(result.Risks, map[string]any{"code": "draft_missing", "severity": "error", "message": "requested draft does not exist locally"})
				return workflowOutput(cmd.OutOrStdout(), result, flags, nil)
			}
			result.Found = true
			result.InboxID = workflowInbox(*draft)
			result.ThreadID = workflowDraftThread(*draft, resources)
			result.Subject = wfString(draft.Data, "subject", "title")
			result.Recipients = workflowRecipientStrings(draft.Data)
			result.Evidence["attachment_count"] = len(attachments)
			if len(result.Recipients) == 0 {
				result.Risks = append(result.Risks, map[string]any{"code": "recipients_missing", "severity": "error", "message": "draft has no to/cc/bcc recipients"})
			}
			if strings.TrimSpace(wfString(draft.Data, "text", "body", "content", "html", "extracted_text", "extracted_html")) == "" {
				result.Risks = append(result.Risks, map[string]any{"code": "body_missing", "severity": "warning", "message": "draft body is empty"})
			}
			scheduled := parseWorkflowTime(wfString(draft.Data, "send_at", "scheduled_at", "schedule_at"))
			if !scheduled.IsZero() {
				result.ScheduledAt = scheduled.UTC().Format(time.RFC3339)
				if scheduled.Before(time.Now().UTC()) {
					result.Risks = append(result.Risks, map[string]any{"code": "schedule_overdue", "severity": "error", "message": "scheduled send time is in the past"})
				}
			}
			if len(attachments) == 0 && len(wfList(draft.Data, "attachments", "attachment_ids")) > 0 {
				result.Risks = append(result.Risks, map[string]any{"code": "attachments_missing", "severity": "error", "message": "draft references attachments absent from the local mirror"})
			}
			duplicate := false
			for _, m := range messages {
				if workflowDirectionFor(m.Data, workflowInboxAddress(workflowInbox(m), resources)) == "outbound" && strings.EqualFold(workflowInbox(m), result.InboxID) && strings.EqualFold(wfString(m.Data, "subject", "title"), result.Subject) {
					duplicate = true
					break
				}
			}
			if duplicate {
				result.Risks = append(result.Risks, map[string]any{"code": "possible_duplicate", "severity": "warning", "message": "a matching outbound subject already exists for this inbox"})
			}
			if wfString(draft.Data, "idempotency_key", "client_id") == "" {
				result.Risks = append(result.Risks, map[string]any{"code": "idempotency_missing", "severity": "warning", "message": "draft has no idempotency key or client ID"})
			}
			result.Safe = true
			for _, risk := range result.Risks {
				if risk["severity"] == "error" {
					result.Safe = false
				}
			}
			if len(result.Risks) > 0 {
				result.Reason = "draft has risks requiring review"
			}
			return workflowOutput(cmd.OutOrStdout(), result, flags, []map[string]any{{"draft_id": result.DraftID, "safe": result.Safe, "risk_count": len(result.Risks), "reason": result.Reason}})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: local AgentMail data.db")
	cmd.Flags().BoolVar(&live, "live", false, "Reject local-only analysis rather than contacting the AgentMail API")
	return cmd
}
