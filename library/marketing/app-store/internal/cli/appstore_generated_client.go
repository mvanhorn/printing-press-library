// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The generated internal/client still serves the MCP server's typed
// categories_list / categories_games tools (and export and doctor). Left
// alone, it sends to apps.apple.com outside the App Store governor and
// retries a 429 up to three times with Retry-After waits, then waits for its
// limiter. This client hook (the generator's extension seam; no generated
// file changes) puts every generated-client request to an App Store host
// under the same governor state the hand-built client uses: it waits for the
// shared send slot, is refused during a throttle hold, records a 403/429 it
// receives as a hold, and rewrites that answer's Retry-After to the hold so
// the generated retry loop gives up at once (exit 7) instead of retrying.

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

func init() {
	registerClientHook(governGeneratedAppStoreRequests)
}

// governGeneratedAppStoreRequests wraps a generated client's transport with
// the App Store governor (idempotent; a client without an HTTP client is
// left as is).
func governGeneratedAppStoreRequests(c *client.Client) error {
	if c == nil || c.HTTPClient == nil {
		return nil
	}
	if _, done := c.HTTPClient.Transport.(*governedAppStoreTransport); done {
		return nil
	}
	gov, err := appstore.New(appstore.Options{NoCache: true, StatePath: appStoreGovernorPath(), WaitUntil: harnessWaitUntil()})
	if err != nil {
		return nil // never block client construction; requests then go ungoverned
	}
	base := c.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.HTTPClient.Transport = &governedAppStoreTransport{base: base, gov: gov}
	return nil
}

// governedAppStoreTransport governs requests to Apple's App Store hosts and
// passes every other request through untouched.
type governedAppStoreTransport struct {
	base http.RoundTripper
	gov  *appstore.Client
}

func (t *governedAppStoreTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, err := appstore.ClassifyURL(req.URL); err != nil {
		return t.base.RoundTrip(req)
	}
	if err := t.gov.Govern(req.Context(), req.URL); err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return governorRefusal(req, err), nil
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
		// Apple's full Retry-After, not cliutil's one-minute retry cap.
		hold := t.gov.RecordThrottle(req.URL, resp.StatusCode, appstore.HoldRetryAfter(resp.Header, time.Now()))
		if resp.StatusCode == http.StatusForbidden {
			// The generated client reads a bare 403 as an API error (exit 5);
			// answer the throttle as a 429 so it exits 7 like the hand-built
			// client does.
			_ = resp.Body.Close()
			return governorRefusal(req, &cliutil.RateLimitError{
				URL:        req.URL.String(),
				RetryAfter: time.Until(hold.Until),
				Body:       fmt.Sprintf("HTTP 403 from Apple; the governor holds every request to %s until %s", hold.Host, appstore.FormatTime(hold.Until)),
			}), nil
		}
		if resp.Header == nil {
			resp.Header = http.Header{}
		}
		resp.Header.Set("Retry-After", strconv.Itoa(governorRetryAfterSeconds(time.Until(hold.Until))))
	}
	return resp, nil
}

// governorRetryAfterSeconds is the Retry-After for an answer the governor
// shapes: at least two minutes, beyond the generated client's one-minute
// retry budget, so it reports the throttle instead of retrying it.
func governorRetryAfterSeconds(wait time.Duration) int {
	secs := int((wait + time.Second - 1) / time.Second)
	if secs < 120 {
		secs = 120
	}
	return secs
}

// governorRefusal answers, without sending it, a request the governor
// refused (a throttle hold, a stopped run, or a send slot past the deadline):
// a 429 the generated client turns into a rate-limit error (exit 7) with the
// governor's reason in the body.
func governorRefusal(req *http.Request, cause error) *http.Response {
	wait := time.Duration(0)
	if rl, ok := appstore.AsRateLimit(cause); ok {
		wait = rl.RetryAfter
	}
	body := "App Store governor: " + cause.Error()
	return &http.Response{
		Status:     "429 Too Many Requests",
		StatusCode: http.StatusTooManyRequests,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Retry-After":  {strconv.Itoa(governorRetryAfterSeconds(wait))},
			"Content-Type": {"text/plain; charset=utf-8"},
		},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}
