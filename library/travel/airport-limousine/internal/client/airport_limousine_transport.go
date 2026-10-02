// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package client

import (
	"net/http"
	"time"
)

// ProviderHTTPClient shares the generated verified Chrome TLS transport, with TLS verification enabled.
func ProviderHTTPClient(timeout time.Duration, jar http.CookieJar) *http.Client {
	return newHTTPClient(timeout, jar, false)
}
