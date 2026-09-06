// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// Bing Webmaster API response normalization. Hand-authored (not generated):
// every Bing JSON response wraps its payload in a WCF `{"d": ...}` envelope and
// serializes dates in the Microsoft ASP.NET-AJAX `/Date(ms±offset)/` format.
// These helpers run once in client.do() so every command, the local store, and
// the transcendence commands all see clean, unwrapped data with RFC3339 dates.

package client

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"time"
)

// msDateRe matches Microsoft ASP.NET-AJAX date literals as they appear inside
// JSON strings, e.g. "\/Date(1316156400000-0700)\/" or "/Date(1316156400000)/".
// The forward slashes may or may not be backslash-escaped depending on the
// serializer, and the trailing timezone offset is optional.
var msDateRe = regexp.MustCompile(`\\?/Date\((-?\d+)([+-]\d{4})?\)\\?/`)

// normalizeMSDates rewrites Microsoft date literals into RFC3339 UTC timestamps
// so JSON output and the local store carry readable, sortable dates instead of
// opaque epoch wrappers. Bodies with no date literal are returned untouched.
func normalizeMSDates(body []byte) []byte {
	if !bytes.Contains(body, []byte("Date(")) {
		return body
	}
	return msDateRe.ReplaceAllFunc(body, func(m []byte) []byte {
		sub := msDateRe.FindSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		ms, err := strconv.ParseInt(string(sub[1]), 10, 64)
		if err != nil {
			return m
		}
		return []byte(time.UnixMilli(ms).UTC().Format(time.RFC3339))
	})
}

// unwrapBingEnvelope strips the WCF `{"d": <payload>}` wrapper that every Bing
// Webmaster API response uses, returning the inner payload. Bodies that are not
// a single-key "d" object — including the unwrapped {"ErrorCode","Message"}
// error shape and void `{"d":null}` results handled by the caller — are
// returned unchanged. A `{"d":null}` response yields the literal `null`.
func unwrapBingEnvelope(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return body
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return body
	}
	d, ok := obj["d"]
	if !ok || len(obj) != 1 {
		return body
	}
	return []byte(d)
}
