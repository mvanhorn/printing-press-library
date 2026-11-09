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

// PATCH(greptile-review-g5): QA and send preflight must read the same complete message collection.
func fetchCampaignMessages(c templateReader, campaignID string) (json.RawMessage, error) {
	if campaignID == "" {
		return nil, fmt.Errorf("campaign compliance preflight failed: campaign ID is missing")
	}
	endpoint := "/api/campaigns/" + url.PathEscape(campaignID) + "/campaign-messages"
	seen := map[string]bool{}
	var messages []json.RawMessage
	var envelope map[string]json.RawMessage
	for {
		if seen[endpoint] {
			return nil, fmt.Errorf("campaign compliance preflight failed: repeated campaign message page")
		}
		seen[endpoint] = true
		resp, err := c.Get(endpoint, nil)
		if err != nil {
			return nil, classifyAPIError(err)
		}
		var page struct {
			Data  []json.RawMessage `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(resp, &page); err != nil {
			return nil, fmt.Errorf("campaign compliance preflight failed: invalid campaign messages: %w", err)
		}
		if page.Data == nil {
			return nil, fmt.Errorf("campaign compliance preflight failed: campaign message data is missing")
		}
		if envelope == nil {
			if err := json.Unmarshal(resp, &envelope); err != nil {
				return nil, err
			}
		}
		messages = append(messages, page.Data...)
		if page.Links.Next == "" {
			break
		}
		next, err := url.Parse(page.Links.Next)
		if err != nil || next.Path != "/api/campaigns/"+url.PathEscape(campaignID)+"/campaign-messages" {
			return nil, fmt.Errorf("campaign compliance preflight failed: invalid next campaign message page")
		}
		endpoint = next.RequestURI()
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("campaign compliance preflight failed: campaign has no readable message")
	}
	envelope["data"], _ = json.Marshal(messages)
	envelope["links"] = json.RawMessage(`{"next":null}`)
	return json.Marshal(envelope)
}

func requireCampaignCompliance(c templateReader, campaignID string) error {
	resp, err := fetchCampaignMessages(c, campaignID)
	if err != nil {
		return err
	}
	var messages struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Definition struct {
					Channel string `json:"channel"`
				} `json:"definition"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &messages); err != nil {
		return fmt.Errorf("campaign compliance preflight failed: invalid campaign messages: %w", err)
	}
	for _, message := range messages.Data {
		if message.ID == "" {
			return fmt.Errorf("campaign compliance preflight failed: campaign message ID is missing")
		}
		channel := message.Attributes.Definition.Channel
		switch channel {
		case "sms", "push", "whatsapp", "mobile_push":
			continue
		case "email":
		case "":
			return fmt.Errorf("campaign compliance preflight failed: campaign message %s channel is missing", message.ID)
		default:
			return fmt.Errorf("campaign compliance preflight failed: campaign message %s has unsupported channel %q", message.ID, channel)
		}
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
