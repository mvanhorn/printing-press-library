// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Custom HTTP/2 client with TLS handshake fingerprinting via utls. Used by
// the calendar endpoint (dates_native.go) to defend against Google flipping
// on TLS-based anti-bot detection. Empirically the calendar endpoint accepts
// vanilla net/http today, but fli's Python equivalent uses curl_cffi with
// chrome impersonation — fli's authors found it necessary, so we match.
//
// We use HTTP/2 because Chrome's preset advertises both `h2` and `http/1.1`
// in ALPN and Google picks `h2`. Speaking h2 properly is also more
// Chrome-like; falling back to h1-only would require mutating the preset
// and would still look slightly less authentic.
//
// Asymmetry note: krisukox/google-flights-api (used by Search) does NOT take
// a custom http.Client. So Search() still hits Google with vanilla Go's TLS.
// If Google enables fingerprint enforcement, Search will break first; that
// signals time to either fork krisukox or vendor it with utls.

package gflights

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// utlsClient returns the package-shared HTTP client for calendar requests.
// Shared so the underlying http2.Transport pools TLS connections across
// chunked requests AND across per-destination fanout in cheapest-longhaul
// (~30 destinations × 2 chunks each = 60 handshakes if not pooled). Caller-
// supplied context.Context handles per-request deadlines, so the client has
// no Timeout — Timeout would race with longer chunked queries.
//
// The TLS handshake fingerprints as Chrome (via utls), application protocol
// is HTTP/2 (via x/net/http2). The vanilla net/http stack can't be used here:
// (1) http.Transport's HTTP/2 path bypasses DialTLSContext and does its own
// TLS, so we'd never get utls. (2) Chrome's preset always negotiates h2;
// without HTTP/2 wiring an http.Transport receives an h2 SETTINGS frame and
// fails with "malformed HTTP response" — observed empirically while building
// this.
var utlsClient = sync.OnceValue(func() *http.Client {
	transport := &http2.Transport{
		// http2's DialTLSContext signature includes *tls.Config; we ignore it
		// and use our own utls.Config so the handshake fingerprints as Chrome.
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return dialUTLS(ctx, network, addr)
		},
	}
	return &http.Client{Transport: transport}
})

// dialUTLS performs the TCP connect, then runs a Chrome-fingerprinted TLS
// handshake via utls. Returns a *utls.UConn that http2.Transport reads
// plaintext through.
func dialUTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	rawConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	cfg := &utls.Config{
		ServerName: host,
		// We ALWAYS speak h2 with this client. The preset's ALPN extension
		// advertises [h2, http/1.1] and Google picks h2; setting NextProtos
		// here would be ignored anyway because the preset's hello dictates
		// the wire bytes. Documented for future readers.
		NextProtos: []string{"h2"},
	}
	conn := utls.UClient(rawConn, cfg, utls.HelloChrome_Auto)

	if err := conn.HandshakeContext(ctx); err != nil {
		_ = rawConn.Close()
		var deadlineErr interface{ Timeout() bool }
		if errors.As(err, &deadlineErr) && deadlineErr.Timeout() {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return conn, nil
}
