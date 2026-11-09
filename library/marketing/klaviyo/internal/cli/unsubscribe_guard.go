// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

type templateReader interface {
	Get(string, map[string]string) (json.RawMessage, error)
}

func requireTemplateCompliance(c templateReader, templateID string) error {
	if templateID == "" {
		return fmt.Errorf("campaign compliance preflight failed: template ID is missing")
	}
	resp, err := c.Get("/api/templates/"+url.PathEscape(templateID), map[string]string{"additional-fields[template]": "definition", "fields[template]": "definition,html"})
	if err != nil {
		return classifyAPIError(err)
	}
	html := firstString(resp, "data.attributes.html", "data.attributes.definition.html")
	status, detail := unsubscribeCompliance(html)
	if status != "pass" {
		return fmt.Errorf("campaign compliance preflight failed: %s", detail)
	}
	return nil
}

func requireCampaignCompliance(c templateReader, campaignID string) error {
	if campaignID == "" {
		return fmt.Errorf("campaign compliance preflight failed: campaign ID is missing")
	}
	endpoint := "/api/campaigns/" + url.PathEscape(campaignID) + "/campaign-messages"
	seen := map[string]bool{}
	checked := 0
	var query map[string]string
	for {
		if seen[endpoint] {
			return fmt.Errorf("campaign compliance preflight failed: repeated campaign message page")
		}
		seen[endpoint] = true
		resp, err := c.Get(endpoint, query)
		if err != nil {
			return classifyAPIError(err)
		}
		var page struct {
			Data []struct {
				ID         string `json:"id"`
				Attributes struct {
					Definition struct {
						Channel string `json:"channel"`
					} `json:"definition"`
				} `json:"attributes"`
			} `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(resp, &page); err != nil {
			return fmt.Errorf("campaign compliance preflight failed: invalid campaign messages: %w", err)
		}
		if page.Data == nil {
			return fmt.Errorf("campaign compliance preflight failed: campaign message data is missing")
		}
		for _, message := range page.Data {
			if message.ID == "" {
				return fmt.Errorf("campaign compliance preflight failed: campaign message ID is missing")
			}
			channel := message.Attributes.Definition.Channel
			switch channel {
			case "sms", "push", "whatsapp", "mobile_push":
				checked++
				continue
			case "email":
			case "":
				return fmt.Errorf("campaign compliance preflight failed: campaign message %s channel is missing", message.ID)
			default:
				return fmt.Errorf("campaign compliance preflight failed: campaign message %s has unsupported channel %q", message.ID, channel)
			}
			checked++
			template, err := c.Get("/api/campaign-messages/"+url.PathEscape(message.ID)+"/template", map[string]string{"fields[template]": "html"})
			if err != nil {
				return classifyAPIError(err)
			}
			html := firstString(template, "data.attributes.html")
			if html == "" {
				if err := requireTemplateCompliance(c, jsonAPIID(template)); err != nil {
					return fmt.Errorf("campaign message %s: %w", message.ID, err)
				}
				continue
			}
			if status, detail := unsubscribeCompliance(html); status != "pass" {
				return fmt.Errorf("campaign compliance preflight failed for message %s: %s", message.ID, detail)
			}
		}
		if page.Links.Next == "" {
			break
		}
		next, err := url.Parse(page.Links.Next)
		if err != nil || next.Path != "/api/campaigns/"+url.PathEscape(campaignID)+"/campaign-messages" {
			return fmt.Errorf("campaign compliance preflight failed: invalid next campaign message page")
		}
		endpoint = next.RequestURI()
		query = nil
	}
	if checked == 0 {
		return fmt.Errorf("campaign compliance preflight failed: campaign has no readable message")
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
