package client

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// NewBrowserClient returns an HTTP client with the Chrome TLS fingerprint for
// the hand-written REWE commands. Unlike the generated client it may follow
// redirects between www.rewe.de and account.rewe.de (the single-sign-on
// renewal hops between both), and it refuses any host outside rewe.de.
func NewBrowserClient(timeout time.Duration, jar http.CookieJar) (*http.Client, error) {
	trust, err := resolveTLS(tlsSettings{})
	if err != nil {
		return nil, err
	}
	rt, err := newChromeTransport(timeout, trust)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   timeout,
		Jar:       jar,
		Transport: reweOnlyTripper{next: chromeHeaderTripper{next: rt}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if !isREWEHost(req.URL.Hostname()) || req.URL.Scheme != "https" {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}, nil
}

type reweOnlyTripper struct{ next http.RoundTripper }

func (t reweOnlyTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || !isREWEHost(req.URL.Hostname()) {
		return nil, errors.New("refusing request outside https://*.rewe.de: " + req.URL.Host)
	}
	return t.next.RoundTrip(req)
}

func isREWEHost(h string) bool {
	h = strings.ToLower(h)
	return h == "rewe.de" || strings.HasSuffix(h, ".rewe.de")
}
