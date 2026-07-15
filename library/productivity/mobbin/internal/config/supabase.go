// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Mobbin API-truth: the Supabase access token lives in an
// SSR cookie that Supabase either stores whole (`<name>`) or splits across
// numbered chunks (`<name>.0`, `<name>.1`, ...) by byte length. The chunks are
// concatenated in index order and the value is base64/JSON-decoded to recover
// the bearer token used against Supabase PostgREST endpoints (collections,
// workspaces).
const supabaseAnonKeyDefault = "sb_publishable_YptnKskI90SD2g25sAvVxQ_tZltjYFE"
const SupabaseHost = "ujasntkfphywizsdaapi.supabase.co"
const supabaseAuthCookieName = "sb-ujasntkfphywizsdaapi-auth-token"

// SupabaseAnonKey returns the publishable Supabase key, overridable via the
// MOBBIN_SUPABASE_ANON_KEY env var so a rotated key needs no new release.
func SupabaseAnonKey() string {
	if v := os.Getenv("MOBBIN_SUPABASE_ANON_KEY"); v != "" {
		return v
	}
	return supabaseAnonKeyDefault
}

// SupabaseAccessToken reassembles the split SSR cookie chunks into the bearer
// token for Supabase PostgREST. The stored cookie credential lives in
// c.AccessToken (see CookieCredential).
func (c *Config) SupabaseAccessToken() (string, error) {
	if c.AccessToken == "" {
		return "", fmt.Errorf("supabase session not initialized")
	}

	chunks := map[string]string{}
	for _, part := range strings.Split(c.AccessToken, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		chunks[name] = value
	}

	// Chrome Cookie headers are unordered; reassemble by cookie suffix rather
	// than by header position. Accept either a single unchunked cookie or an
	// arbitrary-length `.0`, `.1`, ... chunk sequence (stop at the first gap),
	// then strip the single leading `base64-` marker on the whole value.
	var encoded string
	if whole, ok := chunks[supabaseAuthCookieName]; ok {
		encoded = whole
	} else if _, ok := chunks[supabaseAuthCookieName+".0"]; ok {
		var b strings.Builder
		for i := 0; ; i++ {
			chunk, ok := chunks[fmt.Sprintf("%s.%d", supabaseAuthCookieName, i)]
			if !ok {
				break
			}
			b.WriteString(chunk)
		}
		encoded = b.String()
	} else {
		return "", fmt.Errorf("supabase session not initialized")
	}
	encoded = strings.TrimPrefix(encoded, "base64-")

	var decoded []byte
	var err error
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		if decoded, err = enc.DecodeString(encoded); err == nil {
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}

	var session struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(decoded, &session); err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}
	if session.AccessToken == "" {
		return "", fmt.Errorf("supabase session access_token missing")
	}
	return session.AccessToken, nil
}
