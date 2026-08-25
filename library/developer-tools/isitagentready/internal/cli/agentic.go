// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/config"
)

// agenticBaseURL is the second scanner's host. The generator's spec.yaml models
// exactly one base_url per CLI, so this second source is hand-authored.
const agenticBaseURL = "https://is-agentic.com"

// agenticReportPath is READ-ONLY: it returns an already-completed report and
// never triggers a scan. A never-scanned domain 404s immediately.
const agenticReportPath = "/api/v1/report"

// agenticRateLimit is the host's advertised public-report budget:
// `ratelimit-policy: "public-report";q=120;w=60` = 120 requests / 60s = 2 req/s.
const agenticRateLimit = 2.0

// The is-agentic budget is per-HOST, but a client's limiter is per-CLIENT:
// client.New builds a fresh cliutil.AdaptiveLimiter every call. Constructing a
// client per request therefore gave each of the 4 compare/batch fan-out workers
// its own limiter, so 4 requests could leave at once against a 2 req/s budget —
// exactly the 429 storm the limiter exists to prevent. Memoize ONE client (and
// therefore ONE limiter) per process so every worker queues behind one budget.
//
// The key covers every setting client.New bakes in. Within a single command run
// these are fixed, so all fan-out workers share one client; a caller configured
// differently gets its own rather than silently inheriting another's DryRun /
// NoCache. Fields are only written under the mutex at construction and never
// mutated afterwards, so concurrent readers stay race-free.
var (
	agenticClientMu     sync.Mutex
	agenticClientCached *client.Client
	agenticClientKey    string
)

// newAgenticClient returns the process-wide client pointed at is-agentic.com.
// client.Client exports BaseURL and takes no host at construction beyond the
// config, so the second source needs an override, not a client fork.
func (f *rootFlags) newAgenticClient() (*client.Client, error) {
	// The is-agentic host's public-report budget applies whether or not the
	// user set --rate-limit: clamp a configured higher rate down, and use the
	// host budget when none is set.
	rl := f.rateLimit
	if rl <= 0 || rl > agenticRateLimit {
		rl = agenticRateLimit
	}
	key := fmt.Sprintf("%s|%s|%v|%t|%t", f.configPath, f.timeout, rl, f.dryRun, f.noCache)

	agenticClientMu.Lock()
	defer agenticClientMu.Unlock()
	if agenticClientCached != nil && agenticClientKey == key {
		return agenticClientCached, nil
	}
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	c := client.New(cfg, f.timeout, rl)
	c.BaseURL = agenticBaseURL
	c.DryRun = f.dryRun
	c.NoCache = f.noCache
	agenticClientCached = c
	agenticClientKey = key
	return c, nil
}

// resetAgenticClient drops the memoized client. Tests use it so one test's
// client (with a test-server BaseURL) cannot leak into another.
func resetAgenticClient() {
	agenticClientMu.Lock()
	defer agenticClientMu.Unlock()
	agenticClientCached = nil
	agenticClientKey = ""
}

// agenticProblem is an RFC 9457 application/problem+json error body.
type agenticProblem struct {
	Type             string `json:"type"`
	Title            string `json:"title"`
	Status           int    `json:"status"`
	Detail           string `json:"detail"`
	Instance         string `json:"instance"`
	Code             string `json:"code"`
	Resolution       string `json:"resolution"`
	DocumentationURL string `json:"documentation_url"`
}

// fetchAgenticReport GETs the completed is-agentic report for url.
func fetchAgenticReport(ctx context.Context, flags *rootFlags, url string) (json.RawMessage, error) {
	c, err := flags.newAgenticClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, agenticReportPath, map[string]string{"url": url})
	if err != nil {
		return nil, classifyAgenticError(err, flags)
	}
	return data, nil
}

// classifyAgenticError maps an RFC 9457 problem+json response to a typed CLI
// error so exit codes stay meaningful:
//
//	404 report_not_found -> notFoundErr (3), carrying the upstream `resolution`
//	                        string verbatim — it names the URL where the user
//	                        can start the missing scan.
//	400 invalid_url      -> usageErr (2)
//	429                  -> rateLimitErr (7), never an empty result: an empty
//	                        result is indistinguishable from "no data" and
//	                        silently corrupts downstream queries.
//
// Anything else falls through to classifyAPIError.
func classifyAgenticError(err error, flags *rootFlags) error {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return classifyAPIError(err, flags)
	}
	var p agenticProblem
	if json.Unmarshal([]byte(apiErr.Body), &p) != nil {
		return classifyAPIError(err, flags)
	}
	switch {
	case apiErr.StatusCode == 404 && p.Code == "report_not_found":
		// The upstream `resolution` names the scan URL the user must run
		// first; pass it through verbatim, do not paraphrase.
		msg := p.Detail
		if p.Resolution != "" {
			msg = p.Detail + " " + p.Resolution
		}
		// No writeAPIErrorEnvelope here: crossref treats this 404 as an
		// expected outcome, catches it, and still prints the other scanner's
		// verdict. An envelope written from the classifier would land in the
		// middle of that JSON document. Envelope emission belongs at the
		// command boundary, where the error is known to be terminal.
		return notFoundErr(fmt.Errorf("is-agentic has no completed report for this URL: %s", msg))
	case apiErr.StatusCode == 400 && p.Code == "invalid_url":
		msg := p.Detail
		if p.Resolution != "" {
			msg = p.Detail + " " + p.Resolution
		}
		return usageErr(fmt.Errorf("invalid URL for is-agentic scan: %s", msg))
	case apiErr.StatusCode == 429:
		// Never collapse a rate-limit into an empty/no-data result: downstream
		// queries would silently read it as "no scan exists".
		return rateLimitErr(fmt.Errorf("is-agentic rate limit reached: %s", p.Detail))
	default:
		return classifyAPIError(err, flags)
	}
}
