// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// TestGamItemsForNetwork guards the store-read network scoping: a mirror keyed
// by resource type alone must not serve one network's cached rows when a
// different --network is requested.
func TestGamItemsForNetwork(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"name":"networks/111/adUnits/1","id":"1"}`),
		json.RawMessage(`{"name":"networks/222/adUnits/2","id":"2"}`),
		json.RawMessage(`{"name":"networks/111/adUnits/3","id":"3"}`),
		json.RawMessage(`{"id":"4"}`), // no name: never matches a non-empty network
	}

	tests := []struct {
		name    string
		network string
		wantIDs []string
	}{
		{"bare code filters to that network", "111", []string{"1", "3"}},
		{"networks-prefixed code normalizes", "networks/111", []string{"1", "3"}},
		{"other network", "222", []string{"2"}},
		{"prefix is not a substring match", "11", nil}, // must not match networks/111/
		{"unknown network yields none", "999", nil},
		{"empty network returns all unchanged", "", []string{"1", "2", "3", "4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gamItemsForNetwork(items, tt.network)
			gotIDs := make([]string, 0, len(got))
			for _, it := range got {
				var o struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal(it, &o)
				gotIDs = append(gotIDs, o.ID)
			}
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("network %q: got ids %v, want %v", tt.network, gotIDs, tt.wantIDs)
			}
			for i := range gotIDs {
				if gotIDs[i] != tt.wantIDs[i] {
					t.Fatalf("network %q: got ids %v, want %v", tt.network, gotIDs, tt.wantIDs)
				}
			}
		})
	}
}
