// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// groupsEnvelope is the wrapper returned by GET /v1/groups. Groups is flat
// (no members hydrated); call Group(ctx, id) for the singular payload.
type groupsEnvelope struct {
	Groups []Group `json:"groups"`
}

// Groups calls GET /v1/groups. Returns the flat list of groups the caller
// has access to. Free probe — no credits spent. Members are not hydrated on
// this endpoint; call Group(ctx, id) for the singular payload when you need
// membership.
func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	raw, err := c.do(ctx, http.MethodGet, "/groups", nil)
	if err != nil {
		return nil, err
	}
	var env groupsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("happenstance api: decoding /groups response: %w", err)
	}
	if env.Groups == nil {
		// Defensive: downstream code ranges over the slice.
		env.Groups = []Group{}
	}
	return env.Groups, nil
}

// Group calls GET /v1/groups/{id}. Returns the full group payload including
// Members. Free probe — no credits spent. Members contain display names only;
// further hydration requires a separate Research(ctx, memberName) call.
func (c *Client) Group(ctx context.Context, id string) (Group, error) {
	if strings.TrimSpace(id) == "" {
		return Group{}, errors.New("happenstance api: Group requires a non-empty group id")
	}
	path := "/groups/" + url.PathEscape(id)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Group{}, err
	}
	var g Group
	if err := json.Unmarshal(raw, &g); err != nil {
		return Group{}, fmt.Errorf("happenstance api: decoding /groups/%s response: %w", id, err)
	}
	if g.Members == nil {
		g.Members = []GroupMember{}
	}
	return g, nil
}
