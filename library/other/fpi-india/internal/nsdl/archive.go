package nsdl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	archiveViewStateRe       = regexp.MustCompile(`id="__VIEWSTATE" value="([^"]*)"`)
	archiveViewStateGenRe    = regexp.MustCompile(`id="__VIEWSTATEGENERATOR" value="([^"]*)"`)
	archiveEventValidationRe = regexp.MustCompile(`id="__EVENTVALIDATION" value="([^"]*)"`)
)

const archiveURL = "https://www.fpi.nsdl.co.in/web/Reports/Archive.aspx"

// FetchArchiveReport replays NSDL's Archive.aspx ASP.NET WebForms
// VIEWSTATE POST flow to fetch daily FPI granular data (gross
// purchases/sales by asset class and investment route) for an arbitrary
// historical date. Every other report this package fetches has a stable
// GET URL; Archive.aspx does not — the server issues a fresh
// VIEWSTATE/EVENTVALIDATION token pair per request and only returns real
// data on a matching POST replay carrying the current tokens plus the
// requested date. date must be in NSDL's "DD-Mon-YYYY" format (e.g.
// "10-Feb-2020"); the response covers every trading day from the 1st of
// that month up to and including date, in the same rowspan GridView shape
// ParseGenericRecords already handles.
func FetchArchiveReport(ctx context.Context, date string) ([]byte, error) {
	userAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
	httpClient := &http.Client{Timeout: 30 * time.Second}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, err
	}
	getReq.Header.Set("User-Agent", userAgent)

	getResp, err := httpClient.Do(getReq)
	if err != nil {
		return nil, fmt.Errorf("fetching Archive.aspx tokens: %w", err)
	}
	defer getResp.Body.Close()
	getBody, err := io.ReadAll(getResp.Body)
	if err != nil {
		return nil, err
	}
	if getResp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetching Archive.aspx tokens: HTTP %d", getResp.StatusCode)
	}

	viewState := extractFormValue(archiveViewStateRe, getBody)
	viewStateGen := extractFormValue(archiveViewStateGenRe, getBody)
	eventValidation := extractFormValue(archiveEventValidationRe, getBody)
	if viewState == "" || eventValidation == "" {
		return nil, fmt.Errorf("could not extract VIEWSTATE tokens from Archive.aspx")
	}

	form := url.Values{
		"__EVENTTARGET":        {"btnSubmit1"},
		"__EVENTARGUMENT":      {""},
		"__VIEWSTATE":          {viewState},
		"__VIEWSTATEGENERATOR": {viewStateGen},
		"__EVENTVALIDATION":    {eventValidation},
		"txtDate":              {date},
		"hdnDate":              {date},
		"hdnFlag":              {""},
	}

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, archiveURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	postReq.Header.Set("User-Agent", userAgent)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Carry the session cookie(s) the GET set, matching real browser
	// behavior; the VIEWSTATE MAC may be session-scoped server-side.
	for _, c := range getResp.Cookies() {
		postReq.AddCookie(c)
	}

	postResp, err := httpClient.Do(postReq)
	if err != nil {
		return nil, fmt.Errorf("posting Archive.aspx date form: %w", err)
	}
	defer postResp.Body.Close()
	postBody, err := io.ReadAll(postResp.Body)
	if err != nil {
		return nil, err
	}
	if postResp.StatusCode >= 400 {
		return nil, fmt.Errorf("posting Archive.aspx date form: HTTP %d", postResp.StatusCode)
	}
	return postBody, nil
}

func extractFormValue(re *regexp.Regexp, body []byte) string {
	m := re.FindSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}
