package gplay

import (
	"context"
	"fmt"
)

// Permissions returns the declared Android permissions for an app via the
// xdSrCf RPC, grouped by permission group.
func (c *Client) Permissions(ctx context.Context, appID string) ([]Permission, error) {
	if appID == "" {
		return nil, fmt.Errorf("appId is required")
	}
	inner := fmt.Sprintf(`[[null,[%q,7],[]]]`, appID)
	payload, err := c.batchExecute(ctx, "xdSrCf", inner, "1", "")
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, nil
	}
	root := decode(payload)
	var out []Permission
	// Payload shape: [<groups-with-icons>, <more-groups>, <uncategorized>].
	// Sections [0] and [1] each hold groups [name, icon, [[null,"perm"],...]];
	// section [2] is a flat list of uncategorized [null,"perm"] entries.
	for _, section := range []node{root.at(0), root.at(1)} {
		for _, grp := range section.arr() {
			group := grp.path(0).cleanStr()
			for _, p := range grp.path(2).arr() {
				if perm := p.path(1).cleanStr(); perm != "" {
					out = append(out, Permission{Group: group, Permission: perm})
				}
			}
		}
	}
	for _, p := range root.path(2).arr() {
		if perm := p.path(1).cleanStr(); perm != "" {
			out = append(out, Permission{Group: "Other", Permission: perm})
		}
	}
	return out, nil
}
