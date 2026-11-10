// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
)

// Historical campaign IDs are now message IDs, including in reporting and stored events.
type campaignModel struct {
	Campaign map[string]any         `json:"campaign"`
	Messages []campaignModelMessage `json:"messages"`
}

type campaignModelMessage struct {
	ID         string                   `json:"id"`
	Name       string                   `json:"name"`
	Status     string                   `json:"status"`
	SendTime   string                   `json:"send_time"`
	CampaignID string                   `json:"campaign_id"`
	Audience   map[string]any           `json:"audience"`
	Variations []campaignModelVariation `json:"variations"`
	Data       map[string]any           `json:"-"`
}

type campaignModelVariation struct {
	ID          string `json:"id"`
	Channel     string `json:"channel"`
	TemplateID  string `json:"template_id"`
	Subject     string `json:"subject"`
	PreviewText string `json:"preview_text"`
	FromEmail   string `json:"from_email"`
	FromLabel   string `json:"from_label"`
	ReplyTo     string `json:"reply_to_email"`
}

func campaignNotFound(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

func campaignObject(c templateReader, endpoint string) (map[string]any, error) {
	raw, err := c.Get(klaviyoRequestContext(c), endpoint, nil)
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%s: %w", endpoint, err)
	}
	if stringFromMapPath(envelope.Data, "id") == "" {
		return nil, fmt.Errorf("%s: response has no resource ID", endpoint)
	}
	return envelope.Data, nil
}

// Follow complete relationship URLs; never infer completion from a short page.
func campaignCollection(c templateReader, endpoint string, params map[string]string) ([]map[string]any, error) {
	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	messageList := strings.TrimRight(base.Path, "/") == "/api/campaign-messages" || strings.TrimRight(base.Path, "/") == "/api/campaigns"
	if messageList {
		copied := map[string]string{"page[size]": "100"}
		for key, value := range params {
			if key != "page[size]" {
				copied[key] = value
			}
		}
		params = copied
	}
	filter := params["filter"]
	seen := map[string]bool{}
	items := []map[string]any{}
	for {
		key := reportPathWithParams(endpoint, params)
		if seen[key] {
			return nil, fmt.Errorf("repeated campaign page: %s", endpoint)
		}
		seen[key] = true
		raw, err := c.Get(klaviyoRequestContext(c), endpoint, params)
		if err != nil {
			return nil, classifyAPIErrorOnly(err)
		}
		var page struct {
			Data  []map[string]any `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("%s: %w", endpoint, err)
		}
		if page.Data == nil {
			return nil, fmt.Errorf("%s: campaign collection data is missing", endpoint)
		}
		for _, item := range page.Data {
			if stringFromMapPath(item, "id") == "" {
				return nil, fmt.Errorf("%s: resource ID is missing", endpoint)
			}
		}
		items = append(items, page.Data...)
		if page.Links.Next == "" {
			return items, nil
		}
		next, err := url.Parse(page.Links.Next)
		if err != nil || strings.TrimRight(next.Path, "/") != strings.TrimRight(base.Path, "/") || next.Fragment != "" {
			return nil, fmt.Errorf("invalid next campaign page: %q", page.Links.Next)
		}
		if messageList {
			query := next.Query()
			query.Set("page[size]", "100")
			if filter != "" && query.Get("filter") == "" {
				query.Set("filter", filter)
			}
			next.RawQuery = query.Encode()
		}
		endpoint, params = next.RequestURI(), nil
	}
}

func resolveCampaign(c templateReader, id string) (*campaignModel, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("campaign or message ID is required")
	}
	container, err := campaignObject(c, "/api/campaigns/"+url.PathEscape(id))
	var messages []map[string]any
	if campaignNotFound(err) {
		message, msgErr := campaignObject(c, "/api/campaign-messages/"+url.PathEscape(id))
		if msgErr != nil {
			return nil, msgErr
		}
		containerID := relationshipID(message, "campaign")
		if containerID != "" {
			container, err = campaignObject(c, "/api/campaigns/"+url.PathEscape(containerID))
		} else {
			container, err = campaignObject(c, "/api/campaign-messages/"+url.PathEscape(id)+"/campaign")
		}
		messages = []map[string]any{message}
	} else if err == nil {
		messages, err = campaignCollection(c, "/api/campaigns/"+url.PathEscape(id)+"/campaign-messages", nil)
	}
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("campaign %s has no readable messages", id)
	}
	model := &campaignModel{Campaign: container, Messages: []campaignModelMessage{}}
	reader := campaignMessageReader{client: c}
	for _, data := range messages {
		message, err := reader.resolve(data)
		if err != nil {
			return nil, err
		}
		message.CampaignID = stringFromMapPath(container, "id")
		if message.Name == "" {
			message.Name = stringFromMapPath(container, "attributes.definition.name")
		}
		model.Messages = append(model.Messages, message)
	}
	return model, nil
}

func resolveCampaignMessage(c templateReader, data map[string]any) (campaignModelMessage, error) {
	m := campaignModelMessage{ID: stringFromMapPath(data, "id"), Data: data, CampaignID: relationshipID(data, "campaign")}
	if m.ID == "" {
		return m, fmt.Errorf("campaign message ID is missing")
	}
	m.Name = stringFromMapPath(data, "attributes.definition.name")
	m.Status = stringFromMapPath(data, "attributes.definition.status")
	m.SendTime = stringFromMapPath(data, "attributes.definition.send_options.send_time")
	if m.Status == "" {
		return m, fmt.Errorf("campaign message %s status is missing", m.ID)
	}
	base := "/api/campaign-messages/" + url.PathEscape(m.ID)
	audience, err := campaignObject(c, base+"/campaign-audience")
	if err != nil {
		return m, fmt.Errorf("message %s audience: %w", m.ID, err)
	}
	m.Audience, _ = anyPath(audience, "attributes.definition").(map[string]any)
	if m.Audience == nil {
		return m, fmt.Errorf("message %s audience definition is missing", m.ID)
	}
	variations, err := campaignCollection(c, base+"/campaign-variations", nil)
	if err != nil {
		return m, err
	}
	if len(variations) == 0 {
		return m, fmt.Errorf("message %s has no readable variations", m.ID)
	}
	for _, data := range variations {
		prefix := "attributes.definition.details."
		v := campaignModelVariation{ID: stringFromMapPath(data, "id"), Channel: stringFromMapPath(data, prefix+"channel"), TemplateID: stringFromMapPath(data, prefix+"template_id"), Subject: stringFromMapPath(data, prefix+"subject"), PreviewText: stringFromMapPath(data, prefix+"preview_text"), FromEmail: stringFromMapPath(data, prefix+"from_email"), FromLabel: stringFromMapPath(data, prefix+"from_label"), ReplyTo: stringFromMapPath(data, prefix+"reply_to_email")}
		if v.Channel == "" {
			return m, fmt.Errorf("variation %s channel is missing", v.ID)
		}
		m.Variations = append(m.Variations, v)
	}
	return m, nil
}

// A reader belongs to one operation; mutation verification gets a fresh reader.
type campaignMessageReader struct {
	client   templateReader
	mu       sync.Mutex
	messages map[string]*campaignMessageResult
}

type campaignMessageResult struct {
	done    chan struct{}
	message campaignModelMessage
	err     error
}

func (r *campaignMessageReader) resolve(data map[string]any) (campaignModelMessage, error) {
	r.mu.Lock()
	id := stringFromMapPath(data, "id")
	if result, ok := r.messages[id]; ok {
		r.mu.Unlock()
		<-result.done
		return result.message, result.err
	}
	result := &campaignMessageResult{done: make(chan struct{})}
	if r.messages == nil {
		r.messages = map[string]*campaignMessageResult{}
	}
	r.messages[id] = result
	r.mu.Unlock()
	result.message, result.err = resolveCampaignMessage(r.client, data)
	close(result.done)
	return result.message, result.err
}

func campaignMessageRows(c templateReader, params map[string]string) ([]map[string]any, error) {
	reader := campaignMessageReader{client: c}
	return reader.rows(params)
}

func (r *campaignMessageReader) rows(params map[string]string) ([]map[string]any, error) {
	data, err := campaignCollection(r.client, "/api/campaign-messages", params)
	if err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	for _, item := range data {
		m, err := r.resolve(item)
		if err != nil {
			return nil, err
		}
		rows = append(rows, campaignAnalyticsRow(m))
	}
	return rows, nil
}

func campaignMessageRowsByID(c templateReader, ids []string) ([]map[string]any, error) {
	ids = uniqueStrings(nonEmptyStrings(ids))
	rows := []map[string]any{}
	reader := campaignMessageReader{client: c}
	for start := 0; start < len(ids); start += 100 {
		encoded, err := json.Marshal(ids[start:min(start+100, len(ids))])
		if err != nil {
			return nil, err
		}
		batch, err := reader.rows(map[string]string{"filter": fmt.Sprintf("any(id,%s)", encoded)})
		if err != nil {
			return nil, err
		}
		rows = append(rows, batch...)
	}
	return rows, nil
}

func campaignAnalyticsRow(m campaignModelMessage) map[string]any {
	// Keep the historical message key while exposing all variations to analytics.
	channels, subjects := []string{}, []string{}
	for _, v := range m.Variations {
		channels = append(channels, v.Channel)
		if v.Channel == "email" {
			subjects = append(subjects, v.Subject)
		}
	}
	attrs := map[string]any{}
	if original, ok := m.Data["attributes"].(map[string]any); ok {
		for k, v := range original {
			attrs[k] = v
		}
	}
	attrs["name"], attrs["status"], attrs["send_time"], attrs["audiences"] = m.Name, m.Status, m.SendTime, m.Audience
	attrs["channel"], attrs["subject"] = strings.Join(uniqueStrings(channels), ","), strings.Join(subjects, " | ")
	return map[string]any{"id": m.ID, "type": "campaign-message", "attributes": attrs, "campaign_id": m.CampaignID, "variations": m.Variations, "relationships": m.Data["relationships"]}
}

func verifyCampaignDraft(model *campaignModel) error {
	if len(model.Messages) == 0 {
		return fmt.Errorf("draft verification failed: campaign has no messages")
	}
	for _, m := range model.Messages {
		if m.Status != "draft" || m.SendTime != "" {
			return fmt.Errorf("draft verification failed: message %s status=%q send_time=%q; expected draft with no send_time", m.ID, m.Status, m.SendTime)
		}
	}
	return nil
}

func campaignTemplateHTML(c templateReader, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("email variation has no template_id")
	}
	raw, err := c.Get(klaviyoRequestContext(c), "/api/templates/"+url.PathEscape(id), map[string]string{"additional-fields[template]": "definition", "fields[template]": "definition,html"})
	if err != nil {
		return "", classifyAPIErrorOnly(err)
	}
	return firstString(raw, "data.attributes.html", "data.attributes.definition.html"), nil
}

func resolveAssignVariation(c templateReader, id string) (string, error) {
	_, err := campaignObject(c, "/api/campaign-variations/"+url.PathEscape(id))
	if err == nil {
		return id, nil
	}
	if !campaignNotFound(err) {
		return "", err
	}
	model, err := resolveCampaign(c, id)
	if err != nil {
		return "", err
	}
	if len(model.Messages) != 1 || model.Messages[0].ID != id {
		return "", fmt.Errorf("assign-template requires a variation ID or one message ID")
	}
	ids := []string{}
	for _, v := range model.Messages[0].Variations {
		if v.Channel == "email" {
			ids = append(ids, v.ID)
		}
	}
	if len(ids) != 1 {
		return "", fmt.Errorf("message %s has %d email variations; supply a variation ID explicitly", id, len(ids))
	}
	return ids[0], nil
}
