// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored SimpleFIN access-URL resolver. Not generator-emitted; preserved
// across `generate --force` by the regen-merge novel-file path.

package config

import (
	"encoding/base64"
	"net/url"
	"strings"
)

// resolveSimpleFINAccess turns a SimpleFIN Access URL into a base URL plus an
// HTTP Basic Authorization header value.
//
// SimpleFIN has no API key. The single credential is an Access URL of the form
//
//	https://user:pass@host/simplefin
//
// which encodes BOTH the server host and the Basic-auth credentials. The
// generated AuthHeader() would otherwise base64-encode the entire URL (wrong:
// the Basic payload must be user:pass only) and BaseURL would stay at the
// bridge default even when the credential points at a different host — e.g. the
// beta demo bridge, whose credentials 401 against the production bridge.
//
// Setting AuthHeaderVal here means the generated AuthHeader() returns it
// directly (its first branch), and overriding BaseURL points every request at
// the host baked into the credential.
func (c *Config) resolveSimpleFINAccess() {
	raw := strings.TrimSpace(c.SimplefinAccessUrl)
	if raw == "" {
		return
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return
	}
	if u.User != nil {
		user := u.User.Username()
		pass, hasPass := u.User.Password()
		// HTTP Basic always uses "user:pass"; with no password the convention
		// is the trailing-colon form "user:".
		token := user + ":"
		if hasPass {
			token = user + ":" + pass
		}
		c.AuthHeaderVal = "Basic " + base64.StdEncoding.EncodeToString([]byte(token))
	}
	// Strip credentials from the URL so the base URL never carries secrets
	// (it would otherwise leak into cache keys, logs, and error messages).
	u.User = nil
	c.BaseURL = strings.TrimRight(u.String(), "/")
}
