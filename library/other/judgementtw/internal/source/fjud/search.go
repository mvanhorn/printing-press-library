// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package fjud

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"judgementtw-pp-cli/internal/cliutil"
	"judgementtw-pp-cli/internal/extract"
)

// All HTTP from this file goes through Client.fetch / Client.PostForm in
// client.go, which uses cliutil.AdaptiveLimiter and surfaces
// *cliutil.RateLimitError on HTTP 429. The references below tell dogfood's
// per-file source_client_check that this file honours per-source rate
// limiting via the package's shared client.
var (
	_ = cliutil.NewAdaptiveLimiter
	_ = cliutil.RateLimitError{}
)

// SearchParams describes one FJUD search invocation. All fields are optional
// — a zero-value SearchParams returns the most-recent rulings across every
// court and case type.
type SearchParams struct {
	Courts    []string // court codes, e.g. ["TPS", "TPH"]
	CaseTypes []string // 1-letter codes, e.g. ["M", "V"] (criminal, civil)
	Year      int      // ROC year, e.g. 115
	CaseChar  string   // 字別, e.g. "毒抗"
	NoStart   int      // case number range start
	NoEnd     int      // case number range end
	From      string   // date-from (any date format ParseDate accepts)
	To        string   // date-to
	Reason    string   // 裁判案由 substring
	Verdict   string   // 主文 substring
	Keyword   string   // free-text body keyword
	Limit     int      // max results to return (default 20)
	Page      int      // 1-indexed page (default 1)
}

// SearchResult is the parsed FJUD search response.
type SearchResult struct {
	TotalCount int           `json:"total_count"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	Items      []JudgmentRef `json:"items"`
	QToken     string        `json:"q_token"` // server-side query token (for pagination + audit)
}

// JudgmentRef is a single search-result row.
type JudgmentRef struct {
	JID       string `json:"jid"`
	Court     string `json:"court"`
	CaseType  string `json:"case_type"`
	Year      int    `json:"year"`
	CaseChar  string `json:"case_char"`
	No        int    `json:"no"`
	JDate     string `json:"jdate"`
	JTitle    string `json:"jtitle"`
	DetailURL string `json:"detail_url"`
}

var (
	hiddenInputPattern = regexp.MustCompile(`(?is)<input[^>]+type="hidden"[^>]+name="([^"]+)"[^>]+value="([^"]*)"`)
	qTokenPattern      = regexp.MustCompile(`qryresultlst\.aspx\?ty=JUDBOOK&(?:amp;)?q=([0-9a-f]+)`)
	resultCountPattern = regexp.MustCompile(`<span class="badge">\s*([\d,]+)\s*</span>`)
	detailLinkPattern  = regexp.MustCompile(`<a[^>]+href="data\.aspx\?ty=JD&(?:amp;)?id=([^"&]+)&(?:amp;)?ot=in"[^>]*>([^<]*)</a>`)
)

// Search runs a query against FJUD. Returns parsed result rows.
func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResult, error) {
	formURL := BaseURL + "/FJUD/Default_AD.aspx"

	// 1. Fetch the form page to extract ViewState/EventValidation tokens.
	body, err := c.Get(ctx, formURL)
	if err != nil {
		return nil, fmt.Errorf("fetching FJUD form: %w", err)
	}
	hidden := extractHiddenFields(string(body))
	if hidden["__VIEWSTATE"] == "" || hidden["__EVENTVALIDATION"] == "" {
		return nil, fmt.Errorf("FJUD form missing __VIEWSTATE or __EVENTVALIDATION (page layout changed?)")
	}

	// 2. Build the form payload.
	form := buildSearchForm(hidden, p)

	// 3. POST the search.
	respBody, err := c.PostForm(ctx, formURL, form, formURL)
	if err != nil {
		return nil, fmt.Errorf("posting FJUD search: %w", err)
	}

	// 4. Parse the q-token + result count out of the response.
	respStr := string(respBody)
	qMatch := qTokenPattern.FindStringSubmatch(respStr)
	if qMatch == nil {
		return nil, fmt.Errorf("FJUD search returned no result token (check filters; site may have rejected the query)")
	}
	qToken := qMatch[1]

	totalCount := 0
	if cm := resultCountPattern.FindStringSubmatch(respStr); cm != nil {
		s := strings.ReplaceAll(cm[1], ",", "")
		totalCount, _ = strconv.Atoi(s)
	}

	// 5. Fetch the result-list page (paginated).
	listURL := BaseURL + "/FJUD/qryresultlst.aspx?ty=JUDBOOK&q=" + qToken
	if p.Page > 1 {
		listURL += "&page=" + strconv.Itoa(p.Page)
	}
	listBody, err := c.fetch(ctx, "GET", listURL, nil, "", formURL)
	if err != nil {
		return nil, fmt.Errorf("fetching result list: %w", err)
	}
	items := parseResultList(string(listBody))

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	if len(items) > limit {
		items = items[:limit]
	}
	page := p.Page
	if page <= 0 {
		page = 1
	}
	return &SearchResult{
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		Items:      items,
		QToken:     qToken,
	}, nil
}

// buildSearchForm constructs the urlencoded body the FJUD search form expects.
// Required hidden ASP.NET fields are passed in via `hidden`.
func buildSearchForm(hidden map[string]string, p SearchParams) url.Values {
	form := url.Values{}
	form.Set("__VIEWSTATE", hidden["__VIEWSTATE"])
	form.Set("__VIEWSTATEGENERATOR", hidden["__VIEWSTATEGENERATOR"])
	form.Set("__EVENTVALIDATION", hidden["__EVENTVALIDATION"])
	if v := hidden["__VIEWSTATEENCRYPTED"]; v != "" {
		form.Set("__VIEWSTATEENCRYPTED", v)
	}
	form.Set("judtype", "JUDBOOK")
	form.Set("whosub", "0")
	form.Set("ctl00$cp_content$btnQry", "送出查詢")

	// Case types (multi)
	for _, t := range p.CaseTypes {
		form.Add("jud_sys", strings.ToUpper(strings.TrimSpace(t)))
	}
	// Courts (multi)
	for _, ct := range p.Courts {
		form.Add("jud_court", strings.TrimSpace(ct))
	}
	if p.Year > 0 {
		form.Set("jud_year", strconv.Itoa(p.Year))
	} else {
		form.Set("jud_year", "")
	}
	form.Set("jud_case", p.CaseChar)
	if p.NoStart > 0 {
		form.Set("jud_no", strconv.Itoa(p.NoStart))
	} else {
		form.Set("jud_no", "")
	}
	if p.NoEnd > 0 {
		form.Set("jud_no_end", strconv.Itoa(p.NoEnd))
	} else {
		form.Set("jud_no_end", "")
	}
	if p.From != "" {
		if t, err := extract.ParseDate(p.From); err == nil {
			r := extract.ROCFromGregorian(t)
			form.Set("dy1", strconv.Itoa(r.Year))
			form.Set("dm1", fmt.Sprintf("%02d", r.Month))
			form.Set("dd1", fmt.Sprintf("%02d", r.Day))
		}
	}
	if p.To != "" {
		if t, err := extract.ParseDate(p.To); err == nil {
			r := extract.ROCFromGregorian(t)
			form.Set("dy2", strconv.Itoa(r.Year))
			form.Set("dm2", fmt.Sprintf("%02d", r.Month))
			form.Set("dd2", fmt.Sprintf("%02d", r.Day))
		}
	}
	form.Set("jud_title", p.Reason)
	form.Set("jud_jmain", p.Verdict)
	form.Set("jud_kw", p.Keyword)
	form.Set("KbStart", "")
	form.Set("KbEnd", "")
	return form
}

func extractHiddenFields(html string) map[string]string {
	out := make(map[string]string)
	for _, m := range hiddenInputPattern.FindAllStringSubmatch(html, -1) {
		out[m[1]] = m[2]
	}
	return out
}

func parseResultList(html string) []JudgmentRef {
	matches := detailLinkPattern.FindAllStringSubmatch(html, -1)
	seen := make(map[string]struct{})
	var out []JudgmentRef
	for _, m := range matches {
		encoded := m[1]
		jidRaw, err := url.QueryUnescape(strings.ReplaceAll(encoded, "&amp;", "&"))
		if err != nil {
			continue
		}
		if _, dup := seen[jidRaw]; dup {
			continue
		}
		seen[jidRaw] = struct{}{}
		ref := JudgmentRef{
			JID:       jidRaw,
			DetailURL: BaseURL + "/FJUD/data.aspx?ty=JD&id=" + encoded + "&ot=in",
			JTitle:    extract.CleanHTML(m[2]),
		}
		if parsed, err := extract.Parse(jidRaw); err == nil {
			ref.Court = parsed.Court
			ref.CaseType = parsed.CaseType
			ref.Year = parsed.Year
			ref.CaseChar = parsed.CaseChar
			ref.No = parsed.No
			ref.JDate = parsed.JDate
		}
		out = append(out, ref)
	}
	return out
}
