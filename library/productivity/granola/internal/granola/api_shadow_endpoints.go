// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Internal API endpoints discovered via static analysis of Granola desktop
// 7.867.0 and verified live (read-only HTTP 200) across Granola's backend
// shards.
const (
	userInfoEndpoint              = "/v1/get-user-info"
	subscriptionsEndpoint         = "/v1/get-subscriptions"
	userSessionsEndpoint          = "/v1/list-user-sessions"
	workspaceMembersEndpoint      = "/v1/get-workspace-members"
	workspacesNotesCountEndpoint  = "/v1/get-workspaces-notes-count"
	peopleEndpoint                = "/v1/get-people"
	preMeetingBriefsEndpoint      = "/v1/get-pre-meeting-briefs"
	followUpEmailsEndpoint        = "/v1/get-follow-up-emails"
	mcpAgentsEndpoint             = "/v1/get-mcp-agents"
	availableIntegrationsEndpoint = "/v1/list-available-integrations"
)

func workspaceHeader(workspaceID string) map[string]string {
	ws := strings.TrimSpace(workspaceID)
	if ws == "" {
		return nil
	}
	return map[string]string{"X-Granola-Workspace-Id": ws}
}

func decodeRawJSON(endpoint string, raw []byte) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, shapeError(endpoint, raw)
	}
	return json.RawMessage(raw), nil
}

// GetUserInfo calls POST https://api.granola.ai/v1/get-user-info.
func (c *InternalClient) GetUserInfo() (json.RawMessage, error) {
	raw, err := c.postTo(InternalBaseURL, userInfoEndpoint, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(userInfoEndpoint, raw)
}

// GetSubscriptions calls POST https://api.granola.ai/v1/get-subscriptions.
func (c *InternalClient) GetSubscriptions() (json.RawMessage, error) {
	raw, err := c.postTo(InternalBaseURL, subscriptionsEndpoint, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(subscriptionsEndpoint, raw)
}

// ListUserSessions calls POST https://api.granola.ai/v1/list-user-sessions.
func (c *InternalClient) ListUserSessions() (json.RawMessage, error) {
	raw, err := c.postTo(InternalBaseURL, userSessionsEndpoint, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(userSessionsEndpoint, raw)
}

// GetWorkspaceMembers calls POST https://api.granola.ai/v1/get-workspace-members.
func (c *InternalClient) GetWorkspaceMembers(workspaceID string) (json.RawMessage, error) {
	ws := strings.TrimSpace(workspaceID)
	if ws == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	raw, err := c.postTo(InternalBaseURL, workspaceMembersEndpoint, map[string]any{
		"workspace_id": ws,
	}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(workspaceMembersEndpoint, raw)
}

// GetWorkspacesNotesCount calls POST https://api.granola.ai/v1/get-workspaces-notes-count.
func (c *InternalClient) GetWorkspacesNotesCount() (json.RawMessage, error) {
	raw, err := c.postTo(InternalBaseURL, workspacesNotesCountEndpoint, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(workspacesNotesCountEndpoint, raw)
}

// GetPeople calls POST https://pecan.api.granola.ai/v1/get-people.
func (c *InternalClient) GetPeople() (json.RawMessage, error) {
	raw, err := c.postTo(ShardBaseURLPecan, peopleEndpoint, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(peopleEndpoint, raw)
}

// GetPreMeetingBriefs calls POST https://maple.api.granola.ai/v1/get-pre-meeting-briefs.
func (c *InternalClient) GetPreMeetingBriefs() (json.RawMessage, error) {
	raw, err := c.postTo(ShardBaseURLMaple, preMeetingBriefsEndpoint, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(preMeetingBriefsEndpoint, raw)
}

// GetFollowUpEmails calls POST https://berry.api.granola.ai/v1/get-follow-up-emails.
func (c *InternalClient) GetFollowUpEmails() (json.RawMessage, error) {
	raw, err := c.postTo(ShardBaseURLBerry, followUpEmailsEndpoint, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(followUpEmailsEndpoint, raw)
}

// GetMCPAgents calls POST https://cinnamon.api.granola.ai/v1/get-mcp-agents.
func (c *InternalClient) GetMCPAgents(workspaceID string) (json.RawMessage, error) {
	raw, err := c.postTo(ShardBaseURLCinnamon, mcpAgentsEndpoint, map[string]any{}, workspaceHeader(workspaceID))
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(mcpAgentsEndpoint, raw)
}

// ListAvailableIntegrations calls POST https://chia.api.granola.ai/v1/list-available-integrations.
func (c *InternalClient) ListAvailableIntegrations(workspaceID string) (json.RawMessage, error) {
	raw, err := c.postTo(ShardBaseURLChia, availableIntegrationsEndpoint, map[string]any{}, workspaceHeader(workspaceID))
	if err != nil {
		return nil, err
	}
	return decodeRawJSON(availableIntegrationsEndpoint, raw)
}
