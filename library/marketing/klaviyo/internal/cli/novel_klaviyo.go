// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
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
