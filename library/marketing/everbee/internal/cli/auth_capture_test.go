// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractEverbeeTokenFromURL(t *testing.T) {
	token := testJWT(t, time.Now().Add(time.Hour))
	got := extractEverbeeTokenFromURL("https://app.everbee.io/ext?token=" + token)
	if got != token {
		t.Fatalf("extractEverbeeTokenFromURL() = %q, want token", got)
	}
	if got := extractEverbeeTokenFromURL("https://app.everbee.io/other?token=" + token); got != "" {
		t.Fatalf("extractEverbeeTokenFromURL(non-ext) = %q, want empty", got)
	}
}

func TestExtractEverbeeTokenFromCDPMessage(t *testing.T) {
	token := testJWT(t, time.Now().Add(time.Hour))
	params := mustRawJSON(t, map[string]any{
		"request": map[string]any{
			"url": "https://api.everbee.com/users/show",
			"headers": map[string]any{
				"X-Access-Token": token,
			},
		},
	})
	got := extractEverbeeTokenFromCDPMessage(cdpMessage{
		Method: "Network.requestWillBeSent",
		Params: params,
	})
	if got.Token != token {
		t.Fatalf("captured token = %q, want token", got.Token)
	}
	if got.Source != "chrome-cdp:request-header" {
		t.Fatalf("source = %q, want chrome-cdp:request-header", got.Source)
	}
}

func TestOnePasswordExtensionInstalled(t *testing.T) {
	dir := t.TempDir()
	if onePasswordExtensionInstalled(dir) {
		t.Fatal("onePasswordExtensionInstalled(empty dir) = true, want false")
	}
	extensionDir := filepath.Join(dir, "Default", "Extensions", onePasswordExtensionID)
	if err := os.MkdirAll(extensionDir, 0o700); err != nil {
		t.Fatalf("mkdir extension dir: %v", err)
	}
	if !onePasswordExtensionInstalled(dir) {
		t.Fatal("onePasswordExtensionInstalled(profile with extension) = false, want true")
	}
}

func TestSummarizeCapturedToken(t *testing.T) {
	expires := time.Date(2026, 5, 30, 2, 7, 4, 0, time.UTC)
	token := testJWT(t, expires)
	hash, expiresAt := summarizeCapturedToken(token)
	if len(hash) != 12 {
		t.Fatalf("hash length = %d, want 12", len(hash))
	}
	if expiresAt != "2026-05-30T02:07:04Z" {
		t.Fatalf("expiresAt = %q, want RFC3339 expiration", expiresAt)
	}
}

func mustRawJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return raw
}

func testJWT(t *testing.T, expires time.Time) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS512"}`))
	payload := mustRawJSON(t, map[string]any{"exp": expires.Unix()})
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
