// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseContentsOptionsEnforcesRequestSchema(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantNil bool
		wantErr bool
	}{
		{name: "object", raw: `{"highlights":true}`},
		{name: "null", raw: `null`, wantNil: true},
		{name: "boolean", raw: `true`, wantErr: true},
		{name: "array", raw: `[]`, wantErr: true},
		{name: "string", raw: `"text"`, wantErr: true},
		{name: "number", raw: `1`, wantErr: true},
		{name: "invalid JSON", raw: `{`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseContentsOptions(test.raw)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseContentsOptions(%q) error = %v, wantErr %v", test.raw, err, test.wantErr)
			}
			if !test.wantErr && (got == nil) != test.wantNil {
				t.Fatalf("parseContentsOptions(%q) = %#v, wantNil %v", test.raw, got, test.wantNil)
			}
		})
	}
}
