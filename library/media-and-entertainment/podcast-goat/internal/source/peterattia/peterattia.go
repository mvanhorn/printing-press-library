// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 peterattia cookie tier (auth + first-time-capture honest error).

package peterattia

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const (
	adapterName = "peterattia"
	service     = "peterattia"
	publicHost  = "peterattiamd.com"
)

type Adapter struct{ Client *http.Client }

func New() *Adapter {
	return &Adapter{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierCookie }

var hostRE = regexp.MustCompile(`(?i)^https?://(www\.)?peterattiamd\.com/`)

func (a *Adapter) Match(url string) bool { return hostRE.MatchString(url) }

func (a *Adapter) Fetch(ctx context.Context, url string) (*transcript.Transcript, error) {
	if !source.HasCookie(service) {
		return nil, &source.CookieMissingError{
			Service: service,
			Hint:    "run `podcast-goat-pp-cli auth login --chrome --service peterattia` after logging in at peterattiamd.com",
		}
	}
	raw, err := os.ReadFile(source.CookieFile(service))
	if err != nil {
		return nil, fmt.Errorf("read %s cookie: %w", service, err)
	}
	cookies, err := source.ParseCookieJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s cookie: %w", service, err)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "podcast-goat-pp-cli/0.1")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("peterattia GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return nil, &source.NotImplementedError{
		Adapter:      adapterName,
		NeedsCapture: true,
		Detail: fmt.Sprintf(
			"peterattiamd.com member transcript HTML requires first-time browser capture. Public host is %s; "+
				"as a workaround try `episode get %s --paid --provider spoken`.",
			publicHost, url,
		),
	}
}

var _ source.Adapter = (*Adapter)(nil)
