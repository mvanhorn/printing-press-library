// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Clay exposes two APIs behind one host, with two credentials.
//
//	/v3         (app API)    authenticates with the `claysession` browser cookie
//	/public/v0  (public API) authenticates with a raw `clay-api-key` header
//
// Each API rejects the other's credential. The spec models only the cookie, so
// the public-API key is injected here, read from the environment at request
// time and sent verbatim (no Bearer scheme, which Clay's public API rejects).

package client

import (
	"os"
	"strings"
)

const (
	clayPublicAPIPrefix = "/public/"
	clayAPIKeyHeader    = "clay-api-key"
)

// clayAPIKeyEnvVars are checked in order for the public-API credential.
var clayAPIKeyEnvVars = []string{"CLAY_API_KEY", "CLAY_PUBLIC_API_KEY"}

// ClayIsPublicAPIPath reports whether a request path targets Clay's Public API.
func ClayIsPublicAPIPath(path string) bool {
	return strings.HasPrefix(path, clayPublicAPIPrefix)
}

// ClayPublicAPIKey returns the raw public-API key from the environment.
func ClayPublicAPIKey() string {
	for _, k := range clayAPIKeyEnvVars {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}
