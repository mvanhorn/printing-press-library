// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Home Assistant compound workflows. They deliberately derive
// their answers from the live Home Assistant REST surface instead of local
// canned data so agent output always carries current household evidence.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/home-assistant/internal/client"
)

func householdStates(ctx context.Context, flags *rootFlags) ([]map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	raw, err := c.Get(ctx, "/api/states", nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var states []map[string]any
	if err := json.Unmarshal(raw, &states); err != nil {
		return nil, fmt.Errorf("decode Home Assistant states: %w", err)
	}
	return states, nil
}

func workflowOutput(cmdOutput any, flags *rootFlags, out interface{ Write([]byte) (int, error) }) error {
	raw, err := json.Marshal(cmdOutput)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(out, raw, flags, map[string]any{"source": "live"})
}

func entityID(state map[string]any) string   { v, _ := state["entity_id"].(string); return v }
func stateValue(state map[string]any) string { v, _ := state["state"].(string); return v }
func attributes(state map[string]any) map[string]any {
	v, _ := state["attributes"].(map[string]any)
	return v
}
func friendlyName(state map[string]any) string {
	v, _ := attributes(state)["friendly_name"].(string)
	return v
}

func matchEntity(states []map[string]any, query string) (map[string]any, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	var matches []map[string]any
	for _, item := range states {
		if strings.EqualFold(entityID(item), query) || strings.EqualFold(friendlyName(item), query) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return nil, notFoundErr(fmt.Errorf("no entity matches %q", query))
	}
	if len(matches) > 1 {
		return nil, usageErr(fmt.Errorf("%q matches %d entities; use an exact entity_id", query, len(matches)))
	}
	return matches[0], nil
}

func callHAService(ctx context.Context, flags *rootFlags, domain, service string, body map[string]any) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	raw, _, err := c.Post(ctx, "/api/services/"+domain+"/"+service, body)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	return raw, nil
}

func routineReferences(states []map[string]any, target string) []map[string]any {
	var refs []map[string]any
	for _, item := range states {
		id := entityID(item)
		if strings.HasPrefix(id, "automation.") || strings.HasPrefix(id, "script.") || strings.HasPrefix(id, "scene.") {
			encoded, _ := json.Marshal(item)
			if strings.Contains(string(encoded), target) {
				refs = append(refs, item)
			}
		}
	}
	return refs
}

func isUnavailable(s map[string]any) bool {
	return stateValue(s) == "unavailable" || stateValue(s) == "unknown"
}

var _ = client.ErrPlaceholderCredential
