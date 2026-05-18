// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 shared cookie-JSON parser for cookie-tier adapters.

package source

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// CookieRecord is the on-disk shape captured by `auth login --chrome`.
type CookieRecord struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// ParseCookieJSON accepts either an array or {"cookies": [...]} envelope.
func ParseCookieJSON(raw []byte) ([]*http.Cookie, error) {
	var arr []CookieRecord
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return toHTTPCookies(arr), nil
	}
	var env struct {
		Cookies []CookieRecord `json:"cookies"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Cookies) > 0 {
		return toHTTPCookies(env.Cookies), nil
	}
	return nil, fmt.Errorf("cookie JSON has no recognizable shape (expected array or {cookies: [...]})")
}

func toHTTPCookies(recs []CookieRecord) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(recs))
	for _, r := range recs {
		out = append(out, &http.Cookie{Name: r.Name, Value: r.Value, Domain: r.Domain, Path: r.Path})
	}
	return out
}
