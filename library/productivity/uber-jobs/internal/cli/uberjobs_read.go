// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// corpusRead is one live read of the careers corpus for a set of filters.
type corpusRead struct {
	Postings     []uberjobs.Posting
	Total        int
	Complete     bool
	ScanCapHit   bool
	Requests     int
	Source       string
	Fallback     bool
	FallbackFrom string
	Note         string
	Cache        map[string]string
}

// readLive reads every posting matching the server-side filters. When the
// careers site refuses, and only then, it falls back to Uber's Oracle
// candidate-experience API for the same requisitions.
func readLive(ctx context.Context, c *uberjobs.Client, f uberjobs.Filters) (*corpusRead, error) {
	q, _, err := f.ServerQuery()
	if err != nil {
		return nil, usageErr(err)
	}
	res, err := c.SearchAll(ctx, q)
	if err == nil {
		out := &corpusRead{
			Total: res.Total, Complete: res.Complete, ScanCapHit: res.ScanCapHit,
			Requests: res.Requests, Source: uberjobs.SourceSite, Note: res.Note, Cache: res.Cache,
		}
		out.Postings = make([]uberjobs.Posting, 0, len(res.Rows))
		for _, r := range res.Rows {
			out.Postings = append(out.Postings, uberjobs.Normalize(r, c.BaseURL))
		}
		return out, nil
	}
	if !uberjobs.IsRefusal(err) || !c.OracleEnabled() {
		return nil, err
	}
	siteErr := err
	// Oracle rows carry no team, sub-team, contract type, work pattern, or
	// description, so these filters would silently match nothing (or, for
	// --description-not-contains, everything); refuse instead of guessing.
	if f.Team != "" || f.SubTeam != "" || f.ContractType != "" || f.WorkPattern != "" || len(f.DescriptionContains) > 0 || len(f.DescriptionExcludes) > 0 {
		return nil, fmt.Errorf("%w; the Oracle fallback cannot apply --team, --sub-team, --contract-type, --work-pattern, or description filters, so no fallback was attempted", siteErr)
	}
	ores, oerr := c.OracleSearchAll(ctx, f.Query)
	if oerr != nil {
		return nil, fmt.Errorf("%w; the Oracle fallback also failed: %v", siteErr, oerr)
	}
	want := map[string]bool{}
	for _, cc := range f.Countries {
		if iso, _, ok := uberjobs.ResolveCountry(cc); ok {
			want[iso] = true
		}
	}
	out := &corpusRead{
		Requests: c.Requests(), Source: uberjobs.SourceOracle, Fallback: true,
		FallbackFrom: siteErr.Error(), ScanCapHit: ores.ScanCapHit,
		Complete: ores.Complete && !ores.ScanCapHit,
		Note:     "the careers site refused, so these rows come from Uber's Oracle candidate-experience API; description and job_category are null in this mode",
	}
	for _, r := range ores.Rows {
		p := uberjobs.NormalizeOracleRow(r, c.BaseURL)
		if len(want) > 0 && !anyCountry(p, want) {
			continue
		}
		out.Postings = append(out.Postings, p)
	}
	if out.Postings == nil {
		out.Postings = []uberjobs.Posting{}
	}
	out.Total = len(out.Postings)
	if len(want) == 0 {
		out.Total = ores.Total
	}
	return out, nil
}

func anyCountry(p uberjobs.Posting, want map[string]bool) bool {
	for _, l := range p.Locations {
		if l.CountryCode != nil && want[*l.CountryCode] {
			return true
		}
	}
	return false
}

// describeRead renders the resolved requests for --dry-run (B16): the real
// size-probe URL and the full-page URL the live read would send.
func describeRead(baseURL string, f uberjobs.Filters) (string, error) {
	q, _, err := f.ServerQuery()
	if err != nil {
		return "", err
	}
	probe := q.Values()
	probe.Set("page", "1")
	probe.Set("pagesize", "1")
	full := q.Values()
	full.Set("page", "1")
	full.Set("pagesize", "<total+50>")
	return fmt.Sprintf("GET %s then GET %s", searchURL(baseURL, probe), strings.Replace(searchURL(baseURL, full), "%3Ctotal%2B50%3E", "<total+50>", 1)), nil
}

func searchURL(base string, v url.Values) string {
	return strings.TrimRight(base, "/") + "/api/jobs/search/?" + v.Encode()
}

// applyClientFilters keeps rows matching the client-side filters.
func applyClientFilters(rows []uberjobs.Posting, f uberjobs.Filters, now time.Time) ([]uberjobs.Posting, error) {
	window, err := uberjobs.PostedWithin(f.PostedWithin)
	if err != nil {
		return nil, usageErr(err)
	}
	out := make([]uberjobs.Posting, 0, len(rows))
	for _, p := range rows {
		if f.MatchClient(p, window, now) {
			out = append(out, p)
		}
	}
	return out, nil
}

// splitList splits a comma-separated flag value, dropping blanks.
func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
