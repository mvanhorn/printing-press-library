// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
)

// TestDecodeBinaryResponseEnvelope_RoundTrip proves the sales-reports write
// path emits the raw gzip bytes Apple returned, not the client's _pp_binary
// JSON envelope: wrap a real gzip payload the way client.wrapBinaryResponse
// does, decode it, and gunzip the result.
func TestDecodeBinaryResponseEnvelope_RoundTrip(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write([]byte("Provider\tProvider Country\tSKU\n")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	envelope, err := json.Marshal(map[string]any{
		"_pp_binary":   true,
		"content_type": "application/a-gzip",
		"encoding":     "base64",
		"bytes":        gz.Len(),
		"data":         base64.StdEncoding.EncodeToString(gz.Bytes()),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	raw, ok := decodeBinaryResponseEnvelope(envelope)
	if !ok {
		t.Fatalf("decodeBinaryResponseEnvelope(envelope) ok = false, want true")
	}
	if !bytes.Equal(raw, gz.Bytes()) {
		t.Fatalf("decoded bytes differ from original gzip payload (%d vs %d bytes)", len(raw), gz.Len())
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decoded payload is not valid gzip: %v", err)
	}
	zr.Close()
}

// TestDecodeBinaryResponseEnvelope_PassThrough proves non-envelope payloads
// (dry-run provenance, JSON error bodies, plain API JSON) are left untouched.
func TestDecodeBinaryResponseEnvelope_PassThrough(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"plain json", `{"data":[{"type":"salesReports"}]}`},
		{"pp_binary false", `{"_pp_binary":false,"data":"aGk="}`},
		{"not json", "raw text, not an envelope"},
		{"unknown encoding", `{"_pp_binary":true,"encoding":"hex","data":"6869"}`},
		{"bad base64", `{"_pp_binary":true,"encoding":"base64","data":"!!!not-base64!!!"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := decodeBinaryResponseEnvelope([]byte(tc.in)); ok {
				t.Fatalf("decodeBinaryResponseEnvelope(%q) ok = true, want false", tc.in)
			}
		})
	}
}

// TestASCAuthEnvVerdict pins the doctor env_vars contract to the one the fleet
// commands enforce (ASCPreflight's JWT trio), covering the two failure modes
// the old bearer-token check produced: a false missing-credential report for a
// correctly-JWT-configured user, and a false OK for a bearer-only user who
// would then hit "App Store Connect API key not configured" in cockpit.
func TestASCAuthEnvVerdict(t *testing.T) {
	missingAll := []string{"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH (or ASC_PRIVATE_KEY)"}
	cases := []struct {
		name           string
		trioMissing    []string
		bearerSet      bool
		authConfigured bool
		authSource     string
		preflightErr   error
		wantPrefix     string
		wantContains   []string
	}{
		{
			name:       "jwt trio verified",
			wantPrefix: "OK ",
			wantContains: []string{
				"ASC_KEY_ID", "fleet",
			},
		},
		{
			name:         "jwt trio set but key unusable",
			preflightErr: fmt.Errorf("reading ASC_PRIVATE_KEY_PATH (/tmp/nope.p8): no such file"),
			wantPrefix:   "ERROR invalid ASC API key",
			wantContains: []string{"/tmp/nope.p8"},
		},
		{
			name:           "bearer only is not fleet-ready",
			trioMissing:    missingAll,
			bearerSet:      true,
			authConfigured: true,
			authSource:     "env:APP_STORE_CONNECT_ITC_BEARER_TOKEN",
			wantPrefix:     "WARN ",
			wantContains:   []string{"cockpit", "ASC_KEY_ID", "ASC_ISSUER_ID"},
		},
		{
			name:           "config credentials without key trio",
			trioMissing:    missingAll,
			authConfigured: true,
			authSource:     "config",
			wantPrefix:     "WARN ",
			wantContains:   []string{"config", "ASC_KEY_ID"},
		},
		{
			name:         "nothing configured",
			trioMissing:  missingAll,
			wantPrefix:   "ERROR missing required: ",
			wantContains: []string{"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ascAuthEnvVerdict(tc.trioMissing, tc.bearerSet, tc.authConfigured, tc.authSource, tc.preflightErr)
			if len(got) < len(tc.wantPrefix) || got[:len(tc.wantPrefix)] != tc.wantPrefix {
				t.Fatalf("verdict = %q, want prefix %q", got, tc.wantPrefix)
			}
			for _, want := range tc.wantContains {
				if !bytes.Contains([]byte(got), []byte(want)) {
					t.Fatalf("verdict = %q, missing %q", got, want)
				}
			}
		})
	}
}
