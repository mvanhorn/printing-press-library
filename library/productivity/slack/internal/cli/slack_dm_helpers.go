// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/client"
)

func slackUserAuthHeaders() (map[string]string, error) {
	token := os.Getenv("SLACK_USER_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("SLACK_USER_TOKEN is required for Slack DM or user-scoped history reads")
	}
	return map[string]string{"Authorization": "Bearer " + token}, nil
}

func slackIsDMChannel(channel string) bool {
	return strings.HasPrefix(channel, "D")
}

func slackResolveDMChannel(ctx context.Context, c *client.Client, userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user id is required")
	}
	headers, err := slackUserAuthHeaders()
	if err != nil {
		return "", err
	}
	oldNoCache := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = oldNoCache }()

	data, _, err := c.PostWithHeaders(ctx, "/conversations.open", map[string]any{
		"users":            userID,
		"return_im":        true,
		"prevent_creation": true,
	}, headers)
	if err != nil {
		return "", err
	}
	if err := checkSlackAPIError(data); err != nil {
		return "", err
	}
	var resp struct {
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parsing conversations.open response: %w", err)
	}
	if resp.Channel.ID == "" {
		return "", fmt.Errorf("conversations.open returned no channel id")
	}
	return resp.Channel.ID, nil
}

func slackHistoryEnvelope(ctx context.Context, c *client.Client, channel string, params map[string]string) (json.RawMessage, error) {
	if channel == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	params["channel"] = channel
	headers, err := slackUserAuthHeaders()
	if err != nil {
		return nil, err
	}
	oldNoCache := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = oldNoCache }()
	data, err := c.GetWithHeaders(ctx, "/conversations.history", params, headers)
	if err != nil {
		return nil, err
	}
	if err := checkSlackAPIError(data); err != nil {
		return nil, err
	}
	return data, nil
}

func slackEnvelopeMessages(env json.RawMessage) (json.RawMessage, error) {
	var parsed struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(env, &parsed); err != nil {
		return nil, fmt.Errorf("parsing conversations.history response: %w", err)
	}
	if parsed.Messages == nil {
		parsed.Messages = []json.RawMessage{}
	}
	return json.Marshal(parsed.Messages)
}
