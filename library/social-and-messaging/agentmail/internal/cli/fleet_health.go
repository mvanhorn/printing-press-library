// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelFleetHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath, org string
	cmd := &cobra.Command{
		Use: "health", Short: "Report inbox, domain, webhook, list, metrics, API-key, pod, and organization readiness findings.",
		Example:     "  agentmail-pp-cli fleet health --db /tmp/agentmail.db --org org_demo --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fleet health")
			}
			db, err := openWorkflowStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return workflowOutput(cmd.OutOrStdout(), map[string]any{"ready": false, "findings": []map[string]any{{"severity": "error", "code": "mirror_missing", "message": "local mirror not found; run sync first"}}, "counts": map[string]int{}, "scanned": 0}, flags, nil)
			}
			defer closeWorkflowStore(db)
			hintIfUnsynced(cmd, db, "inboxes")
			hintIfStale(cmd, db, "inboxes", 24*time.Hour)
			resources, err := loadWorkflowResources(cmd.Context(), db)
			if err != nil {
				return err
			}
			findings := []map[string]any{}
			orgMatch := func(r workflowResource) bool {
				return org == "" || strings.EqualFold(wfString(r.Data, "org_id", "organization_id", "organization"), org) || strings.EqualFold(wfString(r.Data, "id", "organization_id"), org)
			}
			counts := map[string]int{"inboxes": 0, "domains": 0, "webhooks": 0, "lists": 0, "metrics": 0, "api_keys": 0, "pods": 0, "organizations": 0}
			for _, r := range resources {
				if !orgMatch(r) {
					continue
				}
				for _, family := range []string{"inboxes", "domains", "webhooks", "lists", "metrics", "api_keys", "pods", "organizations"} {
					if workflowFamily(r, family) {
						counts[family]++
						break
					}
				}
			}
			inboxes := []workflowResource{}
			domains := []workflowResource{}
			webhooks := []workflowResource{}
			metrics := []workflowResource{}
			for _, r := range resources {
				if !orgMatch(r) {
					continue
				}
				if workflowFamily(r, "inboxes") {
					inboxes = append(inboxes, r)
				}
				if workflowFamily(r, "domains") {
					domains = append(domains, r)
				}
				if workflowFamily(r, "webhooks") {
					webhooks = append(webhooks, r)
				}
				if workflowFamily(r, "metrics") {
					metrics = append(metrics, r)
				}
			}
			if len(inboxes) == 0 {
				findings = append(findings, map[string]any{"severity": "error", "code": "no_inboxes", "resource_type": "inboxes", "message": "no inbox records are available for this fleet"})
			}
			if counts["domains"] == 0 {
				findings = append(findings, map[string]any{"severity": "error", "code": "missing_domains", "resource_type": "domains", "message": "no domain records found; inbox addresses cannot be verified"})
			}
			if counts["webhooks"] == 0 {
				findings = append(findings, map[string]any{"severity": "warning", "code": "missing_webhooks", "resource_type": "webhooks", "message": "no webhook records found; delivery notifications are unavailable"})
			}
			if counts["metrics"] == 0 {
				findings = append(findings, map[string]any{"severity": "warning", "code": "missing_metrics", "resource_type": "metrics", "message": "no metrics records found; usage readiness cannot be measured"})
			}
			if counts["api_keys"] == 0 {
				findings = append(findings, map[string]any{"severity": "warning", "code": "missing_api_keys", "resource_type": "api_keys", "message": "no API-key records found; authenticated automation may not be ready"})
			}
			for _, r := range inboxes {
				id := wfID(r)
				hasDomain, hasWebhook, hasMetrics := false, false, false
				address := strings.ToLower(wfString(r.Data, "email", "address"))
				domainPart := address
				if at := strings.LastIndex(domainPart, "@"); at >= 0 {
					domainPart = domainPart[at+1:]
				}
				for _, d := range domains {
					domainName := strings.ToLower(wfString(d.Data, "name", "domain", "email"))
					if strings.EqualFold(wfString(r.Data, "domain_id", "domain"), wfID(d)) || strings.TrimSuffix(domainName, ".") == strings.TrimSuffix(domainPart, ".") || strings.HasSuffix(domainName, "."+domainPart) {
						hasDomain = true
					}
				}
				for _, w := range webhooks {
					if strings.EqualFold(workflowInbox(w), id) || strings.EqualFold(wfString(w.Data, "resource_id"), id) {
						hasWebhook = true
					}
				}
				for _, m := range metrics {
					if strings.EqualFold(workflowInbox(m), id) || strings.EqualFold(wfString(m.Data, "resource_id"), id) {
						hasMetrics = true
					}
				}
				if !hasDomain {
					findings = append(findings, map[string]any{"severity": "error", "code": "inbox_domain_missing", "resource_type": "inboxes", "resource_id": id, "message": "inbox has no related domain record"})
				}
				if !hasWebhook {
					findings = append(findings, map[string]any{"severity": "warning", "code": "inbox_webhook_missing", "resource_type": "inboxes", "resource_id": id, "message": "inbox has no related webhook record"})
				}
				if !hasMetrics {
					findings = append(findings, map[string]any{"severity": "warning", "code": "inbox_metrics_missing", "resource_type": "inboxes", "resource_id": id, "message": "inbox has no related metrics record"})
				}
			}
			for _, r := range resources {
				if !orgMatch(r) || r.SyncedAt.IsZero() || time.Since(r.SyncedAt) <= 24*time.Hour || workflowFamily(r, "inboxes") {
					continue
				}
				for _, family := range []string{"domains", "webhooks", "lists", "metrics", "api_keys", "pods", "organizations"} {
					if workflowFamily(r, family) {
						findings = append(findings, map[string]any{"severity": "warning", "code": "resource_stale", "resource_type": family, "resource_id": wfID(r), "age": durationAge(r.SyncedAt), "message": family + " mirror row is older than 24h"})
						break
					}
				}
			}
			ready := true
			for _, f := range findings {
				if f["severity"] == "error" {
					ready = false
					break
				}
			}
			result := map[string]any{"ready": ready, "findings": findings, "finding_count": len(findings), "counts": counts, "organization_id": org, "scanned": len(resources), "scan_limit": workflowScanLimit, "reason": workflowReason(len(findings), "fleet findings")}
			return workflowOutput(cmd.OutOrStdout(), result, flags, findings)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: local AgentMail data.db")
	cmd.Flags().StringVar(&org, "org", "", "Only inspect records belonging to this organization ID")
	return cmd
}
