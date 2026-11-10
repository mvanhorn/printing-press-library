// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

type templateReader interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}

func requireTemplateCompliance(c templateReader, templateID string) error {
	if templateID == "" {
		return fmt.Errorf("campaign compliance preflight failed: template ID is missing")
	}
	resp, err := c.Get(klaviyoRequestContext(c), "/api/templates/"+url.PathEscape(templateID), map[string]string{"additional-fields[template]": "definition", "fields[template]": "definition,html"})
	if err != nil {
		return classifyAPIErrorOnly(err)
	}
	html := firstString(resp, "data.attributes.html", "data.attributes.definition.html")
	status, detail := unsubscribeCompliance(html)
	if status != "pass" {
		return fmt.Errorf("campaign compliance preflight failed: %s", detail)
	}
	return nil
}

func requireCampaignCompliance(c templateReader, campaignID string) error {
	model, err := resolveCampaign(c, campaignID)
	if err != nil {
		return err
	}
	for _, m := range model.Messages {
		for _, v := range m.Variations {
			switch v.Channel {
			case "sms", "push", "mobile_push", "whatsapp":
				continue
			case "email":
			default:
				return fmt.Errorf("campaign compliance preflight failed: variation %s has unsupported channel %q", v.ID, v.Channel)
			}
			if err := requireTemplateCompliance(c, v.TemplateID); err != nil {
				return fmt.Errorf("message %s variation %s: %w", m.ID, v.ID, err)
			}
		}
	}
	return nil
}

func warnTemplateCompliance(cmd *cobra.Command, html string) string {
	status, detail := unsubscribeCompliance(html)
	if status == "pass" {
		return ""
	}
	warning := "template unsubscribe check: " + detail + " Flow and transactional templates may omit an unsubscribe link."
	fmt.Fprintln(cmd.ErrOrStderr(), "warning:", warning)
	return warning
}

func templateHTMLFromBody(body map[string]any) string {
	for _, key := range []string{"data.attributes.html", "data.attributes.definition.html"} {
		if value := stringFromMapPath(body, key); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func addJSONWarning(raw json.RawMessage, warning string) (json.RawMessage, error) {
	if strings.TrimSpace(warning) == "" {
		return raw, nil
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	envelope["warnings"] = []string{warning}
	return json.Marshal(envelope)
}
