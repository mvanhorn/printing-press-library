// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateReadOnlyGraphQLRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		readOnly bool
		method   string
		path     string
		query    string
		wantErr  bool
	}{
		{name: "query", readOnly: true, method: "GET", path: "/graphql", query: `{ searchOffers { TotalHits } }`},
		{name: "mutation", readOnly: true, method: "GET", path: "/graphql", query: `mutation Bad { nope }`, wantErr: true},
		{name: "subscription", readOnly: true, method: "GET", path: "/graphql", query: `subscription Bad { nope }`, wantErr: true},
		{name: "missing query", readOnly: true, method: "GET", path: "/graphql", wantErr: true},
		{name: "non read-only handler", method: "GET", path: "/graphql", query: `mutation AllowedElsewhere { nope }`},
		{name: "other path", readOnly: true, method: "GET", path: "/health", query: `mutation NotGraphQL { nope }`},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateReadOnlyGraphQLRequest(tc.readOnly, tc.method, tc.path, map[string]string{"query": tc.query})
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateReadOnlyGraphQLRequest() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateReadOnlyGraphQLResponse(t *testing.T) {
	t.Parallel()
	if err := validateReadOnlyGraphQLResponse(true, "GET", "/graphql", json.RawMessage(`{"data":{"ok":true}}`)); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	err := validateReadOnlyGraphQLResponse(true, "GET", "/graphql", json.RawMessage(`{"data":null,"errors":[{"message":"resolver failed"}]}`))
	if err == nil || !strings.Contains(err.Error(), "resolver failed") {
		t.Fatalf("GraphQL error envelope result = %v", err)
	}
	if err := validateReadOnlyGraphQLResponse(false, "GET", "/graphql", json.RawMessage(`{"errors":[{}]}`)); err != nil {
		t.Fatalf("non-read-only response should not use Woot guard: %v", err)
	}
}
