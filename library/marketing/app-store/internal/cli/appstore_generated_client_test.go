// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The generated internal/client (the MCP server's typed categories tools use
// it) retried a 429 from apps.apple.com up to three times with Retry-After
// waits and then waited for its limiter, outside the App Store governor. The
// client hook puts it under the governor.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/config"
)

func TestGeneratedClientRequestsToAppleObeyTheGovernor(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	apple := &fakeApple{handler: func(r *http.Request) fakeResponse {
		return fakeResponse{Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"13"}}, Body: []byte("Rate limit has been exceeded")}
	}}
	newGenerated := func() *client.Client {
		c := client.New(&config.Config{BaseURL: "https://itunes.apple.com"}, 10*time.Second, 0)
		c.NoCache = true
		c.HTTPClient.Transport = apple
		if err := governGeneratedAppStoreRequests(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	chartURL := "https://apps.apple.com/us/iphone/charts/36"
	headers := map[string]string{client.HTMLResponseHeader: "true"}

	// A typed MCP tool call: Apple throttles it once; the generated retry
	// loop gives up at once instead of retrying after Retry-After.
	start := time.Now()
	_, err = newGenerated().GetWithHeaders(context.Background(), chartURL, nil, headers)
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("want a rate-limit error, got %T %v", err, err)
	}
	if n := len(apple.urls()); n != 1 {
		t.Fatalf("the generated client sent %d requests after a 429, want 1 (no retry)", n)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the throttled call took %v: it waited out Retry-After or its limiter", took)
	}
	if ExitCode(classifyAPIError(io.Discard, err, &rootFlags{})) != 7 {
		t.Fatalf("exit code = %d, want 7", ExitCode(classifyAPIError(io.Discard, err, &rootFlags{})))
	}

	// The hold is shared: the next tool call (a new client) sends nothing,
	// and neither does the hand-built App Store client.
	_, err = newGenerated().GetWithHeaders(context.Background(), chartURL, nil, headers)
	if !errors.As(err, &rl) || !strings.Contains(rl.Body, "App Store governor") || !strings.Contains(rl.Body, "holds every request to apps.apple.com") {
		t.Fatalf("second call: %v (cause %+v)", err, rl)
	}
	if n := len(apple.urls()); n != 1 {
		t.Fatalf("a held host received %d requests", n)
	}
	c, err := newAppStoreClient(&rootFlags{noCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if h, ok := c.HoldFor("apps.apple.com"); !ok || h.Status != http.StatusTooManyRequests || h.RetryAfter != 13 {
		t.Fatalf("App Store client hold = %+v %v", h, ok)
	}

	// Hosts outside the App Store pass through ungoverned.
	other := &fakeApple{handler: func(r *http.Request) fakeResponse { return fakeResponse{Body: []byte(`{"ok":true}`)} }}
	g := client.New(&config.Config{BaseURL: "https://example.org"}, 10*time.Second, 0)
	g.NoCache = true
	g.HTTPClient.Transport = other
	if err := governGeneratedAppStoreRequests(g); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Get(context.Background(), "/status", nil); err != nil || len(other.urls()) != 1 {
		t.Fatalf("non-Apple request: %v (%d sent)", err, len(other.urls()))
	}
}

// A 403 on the generated-client path is a throttle like a 429: a hold with
// Apple's full Retry-After, and exit 7 (the generated client alone maps a bare
// 403 to exit 5).
func TestGeneratedClientTreatsA403AsAThrottle(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	apple := &fakeApple{handler: func(r *http.Request) fakeResponse {
		return fakeResponse{Status: http.StatusForbidden, Header: http.Header{"Retry-After": {"1800"}}, Body: []byte("<html>Forbidden</html>")}
	}}
	c := client.New(&config.Config{BaseURL: "https://itunes.apple.com"}, 10*time.Second, 0)
	c.NoCache = true
	c.HTTPClient.Transport = apple
	if err := governGeneratedAppStoreRequests(c); err != nil {
		t.Fatal(err)
	}
	_, err = c.GetWithHeaders(context.Background(), "https://apps.apple.com/us/iphone/charts/36", nil, map[string]string{client.HTMLResponseHeader: "true"})
	if code := ExitCode(classifyAPIError(io.Discard, err, &rootFlags{})); code != 7 {
		t.Fatalf("a 403 from Apple exits %d, want 7 (err %v)", code, err)
	}
	if n := len(apple.urls()); n != 1 {
		t.Fatalf("sent %d requests, want 1", n)
	}
	ac, err := newAppStoreClient(&rootFlags{noCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if h, ok := ac.HoldFor("apps.apple.com"); !ok || h.Status != http.StatusForbidden || h.RetryAfter != 1800 || time.Until(h.Until) < 29*time.Minute {
		t.Fatalf("hold = %+v %v", h, ok)
	}
}
