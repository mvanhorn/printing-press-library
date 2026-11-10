// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "fmt"

type campaignQAResult struct {
	CampaignID string
	Verdict    string
	Findings   []map[string]any
	Evidence   map[string]any
	Variations []campaignModelVariation
	Brand      *brandQAKit
}

// suppliedHTML adds the optional --html checks without replacing template checks.
func runCampaignQA(c templateReader, campaignID, offer, timezone string, suppliedHTML *string, options ...brandQAOptions) (*campaignQAResult, error) {
	model, err := resolveCampaign(c, campaignID)
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	htmlBody := ""
	if suppliedHTML != nil {
		htmlBody = *suppliedHTML
	}
	baseline := qaGate(htmlBody, offer, timezone)
	result := &campaignQAResult{
		CampaignID: stringFromMapPath(model.Campaign, "id"),
		Evidence:   map[string]any{"campaign": model.Campaign, "messages": model.Messages},
		Findings:   []map[string]any{},
		Variations: []campaignModelVariation{},
	}
	if len(options) > 0 && options[0].Enabled {
		result.Brand, err = fetchBrandQAKit(c, options[0])
		if err != nil {
			return nil, err
		}
	}
	if suppliedHTML != nil {
		result.Findings = append(result.Findings, baseline["findings"].([]map[string]any)...)
		if result.Brand != nil {
			result.Findings = append(result.Findings, result.Brand.checkHTML(htmlBody)...)
		}
	}
	templates := map[string]any{}
	for _, m := range model.Messages {
		for _, v := range m.Variations {
			switch v.Channel {
			case "sms", "push", "whatsapp", "mobile_push":
				continue
			case "email":
			default:
				return nil, fmt.Errorf("campaign compliance preflight failed: variation %s has unsupported channel %q", v.ID, v.Channel)
			}
			messageHTML := ""
			var err error
			if v.TemplateID != "" {
				messageHTML, err = campaignTemplateHTML(c, v.TemplateID)
			}
			if v.TemplateID == "" || campaignNotFound(err) {
				result.Evidence["template_status"] = "not_assigned"
			}
			if err != nil && !campaignNotFound(err) {
				return nil, classifyAPIErrorOnly(err)
			}
			templates[v.ID] = map[string]any{"template_id": v.TemplateID, "subject": v.Subject, "preview_text": v.PreviewText, "from_email": v.FromEmail, "from_label": v.FromLabel, "reply_to_email": v.ReplyTo}
			findings := qaCampaignVariation(messageHTML, offer, timezone, v)
			if result.Brand != nil {
				findings = append(findings, result.Brand.checkHTML(messageHTML)...)
			}
			for _, finding := range findings {
				finding["message_id"], finding["variation_id"], finding["template_id"] = m.ID, v.ID, v.TemplateID
				result.Findings = append(result.Findings, finding)
			}
			result.Variations = append(result.Variations, v)
		}
	}
	result.Evidence["templates"] = templates
	if len(result.Findings) == 0 {
		result.Findings = baseline["findings"].([]map[string]any)
	}
	if result.Brand != nil {
		result.Findings = append(result.Findings, result.Brand.availabilityFindings()...)
	}
	result.Verdict = qaVerdict(result.Findings)
	return result, nil
}
