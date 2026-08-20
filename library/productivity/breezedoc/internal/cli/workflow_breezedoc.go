// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/breezedoc/internal/store"
	"github.com/spf13/cobra"
)

type workflowStep struct {
	Name    string `json:"name"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Status  int    `json:"status,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func newWorkflowInvoiceLifecycleCmd(flags *rootFlags) *cobra.Command {
	var invoiceID string
	var customerEmail string
	var customerName string
	var currency string
	var description string
	var paymentDue string
	var itemsJSON string
	var title string
	var invoiceNumber string
	var paymentTerms string
	var footerNote string
	var send bool

	cmd := &cobra.Command{
		Use:   "invoice-lifecycle",
		Short: "Create, update, send, and fetch an invoice in one structured workflow",
		Example: `  breezedoc-pp-cli workflow invoice-lifecycle --dry-run --customer-email client@example.com --currency USD --description "Consulting" --payment-due 2026-06-30 --items-json '[{"description":"Work","quantity":1,"unit_price":1000}]'
  breezedoc-pp-cli workflow invoice-lifecycle --dry-run --invoice-id inv_test --footer-note "Thank you" --send`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := workflowInvoiceBody(cmd)
			if err != nil {
				return err
			}
			if invoiceID == "" && len(body) == 0 && !flags.dryRun {
				return fmt.Errorf("creating an invoice requires invoice body flags")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			out := map[string]any{
				"workflow": "invoice_lifecycle",
				"steps":    []workflowStep{},
			}
			steps := []workflowStep{}
			currentID := invoiceID

			if invoiceID == "" {
				data, status, err := c.PostWithParams(cmd.Context(), "/invoices", map[string]string{}, body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				steps = append(steps, workflowStep{Name: "create_invoice", Method: "POST", Path: "/invoices", Status: status})
				out["create"] = decodeJSON(data)
				currentID = extractWorkflowID(data)
				if currentID == "" && !flags.dryRun {
					return fmt.Errorf("created invoice response did not include an id")
				}
			} else if len(body) > 0 {
				path := replacePathParam("/invoices/{invoice}", "invoice", invoiceID)
				data, status, err := c.PatchWithParams(cmd.Context(), path, map[string]string{}, body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				steps = append(steps, workflowStep{Name: "update_invoice", Method: "PATCH", Path: path, Status: status})
				out["update"] = decodeJSON(data)
			} else {
				steps = append(steps, workflowStep{Name: "update_invoice", Method: "PATCH", Path: "/invoices/{invoice}", Skipped: true, Reason: "no update flags provided"})
			}

			if send && currentID != "" {
				path := replacePathParam("/invoices/{invoice}/send", "invoice", currentID)
				data, status, err := c.PostWithParams(cmd.Context(), path, map[string]string{}, map[string]any{})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				steps = append(steps, workflowStep{Name: "send_invoice", Method: "POST", Path: path, Status: status})
				out["send"] = decodeJSON(data)
			} else if send {
				steps = append(steps, workflowStep{Name: "send_invoice", Method: "POST", Path: "/invoices/{invoice}/send", Skipped: true, Reason: "created invoice id unavailable during dry-run"})
			} else {
				steps = append(steps, workflowStep{Name: "send_invoice", Method: "POST", Path: "/invoices/{invoice}/send", Skipped: true, Reason: "pass --send to send"})
			}

			if currentID != "" && !flags.dryRun {
				path := replacePathParam("/invoices/{invoice}", "invoice", currentID)
				data, err := c.Get(cmd.Context(), path, map[string]string{})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				steps = append(steps, workflowStep{Name: "fetch_invoice", Method: "GET", Path: path})
				out["final"] = decodeJSON(data)
			}
			out["invoice_id"] = currentID
			out["steps"] = steps
			return workflowPrintJSON(cmd, flags, out)
		},
	}

	cmd.Flags().StringVar(&invoiceID, "invoice-id", "", "Existing invoice id to fetch or update")
	cmd.Flags().StringVar(&customerEmail, "customer-email", "", "Invoice customer email")
	cmd.Flags().StringVar(&customerName, "customer-name", "", "Invoice customer name")
	cmd.Flags().StringVar(&currency, "currency", "", "Invoice currency")
	cmd.Flags().StringVar(&description, "description", "", "Invoice description")
	cmd.Flags().StringVar(&paymentDue, "payment-due", "", "Payment due date")
	cmd.Flags().StringVar(&itemsJSON, "items-json", "", "Invoice line items JSON")
	cmd.Flags().StringVar(&title, "title", "", "Invoice title")
	cmd.Flags().StringVar(&invoiceNumber, "invoice-number", "", "Invoice number")
	cmd.Flags().StringVar(&paymentTerms, "payment-terms", "", "Payment terms")
	cmd.Flags().StringVar(&footerNote, "footer-note", "", "Footer note")
	cmd.Flags().BoolVar(&send, "send", false, "Send the invoice after create or update")

	return cmd
}

func newWorkflowClientWorkspaceSnapshotCmd(flags *rootFlags) *cobra.Command {
	var teamIDs []string
	var full bool
	var dbPath string

	cmd := &cobra.Command{
		Use:         "client-workspace-snapshot",
		Short:       "Fetch a compact account snapshot for agent planning",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  breezedoc-pp-cli workflow client-workspace-snapshot --agent
  breezedoc-pp-cli workflow client-workspace-snapshot --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if full {
				if dbPath == "" {
					dbPath = defaultDBPath("breezedoc-pp-cli")
				}
				s, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening store: %w", err)
				}
				defer s.Close()
				for _, resource := range []string{"documents", "invoices", "recipients", "templates"} {
					res := syncResource(cmd.Context(), c, s, resource, "", true, 100, false, nil, cmd.ErrOrStderr())
					if res.Err != nil {
						return fmt.Errorf("syncing %s: %w", resource, res.Err)
					}
				}
			}

			out := map[string]any{
				"workflow":  "client_workspace_snapshot",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"resources": map[string]any{},
			}
			resources := out["resources"].(map[string]any)
			for _, target := range []struct {
				name string
				path string
			}{
				{name: "me", path: "/me"},
				{name: "documents", path: "/documents"},
				{name: "templates", path: "/templates"},
				{name: "recipients", path: "/recipients"},
				{name: "invoices", path: "/invoices"},
			} {
				data, err := c.Get(cmd.Context(), target.path, map[string]string{})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				resources[target.name] = compactWorkflowSummary(data, 3)
			}
			if len(teamIDs) > 0 {
				teams := map[string]any{}
				for _, teamID := range teamIDs {
					teamID = strings.TrimSpace(teamID)
					if teamID == "" {
						continue
					}
					team := map[string]any{}
					for _, target := range []struct {
						name string
						path string
					}{
						{name: "documents", path: replacePathParam("/teams/{team}/documents", "team", teamID)},
						{name: "templates", path: replacePathParam("/teams/{team}/templates", "team", teamID)},
					} {
						data, err := c.Get(cmd.Context(), target.path, map[string]string{})
						if err != nil {
							return classifyAPIError(err, flags)
						}
						team[target.name] = compactWorkflowSummary(data, 3)
					}
					teams[teamID] = team
				}
				out["teams"] = teams
			}
			if full {
				out["archive_refreshed"] = true
				out["store_path"] = dbPath
			}
			return workflowPrintJSON(cmd, flags, out)
		},
	}

	cmd.Flags().StringArrayVar(&teamIDs, "team-id", nil, "Team id to include; repeat for multiple teams")
	cmd.Flags().BoolVar(&full, "full", false, "Refresh the local archive before taking the snapshot")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for --full archive refresh")
	return cmd
}

func newWorkflowDocumentFollowUpCmd(flags *rootFlags) *cobra.Command {
	var documentID string
	var send bool

	cmd := &cobra.Command{
		Use:   "document-follow-up",
		Short: "Inspect a document and recipients for follow-up planning",
		Example: `  breezedoc-pp-cli workflow document-follow-up --document-id 381936 --agent
  breezedoc-pp-cli workflow document-follow-up --dry-run --document-id 381936 --send`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if documentID == "" {
				return fmt.Errorf("required flag %q not set", "document-id")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			documentPath := replacePathParam("/documents/{document}", "document", documentID)
			document, err := c.Get(cmd.Context(), documentPath, map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recipientsPath := replacePathParam("/documents/{document}/recipients", "document", documentID)
			recipients, err := c.Get(cmd.Context(), recipientsPath, map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			out := map[string]any{
				"workflow":             "document_follow_up",
				"document_id":          documentID,
				"document":             decodeJSON(document),
				"recipient_summary":    summarizeWorkflowRecipients(recipients),
				"recommended_actions":  workflowFollowUpActions(recipients),
				"send_requested":       send,
				"send_requires_opt_in": !send,
			}
			if send {
				data, status, err := c.PostWithParams(cmd.Context(), replacePathParam("/documents/{document}/send", "document", documentID), map[string]string{}, map[string]any{})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				out["send"] = map[string]any{"status": status, "data": decodeJSON(data)}
			}
			return workflowPrintJSON(cmd, flags, out)
		},
	}

	cmd.Flags().StringVar(&documentID, "document-id", "", "Document id to inspect")
	cmd.Flags().BoolVar(&send, "send", false, "Send or resend the document after inspection")
	return cmd
}

func newWorkflowSignaturePacketPrepCmd(flags *rootFlags) *cobra.Command {
	var templateID string
	var title string
	var recipientsJSON string
	var send bool

	cmd := &cobra.Command{
		Use:   "signature-packet-prep",
		Short: "Prepare a signature packet from a template or new document",
		Example: `  breezedoc-pp-cli workflow signature-packet-prep --dry-run --template-id 12189 --send
  breezedoc-pp-cli workflow signature-packet-prep --title "NDA" --recipients-json '[{"email":"client@example.com"}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if title != "" {
				body["title"] = title
			}
			if recipientsJSON != "" {
				var recipients any
				if err := json.Unmarshal([]byte(recipientsJSON), &recipients); err != nil {
					return fmt.Errorf("parsing --recipients-json JSON: %w", err)
				}
				body["recipients"] = recipients
			}
			if templateID == "" && title == "" && !flags.dryRun {
				return fmt.Errorf("creating a raw document requires --title")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			createPath := "/documents"
			if templateID != "" {
				createPath = replacePathParam("/templates/{template}/create-document", "template", templateID)
			}
			data, status, err := c.PostWithParams(cmd.Context(), createPath, map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			documentID := extractWorkflowID(data)
			out := map[string]any{
				"workflow":       "signature_packet_prep",
				"template_id":    templateID,
				"document_id":    documentID,
				"create_status":  status,
				"created":        decodeJSON(data),
				"send_requested": send,
			}
			if documentID != "" && !flags.dryRun {
				documentPath := replacePathParam("/documents/{document}", "document", documentID)
				if fetched, err := c.Get(cmd.Context(), documentPath, map[string]string{}); err == nil {
					out["document"] = decodeJSON(fetched)
				}
				recipientsPath := replacePathParam("/documents/{document}/recipients", "document", documentID)
				if recipients, err := c.Get(cmd.Context(), recipientsPath, map[string]string{}); err == nil {
					out["recipients"] = decodeJSON(recipients)
					out["recipient_summary"] = summarizeWorkflowRecipients(recipients)
				}
			}
			if send && documentID != "" {
				sendPath := replacePathParam("/documents/{document}/send", "document", documentID)
				sendData, sendStatus, err := c.PostWithParams(cmd.Context(), sendPath, map[string]string{}, map[string]any{})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				out["send"] = map[string]any{"status": sendStatus, "data": decodeJSON(sendData)}
			} else if send {
				out["send"] = map[string]any{"skipped": true, "reason": "created document id unavailable during dry-run"}
			}
			return workflowPrintJSON(cmd, flags, out)
		},
	}

	cmd.Flags().StringVar(&templateID, "template-id", "", "Template id to create the packet from")
	cmd.Flags().StringVar(&title, "title", "", "Document title when creating a raw packet")
	cmd.Flags().StringVar(&recipientsJSON, "recipients-json", "", "Recipients JSON")
	cmd.Flags().BoolVar(&send, "send", false, "Send the packet after preparing it")
	return cmd
}

func workflowInvoiceBody(cmd *cobra.Command) (map[string]any, error) {
	body := map[string]any{}
	stringFlags := map[string]string{
		"customer-email": "customer_email",
		"customer-name":  "customer_name",
		"currency":       "currency",
		"description":    "description",
		"payment-due":    "payment_due",
		"title":          "title",
		"invoice-number": "invoice_number",
		"payment-terms":  "payment_terms",
		"footer-note":    "footer_note",
	}
	for flagName, fieldName := range stringFlags {
		if cmd.Flags().Changed(flagName) {
			value, err := cmd.Flags().GetString(flagName)
			if err != nil {
				return nil, err
			}
			body[fieldName] = value
		}
	}
	if cmd.Flags().Changed("items-json") {
		value, err := cmd.Flags().GetString("items-json")
		if err != nil {
			return nil, err
		}
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, fmt.Errorf("parsing --items-json JSON: %w", err)
		}
		body["items"] = parsed
	}
	return body, nil
}

func workflowPrintJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

func decodeJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

func extractWorkflowID(raw json.RawMessage) string {
	return extractIDValue(decodeJSON(raw))
}

func extractIDValue(v any) string {
	switch typed := v.(type) {
	case map[string]any:
		for _, key := range []string{"id", "invoice_id", "document_id", "template_id"} {
			if id := stringifyWorkflowID(typed[key]); id != "" {
				return id
			}
		}
		if id := extractIDValue(typed["data"]); id != "" {
			return id
		}
		if id := extractIDValue(typed["invoice"]); id != "" {
			return id
		}
		return extractIDValue(typed["document"])
	case []any:
		if len(typed) == 0 {
			return ""
		}
		return extractIDValue(typed[0])
	default:
		return ""
	}
}

func stringifyWorkflowID(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func compactWorkflowSummary(raw json.RawMessage, sampleLimit int) map[string]any {
	value := decodeJSON(raw)
	items, foundItems := workflowItems(value)
	if foundItems {
		sample := make([]any, 0, minInt(len(items), sampleLimit))
		for i, item := range items {
			if i >= sampleLimit {
				break
			}
			if m, ok := item.(map[string]any); ok {
				sample = append(sample, compactWorkflowObject(m))
			} else {
				sample = append(sample, item)
			}
		}
		return map[string]any{"count": workflowTotal(value, len(items)), "sample": sample}
	}
	if len(items) == 0 {
		if m, ok := value.(map[string]any); ok {
			return map[string]any{"count": 1, "sample": []any{compactWorkflowObject(m)}}
		}
		return map[string]any{"count": 0, "sample": []any{}}
	}
	return map[string]any{"count": 0, "sample": []any{}}
}

func workflowItems(v any) ([]any, bool) {
	switch typed := v.(type) {
	case []any:
		return typed, true
	case map[string]any:
		for _, key := range []string{"data", "items", "results", "recipients", "documents", "templates", "invoices"} {
			if arr, ok := typed[key].([]any); ok {
				return arr, true
			}
			if nested, ok := typed[key].(map[string]any); ok {
				if arr, found := workflowItems(nested); found {
					return arr, true
				}
			}
		}
	}
	return nil, false
}

func workflowTotal(v any, fallback int) int {
	m, ok := v.(map[string]any)
	if !ok {
		return fallback
	}
	for _, key := range []string{"total", "count"} {
		if total := intWorkflowNumber(m[key]); total >= 0 {
			return total
		}
	}
	if meta, ok := m["meta"].(map[string]any); ok {
		for _, key := range []string{"total", "count"} {
			if total := intWorkflowNumber(meta[key]); total >= 0 {
				return total
			}
		}
	}
	if results, ok := m["results"].(map[string]any); ok {
		return workflowTotal(results, fallback)
	}
	return fallback
}

func intWorkflowNumber(v any) int {
	switch typed := v.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return -1
		}
		return int(n)
	default:
		return -1
	}
}

func compactWorkflowObject(m map[string]any) map[string]any {
	keep := []string{"id", "name", "title", "email", "status", "customer_email", "customer_name", "created_at", "updated_at", "sent_at", "opened_at", "completed_at"}
	out := map[string]any{}
	for _, key := range keep {
		if v, ok := m[key]; ok {
			out[key] = v
		}
	}
	if len(out) == 0 {
		for key, value := range m {
			out[key] = value
			if len(out) >= 5 {
				break
			}
		}
	}
	return out
}

func summarizeWorkflowRecipients(raw json.RawMessage) map[string]any {
	summary := map[string]any{
		"total":                0,
		"completed":            0,
		"opened_not_completed": 0,
		"sent_not_opened":      0,
		"not_sent_or_unknown":  0,
		"recipients":           []any{},
	}
	items, _ := workflowItems(decodeJSON(raw))
	recipients := make([]any, 0, len(items))
	for _, item := range items {
		m, _ := item.(map[string]any)
		status := recipientFollowUpStatus(m)
		summary["total"] = summary["total"].(int) + 1
		summary[status] = summary[status].(int) + 1
		entry := compactWorkflowObject(m)
		entry["follow_up_status"] = status
		recipients = append(recipients, entry)
	}
	summary["recipients"] = recipients
	return summary
}

func workflowFollowUpActions(raw json.RawMessage) []string {
	summary := summarizeWorkflowRecipients(raw)
	actions := []string{}
	if summary["completed"].(int) == summary["total"].(int) && summary["total"].(int) > 0 {
		return []string{"No follow-up needed; all recipients are completed."}
	}
	if summary["opened_not_completed"].(int) > 0 {
		actions = append(actions, "Follow up with recipients who opened but have not completed.")
	}
	if summary["sent_not_opened"].(int) > 0 {
		actions = append(actions, "Send a reminder to recipients who have not opened the document.")
	}
	if summary["not_sent_or_unknown"].(int) > 0 {
		actions = append(actions, "Check recipient delivery details before sending a reminder.")
	}
	if len(actions) == 0 {
		actions = append(actions, "Review recipient status before taking action.")
	}
	return actions
}

func recipientFollowUpStatus(m map[string]any) string {
	if hasWorkflowField(m, "completed_at", "completed", "signed_at") {
		return "completed"
	}
	if hasWorkflowField(m, "opened_at", "viewed_at") {
		return "opened_not_completed"
	}
	if hasWorkflowField(m, "sent_at", "delivered_at") {
		return "sent_not_opened"
	}
	return "not_sent_or_unknown"
}

func hasWorkflowField(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := m[key]; ok && value != nil && fmt.Sprint(value) != "" && fmt.Sprint(value) != "false" {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
