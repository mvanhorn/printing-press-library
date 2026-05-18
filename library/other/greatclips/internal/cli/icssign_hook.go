// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/other/greatclips/internal/icssign"
)

// stylewareHost is the third-party ICS Net Check-In service that
// powers wait times, check-ins, status lookups, and cancellations for
// GreatClips. Every request to this host must carry the HMAC-signed
// ?t=<timestamp>&s=<signature> query parameters or the server returns
// HTTP 500.
const stylewareHost = "www.stylewaretouch.net"

// newICSSignHook returns a PreRequestHook that signs every outbound
// request to stylewareHost with the SPA's HMAC-SHA-256 scheme. Returns
// nil for any other host (so the hook is a no-op on the GreatClips
// customer webservices).
//
// The hook buffers the request body (single-read by default) before
// signing and replaces req.Body with a fresh reader so the live
// request can still send the body. Pre-existing query params on the
// request are preserved.
func newICSSignHook() func(*http.Request) error {
	return func(req *http.Request) error {
		if req == nil || req.URL == nil {
			return nil
		}
		if req.URL.Hostname() != stylewareHost {
			return nil
		}

		// Stylewaretouch.net does NOT use Bearer auth — the HMAC ?t=&s=
		// query params ARE the credential. Captured curl from the SPA
		// shows zero Authorization header on these hosts. Sending a
		// wrong-audience Bearer triggers HTTP 400 "Invalid request" at
		// the gateway, so we strip it here regardless of how the
		// upstream client decided to set it.
		req.Header.Del("Authorization")

		// Buffer the body for both signing and the actual send. The
		// stdlib http.Request.Body is a single-read io.ReadCloser; if
		// we read it for the signature we must replace it with a fresh
		// reader the transport can consume.
		var body []byte
		if req.Body != nil {
			buf, err := io.ReadAll(req.Body)
			if err != nil {
				return err
			}
			if cerr := req.Body.Close(); cerr != nil {
				return cerr
			}
			body = buf
			req.Body = io.NopCloser(bytes.NewReader(body))
			// Restore GetBody so the transport can retry after a 301
			// or similar redirect; without this, retries would see an
			// empty body.
			cloned := body
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(cloned)), nil
			}
		}

		t, s := icssign.SignRequest(icssign.Timestamp(), string(body))
		// PATCH(remove-debug-sign-dump): the GREATCLIPS_DEBUG_SIGN=1 stderr
		// dump (request body + outgoing HMAC signature) was kept in the
		// prior commit with a "remove before publish" note. Deleted now per
		// greptile P2 so request signatures aren't captured by shell history
		// or CI log collectors when the env var is set.

		// Append t and s to the existing query string, preserving any
		// other params that were already on the URL.
		// The SPA writes the URL as ?t=...&s=... (timestamp first). url.Values
		// sorts alphabetically on Encode so we get ?s=...&t=...; we use
		// RawQuery directly to preserve the SPA's order in case the server
		// is order-sensitive.
		existing := req.URL.RawQuery
		signed := "t=" + t + "&s=" + s
		if existing == "" {
			req.URL.RawQuery = signed
		} else {
			req.URL.RawQuery = existing + "&" + signed
		}
		return nil
	}
}

// installICSSignHook attaches the stylewaretouch.net signer to the
// shared client. Callers should invoke this once at startup after
// the client is constructed.
//
// The pre-existing client.PreRequestHook (if any) is wrapped, not
// replaced.
//
// Signature uses an anonymous type to avoid importing internal/client
// here (the hook itself lives below in the package and only needs to
// satisfy the func type the client struct declares).
func installICSSignHook(setter func(func(*http.Request) error)) {
	setter(newICSSignHook())
}

// HostFromRequest returns the host portion of a request's URL.
// Exposed for unit tests that exercise the hook against a synthetic
// request.
func hostFromRequest(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	if req.URL.Host != "" {
		return req.URL.Hostname()
	}
	return req.Host
}

// ensure imports are referenced (silence linters when the package is
// pruned at build time).
var _ = url.URL{}
