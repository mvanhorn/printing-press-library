// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package graphqlguard

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateReadOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		document string
		wantErr  bool
	}{
		{name: "anonymous query", document: `{ searchOffers { TotalHits } }`},
		{name: "named query", document: `query Deals { searchOffers { TotalHits } }`},
		{name: "fragment on type named Mutation", document: `query Deals { searchOffers { ...Fields } } fragment Fields on Mutation { id }`},
		{name: "operation and fields named mutation", document: `query Mutation { mutation { id } subscription }`},
		{name: "comments and strings", document: "# mutation ignored\nquery Deals { searchOffers(Filter:{Term:\"subscription mutation\"}) { TotalHits } }"},
		{name: "empty", document: "", wantErr: true},
		{name: "invalid syntax", document: `query {`, wantErr: true},
		{name: "mutation", document: `mutation AddThing { addThing { id } }`, wantErr: true},
		{name: "subscription", document: `subscription WatchThing { thingChanged { id } }`, wantErr: true},
		{name: "multiple operations", document: `query One { one } query Two { two }`, wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateReadOnly(tc.document)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateReadOnly() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "data", body: `{"data":{"searchOffers":{"TotalHits":1}}}`},
		{name: "explicit null data", body: `{"data":null}`, wantErr: "missing usable data"},
		{name: "errors", body: `{"data":null,"errors":[{"message":"Cannot query field"}]}`, wantErr: "GraphQL returned errors: Cannot query field"},
		{name: "blank error", body: `{"errors":[{}]}`, wantErr: "unspecified GraphQL error"},
		{name: "missing data", body: `{}`, wantErr: "missing usable data"},
		{name: "invalid json", body: `{`, wantErr: "invalid GraphQL response"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateResponse(json.RawMessage(tc.body))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateResponse() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateResponse() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
