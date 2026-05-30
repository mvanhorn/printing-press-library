// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for novel-feature commands: cross-team enumeration, blueprint
// walking, and other utilities that span multiple commands. The generated
// per-endpoint command files stay one-per-file; this is the single home for
// glue used by hand-written transcendence features.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/make/internal/client"
)

// listOrgs returns the organizations visible to the token.
func listOrgs(ctx context.Context, c *client.Client) ([]map[string]any, error) {
	raw, err := c.Get(ctx, "/organizations", nil)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Organizations []map[string]any `json:"organizations"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Organizations, nil
}

// listTeams returns the teams in an organization.
func listTeams(ctx context.Context, c *client.Client, orgID int64) ([]map[string]any, error) {
	raw, err := c.Get(ctx, "/teams", map[string]string{"organizationId": strconv.FormatInt(orgID, 10)})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Teams []map[string]any `json:"teams"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Teams, nil
}

// listScenarios returns scenarios for a team.
func listScenarios(ctx context.Context, c *client.Client, teamID int64) ([]map[string]any, error) {
	raw, err := c.Get(ctx, "/scenarios", map[string]string{"teamId": strconv.FormatInt(teamID, 10)})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Scenarios []map[string]any `json:"scenarios"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Scenarios, nil
}

// listDLQs returns DLQ rows for one scenario.
func listDLQs(ctx context.Context, c *client.Client, scenarioID, teamID int64) ([]map[string]any, error) {
	params := map[string]string{
		"scenarioId": strconv.FormatInt(scenarioID, 10),
		"teamId":     strconv.FormatInt(teamID, 10),
	}
	raw, err := c.Get(ctx, "/dlqs", params)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Dlqs []map[string]any `json:"dlqs"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Dlqs, nil
}

// listConnections returns connections for a team.
func listConnections(ctx context.Context, c *client.Client, teamID int64) ([]map[string]any, error) {
	raw, err := c.Get(ctx, "/connections", map[string]string{"teamId": strconv.FormatInt(teamID, 10)})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Connections []map[string]any `json:"connections"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Connections, nil
}

// listHooks returns webhooks for a team.
func listHooks(ctx context.Context, c *client.Client, teamID int64) ([]map[string]any, error) {
	raw, err := c.Get(ctx, "/hooks", map[string]string{"teamId": strconv.FormatInt(teamID, 10)})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Hooks []map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.Hooks, nil
}

// listDataStores returns data stores for a team.
func listDataStores(ctx context.Context, c *client.Client, teamID int64) ([]map[string]any, error) {
	raw, err := c.Get(ctx, "/data-stores", map[string]string{"teamId": strconv.FormatInt(teamID, 10)})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		DataStores []map[string]any `json:"dataStores"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	return wrap.DataStores, nil
}

// getBlueprint fetches a scenario's blueprint (unwrapping the response envelope).
func getBlueprint(ctx context.Context, c *client.Client, scenarioID int64) (json.RawMessage, error) {
	raw, err := c.Get(ctx, "/scenarios/"+strconv.FormatInt(scenarioID, 10)+"/blueprint", nil)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Response json.RawMessage `json:"response"`
	}
	if json.Unmarshal(raw, &wrap) == nil && len(wrap.Response) > 0 {
		return wrap.Response, nil
	}
	return raw, nil
}

// allVisibleTeamIDs walks every org → team accessible to the token.
func allVisibleTeamIDs(ctx context.Context, c *client.Client) ([]int64, error) {
	orgs, err := listOrgs(ctx, c)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, o := range orgs {
		oid, _ := o["id"].(float64)
		if oid == 0 {
			continue
		}
		teams, err := listTeams(ctx, c, int64(oid))
		if err != nil {
			continue
		}
		for _, t := range teams {
			tid, _ := t["id"].(float64)
			if tid != 0 {
				ids = append(ids, int64(tid))
			}
		}
	}
	return ids, nil
}

// teamIDsFromFlags resolves --team / --all-teams into a slice. Returns
// (nil, nil) when neither is supplied so callers can produce a friendly error.
func teamIDsFromFlags(ctx context.Context, c *client.Client, team string, allTeams bool) ([]int64, error) {
	team = strings.TrimSpace(team)
	if team != "" {
		n, err := strconv.ParseInt(team, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid --team %q: %w", team, err)
		}
		return []int64{n}, nil
	}
	if allTeams {
		return allVisibleTeamIDs(ctx, c)
	}
	return nil, nil
}

// walkBlueprintWebhookRefs returns the hookIds referenced by gateway:CustomWebHook
// (and gateway:CustomMailHook) modules in a blueprint tree.
func walkBlueprintWebhookRefs(blueprint json.RawMessage) []int64 {
	return walkBlueprintModuleParam(blueprint, []string{"gateway:CustomWebHook", "gateway:CustomMailHook"}, "hook")
}

// walkBlueprintConnectionRefs returns the connection IDs referenced by any
// module whose parameters include `__IMTCONN__` (Make's connection-reference
// parameter key).
func walkBlueprintConnectionRefs(blueprint json.RawMessage) []int64 {
	var ids []int64
	visit := func(m map[string]any) {
		params, _ := m["parameters"].(map[string]any)
		if v, ok := params["__IMTCONN__"]; ok {
			if id := numberLike(v); id != 0 {
				ids = append(ids, id)
			}
		}
	}
	visitFlowModules(blueprint, visit)
	return uniqueInt64(ids)
}

// walkBlueprintDataStoreRefs collects dataStore IDs referenced in module params.
func walkBlueprintDataStoreRefs(blueprint json.RawMessage) []int64 {
	var ids []int64
	visit := func(m map[string]any) {
		params, _ := m["parameters"].(map[string]any)
		for k, v := range params {
			if strings.EqualFold(k, "datastore") || strings.HasSuffix(k, "DataStoreId") {
				if id := numberLike(v); id != 0 {
					ids = append(ids, id)
				}
			}
		}
	}
	visitFlowModules(blueprint, visit)
	return uniqueInt64(ids)
}

// walkBlueprintModuleParam returns numeric param values for modules whose
// `module` field matches any of moduleNames.
func walkBlueprintModuleParam(blueprint json.RawMessage, moduleNames []string, paramName string) []int64 {
	var ids []int64
	want := map[string]bool{}
	for _, n := range moduleNames {
		want[n] = true
	}
	visit := func(m map[string]any) {
		name, _ := m["module"].(string)
		if !want[name] {
			return
		}
		params, _ := m["parameters"].(map[string]any)
		if v, ok := params[paramName]; ok {
			if id := numberLike(v); id != 0 {
				ids = append(ids, id)
			}
		}
	}
	visitFlowModules(blueprint, visit)
	return uniqueInt64(ids)
}

// visitFlowModules walks `blueprint.flow[]` plus nested `routes[].flow[]`
// emitting each module map to fn.
func visitFlowModules(blueprint json.RawMessage, fn func(map[string]any)) {
	var top map[string]any
	if err := json.Unmarshal(blueprint, &top); err != nil {
		return
	}
	bp, _ := top["blueprint"].(map[string]any)
	if bp == nil {
		bp = top
	}
	walkFlow(bp["flow"], fn)
}

func walkFlow(flow any, fn func(map[string]any)) {
	arr, ok := flow.([]any)
	if !ok {
		return
	}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn(m)
		// Recurse into router / iterator structures.
		if routes, ok := m["routes"].([]any); ok {
			for _, r := range routes {
				rm, ok := r.(map[string]any)
				if !ok {
					continue
				}
				walkFlow(rm["flow"], fn)
			}
		}
		walkFlow(m["flow"], fn)
	}
}

func numberLike(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

func uniqueInt64(in []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(in))
	for _, n := range in {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// canonicalBlueprintJSON returns a stable JSON encoding suitable for diffing.
// Drops the noisy `metadata.expect/restore/designer` sub-trees that round-trip
// without affecting behavior; their stable form is written separately by
// blueprint sync as a sidecar.
func canonicalBlueprintJSON(blueprint json.RawMessage, keepMetadata bool) ([]byte, error) {
	var top any
	if err := json.Unmarshal(blueprint, &top); err != nil {
		return nil, err
	}
	if !keepMetadata {
		stripMetadataNoise(top)
	}
	return json.MarshalIndent(top, "", "  ")
}

func stripMetadataNoise(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if k == "metadata" {
				if m, ok := val.(map[string]any); ok {
					delete(m, "expect")
					delete(m, "restore")
					delete(m, "designer")
				}
			}
			stripMetadataNoise(val)
		}
	case []any:
		for _, item := range x {
			stripMetadataNoise(item)
		}
	}
}
