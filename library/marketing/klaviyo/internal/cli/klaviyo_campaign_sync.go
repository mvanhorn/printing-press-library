// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

type campaignSyncClient struct {
	reader interface {
		Get(context.Context, string, map[string]string) (json.RawMessage, error)
		RateLimit() float64
	}
	db       *store.Store
	resource string
	messages *campaignMessageReader
}

type campaignSyncMessagesKey struct{}

func (c *campaignSyncClient) PlatformSession() *platform.Session {
	if bound, ok := c.reader.(interface{ PlatformSession() *platform.Session }); ok {
		return bound.PlatformSession()
	}
	return nil
}

func (c *campaignSyncClient) RateLimit() float64 { return c.reader.RateLimit() }

func (c *campaignSyncClient) Get(ctx context.Context, endpoint string, params map[string]string) (json.RawMessage, error) {
	raw, err := c.reader.Get(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if strings.TrimRight(u.Path, "/") != "/api/"+c.resource {
		return raw, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	var data []map[string]any
	if err := json.Unmarshal(envelope["data"], &data); err != nil {
		return nil, err
	}
	for i, item := range data {
		messages := []map[string]any{item}
		if c.resource == "campaigns" {
			id := stringFromMapPath(item, "id")
			if id == "" {
				return nil, fmt.Errorf("sync campaigns: missing container ID")
			}
			messages, err = campaignCollection(c.reader, "/api/campaigns/"+url.PathEscape(id)+"/campaign-messages", nil)
			if err != nil {
				return nil, err
			}
		}
		for _, message := range messages {
			enriched, err := c.messageRow(message)
			if err != nil {
				return nil, err
			}
			if c.resource == "campaigns" {
				enriched["campaign_id"] = stringFromMapPath(item, "id")
			}
			if c.resource == "campaign-messages" {
				data[i] = enriched
			} else {
				encoded, err := json.Marshal(enriched)
				if err != nil {
					return nil, err
				}
				if err := c.db.UpsertCampaignMessages(encoded); err != nil {
					return nil, err
				}
			}
		}
	}
	envelope["data"], err = json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope)
}

func (c *campaignSyncClient) messageRow(message map[string]any) (map[string]any, error) {
	if updated := stringFromMapPath(message, "attributes.updated"); updated != "" {
		raw, err := c.db.Get("campaign-messages", stringFromMapPath(message, "id"))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			var stored map[string]any
			if err := json.Unmarshal(raw, &stored); err != nil {
				return nil, err
			}
			if stringFromMapPath(stored, "attributes.updated") == updated && stringFromMapPath(stored, "attributes.status") != "" && anyPath(stored, "attributes.audiences") != nil && len(anySlice(stored["variations"])) > 0 {
				return stored, nil
			}
		}
	}
	model, err := c.messages.resolve(message)
	if err != nil {
		return nil, err
	}
	return campaignAnalyticsRow(model), nil
}

// Existing campaigns rows use message IDs. Preserve them until a richer message row replaces the same key.
func localCampaignAnalyticsRows(ctx context.Context, db *store.Store) ([]json.RawMessage, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type IN ('campaigns', 'campaign-messages') ORDER BY CASE resource_type WHEN 'campaigns' THEN 0 ELSE 1 END, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]json.RawMessage{}
	order := []string{}
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			return nil, err
		}
		raw := json.RawMessage(stored)
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
		if stringFromMapPath(data, "attributes.status") == "" {
			continue
		}
		id := stringFromMapPath(data, "id")
		if id == "" {
			continue
		}
		if _, exists := byID[id]; !exists {
			order = append(order, id)
		}
		byID[id] = raw
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, nil
}
