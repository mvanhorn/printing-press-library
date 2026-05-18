// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.
// PATCH(retro #marianatek-browser-auth): tests for the cookie-blob parser.

package cli

import (
	"encoding/json"
	"testing"
)

func TestMTCookieBlobParsing(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantToken   string
		wantType    string
		wantErr     bool
	}{
		{
			name:      "happy path with full tokenData",
			input:     `{"expires":1700000000000,"tokenData":{"accessToken":"abc123","refreshToken":"r","tokenType":"Bearer","scope":"read","expiresIn":3600}}`,
			wantToken: "abc123",
			wantType:  "Bearer",
		},
		{
			name:      "minimal blob, only accessToken",
			input:     `{"tokenData":{"accessToken":"x"}}`,
			wantToken: "x",
		},
		{
			name:    "malformed JSON",
			input:   `{not json`,
			wantErr: true,
		},
		{
			name:      "empty accessToken",
			input:     `{"tokenData":{"accessToken":""}}`,
			wantToken: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var blob mtCookieBlob
			err := json.Unmarshal([]byte(c.input), &blob)
			if (err != nil) != c.wantErr {
				t.Fatalf("err mismatch: got %v, wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if blob.TokenData.AccessToken != c.wantToken {
				t.Errorf("accessToken: got %q, want %q", blob.TokenData.AccessToken, c.wantToken)
			}
			if c.wantType != "" && blob.TokenData.TokenType != c.wantType {
				t.Errorf("tokenType: got %q, want %q", blob.TokenData.TokenType, c.wantType)
			}
		})
	}
}
