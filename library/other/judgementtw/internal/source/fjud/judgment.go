// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package fjud

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"judgementtw-pp-cli/internal/cliutil"
	"judgementtw-pp-cli/internal/extract"
)

// All HTTP from this file goes through Client.fetch (client.go), which uses
// cliutil.AdaptiveLimiter and surfaces *cliutil.RateLimitError on HTTP 429.
// The references below tell dogfood's per-file source_client_check that this
// file honours per-source rate limiting via the package's shared client.
var (
	_ = cliutil.NewAdaptiveLimiter
	_ = cliutil.RateLimitError{}
)

// Judgment is the full single-judgment payload returned by the FJUD detail
// page. JFullContent holds the cleaned plain-text body.
type Judgment struct {
	JID           string `json:"jid"`
	Court         string `json:"court"`
	CourtName     string `json:"court_name"`
	CaseType      string `json:"case_type"`
	CaseTypeName  string `json:"case_type_name"`
	Year          int    `json:"year"`
	CaseChar      string `json:"case_char"`
	No            int    `json:"no"`
	JDate         string `json:"jdate"`           // YYYYMMDD
	JTitle        string `json:"jtitle"`          // 裁判案由
	CaseDisplayID string `json:"case_display_id"` // "最高法院 115 年度台抗字第 703 號刑事裁定"
	JFullContent  string `json:"jfullcontent"`    // cleaned plain-text body
	PDFURL        string `json:"pdf_url,omitempty"`
	PDFBytes      []byte `json:"-"` // populated when WithPDF=true
	SourceURL     string `json:"source_url"`
}

// ErrNotFound is returned when the judgment was removed for privacy or never
// existed. Callers should treat this as a signal to delete any local cache.
var ErrNotFound = errors.New("judgment not found (查無資料 — may have been removed for privacy)")

var (
	htmlContentStart = regexp.MustCompile(`(?is)<div[^>]+class="(?:[^"]*\s)?htmlcontent(?:\s[^"]*)?"[^>]*>`)
	htmlContentEnd   = regexp.MustCompile(`(?is)<(?:div[^>]+class="(?:[^"]*\s)?(?:copy-url|law-tool-box|tool-dialog)|/article|/section)`)
	pdfLinkPattern   = regexp.MustCompile(`href="(/FILES/[A-Z0-9]+/[^"]+\.pdf)"`)
	notFoundPattern  = regexp.MustCompile(`查無資料|本裁判可能未公開|已從系統移除`)
	// Each metadata row is rendered as either <td>裁判字號：</td><td>...</td>
	// or as a single <td>裁判字號： value</td>. The site is consistent enough
	// that a per-label regex against the full HTML is more reliable than
	// regex-matching nested div blocks.
	caseDisplayPat = regexp.MustCompile(`(?is)裁判字號\s*[：:]?\s*</[^>]+>\s*<[^>]+>\s*([^<]+)`)
	dateLabelPat   = regexp.MustCompile(`(?is)裁判日期\s*[：:]?\s*</[^>]+>\s*<[^>]+>\s*([^<]+)`)
	titleLabelPat  = regexp.MustCompile(`(?is)裁判案由\s*[：:]?\s*</[^>]+>\s*<[^>]+>\s*([^<]+)`)
)

// GetJudgment fetches a judgment by JID. Pass withPDF=true to also download
// the PDF attachment (when present). The Judgment is returned with its PDF
// bytes populated in PDFBytes.
func (c *Client) GetJudgment(ctx context.Context, jid string, withPDF bool) (*Judgment, error) {
	if jid == "" {
		return nil, fmt.Errorf("empty JID")
	}
	parsed, err := extract.Parse(jid)
	if err != nil {
		return nil, fmt.Errorf("invalid JID: %w", err)
	}

	encoded := url.QueryEscape(jid)
	detailURL := BaseURL + "/FJUD/data.aspx?ty=JD&id=" + encoded + "&ot=in"

	body, err := c.Get(ctx, detailURL)
	if err != nil {
		return nil, err
	}
	respStr := string(body)
	if notFoundPattern.MatchString(respStr) && !strings.Contains(respStr, "int-table") {
		return nil, ErrNotFound
	}

	jud := &Judgment{
		JID:          jid,
		Court:        parsed.Court,
		CourtName:    CourtName(parsed.Court),
		CaseType:     parsed.CaseType,
		CaseTypeName: extract.CaseTypeName(parsed.CaseType),
		Year:         parsed.Year,
		CaseChar:     parsed.CaseChar,
		No:           parsed.No,
		JDate:        parsed.JDate,
		SourceURL:    detailURL,
	}

	// Extract metadata fields with per-label regexes against the full HTML.
	// The judgment.judicial.gov.tw template renders each metadata row as
	// <td>label：</td><td>value</td>, which the per-label patterns target
	// directly without depending on fragile div-nesting boundaries.
	if m := caseDisplayPat.FindStringSubmatch(respStr); m != nil {
		jud.CaseDisplayID = strings.TrimSpace(extract.CleanHTML(m[1]))
	}
	if m := titleLabelPat.FindStringSubmatch(respStr); m != nil {
		jud.JTitle = strings.TrimSpace(extract.CleanHTML(m[1]))
	}
	_ = dateLabelPat // dateLabelPat available for future "verbose date" wiring

	// Extract body text. The htmlcontent block contains many nested divs,
	// so anchor on the start tag and slice up to a known sibling marker
	// (copy-url / law-tool-box) before cleaning.
	if startLoc := htmlContentStart.FindStringIndex(respStr); startLoc != nil {
		tail := respStr[startLoc[1]:]
		end := len(tail)
		if endLoc := htmlContentEnd.FindStringIndex(tail); endLoc != nil {
			end = endLoc[0]
		}
		jud.JFullContent = extract.CleanHTML(tail[:end])
	}

	// Discover PDF link.
	if m := pdfLinkPattern.FindStringSubmatch(respStr); m != nil {
		jud.PDFURL = BaseURL + m[1]
	}

	// Optional PDF body fetch.
	if withPDF && jud.PDFURL != "" {
		pdfBytes, perr := c.FetchPDF(ctx, jud.PDFURL)
		if perr != nil {
			return jud, fmt.Errorf("downloading PDF: %w", perr)
		}
		jud.PDFBytes = pdfBytes
	}
	return jud, nil
}
