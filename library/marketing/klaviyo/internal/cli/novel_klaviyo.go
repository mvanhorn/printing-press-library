// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

type campaignDeployResult struct {
	TemplateID string         `json:"template_id,omitempty"`
	CampaignID string         `json:"campaign_id,omitempty"`
	MessageID  string         `json:"message_id,omitempty"`
	Assigned   bool           `json:"assigned"`
	DryRun     bool           `json:"dry_run,omitempty"`
	Steps      []string       `json:"steps"`
	Responses  map[string]any `json:"responses,omitempty"`
}

func newCampaignsDeployCmd(flags *rootFlags) *cobra.Command {
	var templateHTML string
	var templateFile string
	var campaignName string
	var listID string
	var subject string
	var fromEmail string
	var fromLabel string
	var messageID string

	cmd := &cobra.Command{
		Use:     "deploy",
		Short:   "Create a template, draft campaign, and assign the template to a campaign message",
		Example: "  klaviyo-pp-cli campaigns deploy --template-html ./email.html --campaign-name \"May offer\" --list-id LIST_ID --subject \"May offer\" --from-email marketing@example.com --from-label Marketing --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if templateFile != "" {
				b, err := os.ReadFile(templateFile)
				if err != nil {
					return fmt.Errorf("reading template file: %w", err)
				}
				templateHTML = string(b)
			}
			if templateHTML == "" || campaignName == "" || listID == "" || subject == "" || fromEmail == "" || fromLabel == "" {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), campaignDeployResult{DryRun: true, Steps: []string{"create_template", "create_campaign", "assign_template"}}, flags)
				}
				return usageErr(fmt.Errorf("required flags: --template-html or --template-file, --campaign-name, --list-id, --subject, --from-email, --from-label"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":        true,
					"template_name":  campaignName,
					"campaign_name":  campaignName,
					"list_id":        listID,
					"message_id":     messageID,
					"planned_steps":  []string{"POST /api/templates", "POST /api/campaigns", "POST /api/campaign-message-assign-template"},
					"template_bytes": len(templateHTML),
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result := campaignDeployResult{Steps: []string{"create_template", "create_campaign", "assign_template"}, Responses: map[string]any{}}
			templateBody := jsonAPIBody("template", map[string]any{
				"name":        campaignName,
				"html":        templateHTML,
				"text":        stripTags(templateHTML),
				"editor_type": "CODE",
			}, nil)
			templateResp, _, err := c.Post("/api/templates", templateBody)
			if err != nil {
				return classifyAPIError(err)
			}
			result.TemplateID = jsonAPIID(templateResp)
			result.Responses["template"] = mustJSONAny(templateResp)

			campaignBody := jsonAPIBody("campaign", map[string]any{
				"name": campaignName,
				"definition": map[string]any{
					"channel": "email",
					"content": map[string]any{
						"subject":    subject,
						"from_email": fromEmail,
						"from_label": fromLabel,
					},
					"audiences": map[string]any{
						"included": []string{listID},
					},
				},
				"send_strategy": map[string]any{
					"method":   "static",
					"datetime": time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339),
				},
			}, nil)
			campaignResp, _, err := c.Post("/api/campaigns", campaignBody)
			if err != nil {
				return classifyAPIError(err)
			}
			result.CampaignID = jsonAPIID(campaignResp)
			result.Responses["campaign"] = mustJSONAny(campaignResp)

			if messageID == "" && result.CampaignID != "" {
				messages, err := c.Get("/api/campaigns/"+result.CampaignID+"/campaign-messages", nil)
				if err == nil {
					messageID = firstJSONAPIID(messages)
					result.Responses["campaign_messages"] = mustJSONAny(messages)
				}
			}
			result.MessageID = messageID
			if result.TemplateID != "" && messageID != "" {
				assignBody := jsonAPIBody("campaign-message-assign-template", map[string]any{}, map[string]any{
					"template":         map[string]any{"data": map[string]string{"type": "template", "id": result.TemplateID}},
					"campaign-message": map[string]any{"data": map[string]string{"type": "campaign-message", "id": messageID}},
				})
				assignResp, _, err := c.Post("/api/campaign-message-assign-template", assignBody)
				if err != nil {
					return classifyAPIError(err)
				}
				result.Assigned = true
				result.Responses["assign_template"] = mustJSONAny(assignResp)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&templateHTML, "template-html", "", "Inline HTML for the email template")
	cmd.Flags().StringVar(&templateFile, "template-file", "", "Path to an HTML file for the email template")
	cmd.Flags().StringVar(&campaignName, "campaign-name", "", "Draft campaign name")
	cmd.Flags().StringVar(&listID, "list-id", "", "Audience list ID")
	cmd.Flags().StringVar(&subject, "subject", "", "Email subject")
	cmd.Flags().StringVar(&fromEmail, "from-email", "", "Sender email")
	cmd.Flags().StringVar(&fromLabel, "from-label", "", "Sender label")
	cmd.Flags().StringVar(&messageID, "message-id", "", "Existing campaign message ID to assign; auto-discovered after campaign create when omitted")
	return cmd
}

func newCampaignsImageSwapCmd(flags *rootFlags) *cobra.Command {
	var campaignID string
	var messageID string
	var templateID string
	var oldURL string
	var newURL string

	cmd := &cobra.Command{
		Use:     "image-swap",
		Short:   "Replace an image URL in a campaign message template",
		Example: "  klaviyo-pp-cli campaigns image-swap --campaign-id CAMPAIGN_ID --old-url https://cdn.example.com/old.jpg --new-url https://cdn.example.com/new.jpg --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (campaignID == "" && messageID == "" && templateID == "") || oldURL == "" || newURL == "" {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"resolve_message", "resolve_template", "patch_template"}}, flags)
				}
				return usageErr(fmt.Errorf("required flags: --old-url, --new-url, and one of --campaign-id, --message-id, --template-id"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":     true,
					"campaign_id": campaignID,
					"message_id":  messageID,
					"template_id": templateID,
					"old_url":     oldURL,
					"new_url":     newURL,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if messageID == "" && campaignID != "" {
				resp, err := c.Get("/api/campaigns/"+campaignID+"/campaign-messages", nil)
				if err != nil {
					return classifyAPIError(err)
				}
				messageID = firstJSONAPIID(resp)
			}
			if templateID == "" && messageID != "" {
				resp, err := c.Get("/api/campaign-messages/"+messageID+"/template", map[string]string{"fields[template]": "definition,html"})
				if err != nil {
					return classifyAPIError(err)
				}
				templateID = jsonAPIID(resp)
				if templateID == "" {
					templateID = firstJSONAPIID(resp)
				}
				templateHTML := firstString(resp, "data.attributes.html", "data.attributes.definition.html", "data.attributes.text")
				if templateHTML != "" {
					return patchTemplateHTML(cmd, flags, c, templateID, templateHTML, oldURL, newURL, campaignID, messageID)
				}
			}
			if templateID == "" {
				return notFoundErr(fmt.Errorf("could not resolve a template id"))
			}
			resp, err := c.Get("/api/templates/"+templateID, map[string]string{"additional-fields[template]": "definition", "fields[template]": "definition,html"})
			if err != nil {
				return classifyAPIError(err)
			}
			templateHTML := firstString(resp, "data.attributes.html", "data.attributes.definition.html", "data.attributes.text")
			return patchTemplateHTML(cmd, flags, c, templateID, templateHTML, oldURL, newURL, campaignID, messageID)
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Campaign ID to inspect for a message and template")
	cmd.Flags().StringVar(&messageID, "message-id", "", "Campaign message ID to inspect for a template")
	cmd.Flags().StringVar(&templateID, "template-id", "", "Template ID to patch directly")
	cmd.Flags().StringVar(&oldURL, "old-url", "", "Existing image URL")
	cmd.Flags().StringVar(&newURL, "new-url", "", "Replacement image URL")
	return cmd
}

func patchTemplateHTML(cmd *cobra.Command, flags *rootFlags, c interface {
	Patch(path string, body any) (json.RawMessage, int, error)
}, templateID, templateHTML, oldURL, newURL, campaignID, messageID string) error {
	if templateHTML == "" {
		return notFoundErr(fmt.Errorf("template %s did not include editable HTML in the API response", templateID))
	}
	updated := strings.ReplaceAll(templateHTML, oldURL, newURL)
	if updated == templateHTML {
		return notFoundErr(fmt.Errorf("old URL not found in template %s", templateID))
	}
	body := jsonAPIBody("template", map[string]any{"html": updated}, nil)
	resp, status, err := c.Patch("/api/templates/"+templateID, body)
	if err != nil {
		return classifyAPIError(err)
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"campaign_id": campaignID,
		"message_id":  messageID,
		"template_id": templateID,
		"old_url":     oldURL,
		"new_url":     newURL,
		"status":      status,
		"response":    mustJSONAny(resp),
	}, flags)
}

func newFlowDecayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var threshold float64

	cmd := &cobra.Command{
		Use:         "flow-decay",
		Short:       "Find flows with decaying local performance evidence",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "source": "local_store"}, flags)
			}
			db, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
			}
			defer db.Close()
			rows, err := readResourceRows(cmd.Context(), db, "flows", 250)
			if err != nil {
				return err
			}
			results := flowDecay(rows, days, threshold)
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	cmd.Flags().IntVar(&days, "days", 90, "Lookback window")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.15, "Decay threshold as a fraction")
	return cmd
}

func newCohortCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var metric string
	var interval string

	cmd := &cobra.Command{
		Use:         "cohort",
		Short:       "Compute retention cohorts from synced local event data",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "source": "local_store"}, flags)
			}
			db, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
			}
			defer db.Close()
			rows, err := readResourceRows(cmd.Context(), db, "events", 2000)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), cohort(rows, metric, interval), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	cmd.Flags().StringVar(&metric, "metric", "", "Metric name to include")
	cmd.Flags().StringVar(&interval, "interval", "month", "Cohort interval: month or week")
	return cmd
}

func newAttributionCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var metric string
	var groupBy string
	var since string

	cmd := &cobra.Command{
		Use:         "attribution",
		Short:       "Summarize revenue attribution from synced local order events",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "source": "local_store"}, flags)
			}
			db, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
			}
			defer db.Close()
			rows, err := readResourceRows(cmd.Context(), db, "events", 5000)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), attribution(rows, metric, groupBy, since), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	cmd.Flags().StringVar(&metric, "metric", "Placed Order", "Revenue metric name")
	cmd.Flags().StringVar(&groupBy, "group-by", "flow", "Attribution field: flow, campaign, channel, or metric")
	cmd.Flags().StringVar(&since, "since", "", "Include events on or after YYYY-MM-DD")
	return cmd
}

func newDedupCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var by string

	cmd := &cobra.Command{
		Use:         "dedup",
		Short:       "Find duplicated profile identities in the local profile mirror",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "source": "local_store"}, flags)
			}
			db, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
			}
			defer db.Close()
			rows, err := readResourceRows(cmd.Context(), db, "profiles", 5000)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				rows, err = readResourceRows(cmd.Context(), db, "profile", 5000)
				if err != nil {
					return err
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), dedup(rows, by), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	cmd.Flags().StringVar(&by, "by", "email,phone", "Comma-separated identity fields: email, phone")
	return cmd
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var campaignID string
	var since string

	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Compare Klaviyo campaign attribution evidence with optional Shopify setup",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "source": "local_store"}, flags)
			}
			db, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"campaign_id": campaignID, "orders": 0, "revenue": 0, "shopify_available": shopifyAvailable()}, flags)
			}
			defer db.Close()
			rows, err := readResourceRows(cmd.Context(), db, "events", 5000)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), reconcile(rows, campaignID, since), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Campaign ID or UTM campaign to reconcile")
	cmd.Flags().StringVar(&since, "since", "", "Include events on or after YYYY-MM-DD")
	return cmd
}

func newPlanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "plan",
		Short:       "Growth planning workflows for Klaviyo",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newPlanBriefToStrategyCmd(flags))
	cmd.AddCommand(newPlanQAGateCmd(flags))
	return cmd
}

func newPlanBriefToStrategyCmd(flags *rootFlags) *cobra.Command {
	var briefPath string
	var briefText string

	cmd := &cobra.Command{
		Use:         "brief-to-strategy",
		Short:       "Turn a growth brief into a Klaviyo execution strategy",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if briefPath != "" {
				b, err := os.ReadFile(briefPath)
				if err != nil {
					return fmt.Errorf("reading brief: %w", err)
				}
				briefText = string(b)
			}
			if briefText == "" {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_sections": []string{"audience", "campaign", "flows", "segments", "experiments", "qa"}}, flags)
				}
				return usageErr(fmt.Errorf("provide --brief or --brief-text"))
			}
			return printJSONFiltered(cmd.OutOrStdout(), briefToStrategy(briefText), flags)
		},
	}
	cmd.Flags().StringVar(&briefPath, "brief", "", "Path to a growth brief")
	cmd.Flags().StringVar(&briefText, "brief-text", "", "Inline growth brief")
	return cmd
}

func newPlanQAGateCmd(flags *rootFlags) *cobra.Command {
	var campaignID string
	var htmlPath string
	var offer string
	var timezone string

	cmd := &cobra.Command{
		Use:         "qa-gate",
		Short:       "Run a launch-readiness checklist for a Klaviyo campaign",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_checks": qaChecks()}, flags)
			}
			htmlBody := ""
			if htmlPath != "" {
				b, err := os.ReadFile(htmlPath)
				if err != nil {
					return fmt.Errorf("reading html: %w", err)
				}
				htmlBody = string(b)
			}
			evidence := map[string]any{}
			if campaignID != "" {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				campaignResp, err := c.Get("/api/campaigns/"+campaignID, nil)
				if err != nil {
					return classifyAPIError(err)
				}
				evidence["campaign"] = mustJSONAny(campaignResp)
				if htmlBody == "" {
					msgResp, err := c.Get("/api/campaigns/"+campaignID+"/campaign-messages", nil)
					if err == nil {
						evidence["campaign_messages"] = mustJSONAny(msgResp)
					}
				}
			}
			result := qaGate(htmlBody, offer, timezone)
			result["campaign_id"] = campaignID
			result["evidence"] = evidence
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Campaign ID to fetch for API evidence")
	cmd.Flags().StringVar(&htmlPath, "html", "", "Path to campaign HTML for link and token checks")
	cmd.Flags().StringVar(&offer, "offer", "", "Expected offer text")
	cmd.Flags().StringVar(&timezone, "timezone", "America/Chicago", "Expected launch timezone")
	return cmd
}

func openNovelStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath != "" {
		return store.OpenWithContext(ctx, dbPath)
	}
	return openStoreForRead(ctx, "klaviyo-pp-cli")
}

type resourceRow struct {
	ID   string         `json:"id"`
	Data map[string]any `json:"data"`
}

func readResourceRows(ctx context.Context, db *store.Store, resourceType string, limit int) ([]resourceRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := db.DB().QueryContext(ctx, fmt.Sprintf(`SELECT id, data FROM "%s" LIMIT ?`, strings.ReplaceAll(resourceType, `"`, `""`)), limit)
	if err != nil {
		rows, err = db.DB().QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = ? LIMIT ?`, resourceType, limit)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "no such table") {
				return nil, nil
			}
			return nil, err
		}
	}
	defer rows.Close()
	var out []resourceRow
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			data = map[string]any{"raw": string(raw)}
		}
		out = append(out, resourceRow{ID: id, Data: data})
	}
	return out, rows.Err()
}

func flowDecay(rows []resourceRow, days int, threshold float64) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		name := rowValue(row, "name", "data.attributes.name", "attributes.name")
		status := rowValue(row, "status", "data.attributes.status", "attributes.status")
		openRate := rowFloat(row, "open_rate", "data.attributes.open_rate", "attributes.open_rate")
		prevOpenRate := rowFloat(row, "previous_open_rate", "data.attributes.previous_open_rate", "attributes.previous_open_rate")
		decay := 0.0
		flagged := false
		if prevOpenRate > 0 {
			decay = (prevOpenRate - openRate) / prevOpenRate
			flagged = decay >= threshold
		}
		out = append(out, map[string]any{
			"flow_id":            row.ID,
			"name":               name,
			"status":             status,
			"days":               days,
			"open_rate":          openRate,
			"previous_open_rate": prevOpenRate,
			"decay":              decay,
			"flagged":            flagged,
		})
	}
	sort.Slice(out, func(i, j int) bool { return anyFloat(out[i]["decay"]) > anyFloat(out[j]["decay"]) })
	return out
}

func cohort(rows []resourceRow, metric, interval string) []map[string]any {
	first := map[string]time.Time{}
	active := map[string]map[string]bool{}
	for _, row := range rows {
		if metric != "" && !strings.EqualFold(rowValue(row, "metric_name", "data.attributes.metric.name", "data.attributes.metric_name", "attributes.metric_name"), metric) {
			continue
		}
		profileID := rowValue(row, "profile_id", "data.relationships.profile.data.id", "relationships.profile.data.id")
		if profileID == "" {
			profileID = rowValue(row, "data.attributes.profile_id", "attributes.profile_id")
		}
		if profileID == "" {
			continue
		}
		ts := rowTime(row, "datetime", "timestamp", "data.attributes.datetime", "attributes.datetime")
		if ts.IsZero() {
			continue
		}
		if cur, ok := first[profileID]; !ok || ts.Before(cur) {
			first[profileID] = ts
		}
		key := bucket(ts, interval)
		if active[profileID] == nil {
			active[profileID] = map[string]bool{}
		}
		active[profileID][key] = true
	}
	counts := map[string]map[string]int{}
	for profileID, firstSeen := range first {
		cohortKey := bucket(firstSeen, interval)
		if counts[cohortKey] == nil {
			counts[cohortKey] = map[string]int{"profiles": 0, "retained": 0}
		}
		counts[cohortKey]["profiles"]++
		for activeBucket := range active[profileID] {
			if activeBucket != cohortKey {
				counts[cohortKey]["retained"]++
				break
			}
		}
	}
	var out []map[string]any
	for cohortKey, vals := range counts {
		rate := 0.0
		if vals["profiles"] > 0 {
			rate = float64(vals["retained"]) / float64(vals["profiles"])
		}
		out = append(out, map[string]any{"cohort": cohortKey, "profiles": vals["profiles"], "retained": vals["retained"], "retention_rate": rate})
	}
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]["cohort"]) < fmt.Sprint(out[j]["cohort"]) })
	return out
}

func attribution(rows []resourceRow, metric, groupBy, since string) []map[string]any {
	sinceTime := parseDate(since)
	type agg struct {
		orders  int
		revenue float64
	}
	groups := map[string]*agg{}
	for _, row := range rows {
		if metric != "" && !strings.EqualFold(rowValue(row, "metric_name", "data.attributes.metric.name", "data.attributes.metric_name", "attributes.metric_name"), metric) {
			continue
		}
		if ts := rowTime(row, "datetime", "timestamp", "data.attributes.datetime", "attributes.datetime"); !sinceTime.IsZero() && (ts.IsZero() || ts.Before(sinceTime)) {
			continue
		}
		key := attributionKey(row, groupBy)
		if key == "" {
			key = "unattributed"
		}
		if groups[key] == nil {
			groups[key] = &agg{}
		}
		groups[key].orders++
		groups[key].revenue += rowFloat(row, "value", "data.attributes.value", "data.attributes.properties.value", "attributes.properties.value")
	}
	var out []map[string]any
	for key, val := range groups {
		out = append(out, map[string]any{"group": key, "orders": val.orders, "revenue": val.revenue})
	}
	sort.Slice(out, func(i, j int) bool { return anyFloat(out[i]["revenue"]) > anyFloat(out[j]["revenue"]) })
	return out
}

func dedup(rows []resourceRow, by string) []map[string]any {
	fields := strings.Split(by, ",")
	seen := map[string][]string{}
	for _, row := range rows {
		for _, field := range fields {
			field = strings.TrimSpace(strings.ToLower(field))
			if field == "" {
				continue
			}
			value := strings.ToLower(rowValue(row, field, "data.attributes."+field, "attributes."+field, "data.attributes."+field+"_number", "attributes."+field+"_number"))
			if value == "" {
				continue
			}
			seen[field+":"+value] = append(seen[field+":"+value], row.ID)
		}
	}
	var out []map[string]any
	for key, ids := range seen {
		if len(ids) < 2 {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		out = append(out, map[string]any{"field": parts[0], "value": parts[1], "profile_ids": ids, "count": len(ids)})
	}
	sort.Slice(out, func(i, j int) bool { return anyInt(out[i]["count"]) > anyInt(out[j]["count"]) })
	return out
}

func reconcile(rows []resourceRow, campaignID, since string) map[string]any {
	sinceTime := parseDate(since)
	var orders int
	var revenue float64
	for _, row := range rows {
		if campaignID != "" {
			found := strings.EqualFold(rowValue(row, "data.attributes.properties.utm_campaign", "attributes.properties.utm_campaign", "data.attributes.properties.$attributed_campaign", "attributes.properties.$attributed_campaign"), campaignID)
			found = found || strings.EqualFold(rowValue(row, "campaign_id", "data.attributes.campaign_id", "attributes.campaign_id"), campaignID)
			if !found {
				continue
			}
		}
		if ts := rowTime(row, "datetime", "timestamp", "data.attributes.datetime", "attributes.datetime"); !sinceTime.IsZero() && (ts.IsZero() || ts.Before(sinceTime)) {
			continue
		}
		orders++
		revenue += rowFloat(row, "value", "data.attributes.value", "data.attributes.properties.value", "attributes.properties.value")
	}
	return map[string]any{"campaign_id": campaignID, "orders": orders, "revenue": revenue, "shopify_available": shopifyAvailable(), "shopify_note": shopifyNote()}
}

func briefToStrategy(brief string) map[string]any {
	lines := strings.Fields(brief)
	keywords := extractKeywords(brief)
	return map[string]any{
		"summary":     strings.TrimSpace(truncateWords(lines, 32)),
		"audience":    []string{"primary buyers from the brief", "high-intent repeat purchasers", "engaged subscribers"},
		"campaigns":   []string{"one launch campaign", "one reminder campaign", "one last-chance campaign"},
		"flows":       []string{"welcome", "browse abandonment", "cart abandonment", "post-purchase"},
		"segments":    keywords,
		"experiments": []string{"subject line", "offer framing", "send time"},
		"qa":          qaChecks(),
	}
}

func qaGate(htmlBody, offer, timezone string) map[string]any {
	findings := []map[string]any{}
	add := func(check, status, detail string) {
		findings = append(findings, map[string]any{"check": check, "status": status, "detail": detail})
	}
	if htmlBody == "" {
		add("links", "warn", "No HTML supplied; link validation needs --html or campaign template evidence.")
	} else if strings.Contains(htmlBody, "http://") || strings.Contains(htmlBody, "https://") {
		add("links", "pass", "HTML contains absolute links.")
	} else {
		add("links", "fail", "No absolute links found.")
	}
	if offer == "" {
		add("offer", "warn", "No expected offer supplied.")
	} else if htmlBody == "" || strings.Contains(strings.ToLower(htmlBody), strings.ToLower(offer)) {
		add("offer", "pass", "Expected offer was provided.")
	} else {
		add("offer", "fail", "Expected offer text was not found.")
	}
	if timezone == "" {
		add("timezone", "warn", "No timezone supplied.")
	} else {
		add("timezone", "pass", "Timezone set to "+timezone+".")
	}
	if strings.Contains(htmlBody, "{{") && strings.Contains(htmlBody, "default") {
		add("token_fallbacks", "pass", "Template tokens appear to include fallback handling.")
	} else if strings.Contains(htmlBody, "{{") {
		add("token_fallbacks", "warn", "Template tokens found; confirm fallback text.")
	} else {
		add("token_fallbacks", "pass", "No template tokens found.")
	}
	add("compliance", "warn", "Confirm unsubscribe, sender identity, and physical address in Klaviyo preview.")
	add("deliverability", "warn", "Confirm inbox preview, image weight, and spam-risk terms before launch.")
	verdict := "pass"
	for _, f := range findings {
		if f["status"] == "fail" {
			verdict = "fail"
			break
		}
		if f["status"] == "warn" && verdict == "pass" {
			verdict = "warn"
		}
	}
	return map[string]any{"verdict": verdict, "findings": findings}
}

func qaChecks() []string {
	return []string{"links", "offer", "dates", "timezone", "token_fallbacks", "compliance", "deliverability"}
}

func jsonAPIBody(kind string, attrs map[string]any, relationships map[string]any) map[string]any {
	data := map[string]any{"type": kind, "attributes": attrs}
	if len(relationships) > 0 {
		data["relationships"] = relationships
	}
	return map[string]any{"data": data}
}

func jsonAPIID(raw json.RawMessage) string {
	return firstString(raw, "data.id", "id")
}

func firstJSONAPIID(raw json.RawMessage) string {
	id := jsonAPIID(raw)
	if id != "" {
		return id
	}
	return firstString(raw, "data.0.id", "0.id", "results.0.id")
}

func firstString(raw json.RawMessage, paths ...string) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	for _, path := range paths {
		if got := anyPath(v, path); got != nil {
			if s := fmt.Sprint(got); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func rowValue(row resourceRow, paths ...string) string {
	for _, path := range paths {
		if got := anyPath(row.Data, path); got != nil {
			if s := fmt.Sprint(got); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func rowFloat(row resourceRow, paths ...string) float64 {
	for _, path := range paths {
		if got := anyPath(row.Data, path); got != nil {
			return anyFloat(got)
		}
	}
	return 0
}

func rowTime(row resourceRow, paths ...string) time.Time {
	for _, path := range paths {
		if got := rowValue(row, path); got != "" {
			if t, err := time.Parse(time.RFC3339, got); err == nil {
				return t
			}
			if t := parseDate(got); !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}

func anyPath(v any, path string) any {
	cur := v
	for _, part := range strings.Split(path, ".") {
		switch typed := cur.(type) {
		case map[string]any:
			cur = typed[part]
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil
			}
			cur = typed[idx]
		default:
			return nil
		}
	}
	return cur
}

func mustJSONAny(raw json.RawMessage) any {
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	return out
}

func bucket(t time.Time, interval string) string {
	if interval == "week" {
		year, week := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	}
	return t.Format("2006-01")
}

func parseDate(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}

func attributionKey(row resourceRow, groupBy string) string {
	switch strings.ToLower(groupBy) {
	case "campaign":
		return rowValue(row, "data.attributes.properties.$attributed_campaign", "attributes.properties.$attributed_campaign", "data.attributes.properties.utm_campaign", "attributes.properties.utm_campaign")
	case "channel":
		return rowValue(row, "data.attributes.properties.$attributed_channel", "attributes.properties.$attributed_channel", "data.attributes.channel", "attributes.channel")
	case "metric":
		return rowValue(row, "metric_name", "data.attributes.metric.name", "data.attributes.metric_name", "attributes.metric_name")
	default:
		return rowValue(row, "data.attributes.properties.$attributed_flow", "attributes.properties.$attributed_flow", "data.relationships.flow.data.id", "relationships.flow.data.id")
	}
}

func anyFloat(v any) float64 {
	switch typed := v.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		n, _ := typed.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return n
	default:
		return 0
	}
}

func anyInt(v any) int {
	return int(anyFloat(v))
}

func shopifyAvailable() bool {
	return os.Getenv("SHOPIFY_SHOP") != "" && (os.Getenv("SHOPIFY_ACCESS_TOKEN") != "" || os.Getenv("SHOPIFY_ADMIN_TOKEN") != "")
}

func shopifyNote() string {
	if shopifyAvailable() {
		return "Shopify credentials detected; compare this local Klaviyo total with Shopify order exports."
	}
	return "SHOPIFY_SHOP and SHOPIFY_ACCESS_TOKEN are not set; returning Klaviyo-local reconciliation only."
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(html.UnescapeString(b.String())), " ")
}

func extractKeywords(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(word) < 5 || seen[word] {
			continue
		}
		seen[word] = true
		out = append(out, word)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func truncateWords(words []string, n int) string {
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ")
}

// ── Flow novel commands ─────────────────────────────────────────────────

func deepCopyMap(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var out map[string]any
	json.Unmarshal(b, &out)
	return out
}

func transformFlowIDs(definition map[string]any) map[string]any {
	def := deepCopyMap(definition)
	actions, ok := def["actions"].([]any)
	if !ok || len(actions) == 0 {
		return def
	}
	idMap := map[string]string{}
	for i, raw := range actions {
		a, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		oldID := fmt.Sprint(a["id"])
		if oldID == "" || oldID == "<nil>" {
			continue
		}
		newID := fmt.Sprintf("tmp-%d", i+1)
		idMap[oldID] = newID
	}
	for _, raw := range actions {
		a, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		oldID := fmt.Sprint(a["id"])
		if mapped, ok := idMap[oldID]; ok {
			a["temporary_id"] = mapped
			delete(a, "id")
		}
		if links, ok := a["links"].(map[string]any); ok {
			for k, v := range links {
				if s, ok := v.(string); ok {
					if mapped, ok := idMap[s]; ok {
						links[k] = mapped
					}
				}
			}
		}
	}
	if entryID, ok := def["entry_action_id"].(string); ok {
		if mapped, ok := idMap[entryID]; ok {
			def["entry_action_id"] = mapped
		}
	}
	return def
}

func cleanFlowDefinition(definition map[string]any) map[string]any {
	def := deepCopyMap(definition)
	delete(def, "trigger_type")
	delete(def, "created")
	delete(def, "updated")
	actions, ok := def["actions"].([]any)
	if !ok {
		return def
	}
	for _, raw := range actions {
		a, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		data, ok := a["data"].(map[string]any)
		if !ok {
			continue
		}
		if msg, ok := data["message"].(map[string]any); ok {
			delete(msg, "id")
		}
		if fmt.Sprint(a["type"]) == "time-delay" {
			unit, _ := data["unit"].(string)
			if unit != "day" && unit != "days" {
				delete(data, "delay_until_weekdays")
				delete(data, "delay_until_time")
			}
		}
	}
	return def
}

type flowClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
	Patch(path string, body any) (json.RawMessage, int, error)
	Delete(path string) (json.RawMessage, int, error)
}

func fetchAndTransformFlow(c flowClient, flowID string, keepIDs bool) (definition map[string]any, name string, err error) {
	resp, err := c.Get("/api/flows/"+flowID, map[string]string{
		"additional-fields[flow]": "definition",
	})
	if err != nil {
		return nil, "", classifyAPIError(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return nil, "", fmt.Errorf("parsing flow response: %w", err)
	}
	data, _ := parsed["data"].(map[string]any)
	if data == nil {
		return nil, "", notFoundErr(fmt.Errorf("flow %s not found", flowID))
	}
	attrs, _ := data["attributes"].(map[string]any)
	if attrs == nil {
		return nil, "", fmt.Errorf("flow %s has no attributes", flowID)
	}
	def, _ := attrs["definition"].(map[string]any)
	if def == nil {
		return nil, "", fmt.Errorf("flow %s has no definition — try adding ?additional-fields[flow]=definition", flowID)
	}
	name, _ = attrs["name"].(string)
	def = cleanFlowDefinition(def)
	if !keepIDs {
		def = transformFlowIDs(def)
	}
	return def, name, nil
}

func resolveMetricID(c flowClient, metricName string) (string, error) {
	var matches []string
	cursor := ""
	for {
		params := map[string]string{"fields[metric]": "name"}
		if cursor != "" {
			params["page[cursor]"] = cursor
		}
		resp, err := c.Get("/api/metrics", params)
		if err != nil {
			return "", classifyAPIError(err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(resp, &parsed); err != nil {
			return "", fmt.Errorf("parsing metrics response: %w", err)
		}
		items, _ := parsed["data"].([]any)
		for _, raw := range items {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			attrs, _ := item["attributes"].(map[string]any)
			name, _ := attrs["name"].(string)
			if name == metricName {
				id, _ := item["id"].(string)
				matches = append(matches, id)
			}
		}
		links, _ := parsed["links"].(map[string]any)
		nextLink, _ := links["next"].(string)
		if nextLink == "" {
			break
		}
		cursor = extractCursor(nextLink)
		if cursor == "" {
			break
		}
	}
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	return "", fmt.Errorf("metric %q not found — check that your Shopify/Klaviyo integration is active", metricName)
}

func extractCursor(nextLink string) string {
	for _, sep := range []string{"page%5Bcursor%5D=", "page[cursor]="} {
		parts := strings.Split(nextLink, sep)
		if len(parts) > 1 {
			return strings.Split(parts[1], "&")[0]
		}
	}
	return ""
}

func normalizeActionStatus(definition map[string]any, status string) {
	actions, ok := definition["actions"].([]any)
	if !ok {
		return
	}
	for _, raw := range actions {
		a, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if data, ok := a["data"].(map[string]any); ok {
			if _, has := data["status"]; has {
				data["status"] = status
			}
		}
	}
}

func overrideSender(definition map[string]any, fromEmail, fromLabel string) {
	actions, ok := definition["actions"].([]any)
	if !ok {
		return
	}
	for _, raw := range actions {
		a, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(a["type"]) != "send-email" {
			continue
		}
		data, ok := a["data"].(map[string]any)
		if !ok {
			continue
		}
		msg, ok := data["message"].(map[string]any)
		if !ok {
			continue
		}
		if fromEmail != "" {
			msg["from_email"] = fromEmail
		}
		if fromLabel != "" {
			msg["from_label"] = fromLabel
		}
	}
}

func createFlowAndSetStatus(c flowClient, name string, definition map[string]any, status string) (map[string]any, error) {
	body := jsonAPIBody("flow", map[string]any{
		"name":       name,
		"definition": definition,
	}, nil)
	resp, _, err := c.Post("/api/flows", body)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	flowID := jsonAPIID(resp)
	result := map[string]any{
		"flow_id":  flowID,
		"name":     name,
		"status":   "draft",
		"url":      "https://www.klaviyo.com/flow/" + flowID + "/edit",
		"response": mustJSONAny(resp),
	}
	if status == "live" && flowID != "" {
		patchBody := jsonAPIBody("flow", map[string]any{"status": "live"}, nil)
		patchResp, _, err := c.Patch("/api/flows/"+flowID, patchBody)
		if err != nil {
			result["status_error"] = err.Error()
		} else {
			result["status"] = "live"
			result["patch_response"] = mustJSONAny(patchResp)
		}
	}
	return result, nil
}

// ── Export ───────────────────────────────────────────────────────────────

func newFlowsExportCmd(flags *rootFlags) *cobra.Command {
	var outputFile string
	var keepIDs bool

	cmd := &cobra.Command{
		Use:         "export <flow-id>",
		Short:       "Export a flow definition as reusable JSON",
		Example:     "  klaviyo-pp-cli flows export VHSEFK --json\n  klaviyo-pp-cli flows export VHSEFK --output flow.json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_flow", "clean", "transform_ids"}}, flags)
				}
				return usageErr(fmt.Errorf("flow ID required"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "flow_id": args[0], "keep_ids": keepIDs, "planned_steps": []string{"fetch_flow", "clean", "transform_ids"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			def, name, err := fetchAndTransformFlow(c, args[0], keepIDs)
			if err != nil {
				return err
			}
			output := map[string]any{
				"name":       name,
				"definition": def,
			}
			if keepIDs {
				output["inspection_only"] = true
			}
			b, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return err
			}
			if outputFile != "" {
				if err := os.WriteFile(outputFile, b, 0644); err != nil {
					return fmt.Errorf("writing output file: %w", err)
				}
				actionCount := 0
				if actions, ok := def["actions"].([]any); ok {
					actionCount = len(actions)
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"file": outputFile, "name": name, "actions": actionCount}, flags)
			}
			_, err = cmd.OutOrStdout().Write(append(b, '\n'))
			return err
		},
	}
	cmd.Flags().StringVar(&outputFile, "output", "", "Write to file instead of stdout")
	cmd.Flags().BoolVar(&keepIDs, "keep-ids", false, "Keep original action IDs (inspection only — not usable for create)")
	return cmd
}

// ── Clone ───────────────────────────────────────────────────────────────

func newFlowsCloneCmd(flags *rootFlags) *cobra.Command {
	var name string
	var status string

	cmd := &cobra.Command{
		Use:     "clone <flow-id>",
		Short:   "Clone an existing flow to a new draft",
		Example: "  klaviyo-pp-cli flows clone VHSEFK --name \"Browse Abandonment v2\" --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_flow", "transform_ids", "create_flow"}}, flags)
				}
				return usageErr(fmt.Errorf("source flow ID required"))
			}
			if name == "" {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "source_flow_id": args[0], "planned_steps": []string{"fetch_flow", "transform_ids", "create_flow"}}, flags)
				}
				return usageErr(fmt.Errorf("--name is required"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "source_flow_id": args[0], "name": name, "status": status, "planned_steps": []string{"fetch_flow", "transform_ids", "create_flow"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			def, _, err := fetchAndTransformFlow(c, args[0], false)
			if err != nil {
				return err
			}
			if status == "draft" {
				normalizeActionStatus(def, "draft")
			}
			result, err := createFlowAndSetStatus(c, name, def, status)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Name for the new flow (required)")
	cmd.Flags().StringVar(&status, "status", "draft", "Initial flow status: draft or live")
	return cmd
}

// ── Deploy ──────────────────────────────────────────────────────────────

func newFlowsDeployCmd(flags *rootFlags) *cobra.Command {
	var name string
	var definitionFile string
	var stdinDef bool
	var preset string
	var status string
	var fromEmail string
	var fromLabel string
	var product string
	var days int
	var triggerProduct string
	var crossSell string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Create a flow from a definition file or preset",
		Example: "  klaviyo-pp-cli flows deploy --preset cart-abandonment --json\n" +
			"  klaviyo-pp-cli flows deploy --name \"My Flow\" --definition-file flow.json --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if preset == "" && definitionFile == "" && !stdinDef {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"read_definition", "create_flow"}}, flags)
				}
				return usageErr(fmt.Errorf("provide --preset, --definition-file, or --stdin"))
			}

			if preset != "" {
				if name == "" {
					name = presetDefaultName(preset)
				}
				if dryRunOK(flags) {
					displayFromEmail := fromEmail
					if displayFromEmail == "" {
						displayFromEmail = "sender@example.com"
					}
					displayFromLabel := fromLabel
					if displayFromLabel == "" {
						displayFromLabel = "Your Brand"
					}
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run":         true,
						"preset":          preset,
						"name":            name,
						"from_email":      displayFromEmail,
						"from_label":      displayFromLabel,
						"status":          status,
						"product":         product,
						"days":            days,
						"trigger_product": triggerProduct,
						"cross_sell":      crossSell,
						"planned_steps": []string{
							"resolve_metrics",
							"create_templates",
							"build_flow_definition",
							"create_flow",
						},
					}, flags)
				}
				if fromEmail == "" || fromLabel == "" {
					return usageErr(fmt.Errorf("--from-email and --from-label are required with --preset"))
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				def, templateIDs, err := buildPresetFlow(c, flowPresetOptions{
					Preset:         preset,
					FromEmail:      fromEmail,
					FromLabel:      fromLabel,
					Product:        product,
					Days:           days,
					TriggerProduct: triggerProduct,
					CrossSell:      crossSell,
				})
				if err != nil {
					return err
				}
				result, err := createFlowAndSetStatus(c, name, def, status)
				if err != nil {
					return err
				}
				result["preset"] = preset
				result["template_ids"] = templateIDs
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			var defBytes []byte
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if definitionFile != "" {
				defBytes, err = os.ReadFile(definitionFile)
				if err != nil {
					return fmt.Errorf("reading definition file: %w", err)
				}
			} else {
				defBytes, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
			}

			var wrapper map[string]any
			if err := json.Unmarshal(defBytes, &wrapper); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			def, _ := wrapper["definition"].(map[string]any)
			if def == nil {
				def = wrapper
			}
			if _, ok := def["actions"].([]any); !ok {
				return usageErr(fmt.Errorf("definition must be a JSON object with an 'actions' array"))
			}

			if name == "" {
				if n, ok := wrapper["name"].(string); ok && n != "" {
					name = n
				} else {
					return usageErr(fmt.Errorf("--name is required when deploying from file"))
				}
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":    true,
					"name":       name,
					"actions":    len(def["actions"].([]any)),
					"from_email": fromEmail,
					"from_label": fromLabel,
				}, flags)
			}

			if fromEmail != "" || fromLabel != "" {
				overrideSender(def, fromEmail, fromLabel)
			}

			result, err := createFlowAndSetStatus(c, name, def, status)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Flow name (required for file deploy; presets have defaults)")
	cmd.Flags().StringVar(&definitionFile, "definition-file", "", "Path to a JSON flow definition file")
	cmd.Flags().BoolVar(&stdinDef, "stdin", false, "Read definition JSON from stdin")
	cmd.Flags().StringVar(&preset, "preset", "", "Use a built-in preset: cart-abandonment, browse-abandonment, post-purchase, welcome-series, winback, gift-followup, replenishment, cross-sell")
	cmd.Flags().StringVar(&status, "status", "draft", "Initial flow status: draft or live")
	cmd.Flags().StringVar(&fromEmail, "from-email", "", "Sender email for preset emails")
	cmd.Flags().StringVar(&fromLabel, "from-label", "", "Sender label for preset emails")
	cmd.Flags().StringVar(&product, "product", "", "Product name for product-specific presets such as replenishment")
	cmd.Flags().IntVar(&days, "days", 77, "Delay days for replenishment preset")
	cmd.Flags().StringVar(&triggerProduct, "trigger-product", "", "Trigger product for gift-followup and cross-sell presets")
	cmd.Flags().StringVar(&crossSell, "cross-sell", "", "Comma-separated products to recommend for cross-sell preset")
	return cmd
}

// ── Preset flow builders ────────────────────────────────────────────────

func presetDefaultName(preset string) string {
	switch preset {
	case "cart-abandonment":
		return "Cart Abandonment"
	case "browse-abandonment":
		return "Browse Abandonment"
	case "post-purchase":
		return "Post-Purchase"
	case "welcome-series":
		return "Welcome Series"
	case "winback":
		return "Winback"
	case "gift-followup":
		return "Gift Follow-Up"
	case "replenishment":
		return "Replenishment"
	case "cross-sell":
		return "Cross-Sell"
	default:
		return "Flow"
	}
}

type presetEmail struct {
	TmpID       string
	Name        string
	Subject     string
	PreviewText string
	TemplateKey string
}

type flowPresetOptions struct {
	Preset         string
	FromEmail      string
	FromLabel      string
	Product        string
	Days           int
	TriggerProduct string
	CrossSell      string
}

func buildPresetFlow(c flowClient, opts flowPresetOptions) (map[string]any, []string, error) {
	switch opts.Preset {
	case "cart-abandonment":
		return buildCartAbandonmentFlow(c, opts.FromEmail, opts.FromLabel)
	case "browse-abandonment":
		return buildBrowseAbandonmentFlow(c, opts.FromEmail, opts.FromLabel)
	case "post-purchase":
		return buildPostPurchaseFlow(c, opts.FromEmail, opts.FromLabel)
	case "welcome-series":
		return buildWelcomeSeriesFlow(c, opts.FromEmail, opts.FromLabel)
	case "winback":
		return buildWinbackFlow(c, opts.FromEmail, opts.FromLabel)
	case "gift-followup":
		return buildGiftFollowupFlow(c, opts.FromEmail, opts.FromLabel, opts.TriggerProduct)
	case "replenishment":
		return buildReplenishmentFlow(c, opts.FromEmail, opts.FromLabel, opts.Product, opts.Days)
	case "cross-sell":
		return buildCrossSellFlow(c, opts.FromEmail, opts.FromLabel, opts.TriggerProduct, opts.CrossSell)
	default:
		return nil, nil, usageErr(fmt.Errorf("unknown preset %q — supported: cart-abandonment, browse-abandonment, post-purchase, welcome-series, winback, gift-followup, replenishment, cross-sell", opts.Preset))
	}
}

func buildCartAbandonmentFlow(c flowClient, fromEmail, fromLabel string) (map[string]any, []string, error) {
	triggerID, err := resolveMetricID(c, "Added to Cart")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving trigger metric: %w", err)
	}
	placedOrderID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving Placed Order metric: %w", err)
	}

	emails := []presetEmail{
		{TmpID: "tmp-2", Name: "Cart Abandonment: Email 1", Subject: "You left something behind", PreviewText: "Your cart is waiting for you", TemplateKey: "cart-1"},
		{TmpID: "tmp-4", Name: "Cart Abandonment: Email 2", Subject: "Still thinking it over?", PreviewText: "Here's why customers love it", TemplateKey: "cart-2"},
		{TmpID: "tmp-6", Name: "Cart Abandonment: Email 3", Subject: "Last chance — your cart is expiring", PreviewText: "Don't miss out on what you picked", TemplateKey: "cart-3"},
	}

	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}

	def := map[string]any{
		"triggers": []any{
			map[string]any{"type": "metric", "id": triggerID, "trigger_filter": nil},
		},
		"profile_filter": map[string]any{
			"condition_groups": []any{
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":               "profile-metric",
							"metric_id":          placedOrderID,
							"measurement":        "count",
							"measurement_filter": map[string]any{"type": "numeric", "operator": "equals", "value": 0},
							"timeframe_filter":   map[string]any{"type": "date", "operator": "flow-start"},
							"metric_filters":     nil,
						},
					},
				},
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":             "profile-not-in-flow",
							"timeframe_filter": map[string]any{"type": "date", "operator": "in-the-last", "unit": "day", "quantity": 14},
						},
					},
				},
			},
		},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": 14, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, []int{1, 24, 48}),
	}
	return def, templateIDs, nil
}

func buildBrowseAbandonmentFlow(c flowClient, fromEmail, fromLabel string) (map[string]any, []string, error) {
	triggerID, err := resolveMetricID(c, "Viewed Product")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving trigger metric: %w", err)
	}
	addedToCartID, err := resolveMetricID(c, "Added to Cart")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving Added to Cart metric: %w", err)
	}
	placedOrderID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving Placed Order metric: %w", err)
	}

	emails := []presetEmail{
		{TmpID: "tmp-2", Name: "Browse Abandonment: Email 1", Subject: "Spotted something you like?", PreviewText: "The items you viewed are waiting", TemplateKey: "browse-1"},
		{TmpID: "tmp-4", Name: "Browse Abandonment: Email 2", Subject: "Still browsing?", PreviewText: "See what others are saying", TemplateKey: "browse-2"},
		{TmpID: "tmp-6", Name: "Browse Abandonment: Email 3", Subject: "Picked just for you", PreviewText: "Products we think you'll love", TemplateKey: "browse-3"},
	}

	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}

	def := map[string]any{
		"triggers": []any{
			map[string]any{"type": "metric", "id": triggerID, "trigger_filter": nil},
		},
		"profile_filter": map[string]any{
			"condition_groups": []any{
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":               "profile-metric",
							"metric_id":          addedToCartID,
							"measurement":        "count",
							"measurement_filter": map[string]any{"type": "numeric", "operator": "equals", "value": 0},
							"timeframe_filter":   map[string]any{"type": "date", "operator": "flow-start"},
							"metric_filters":     nil,
						},
					},
				},
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":               "profile-metric",
							"metric_id":          placedOrderID,
							"measurement":        "count",
							"measurement_filter": map[string]any{"type": "numeric", "operator": "equals", "value": 0},
							"timeframe_filter":   map[string]any{"type": "date", "operator": "flow-start"},
							"metric_filters":     nil,
						},
					},
				},
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":             "profile-not-in-flow",
							"timeframe_filter": map[string]any{"type": "date", "operator": "in-the-last", "unit": "day", "quantity": 14},
						},
					},
				},
			},
		},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": 7, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, []int{2, 24, 48}),
	}
	return def, templateIDs, nil
}

func buildEmailSequence(emails []presetEmail, templateIDs []string, fromEmail, fromLabel string, delayHours []int) []any {
	allDays := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	var actions []any
	for i, em := range emails {
		delayUnit := "hours"
		delayVal := delayHours[i]
		if delayVal >= 24 {
			delayUnit = "days"
			delayVal = delayVal / 24
		}
		delayTmpID := fmt.Sprintf("tmp-%d", i*2+1)
		emailTmpID := em.TmpID
		var nextDelay any
		if i+1 < len(emails) {
			nextDelay = fmt.Sprintf("tmp-%d", (i+1)*2+1)
		}

		actions = append(actions, map[string]any{
			"temporary_id": delayTmpID,
			"type":         "time-delay",
			"data": map[string]any{
				"unit": delayUnit, "value": delayVal,
				"secondary_value": nil, "timezone": "profile",
				"delay_until_weekdays": allDays,
			},
			"links": map[string]any{"next": emailTmpID},
		})
		actions = append(actions, map[string]any{
			"temporary_id": emailTmpID,
			"type":         "send-email",
			"data": map[string]any{
				"message": map[string]any{
					"from_email":            fromEmail,
					"from_label":            fromLabel,
					"reply_to_email":        nil,
					"cc_email":              nil,
					"bcc_email":             nil,
					"subject_line":          em.Subject,
					"preview_text":          em.PreviewText,
					"template_id":           templateIDs[i],
					"smart_sending_enabled": false,
					"transactional":         false,
					"name":                  em.Name,
				},
				"status": "draft",
			},
			"links": map[string]any{"next": nextDelay},
		})
	}
	return actions
}

func createPresetTemplates(c flowClient, emails []presetEmail) ([]string, error) {
	var ids []string
	for _, em := range emails {
		html := presetTemplateHTML(em.TemplateKey, em.Subject)
		body := jsonAPIBody("template", map[string]any{
			"name":        em.Name,
			"html":        html,
			"text":        stripTags(html),
			"editor_type": "CODE",
		}, nil)
		resp, _, err := c.Post("/api/templates", body)
		if err != nil {
			return ids, fmt.Errorf("creating template %q: %w", em.Name, classifyAPIError(err))
		}
		ids = append(ids, jsonAPIID(resp))
	}
	return ids, nil
}

func presetTemplateHTML(key, subject string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
body { margin: 0; padding: 0; background: #f7f7f7; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
.wrapper { max-width: 600px; margin: 0 auto; background: #ffffff; }
.header { padding: 32px 40px 24px; text-align: center; }
.header img { max-width: 160px; height: auto; }
.content { padding: 0 40px 32px; color: #333333; font-size: 16px; line-height: 1.6; }
.content h1 { font-size: 24px; color: #1a1a1a; margin: 0 0 16px; font-weight: 600; }
.product-block { text-align: center; padding: 24px 0; }
.product-block img { max-width: 280px; height: auto; border-radius: 4px; }
.product-name { font-size: 18px; font-weight: 600; color: #1a1a1a; margin: 16px 0 8px; }
.btn { display: inline-block; padding: 14px 32px; background: #1a1a1a; color: #ffffff; text-decoration: none; border-radius: 4px; font-weight: 600; font-size: 16px; }
.footer { padding: 24px 40px; text-align: center; font-size: 13px; color: #999999; border-top: 1px solid #eeeeee; }
</style>
</head>
<body>
<div class="wrapper">
	  <div class="header">
	    <strong>Your Brand</strong>
	  </div>
  <div class="content">
    <h1>%s</h1>
    <div class="product-block">
      <a href="{{ event.ProductURL }}"><img src="{{ event.ProductImageURL }}" alt="{{ event.ProductName }}" /></a>
      <div class="product-name">{{ event.ProductName }}</div>
    </div>
    <p style="text-align:center;"><a href="{{ event.ProductURL }}" class="btn">View Now</a></p>
  </div>
  <div class="footer">
	    <p>Your Brand — update this footer before sending.</p>
    <p>{%% unsubscribe 'Unsubscribe' %%}</p>
  </div>
</div>
</body>
</html>`, subject, subject)
}

// ── Bulk flow pause/resume ──────────────────────────────────────────────

func newFlowsPauseCmd(flags *rootFlags) *cobra.Command {
	var tag string
	cmd := &cobra.Command{
		Use:     "pause",
		Short:   "Pause flows (set to draft) by tag name",
		Example: "  klaviyo-pp-cli flows pause --tag \"Active/Live Flows\" --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkFlowStatus(cmd, flags, tag, "draft")
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Tag name to filter flows by (required)")
	return cmd
}

func newFlowsResumeCmd(flags *rootFlags) *cobra.Command {
	var tag string
	cmd := &cobra.Command{
		Use:     "resume",
		Short:   "Resume flows (set to live) by tag name",
		Example: "  klaviyo-pp-cli flows resume --tag \"Active/Live Flows\" --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkFlowStatus(cmd, flags, tag, "live")
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Tag name to filter flows by (required)")
	return cmd
}

func bulkFlowStatus(cmd *cobra.Command, flags *rootFlags, tag, targetStatus string) error {
	if tag == "" {
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"resolve_tag", "fetch_flows", "patch_status"}}, flags)
		}
		return usageErr(fmt.Errorf("--tag is required"))
	}
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "tag": tag, "target_status": targetStatus, "planned_steps": []string{"resolve_tag", "fetch_flows", "patch_status"}}, flags)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	tagID, err := resolveTagID(c, tag)
	if err != nil {
		return err
	}
	resp, err := c.Get("/api/tags/"+tagID+"/relationships/flows", nil)
	if err != nil {
		return classifyAPIError(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return fmt.Errorf("parsing flows response: %w", err)
	}
	relData, _ := parsed["data"].([]any)
	var results []map[string]any
	for _, raw := range relData {
		rel, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		fID, _ := rel["id"].(string)
		if fID == "" {
			continue
		}
		flowResp, fErr := c.Get("/api/flows/"+fID, map[string]string{"fields[flow]": "name,status"})
		if fErr != nil {
			results = append(results, map[string]any{"flow_id": fID, "action": "error", "error": fErr.Error()})
			continue
		}
		fName := firstString(flowResp, "data.attributes.name")
		currentStatus := firstString(flowResp, "data.attributes.status")
		if currentStatus == targetStatus {
			results = append(results, map[string]any{"flow_id": fID, "name": fName, "status": currentStatus, "action": "skipped (already " + targetStatus + ")"})
			continue
		}
		patchBody := jsonAPIBody("flow", map[string]any{"status": targetStatus}, nil)
		_, _, pErr := c.Patch("/api/flows/"+fID, patchBody)
		if pErr != nil {
			results = append(results, map[string]any{"flow_id": fID, "name": fName, "status": currentStatus, "action": "error", "error": pErr.Error()})
		} else {
			results = append(results, map[string]any{"flow_id": fID, "name": fName, "status": targetStatus, "action": "updated"})
		}
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"tag":           tag,
		"target_status": targetStatus,
		"total":         len(relData),
		"results":       results,
	}, flags)
}

func resolveTagID(c flowClient, tagName string) (string, error) {
	escapedName := strings.ReplaceAll(strings.ReplaceAll(tagName, `\`, `\\`), `"`, `\"`)
	resp, err := c.Get("/api/tags", map[string]string{
		"filter":      fmt.Sprintf("equals(name,\"%s\")", escapedName),
		"fields[tag]": "name",
	})
	if err != nil {
		return "", classifyAPIError(err)
	}
	id := firstJSONAPIID(resp)
	if id == "" {
		return "", notFoundErr(fmt.Errorf("tag %q not found", tagName))
	}
	return id, nil
}

// ── Coupon pool monitoring ──────────────────────────────────────────────

type couponPoolClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

type couponPoolStatus struct {
	CouponID       string `json:"coupon_id"`
	ExternalID     string `json:"external_id,omitempty"`
	Description    string `json:"description,omitempty"`
	RemainingCodes int    `json:"remaining_codes"`
	ExpiredCodes   int    `json:"expired_unassigned_codes,omitempty"`
	PagesScanned   int    `json:"pages_scanned"`
	Alert          bool   `json:"alert"`
	Status         string `json:"status"`
}

type couponPoolReport struct {
	CheckedAt    string             `json:"checked_at"`
	AlertBelow   int                `json:"alert_below"`
	TotalCoupons int                `json:"total_coupons"`
	AlertCount   int                `json:"alert_count"`
	OK           bool               `json:"ok"`
	Pools        []couponPoolStatus `json:"pools"`
}

func newCouponsCheckPoolsCmd(flags *rootFlags) *cobra.Command {
	var alertBelow int
	var couponID string
	var failOnAlert bool

	cmd := &cobra.Command{
		Use:     "check-pools",
		Short:   "Check Klaviyo coupon pools for low remaining code counts",
		Example: "  klaviyo-pp-cli coupons check-pools --alert-below 500 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if alertBelow < 0 {
				return usageErr(fmt.Errorf("--alert-below must be zero or greater"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":       true,
					"alert_below":   alertBelow,
					"coupon_id":     couponID,
					"planned_steps": []string{"fetch_coupons", "count_unassigned_coupon_codes", "flag_low_pools"},
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report, err := checkCouponPools(c, alertBelow, couponID, time.Now())
			if err != nil {
				return err
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
				return err
			}
			if failOnAlert && report.AlertCount > 0 {
				return fmt.Errorf("%d coupon pool(s) below threshold %d", report.AlertCount, alertBelow)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&alertBelow, "alert-below", 500, "Alert when remaining usable codes are below this count")
	cmd.Flags().StringVar(&couponID, "coupon-id", "", "Check a single coupon ID instead of every coupon")
	cmd.Flags().BoolVar(&failOnAlert, "fail-on-alert", false, "Exit non-zero when any coupon pool is below the threshold")
	return cmd
}

func checkCouponPools(c couponPoolClient, alertBelow int, couponID string, now time.Time) (couponPoolReport, error) {
	coupons, err := fetchCouponSummaries(c, couponID)
	if err != nil {
		return couponPoolReport{}, err
	}
	report := couponPoolReport{
		CheckedAt:    now.UTC().Format(time.RFC3339),
		AlertBelow:   alertBelow,
		TotalCoupons: len(coupons),
		OK:           true,
		Pools:        make([]couponPoolStatus, 0, len(coupons)),
	}
	for _, coupon := range coupons {
		id := fmt.Sprint(coupon["id"])
		if id == "" || id == "<nil>" {
			continue
		}
		remaining, expired, pages, err := countRemainingCouponCodes(c, id, now)
		if err != nil {
			return couponPoolReport{}, err
		}
		status := couponPoolStatus{
			CouponID:       id,
			ExternalID:     stringFromMapPath(coupon, "attributes.external_id"),
			Description:    stringFromMapPath(coupon, "attributes.description"),
			RemainingCodes: remaining,
			ExpiredCodes:   expired,
			PagesScanned:   pages,
			Alert:          remaining < alertBelow,
			Status:         "ok",
		}
		if status.Alert {
			status.Status = "alert"
			report.AlertCount++
		}
		report.Pools = append(report.Pools, status)
	}
	report.OK = report.AlertCount == 0
	sort.Slice(report.Pools, func(i, j int) bool {
		if report.Pools[i].Alert != report.Pools[j].Alert {
			return report.Pools[i].Alert
		}
		if report.Pools[i].RemainingCodes != report.Pools[j].RemainingCodes {
			return report.Pools[i].RemainingCodes < report.Pools[j].RemainingCodes
		}
		return report.Pools[i].CouponID < report.Pools[j].CouponID
	})
	return report, nil
}

func fetchCouponSummaries(c couponPoolClient, couponID string) ([]map[string]any, error) {
	if couponID != "" {
		return []map[string]any{{"id": couponID}}, nil
	}
	var coupons []map[string]any
	cursor := ""
	for {
		params := map[string]string{
			"fields[coupon]": "id,external_id,description,monitor_configuration",
			"page[size]":     "100",
		}
		if cursor != "" {
			params["page[cursor]"] = cursor
		}
		resp, err := c.Get("/api/coupons", params)
		if err != nil {
			return nil, classifyAPIError(err)
		}
		items, nextCursor, err := parseJSONAPICollection(resp)
		if err != nil {
			return nil, fmt.Errorf("parsing coupons response: %w", err)
		}
		coupons = append(coupons, items...)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return coupons, nil
}

func countRemainingCouponCodes(c couponPoolClient, couponID string, now time.Time) (remaining int, expired int, pages int, err error) {
	cursor := ""
	for {
		params := map[string]string{
			"fields[coupon-code]": "status,expires_at,unique_code",
			"filter":              `equals(status,"UNASSIGNED")`,
			"page[size]":          "100",
		}
		if cursor != "" {
			params["page[cursor]"] = cursor
		}
		resp, err := c.Get("/api/coupons/"+url.PathEscape(couponID)+"/coupon-codes", params)
		if err != nil {
			return 0, 0, pages, classifyAPIError(err)
		}
		pages++
		items, nextCursor, err := parseJSONAPICollection(resp)
		if err != nil {
			return 0, 0, pages, fmt.Errorf("parsing coupon code response for %s: %w", couponID, err)
		}
		for _, item := range items {
			if couponCodeIsUsable(item, now) {
				remaining++
			} else {
				expired++
			}
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return remaining, expired, pages, nil
}

func parseJSONAPICollection(raw json.RawMessage) ([]map[string]any, string, error) {
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", err
	}
	data, _ := parsed["data"].([]any)
	items := make([]map[string]any, 0, len(data))
	for _, rawItem := range data {
		item, ok := rawItem.(map[string]any)
		if ok {
			items = append(items, item)
		}
	}
	links, _ := parsed["links"].(map[string]any)
	nextLink, _ := links["next"].(string)
	return items, extractCursor(nextLink), nil
}

func couponCodeIsUsable(code map[string]any, now time.Time) bool {
	status := stringFromMapPath(code, "attributes.status")
	if status != "" && status != "UNASSIGNED" {
		return false
	}
	expiresAt := stringFromMapPath(code, "attributes.expires_at")
	if expiresAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false
	}
	return t.After(now)
}

func stringFromMapPath(v map[string]any, path string) string {
	got := anyPath(v, path)
	if got == nil {
		return ""
	}
	s := fmt.Sprint(got)
	if s == "<nil>" {
		return ""
	}
	return s
}

// ── Revenue report ──────────────────────────────────────────────────────

func newReportRevenueCmd(flags *rootFlags) *cobra.Command {
	var groupBy string
	var since string
	var until string

	cmd := &cobra.Command{
		Use:         "revenue",
		Short:       "Revenue attribution report (uses metric-aggregates, not the buggy values-reports endpoint)",
		Example:     "  klaviyo-pp-cli report revenue --by flow --json\n  klaviyo-pp-cli report revenue --by campaign --since 2026-03-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "group_by": groupBy, "planned_steps": []string{"resolve_placed_order_metric", "query_metric_aggregates"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			metricID, err := resolveMetricID(c, "Placed Order")
			if err != nil {
				return err
			}
			attrField := "$attributed_flow"
			switch strings.ToLower(groupBy) {
			case "campaign":
				attrField = "Campaign Name"
			case "channel":
				attrField = "$attributed_channel"
			case "message":
				attrField = "$attributed_message"
			}
			if since == "" {
				since = time.Now().AddDate(0, -3, 0).Format("2006-01-02")
			}
			if until == "" {
				until = time.Now().Format("2006-01-02")
			}
			body := map[string]any{
				"data": map[string]any{
					"type": "metric-aggregate",
					"attributes": map[string]any{
						"metric_id":    metricID,
						"measurements": []string{"sum_value", "count", "unique"},
						"interval":     "month",
						"page_size":    500,
						"by":           []string{attrField},
						"filter":       []string{"greater-or-equal(datetime," + since + "T00:00:00)", "less-than(datetime," + until + "T23:59:59)"},
						"timezone":     "US/Central",
					},
				},
			}
			resp, _, err := c.Post("/api/metric-aggregates", body)
			if err != nil {
				return classifyAPIError(err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(resp, &parsed); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}
			data, _ := parsed["data"].(map[string]any)
			attrs, _ := data["attributes"].(map[string]any)
			dates, _ := attrs["dates"].([]any)
			results, _ := attrs["data"].([]any)
			var rows []map[string]any
			for _, r := range results {
				row, ok := r.(map[string]any)
				if !ok {
					continue
				}
				dims, _ := row["dimensions"].([]any)
				name := ""
				if len(dims) > 0 {
					name = fmt.Sprint(dims[0])
				}
				if name == "" || name == "<nil>" {
					name = "(unattributed)"
				}
				meas, _ := row["measurements"].(map[string]any)
				sumVals, _ := meas["sum_value"].([]any)
				countVals, _ := meas["count"].([]any)
				uniqueVals, _ := meas["unique"].([]any)
				totalRev := 0.0
				totalOrders := 0.0
				totalUnique := 0.0
				for _, v := range sumVals {
					totalRev += anyFloat(v)
				}
				for _, v := range countVals {
					totalOrders += anyFloat(v)
				}
				for _, v := range uniqueVals {
					totalUnique += anyFloat(v)
				}
				rows = append(rows, map[string]any{
					"name":          name,
					"revenue":       totalRev,
					"orders":        int(totalOrders),
					"unique_buyers": int(totalUnique),
					"aov":           0.0,
				})
				if totalOrders > 0 {
					rows[len(rows)-1]["aov"] = totalRev / totalOrders
				}
			}
			if strings.ToLower(groupBy) == "flow" {
				flowNames := map[string]string{}
				fCursor := ""
				for {
					fParams := map[string]string{"fields[flow]": "name"}
					if fCursor != "" {
						fParams["page[cursor]"] = fCursor
					}
					flowsResp, fErr := c.Get("/api/flows", fParams)
					if fErr != nil {
						break
					}
					var fp map[string]any
					if json.Unmarshal(flowsResp, &fp) != nil {
						break
					}
					items, _ := fp["data"].([]any)
					for _, raw := range items {
						f, _ := raw.(map[string]any)
						fid, _ := f["id"].(string)
						attrs, _ := f["attributes"].(map[string]any)
						fname, _ := attrs["name"].(string)
						if fid != "" && fname != "" {
							flowNames[fid] = fname
						}
					}
					fLinks, _ := fp["links"].(map[string]any)
					fNext, _ := fLinks["next"].(string)
					if fNext == "" {
						break
					}
					fCursor = extractCursor(fNext)
					if fCursor == "" {
						break
					}
				}
				for _, r := range rows {
					if id, ok := r["name"].(string); ok {
						if fname, ok := flowNames[id]; ok {
							r["name"] = fname
							r["flow_id"] = id
						}
					}
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				return anyFloat(rows[i]["revenue"]) > anyFloat(rows[j]["revenue"])
			})
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"group_by": groupBy,
				"since":    since,
				"until":    until,
				"periods":  dates,
				"metric":   "Placed Order",
				"rows":     rows,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&groupBy, "by", "flow", "Group by: flow, campaign, or channel")
	cmd.Flags().StringVar(&since, "since", "", "Start date YYYY-MM-DD (default: 3 months ago)")
	cmd.Flags().StringVar(&until, "until", "", "End date YYYY-MM-DD (default: today)")
	return cmd
}

func newReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Revenue and performance reports",
	}
	cmd.AddCommand(newReportDashboardCmd(flags))
	cmd.AddCommand(newReportRevenueCmd(flags))
	cmd.AddCommand(newReportOpenRatesCmd(flags))
	cmd.AddCommand(newReportMetricRankCmd(flags, "unsubscribes", "Rank unsubscribe drivers", "Unsubscribed Email", "flow"))
	cmd.AddCommand(newReportMetricRankCmd(flags, "spam-complaints", "Rank spam complaints by campaign", "Marked Email as Spam", "campaign"))
	cmd.AddCommand(newReportListGrowthCmd(flags))
	cmd.AddCommand(newReportDeliverabilityCmd(flags))
	cmd.AddCommand(newReportDomainReputationCmd(flags))
	cmd.AddCommand(newReportFlowFunnelCmd(flags))
	cmd.AddCommand(newReportFlowComparisonCmd(flags))
	cmd.AddCommand(newReportEmailPerformanceCmd(flags))
	cmd.AddCommand(newReportFormsCmd(flags))
	cmd.AddCommand(newReportSignupSourcesCmd(flags))
	cmd.AddCommand(newReportProductsCmd(flags))
	cmd.AddCommand(newReportProductAffinityCmd(flags))
	cmd.AddCommand(newReportConsentCmd(flags))
	return cmd
}

// ── Bulk template image swap ────────────────────────────────────────────

func newTemplatesUpdateImageCmd(flags *rootFlags) *cobra.Command {
	var oldURL string
	var newURL string
	var includeFlows bool
	var includeCampaigns bool

	cmd := &cobra.Command{
		Use:     "update-image",
		Short:   "Replace an image URL across standalone templates, flow messages, or campaign messages",
		Example: "  klaviyo-pp-cli templates update-image --old-url https://cdn.example.com/old-logo.png --new-url https://cdn.example.com/new-logo.png --include-flows --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if oldURL == "" || newURL == "" {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_standalone_templates", "optionally_fetch_flow_message_templates", "optionally_fetch_campaign_message_templates", "find_matches", "patch_each"}}, flags)
				}
				return usageErr(fmt.Errorf("--old-url and --new-url are required"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "old_url": oldURL, "new_url": newURL, "include_flows": includeFlows, "include_campaigns": includeCampaigns, "planned_steps": []string{"fetch_standalone_templates", "optionally_fetch_flow_message_templates", "optionally_fetch_campaign_message_templates", "find_matches", "patch_each"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var allTemplates []map[string]any
			cursor := ""
			for {
				params := map[string]string{"fields[template]": "name,html", "page[size]": "50"}
				if cursor != "" {
					params["page[cursor]"] = cursor
				}
				resp, err := c.Get("/api/templates", params)
				if err != nil {
					return classifyAPIError(err)
				}
				var parsed map[string]any
				if err := json.Unmarshal(resp, &parsed); err != nil {
					break
				}
				items, _ := parsed["data"].([]any)
				for _, raw := range items {
					t, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					allTemplates = append(allTemplates, t)
				}
				links, _ := parsed["links"].(map[string]any)
				nextLink, _ := links["next"].(string)
				if nextLink == "" {
					break
				}
				cursor = extractCursor(nextLink)
				if cursor == "" {
					break
				}
			}
			var results []map[string]any
			matched := 0
			for _, t := range allTemplates {
				tID, _ := t["id"].(string)
				attrs, _ := t["attributes"].(map[string]any)
				tName, _ := attrs["name"].(string)
				tHTML, _ := attrs["html"].(string)
				if !strings.Contains(tHTML, oldURL) {
					continue
				}
				matched++
				updated := strings.ReplaceAll(tHTML, oldURL, newURL)
				patchBody := jsonAPIBody("template", map[string]any{"html": updated}, nil)
				_, _, pErr := c.Patch("/api/templates/"+tID, patchBody)
				if pErr != nil {
					results = append(results, map[string]any{"template_id": tID, "name": tName, "action": "error", "error": pErr.Error()})
				} else {
					results = append(results, map[string]any{"template_id": tID, "name": tName, "action": "updated"})
				}
			}
			if includeFlows {
				flowResults, flowScanned, flowMatched, err := updateMessageTemplatesForImageSwap(c, "flow", oldURL, newURL)
				if err != nil {
					return err
				}
				results = append(results, flowResults...)
				matched += flowMatched
				allTemplates = append(allTemplates, make([]map[string]any, flowScanned)...)
			}
			if includeCampaigns {
				campaignResults, campaignScanned, campaignMatched, err := updateMessageTemplatesForImageSwap(c, "campaign", oldURL, newURL)
				if err != nil {
					return err
				}
				results = append(results, campaignResults...)
				matched += campaignMatched
				allTemplates = append(allTemplates, make([]map[string]any, campaignScanned)...)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"old_url":           oldURL,
				"new_url":           newURL,
				"include_flows":     includeFlows,
				"include_campaigns": includeCampaigns,
				"templates_scanned": len(allTemplates),
				"templates_matched": matched,
				"results":           results,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&oldURL, "old-url", "", "Image URL to find")
	cmd.Flags().StringVar(&newURL, "new-url", "", "Replacement image URL")
	cmd.Flags().BoolVar(&includeFlows, "include-flows", false, "Also scan flow message templates")
	cmd.Flags().BoolVar(&includeCampaigns, "include-campaigns", false, "Also scan campaign message templates")
	return cmd
}

// ── Additional flow presets ─────────────────────────────────────────────

func buildPostPurchaseFlow(c flowClient, fromEmail, fromLabel string) (map[string]any, []string, error) {
	triggerID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving trigger metric: %w", err)
	}

	emails := []presetEmail{
		{TmpID: "tmp-2", Name: "Post-Purchase: Email 1 - Thank You", Subject: "Thanks for your order!", PreviewText: "Here's how to get the most out of your purchase", TemplateKey: "postpurchase-1"},
		{TmpID: "tmp-4", Name: "Post-Purchase: Email 2 - Getting Started", Subject: "Getting started with your new {{ event.ProductName }}", PreviewText: "Quick tips to make the most of it", TemplateKey: "postpurchase-2"},
		{TmpID: "tmp-6", Name: "Post-Purchase: Email 3 - How's It Going?", Subject: "How are you liking your {{ event.ProductName }}?", PreviewText: "We'd love to hear from you", TemplateKey: "postpurchase-3"},
	}

	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}

	def := map[string]any{
		"triggers": []any{
			map[string]any{"type": "metric", "id": triggerID, "trigger_filter": nil},
		},
		"profile_filter": map[string]any{
			"condition_groups": []any{
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":             "profile-not-in-flow",
							"timeframe_filter": map[string]any{"type": "date", "operator": "in-the-last", "unit": "day", "quantity": 30},
						},
					},
				},
			},
		},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": 30, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, []int{1, 72, 168}),
	}
	return def, templateIDs, nil
}

func buildWelcomeSeriesFlow(c flowClient, fromEmail, fromLabel string) (map[string]any, []string, error) {
	emails := []presetEmail{
		{TmpID: "tmp-2", Name: "Welcome: Email 1 - Welcome", Subject: "Welcome to our community", PreviewText: "We're glad you're here", TemplateKey: "welcome-1"},
		{TmpID: "tmp-4", Name: "Welcome: Email 2 - Our Story", Subject: "Why we built these products", PreviewText: "The story behind the tools", TemplateKey: "welcome-2"},
		{TmpID: "tmp-6", Name: "Welcome: Email 3 - Best Sellers", Subject: "Our most-loved tools", PreviewText: "See what's helping thousands of people level up", TemplateKey: "welcome-3"},
	}

	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}

	// Welcome series needs a list trigger — we use the Newsletter list
	// The user will need to update the trigger list ID after creation
	def := map[string]any{
		"triggers": []any{
			map[string]any{"type": "list", "id": "REPLACE_WITH_LIST_ID", "trigger_filter": nil},
		},
		"profile_filter": map[string]any{
			"condition_groups": []any{},
		},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": 0, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, []int{0, 48, 96}),
	}
	return def, templateIDs, nil
}

func buildWinbackFlow(c flowClient, fromEmail, fromLabel string) (map[string]any, []string, error) {
	triggerID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, nil, fmt.Errorf("resolving trigger metric: %w", err)
	}
	placedOrderID := triggerID

	emails := []presetEmail{
		{TmpID: "tmp-2", Name: "Winback: Email 1 - We Miss You", Subject: "It's been a while...", PreviewText: "We've got something new for you", TemplateKey: "winback-1"},
		{TmpID: "tmp-4", Name: "Winback: Email 2 - What's New", Subject: "See what's new", PreviewText: "New tools to help you level up", TemplateKey: "winback-2"},
		{TmpID: "tmp-6", Name: "Winback: Email 3 - Special Offer", Subject: "A little something for you", PreviewText: "Come back and save", TemplateKey: "winback-3"},
	}

	templateIDs, err := createPresetTemplates(c, emails)
	if err != nil {
		return nil, nil, err
	}

	def := map[string]any{
		"triggers": []any{
			map[string]any{"type": "metric", "id": triggerID, "trigger_filter": nil},
		},
		"profile_filter": map[string]any{
			"condition_groups": []any{
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":               "profile-metric",
							"metric_id":          placedOrderID,
							"measurement":        "count",
							"measurement_filter": map[string]any{"type": "numeric", "operator": "equals", "value": 0},
							"timeframe_filter":   map[string]any{"type": "date", "operator": "in-the-last", "unit": "day", "quantity": 90},
							"metric_filters":     nil,
						},
					},
				},
				map[string]any{
					"conditions": []any{
						map[string]any{
							"type":             "profile-not-in-flow",
							"timeframe_filter": map[string]any{"type": "date", "operator": "in-the-last", "unit": "day", "quantity": 90},
						},
					},
				},
			},
		},
		"entry_action_id":  "tmp-1",
		"reentry_criteria": map[string]any{"duration": 90, "unit": "day"},
		"actions":          buildEmailSequence(emails, templateIDs, fromEmail, fromLabel, []int{24, 168, 336}),
	}
	return def, templateIDs, nil
}
