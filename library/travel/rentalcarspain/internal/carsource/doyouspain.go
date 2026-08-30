// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

// DoYouSpainBaseURL is the aggregator origin.
const DoYouSpainBaseURL = "https://www.doyouspain.com"

// DoYouSpain is the aggregator source client.
type DoYouSpain struct {
	BaseURL string
	client  *http.Client
}

// NewDoYouSpain builds a DoYouSpain client. The provided http.Client is used
// for transport but each Search call installs its own cookie jar because the
// search flow depends on per-search session cookies.
func NewDoYouSpain(hc *http.Client) *DoYouSpain {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &DoYouSpain{BaseURL: DoYouSpainBaseURL, client: hc}
}

func (d *DoYouSpain) base() string {
	if d.BaseURL != "" {
		return d.BaseURL
	}
	return DoYouSpainBaseURL
}

func (d *DoYouSpain) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en")
	return d.client.Do(req)
}

// ResolveLocation queries the autocomplete endpoint and returns matching
// DoYouSpain locations for a place name.
func (d *DoYouSpain) ResolveLocation(ctx context.Context, query string) ([]Location, error) {
	form := url.Values{}
	form.Set("idioma", "EN")
	form.Set("destino", query)
	form.Set("origen", "")
	form.Set("experimento", "[CAR][M]")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base()+"/do2/ajax/autocomplete", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := d.do(req)
	if err != nil {
		return nil, fmt.Errorf("autocomplete request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 406 {
		return nil, fmt.Errorf("doyouspain returned HTTP 406 (WAF): the User-Agent must be a non-browser token")
	}
	if err := httpStatusError(resp, "doyouspain autocomplete"); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return parseLocations(string(body)), nil
}

var (
	liDestinoRe = regexp.MustCompile(`data-destino='([^']*)'`)
	liDescRe    = regexp.MustCompile(`data-destino-description='([^']*)'`)
	liPaisRe    = regexp.MustCompile(`data-pais='([^']*)'`)
	liIataRe    = regexp.MustCompile(`data-iata='([^']*)'`)
	liBlockRe   = regexp.MustCompile(`(?s)<li[^>]*data-destino='[^']*'[^>]*>`)
)

// parseLocations extracts locations from autocomplete <li> fragments. The
// fragment attributes are single-quoted, so a light regex is more robust than
// full HTML parsing here.
func parseLocations(html string) []Location {
	var out []Location
	for _, tag := range liBlockRe.FindAllString(html, -1) {
		loc := Location{}
		if m := liDestinoRe.FindStringSubmatch(tag); m != nil {
			loc.Code = m[1]
		}
		if m := liDescRe.FindStringSubmatch(tag); m != nil {
			loc.Description = m[1]
		}
		if m := liPaisRe.FindStringSubmatch(tag); m != nil {
			loc.Country = m[1]
		}
		if m := liIataRe.FindStringSubmatch(tag); m != nil {
			loc.IATA = m[1]
		}
		if loc.Code != "" {
			out = append(out, loc)
		}
	}
	return out
}

var redirectRe = regexp.MustCompile(`/do/list/[a-z]+\?s=[a-f0-9-]+&b=[a-f0-9-]+`)

// Search runs the full three-step DoYouSpain flow (prime cookies, POST the
// search form, GET the results page) and returns the parsed offers. It retries
// once when the results-redirect token is missing, which happens
// intermittently when the session cookies did not settle on the first attempt.
func (d *DoYouSpain) Search(ctx context.Context, q SearchQuery) ([]Offer, error) {
	q = q.withDefaults()
	var lastErr error
	for attempt := 0; attempt < doyouspainMaxAttempts; attempt++ {
		if attempt > 0 {
			// Back off between attempts so the WAF/session cookies can settle
			// before the retry — the redirect token most often appears on a
			// second try a beat later. Respect context cancellation.
			delay := time.Duration(attempt) * doyouspainRetryBackoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		offers, err := d.searchOnce(ctx, q)
		if err == nil {
			return offers, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !doyouspainRetryable(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// doyouspainMaxAttempts / doyouspainRetryBackoff bound the redirect-token retry.
// Three attempts with a 300ms, then 600ms pause add at most ~0.9s of latency
// while recovering the common transient "no redirect token" failure.
const (
	doyouspainMaxAttempts  = 3
	doyouspainRetryBackoff = 300 * time.Millisecond
)

// doyouspainRetryable reports whether an error is worth retrying. WAF
// User-Agent rejections (406) and throttling won't self-resolve on an
// immediate retry, so those short-circuit; transient token/network/5xx
// failures do retry.
func doyouspainRetryable(err error) bool {
	if err == nil {
		return false
	}
	if IsRateLimit(err) {
		return false
	}
	if strings.Contains(err.Error(), "HTTP 406") {
		return false
	}
	return true
}

func (d *DoYouSpain) searchOnce(ctx context.Context, q SearchQuery) ([]Offer, error) {
	jar, _ := cookiejar.New(nil)
	hc := *d.client
	hc.Jar = jar
	base := d.base()

	// Step 1: prime session cookies via the homepage.
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil); err == nil {
		req.Header.Set("User-Agent", UserAgent)
		if resp, err := hc.Do(req); err == nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
		}
	}

	// Step 2: POST the search form; the response shell carries the s/b tokens.
	form := url.Values{}
	form.Set("pais", "ES")
	form.Set("destino", q.LocationCode)
	form.Set("destino_final", q.DropoffCode)
	form.Set("fechaRecogida", isoToDMY(q.Pickup))
	form.Set("fechaDevolucion", isoToDMY(q.Dropoff))
	hr, mr := splitTime(q.PickupTime)
	hd, md := splitTime(q.DropoffTime)
	form.Set("horarecogida", hr)
	form.Set("minutosrecogida", mr)
	form.Set("horadevolucion", hd)
	form.Set("minutosdevolucion", md)
	form.Set("fechaRecogidaSelHour", q.PickupTime)
	form.Set("fechaDevolucionSelHour", q.DropoffTime)
	form.Set("edad", fmt.Sprintf("%d", q.DriverAge))
	form.Set("chkOneWay", "on")
	form.Set("chkAge", "on")
	form.Set("idioma", q.Language)

	lang := q.Language
	if lang == "" {
		lang = "en"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/do/list/"+lang, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search POST: %w", err)
	}
	// Read up to 8MB: for large markets (Barcelona, Bilbao) DoYouSpain returns
	// the full results page inline with the redirect token near the very end
	// (past 2.5MB), rather than a small redirect shell. A 1MB cap truncated the
	// token away and looked like a persistent failure for those airports.
	shellBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	resp.Body.Close()
	if resp.StatusCode == 406 {
		return nil, fmt.Errorf("doyouspain returned HTTP 406 (WAF): the User-Agent must be a non-browser token")
	}
	redir := redirectRe.FindString(string(shellBytes))
	if redir == "" {
		return nil, fmt.Errorf("could not find results redirect token in DoYouSpain response (site markup may have changed)")
	}

	// Step 3: GET the results page.
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, base+redir, nil)
	if err != nil {
		return nil, err
	}
	req2.Header.Set("User-Agent", UserAgent)
	resp2, err := hc.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("results GET: %w", err)
	}
	defer resp2.Body.Close()
	if err := httpStatusError(resp2, "doyouspain results"); err != nil {
		return nil, err
	}
	doc, err := xhtml.Parse(io.LimitReader(resp2.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("parsing results HTML: %w", err)
	}
	return parseOffers(doc), nil
}

func (q SearchQuery) withDefaults() SearchQuery {
	if q.DropoffCode == "" {
		q.DropoffCode = q.LocationCode
	}
	if q.PickupTime == "" {
		q.PickupTime = "10:00"
	}
	if q.DropoffTime == "" {
		q.DropoffTime = "10:00"
	}
	if q.DriverAge == 0 {
		q.DriverAge = 35
	}
	if q.Language == "" {
		q.Language = "en"
	}
	return q
}

// isoToDMY accepts either dd/mm/yyyy (returned unchanged) or yyyy-mm-dd and
// returns dd/mm/yyyy, the format DoYouSpain expects.
func isoToDMY(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") {
		return s
	}
	parts := strings.Split(s, "-")
	if len(parts) == 3 {
		return fmt.Sprintf("%s/%s/%s", parts[2], parts[1], parts[0])
	}
	return s
}

func splitTime(t string) (hh, mm string) {
	parts := strings.SplitN(t, ":", 2)
	if len(parts) == 2 {
		return strings.TrimLeft(parts[0], " "), parts[1]
	}
	return "10", "00"
}
