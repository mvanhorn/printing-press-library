package client

import (
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
	"net/http"
	"strings"
	"time"
)

type browserTransport struct {
	browser  http.RoundTripper
	fallback http.RoundTripper
	err      error
}

func (t *browserTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Hostname(), "japantravel.navitime.com") {
		return t.fallback.RoundTrip(req)
	}
	if t.err != nil {
		return nil, t.err
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Del("Cookie")
	clone.Header.Del("Authorization")
	clone.Header.Del("User-Agent")
	return t.browser.RoundTrip(clone)
}
func (t *browserTransport) CloseIdleConnections() {
	for _, v := range []http.RoundTripper{t.browser, t.fallback} {
		if c, ok := v.(interface{ CloseIdleConnections() }); ok {
			c.CloseIdleConnections()
		}
	}
}
func browserHTTPClient(timeout time.Duration, jar http.CookieJar, fallback http.RoundTripper) *http.Client {
	browser, err := navitime.NewHTTPClient(timeout)
	var tr http.RoundTripper
	if browser != nil {
		tr = browser.Transport
	}
	return &http.Client{Timeout: timeout, Jar: jar, Transport: &browserTransport{browser: tr, fallback: fallback, err: err}}
}
