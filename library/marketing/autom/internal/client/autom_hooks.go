// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written Autom extensions to the generated client. Kept in its own file
// so generate --force preserves it.

package client

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
)

// ResponseObserver, when set, is called after every response that reached the
// network and succeeded (status < 400). Cached responses never reach it. The
// CLI layer uses it to keep a local ledger of billed Autom calls.
var ResponseObserver func(method, path string, params map[string]string, status int)

func notifyResponseObserver(method, path string, params map[string]string, status int) {
	if ResponseObserver == nil {
		return
	}
	defer func() { _ = recover() }() // a ledger failure must never fail the request
	ResponseObserver(method, path, params, status)
}

// ExtraParams, when set, returns query parameters added to every request whose
// path it accepts (the CLI uses it for Autom's global `fields` projection).
// Explicit per-request parameters win over extras.
var ExtraParams func(path string) map[string]string

func withExtraParams(path string, params map[string]string) map[string]string {
	if ExtraParams == nil {
		return params
	}
	extra := ExtraParams(path)
	if len(extra) == 0 {
		return params
	}
	merged := make(map[string]string, len(params)+len(extra))
	for k, v := range extra {
		merged[k] = v
	}
	for k, v := range params {
		merged[k] = v
	}
	return merged
}

// isRateLimit401 reports Autom's per-second/per-minute rate-limit signal.
// Autom answers a rate limit with HTTP 401 and a JSON "Rate limit exceeded"
// body instead of 429, while a bad key is a 401 "Invalid API Key". Treating
// the first like a 429 gives it the retry and the rate-limit exit code instead
// of an auth failure. A key quota ("Rate limit exceeded for the API key: quota
// monthly") is excluded: waiting does not help until the quota resets, so it
// returns at once and the CLI layer reports it as a quota error.
func isRateLimit401(status int, body []byte) bool {
	if status != http.StatusUnauthorized {
		return false
	}
	lower := bytes.ToLower(body)
	return bytes.Contains(lower, []byte("rate limit")) && !bytes.Contains(lower, []byte("quota"))
}

// uncacheableRequest reports Autom requests whose answer changes between two
// identical calls, so the response cache must neither serve nor store them:
// async submissions (each call starts a new job and returns a new job_id) and
// job reads (a job moves from queued to running to done).
func uncacheableRequest(path string, params map[string]string) bool {
	route, rawQuery, _ := strings.Cut(path, "?")
	if route == "/v1/jobs" || strings.HasPrefix(route, "/v1/jobs/") {
		return true
	}
	if isTrueParam(params["is_async"]) {
		return true
	}
	if rawQuery != "" {
		if q, err := url.ParseQuery(rawQuery); err == nil && isTrueParam(q.Get("is_async")) {
			return true
		}
	}
	return false
}

func isTrueParam(v string) bool {
	v = strings.TrimSpace(v)
	return strings.EqualFold(v, "true") || v == "1"
}
