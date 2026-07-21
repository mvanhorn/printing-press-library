// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestExtractIDFromCreateResponse(t *testing.T) {
	cases := []struct {
		name   string
		resp   string
		keys   []string
		wantID string
	}{
		{
			name:   "flat id field",
			resp:   `{"id": "42", "name": "Brand"}`,
			keys:   []string{"id", "campaignId"},
			wantID: "42",
		},
		{
			name:   "flat campaignId field",
			resp:   `{"campaignId": "99"}`,
			keys:   []string{"id", "campaignId"},
			wantID: "99",
		},
		{
			name:   "data envelope",
			resp:   `{"data": {"id": "77", "name": "Wrapped"}}`,
			keys:   []string{"id", "campaignId"},
			wantID: "77",
		},
		{
			name:   "numeric id coerced to string",
			resp:   `{"id": 123}`,
			keys:   []string{"id"},
			wantID: "123",
		},
		{
			name:   "unparseable response returns empty",
			resp:   `not json at all`,
			keys:   []string{"id", "campaignId"},
			wantID: "",
		},
		{
			name:   "empty object returns empty",
			resp:   `{}`,
			keys:   []string{"id", "campaignId"},
			wantID: "",
		},
		{
			name:   "data envelope missing id returns empty",
			resp:   `{"data": {"name": "no-id"}}`,
			keys:   []string{"id", "campaignId"},
			wantID: "",
		},
		{
			name:   "adgroup keys",
			resp:   `{"data": {"adGroupId": "55"}}`,
			keys:   []string{"id", "adGroupId"},
			wantID: "55",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIDFromCreateResponse(json.RawMessage(tc.resp), tc.keys...)
			if got != tc.wantID {
				t.Errorf("want %q, got %q", tc.wantID, got)
			}
		})
	}
}
