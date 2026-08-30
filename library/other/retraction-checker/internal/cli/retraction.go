// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature support for retraction-checker-pp-cli.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/retraction-checker/internal/cliutil"
)

// retractionTypes are the Crossref update-to/update-by types that indicate a
// paper has been retracted or otherwise flagged.
var retractionTypes = map[string]bool{
	"retraction":            true,
	"withdrawal":            true,
	"removal":               true,
	"expression_of_concern": true,
}

// retractionVerdict is the structured result of a single retraction check.
type retractionVerdict struct {
	Input      string   `json:"input"`
	DOI        string   `json:"doi,omitempty"`
	Title      string   `json:"title,omitempty"`
	Retracted  bool     `json:"retracted"`
	UpdateType string   `json:"update_type,omitempty"`
	Date       string   `json:"date,omitempty"`
	Source     string   `json:"source,omitempty"`
	NoticeDOI  string   `json:"notice_doi,omitempty"`
	NoticeURL  string   `json:"notice_url,omitempty"`
	Published  string   `json:"published,omitempty"`
	Signals    []string `json:"signals,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// crossrefUpdate mirrors an entry of message.update-to / message.update-by.
type crossrefUpdate struct {
	DOI     string          `json:"DOI"`
	Type    string          `json:"type"`
	Label   string          `json:"label"`
	Source  string          `json:"source"`
	Updated crossrefDateObj `json:"updated"`
}

type crossrefDateObj struct {
	DateParts [][]int `json:"date-parts"`
	DateTime  string  `json:"date-time"`
}

func (d crossrefDateObj) iso() string {
	if d.DateTime != "" {
		if len(d.DateTime) >= 10 {
			return d.DateTime[:10]
		}
		return d.DateTime
	}
	if len(d.DateParts) > 0 && len(d.DateParts[0]) > 0 {
		p := d.DateParts[0]
		switch len(p) {
		case 1:
			return fmt.Sprintf("%04d", p[0])
		case 2:
			return fmt.Sprintf("%04d-%02d", p[0], p[1])
		default:
			return fmt.Sprintf("%04d-%02d-%02d", p[0], p[1], p[2])
		}
	}
	return ""
}

type crossrefWorkMessage struct {
	DOI       string           `json:"DOI"`
	Title     []string         `json:"title"`
	Type      string           `json:"type"`
	UpdateTo  []crossrefUpdate `json:"update-to"`
	UpdateBy  []crossrefUpdate `json:"update-by"`
	Published crossrefDateObj  `json:"published"`
	Issued    crossrefDateObj  `json:"issued"`
}

// looksLikePMID reports whether the identifier is a PubMed ID rather than a DOI.
func looksLikePMID(id string) bool {
	s := strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(id), "PMID:"))
	if s == "" {
		return false
	}
	if strings.Contains(s, "/") || strings.HasPrefix(s, "10.") {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cleanPMID(id string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(id)), "PMID:"))
}

// cleanDOI normalizes a DOI: strips a leading URL and doi: prefix.
func cleanDOI(id string) string {
	s := strings.TrimSpace(id)
	for _, p := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "doi:", "DOI:"} {
		s = strings.TrimPrefix(s, p)
	}
	return strings.TrimSpace(s)
}

// sharedHTTP is the single client used for cross-host lookups (NCBI id
// converter, OpenAlex). Crossref calls go through the generated client.
var sharedHTTP = &http.Client{Timeout: 30 * time.Second}

// resolvePMIDToDOI maps a PubMed ID to a DOI via the NCBI E-utilities esummary
// endpoint (free, keyless). It covers all of PubMed, not just the PMC subset.
// Returns an empty string if the record has no DOI. limiter paces requests
// against NCBI's unauthenticated 3 req/s ceiling; pass nil to disable
// (methods on a nil *AdaptiveLimiter no-op).
func resolvePMIDToDOI(ctx context.Context, pmid string, limiter *cliutil.AdaptiveLimiter) (string, error) {
	endpoint := "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esummary.fcgi?db=pubmed&retmode=json&id=" + url.QueryEscape(pmid)
	limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := sharedHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		limiter.OnRateLimit()
		return "", fmt.Errorf("pubmed esummary returned status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pubmed esummary returned status %d", resp.StatusCode)
	}
	limiter.OnSuccess()
	var payload struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	rec, ok := payload.Result[pmid]
	if !ok {
		return "", nil
	}
	var summary struct {
		ArticleIDs []struct {
			IDType string `json:"idtype"`
			Value  string `json:"value"`
		} `json:"articleids"`
	}
	if err := json.Unmarshal(rec, &summary); err != nil {
		return "", err
	}
	for _, a := range summary.ArticleIDs {
		if strings.EqualFold(a.IDType, "doi") && a.Value != "" {
			return a.Value, nil
		}
	}
	return "", nil
}

// checkDOI fetches a work from Crossref and evaluates its retraction status.
func checkDOI(ctx context.Context, c crossrefGetter, mailto, doi string) (retractionVerdict, error) {
	v := retractionVerdict{Input: doi, DOI: doi}
	params := map[string]string{}
	if mailto != "" {
		params["mailto"] = mailto
	}
	raw, err := c.Get(ctx, "/works/"+doi, params)
	if err != nil {
		return v, err
	}
	var envelope struct {
		Message crossrefWorkMessage `json:"message"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return v, err
	}
	m := envelope.Message
	if m.DOI != "" {
		v.DOI = m.DOI
	}
	if len(m.Title) > 0 {
		v.Title = m.Title[0]
	}
	v.Published = m.Published.iso()
	if v.Published == "" {
		v.Published = m.Issued.iso()
	}

	// Signal 1: update-by records pointing at this work (retraction notices).
	// Signal 2: update-to records of type retraction on this record itself.
	updates := append([]crossrefUpdate{}, m.UpdateBy...)
	updates = append(updates, m.UpdateTo...)
	for _, u := range updates {
		if retractionTypes[strings.ToLower(u.Type)] {
			v.Retracted = true
			v.UpdateType = u.Type
			v.Date = u.Updated.iso()
			v.Source = u.Source
			v.NoticeDOI = u.DOI
			if u.DOI != "" {
				v.NoticeURL = "https://doi.org/" + u.DOI
			}
			v.Signals = append(v.Signals, "crossref-update:"+u.Type)
			break
		}
	}

	// Signal 3: title prefix marker.
	upper := strings.ToUpper(v.Title)
	if strings.HasPrefix(upper, "RETRACTED") || strings.HasPrefix(upper, "WITHDRAWN") {
		v.Retracted = true
		v.Signals = append(v.Signals, "title-prefix")
		if v.UpdateType == "" {
			v.UpdateType = "retraction"
		}
	}
	return v, nil
}

// resolveAndCheck resolves a DOI or PMID and returns the verdict. limiter
// paces the NCBI PMID-resolution call; pass nil to disable.
func resolveAndCheck(ctx context.Context, c crossrefGetter, mailto, id string, limiter *cliutil.AdaptiveLimiter) retractionVerdict {
	input := strings.TrimSpace(id)
	if looksLikePMID(input) {
		pmid := cleanPMID(input)
		doi, err := resolvePMIDToDOI(ctx, pmid, limiter)
		if err != nil {
			return retractionVerdict{Input: input, Error: fmt.Sprintf("resolving PMID %s: %v", pmid, err)}
		}
		if doi == "" {
			return retractionVerdict{Input: input, Error: fmt.Sprintf("no DOI found for PMID %s", pmid)}
		}
		v, err := checkDOI(ctx, c, mailto, doi)
		v.Input = input
		if err != nil {
			v.Error = err.Error()
		}
		return v
	}
	doi := cleanDOI(input)
	v, err := checkDOI(ctx, c, mailto, doi)
	v.Input = input
	if err != nil {
		v.Error = err.Error()
	}
	return v
}

// crossrefGetter is the subset of the generated client used here (satisfied by
// *client.Client). Declared as an interface so retraction logic is testable.
type crossrefGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// openAlexWork is a related work returned for the superseded command.
type openAlexWork struct {
	Title           string `json:"title"`
	DOI             string `json:"doi,omitempty"`
	PublicationYear int    `json:"publication_year,omitempty"`
	CitedByCount    int    `json:"cited_by_count"`
	ID              string `json:"id,omitempty"`
}

// fetchOpenAlexRelated queries OpenAlex for works matching a topic, ranked by
// citation count, optionally bounded to those published after fromYear.
// limiter paces requests the same way the generated Crossref client does;
// pass nil to disable (methods on a nil *AdaptiveLimiter no-op).
func fetchOpenAlexRelated(ctx context.Context, search, mailto string, fromYear, limit int, limiter *cliutil.AdaptiveLimiter) ([]openAlexWork, error) {
	if limit <= 0 {
		limit = 10
	}
	q := url.Values{}
	q.Set("search", search)
	q.Set("sort", "cited_by_count:desc")
	q.Set("per_page", fmt.Sprintf("%d", limit))
	q.Set("select", "id,doi,title,publication_year,cited_by_count")
	filter := ""
	if fromYear > 0 {
		filter = fmt.Sprintf("from_publication_date:%d-01-01", fromYear)
	}
	if filter != "" {
		q.Set("filter", filter)
	}
	if mailto != "" {
		q.Set("mailto", mailto)
	}
	endpoint := "https://api.openalex.org/works?" + q.Encode()

	// OpenAlex anonymous search sheds load with HTTP 503 under heavy traffic.
	// Retry a few times with backoff before giving up.
	var lastStatus int
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		resp, err := sharedHTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
			lastStatus = resp.StatusCode
			resp.Body.Close()
			limiter.OnRateLimit()
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("openalex returned status %d", resp.StatusCode)
		}
		limiter.OnSuccess()
		var payload struct {
			Results []openAlexWork `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, err
		}
		return payload.Results, nil
	}
	return nil, fmt.Errorf("openalex unavailable after retries (last status %d); it rate-limits anonymous search under heavy load — retry shortly", lastStatus)
}
