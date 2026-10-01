// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Sync support for tailnet-scoped resources. Every /tailnet/{tailnet}/...
// list resolves {tailnet} from config (default "-"), so the typed tables get
// a NOT NULL tailnet_id scope column. Most Tailscale objects (devices, keys,
// webhooks) carry no tailnet field of their own, so the store cannot derive
// the scope and every typed insert fails. Stamp the resolved tailnet onto
// each item before it is stored.

import (
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
)

// tsAttachTailnetScope adds tailnet_id to items synced from a
// /tailnet/{tailnet}/... path when the item has no tailnet reference. The
// value is the tailnet the request was sent to: the client's resolved
// {tailnet} template variable (config, --path-context, env, or "-").
func tsAttachTailnetScope(c any, resource string, items []json.RawMessage) []json.RawMessage {
	path, err := syncResourcePath(resource)
	if err != nil || !strings.Contains(path, "{tailnet}") {
		return items
	}
	tailnet := tsDefaultTailnet()
	if cc, ok := c.(*client.Client); ok && cc.Config != nil {
		if v := strings.TrimSpace(cc.Config.TemplateVars["tailnet"]); v != "" {
			tailnet = v
		}
	}
	scope, _ := json.Marshal(tailnet)
	out := make([]json.RawMessage, len(items))
	for i, item := range items {
		out[i] = item
		var obj map[string]json.RawMessage
		if json.Unmarshal(item, &obj) != nil || obj == nil {
			continue
		}
		if _, ok := obj["tailnet_id"]; ok {
			continue
		}
		if _, ok := obj["tailnetId"]; ok {
			continue
		}
		obj["tailnet_id"] = scope
		if b, err := json.Marshal(obj); err == nil {
			out[i] = b
		}
	}
	return out
}
