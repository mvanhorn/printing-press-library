// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func campaignDeployBody(name, subject, preview, fromEmail, fromLabel, replyTo, timezone string, included, excluded []string) map[string]any {
	if excluded == nil {
		excluded = []string{}
	}
	return jsonAPIBody("campaign", map[string]any{
		"definition": map[string]any{"name": name, "send_settings": map[string]any{"send_strategy": "simple", "send_timezone": timezone}},
		"audiences":  []any{map[string]any{"temporary_id": "audience-1", "name": name + " audience", "included": included, "excluded": excluded}},
		"messages": []any{map[string]any{"audience_temporary_id": "audience-1", "name": name, "variations": []any{map[string]any{"definition": map[string]any{
			"name": name, "details": map[string]any{"channel": "email", "subject": subject, "preview_text": preview, "from_email": fromEmail, "from_label": fromLabel, "reply_to_email": replyTo},
		}}}}},
	}, nil)
}

func newCampaignsDeployCmd(flags *rootFlags) *cobra.Command {
	var htmlBody, htmlFile, name, subject, preview, fromEmail, fromLabel, replyTo, timezone, messageID string
	var lists, segments, excluded []string
	cmd := &cobra.Command{
		Use: "deploy", Short: "Create a template and campaign, assign its email variation, and verify draft status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if htmlFile != "" {
				b, err := os.ReadFile(htmlFile)
				if err != nil {
					return fmt.Errorf("reading template file: %w", err)
				}
				htmlBody = string(b)
			}
			included := uniqueStrings(append(append([]string{}, lists...), segments...))
			excluded = append([]string{}, uniqueStrings(excluded)...)
			if htmlBody == "" || name == "" || len(included) == 0 || subject == "" || fromEmail == "" || fromLabel == "" {
				if dryRunOK(flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"create_template", "create_campaign", "assign_template", "verify_draft"}}, flags)
				}
				return usageErr(fmt.Errorf("required flags: --template-html or --template-file, --campaign-name, --list-id or --segment-id, --subject, --from-email, --from-label"))
			}
			if status, detail := unsubscribeCompliance(htmlBody); status != "pass" {
				return fmt.Errorf("campaign compliance preflight failed: %s", detail)
			}
			if replyTo == "" {
				replyTo = fromEmail
			}
			if messageID != "" {
				return usageErr(fmt.Errorf("deploy creates its own message; omit --message-id and use campaign-message-assign-template for an existing message"))
			}
			if err := validateCampaignDeploy(name, subject, fromEmail, fromLabel, replyTo, timezone); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "create_body": campaignDeployBody(name, subject, preview, fromEmail, fromLabel, replyTo, timezone, included, excluded), "timezone_source": "--send-timezone or GET /api/accounts", "template_bytes": len(htmlBody), "planned_steps": []string{"POST /api/templates", "POST /api/campaigns", "POST /api/campaign-message-assign-template", "verify_draft"}}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if timezone == "" {
				raw, err := c.Get(klaviyoRequestContext(c), "/api/accounts", nil)
				if err != nil {
					return classifyAPIErrorOnly(err)
				}
				var accounts struct {
					Data []struct {
						Attributes struct {
							Timezone string `json:"timezone"`
						} `json:"attributes"`
					} `json:"data"`
				}
				if err := json.Unmarshal(raw, &accounts); err != nil {
					return err
				}
				if len(accounts.Data) > 0 {
					timezone = accounts.Data[0].Attributes.Timezone
				}
				if timezone == "" {
					return fmt.Errorf("account timezone is missing; supply --send-timezone")
				}
			}
			if err := validateCampaignDeploy(name, subject, fromEmail, fromLabel, replyTo, timezone); err != nil {
				return usageErr(err)
			}
			campaignBody := campaignDeployBody(name, subject, preview, fromEmail, fromLabel, replyTo, timezone, included, excluded)
			template, _, err := c.Post(klaviyoRequestContext(c), "/api/templates", jsonAPIBody("template", map[string]any{"name": name, "html": htmlBody, "text": stripTags(htmlBody), "editor_type": "CODE"}, nil))
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			templateID := jsonAPIID(template)
			if templateID == "" {
				return fmt.Errorf("template creation returned no ID")
			}
			created, _, err := c.Post(klaviyoRequestContext(c), "/api/campaigns", campaignBody)
			if err != nil {
				return cleanupFailedCampaignDeploy(c, templateID, err)
			}
			campaignID := jsonAPIID(created)
			if campaignID == "" {
				return fmt.Errorf("campaign creation returned no ID (template %s)", templateID)
			}
			model, err := resolveCampaign(c, campaignID)
			if err != nil {
				return fmt.Errorf("campaign %s created; read-back failed: %w", campaignID, err)
			}
			if err := verifyCampaignDraft(model); err != nil {
				return fmt.Errorf("campaign %s (template %s): %w", campaignID, templateID, err)
			}
			if len(model.Messages) != 1 || len(model.Messages[0].Variations) != 1 || model.Messages[0].Variations[0].Channel != "email" {
				return fmt.Errorf("campaign %s: expected one new email variation", campaignID)
			}
			v := model.Messages[0].Variations[0]

			assign := map[string]any{"data": map[string]any{"type": "campaign-message", "id": v.ID, "relationships": map[string]any{"template": map[string]any{"data": map[string]any{"type": "template", "id": templateID}}}}}
			if _, _, err := c.Post(klaviyoRequestContext(c), "/api/campaign-message-assign-template", assign); err != nil {
				return fmt.Errorf("campaign %s created; assignment failed: %w", campaignID, err)
			}
			model, err = resolveCampaign(c, campaignID)
			if err != nil {
				return fmt.Errorf("campaign %s assigned; read-back failed: %w", campaignID, err)
			}
			if err := verifyCampaignDraft(model); err != nil {
				return fmt.Errorf("campaign %s (template %s): %w", campaignID, templateID, err)
			}
			messageIDs, variationIDs := []string{}, []string{}
			for _, m := range model.Messages {
				messageIDs = append(messageIDs, m.ID)
				for _, v := range m.Variations {
					variationIDs = append(variationIDs, v.ID)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"campaign_id": campaignID, "message_ids": messageIDs, "variation_ids": variationIDs, "template_id": templateID, "assigned": true, "audience": map[string]any{"included": included, "excluded": excluded}, "verified_draft": true, "messages": model.Messages}, flags)
		},
	}
	cmd.Flags().StringVar(&htmlBody, "template-html", "", "Inline HTML for the email template")
	cmd.Flags().StringVar(&htmlFile, "template-file", "", "Path to an HTML file")
	cmd.Flags().StringVar(&name, "campaign-name", "", "Draft campaign name")
	cmd.Flags().StringArrayVar(&lists, "list-id", nil, "Included list ID (repeatable)")
	cmd.Flags().StringArrayVar(&segments, "segment-id", nil, "Included segment ID (repeatable)")
	cmd.Flags().StringArrayVar(&excluded, "exclude", nil, "Excluded list or segment ID (repeatable)")
	cmd.Flags().StringVar(&subject, "subject", "", "Email subject")
	cmd.Flags().StringVar(&preview, "preview-text", "", "Email preview text")
	cmd.Flags().StringVar(&fromEmail, "from-email", "", "Sender email")
	cmd.Flags().StringVar(&fromLabel, "from-label", "", "Sender label")
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Reply-to address (defaults to sender)")
	cmd.Flags().StringVar(&timezone, "send-timezone", "", "Send timezone (defaults to account timezone)")
	cmd.Flags().StringVar(&messageID, "message-id", "", "Unsupported for deploy: use campaign-message-assign-template for an existing message")
	return cmd
}

func newCampaignsImageSwapCmd(flags *rootFlags) *cobra.Command {
	var campaignID, messageID, templateID, oldURL, newURL string
	cmd := &cobra.Command{Use: "image-swap", Short: "Replace an image URL in every targeted email variation's template", RunE: func(cmd *cobra.Command, args []string) error {
		if (campaignID == "" && messageID == "" && templateID == "") || oldURL == "" || newURL == "" {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"resolve_variations", "patch_templates"}}, flags)
			}
			return usageErr(fmt.Errorf("required: --old-url, --new-url, and --campaign-id, --message-id or --template-id"))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "campaign_id": campaignID, "message_id": messageID, "template_id": templateID, "old_url": oldURL, "new_url": newURL}, flags)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		ids := []string{}
		if templateID != "" {
			ids = append(ids, templateID)
		} else {
			id := firstNonEmptyString(messageID, campaignID)
			model, err := resolveCampaign(c, id)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			for _, m := range model.Messages {
				for _, v := range m.Variations {
					if v.Channel == "email" {
						if v.TemplateID == "" {
							return fmt.Errorf("email variation %s has no template_id", v.ID)
						}
						ids = append(ids, v.TemplateID)
					}
				}
			}
		}
		results, err := swapCampaignTemplateImages(c, uniqueStrings(ids), oldURL, newURL)
		if err != nil {
			return err
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"campaign_id": campaignID, "message_id": messageID, "templates": results}, flags)
	}}
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Container or historical campaign/message ID")
	cmd.Flags().StringVar(&messageID, "message-id", "", "Message ID; targets only that message")
	cmd.Flags().StringVar(&templateID, "template-id", "", "Template ID to patch directly")
	cmd.Flags().StringVar(&oldURL, "old-url", "", "Existing image URL")
	cmd.Flags().StringVar(&newURL, "new-url", "", "Replacement image URL")
	return cmd
}

func swapCampaignTemplateImages(c flowClient, ids []string, oldURL, newURL string) ([]map[string]any, error) {
	results := []map[string]any{}
	matched := false
	for _, id := range ids {
		htmlBody, err := campaignTemplateHTML(c, id)
		if err != nil {
			return nil, err
		}
		count := strings.Count(htmlBody, oldURL)
		result := map[string]any{"template_id": id, "replacements": count, "updated": false}
		if count > 0 {
			matched = true
			body := map[string]any{"data": map[string]any{"type": "template", "id": id, "attributes": map[string]any{"html": strings.ReplaceAll(htmlBody, oldURL, newURL)}}}
			_, code, err := c.Patch(klaviyoRequestContext(c), "/api/templates/"+url.PathEscape(id), body)
			if err != nil {
				return nil, err
			}
			result["updated"], result["status"] = true, code
		}
		results = append(results, result)
	}
	if !matched {
		return nil, notFoundErr(fmt.Errorf("old URL not found in any targeted email template"))
	}
	return results, nil
}

func campaignCloneReadback(cmd *cobra.Command, c templateReader, raw json.RawMessage) (json.RawMessage, error) {
	id := jsonAPIID(raw)
	if id == "" {
		return nil, fmt.Errorf("clone succeeded but returned no campaign/message ID; draft status unverified")
	}
	model, err := resolveCampaign(c, id)
	if err != nil {
		return nil, fmt.Errorf("clone %s succeeded but status read-back failed: %w", id, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	envelope["campaign_messages"] = model.Messages
	for _, m := range model.Messages {
		if m.Status != "draft" {
			warning := fmt.Sprintf("CLONED MESSAGE %s IS %s, NOT DRAFT", m.ID, m.Status)
			fmt.Fprintln(cmd.ErrOrStderr(), "WARNING:", warning)
			warnings, _ := envelope["warnings"].([]string)
			envelope["warnings"] = append(warnings, warning)
		}
	}
	return json.Marshal(envelope)
}

func qaCampaignVariation(htmlBody, offer, timezone string, v campaignModelVariation) []map[string]any {
	findings := qaGate(htmlBody, offer, timezone)["findings"].([]map[string]any)
	for _, field := range []struct{ name, value string }{{"subject", v.Subject}, {"preview_text", v.PreviewText}, {"from_email", v.FromEmail}, {"from_label", v.FromLabel}} {
		status, detail := "pass", field.value
		if strings.TrimSpace(field.value) == "" {
			status, detail = "warn", "Email variation has no "+field.name
		}
		findings = append(findings, map[string]any{"check": field.name, "status": status, "detail": detail})
	}
	return findings
}

func validateCampaignDeploy(name, subject, fromEmail, fromLabel, replyTo, timezone string) error {
	for _, field := range []struct{ name, value string }{{"campaign-name", name}, {"subject", subject}, {"from-label", fromLabel}} {
		if strings.TrimSpace(field.value) == "" || strings.ContainsAny(field.value, "\r\n") {
			return fmt.Errorf("--%s must be non-empty and contain no line breaks", field.name)
		}
	}
	for _, field := range []struct{ name, value string }{{"from-email", fromEmail}, {"reply-to", replyTo}} {
		parsed, err := mail.ParseAddress(field.value)
		if err != nil || parsed.Address != field.value {
			return fmt.Errorf("--%s must be a plain email address", field.name)
		}
	}
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil || timezone == "Local" {
			return fmt.Errorf("send timezone %q must be a valid IANA timezone", timezone)
		}
	}
	return nil
}

func cleanupFailedCampaignDeploy(c flowClient, templateID string, cause error) error {
	_, _, err := c.Delete(klaviyoRequestContext(c), "/api/templates/"+url.PathEscape(templateID))
	if err != nil && !campaignNotFound(err) {
		return fmt.Errorf("campaign creation failed: %w; cleanup of created template %s also failed: %v", classifyAPIErrorOnly(cause), templateID, err)
	}
	return fmt.Errorf("campaign creation failed: %w; created template %s was deleted", classifyAPIErrorOnly(cause), templateID)
}
