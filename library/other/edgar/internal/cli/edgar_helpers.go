// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// Hand-built EDGAR utilities: CIK/accession normalization, ticker→CIK
// resolution against company_tickers.json with 24h cache, primary-doc body
// fetch, Form 4 XML parser, 8-K Item extraction, sections boundary parser.

package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
)

// normalizeCIK accepts any of: "AAPL" (ticker → must caller-resolve),
// "0001833908", "1833908", "CIK0001833908", "320193". Returns the 10-digit
// zero-padded numeric form. Returns an error if not a valid numeric CIK.
// Callers needing ticker-resolution should call resolveTickerToCIK first.
func normalizeCIK(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty CIK")
	}
	s = strings.TrimPrefix(strings.ToUpper(s), "CIK")
	// strip leading zeros for parsing
	s = strings.TrimLeft(s, "0")
	if s == "" {
		s = "0"
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return "", fmt.Errorf("not a numeric CIK: %q", s)
	}
	if n < 0 {
		return "", fmt.Errorf("negative CIK: %q", s)
	}
	return fmt.Sprintf("%010d", n), nil
}

// normalizeAccession accepts "0000320193-22-000049" or "000032019322000049"
// and returns the dashed form (with_dashes) and the no-dashes form.
func normalizeAccession(s string) (withDashes, noDashes string, err error) {
	s = strings.TrimSpace(s)
	noDashes = strings.ReplaceAll(s, "-", "")
	if len(noDashes) != 18 {
		return "", "", fmt.Errorf("accession must be 18 digits (got %d): %q", len(noDashes), s)
	}
	for _, r := range noDashes {
		if r < '0' || r > '9' {
			return "", "", fmt.Errorf("accession must be all digits: %q", s)
		}
	}
	withDashes = noDashes[:10] + "-" + noDashes[10:12] + "-" + noDashes[12:]
	return withDashes, noDashes, nil
}

// edgarUA returns the lodestar User-Agent or the empty string if unset.
// Hand-built commands MUST set this on every outbound request.
func edgarUA(c *client.Client) string {
	if c == nil || c.Config == nil {
		return ""
	}
	return c.Config.AuthHeader()
}

// requireEdgarUA returns an authErr if User-Agent material is unset.
func requireEdgarUA(c *client.Client) error {
	if edgarUA(c) == "" {
		return authErr(errors.New(
			"COMPANY_PP_CONTACT_EMAIL is unset; required for SEC fair-access User-Agent. " +
				"Set it: export COMPANY_PP_CONTACT_EMAIL=<your-email>"))
	}
	return nil
}

// edgarHeaders returns the SEC-fair-access headers for a hand-built request.
// PATCH(phase5: gzip-header-removal): Accept-Encoding is intentionally left
// UNSET so Go's net/http transparently negotiates and decodes gzip. Setting
// it explicitly disables auto-decode and leaves the caller with raw gzipped
// bytes (we hit this on the first company_tickers.json call).
func edgarHeaders(c *client.Client) map[string]string {
	return map[string]string{
		"User-Agent": edgarUA(c),
	}
}

// fetchAbsoluteRaw GETs an absolute SEC URL through the rate-limited client.
// Used for HTML/XML primary documents where c.GetWithHeaders (which expects
// JSON) would fail; this returns the raw body bytes.
func fetchAbsoluteRaw(ctx context.Context, c *client.Client, absURL string) ([]byte, int, error) {
	if !strings.HasPrefix(absURL, "https://") && !strings.HasPrefix(absURL, "http://") {
		return nil, 0, fmt.Errorf("absolute URL required: %q", absURL)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", absURL, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range edgarHeaders(c) {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		return nil, 0, errors.New("missing User-Agent; set COMPANY_PP_CONTACT_EMAIL")
	}
	req.Header.Set("Accept", "*/*")
	// PATCH: route through c.DoRaw so the AdaptiveLimiter paces this hand-rolled
	// fetch identically to c.do(). Previously bypassed the limiter, breaking
	// SEC fair-access pacing for HTML/XML primary docs, submissions index,
	// ticker cache, and Form 4 XML.
	resp, err := c.DoRaw(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return body, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, resp.StatusCode, nil
}

// PATCH(phase5: form4-index-json-fallback): fetchForm4XML resolves the
// correct Form 4 XML document for an accession.
// SEC's submissions index `primaryDocument` field increasingly points at a
// wrapper HTML stylesheet (e.g., `xslF345X05/wf-form4_xxx.xml`) instead of
// the raw XML. We try in order:
//  1. The primaryDocURL as-is (cheap path; works for older filings).
//  2. The accession's index.json directory listing → pick the first non-
//     wrapper XML file.
//
// Returns (bodyBytes, urlActuallyUsed, skipReason). If bodyBytes is nil,
// skipReason is populated for the loud-skip path.
func fetchForm4XML(ctx context.Context, c *client.Client, cik, accession, primaryDocURL string) ([]byte, string, string) {
	tryParse := func(b []byte) bool {
		if len(b) < 50 {
			return false
		}
		// Form 4 XML root is <ownershipDocument>. If we see <html or <!DOCTYPE,
		// it's the wrapper — treat as a miss and fall through to index.json.
		head := strings.ToLower(string(b[:min(400, len(b))]))
		if strings.Contains(head, "<html") || strings.Contains(head, "<!doctype html") {
			return false
		}
		return strings.Contains(head, "<ownershipdocument") || strings.Contains(head, "<?xml")
	}
	// Path 1: try primary_doc URL as-is.
	if primaryDocURL != "" {
		body, _, ferr := fetchAbsoluteRaw(ctx, c, primaryDocURL)
		if ferr == nil && tryParse(body) {
			return body, primaryDocURL, ""
		}
	}
	// Path 2: index.json fallback. Strip dashes from accession; build the
	// canonical archive directory URL.
	_, noDashes, accErr := normalizeAccession(accession)
	if accErr != nil {
		return nil, "", "accession normalization failed: " + accErr.Error()
	}
	cikInt, _ := strconv.Atoi(strings.TrimLeft(cik, "0"))
	if cikInt == 0 {
		return nil, "", "could not parse numeric CIK from " + cik
	}
	indexURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/index.json", cikInt, noDashes)
	idxBody, _, idxErr := fetchAbsoluteRaw(ctx, c, indexURL)
	if idxErr != nil {
		return nil, "", "index.json fetch failed (" + indexURL + "): " + idxErr.Error()
	}
	var idx struct {
		Directory struct {
			Item []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"item"`
		} `json:"directory"`
	}
	if err := json.Unmarshal(idxBody, &idx); err != nil {
		return nil, "", "index.json parse failed: " + err.Error()
	}
	// Pick the first top-level .xml file that is NOT inside the xsl* wrapper
	// directory. Form 4 XML files commonly named `wf-form4_*.xml` or
	// `primary_doc.xml`, but we accept any .xml at the top level.
	var xmlName string
	for _, it := range idx.Directory.Item {
		name := it.Name
		if !strings.HasSuffix(strings.ToLower(name), ".xml") {
			continue
		}
		if strings.Contains(name, "/") {
			continue // skip nested (xslF345X05/...)
		}
		// Prefer names that look like Form 4 specifically
		lower := strings.ToLower(name)
		if strings.Contains(lower, "form4") || strings.Contains(lower, "form-4") ||
			strings.Contains(lower, "ownership") || lower == "primary_doc.xml" {
			xmlName = name
			break
		}
		if xmlName == "" {
			xmlName = name // fallback first match
		}
	}
	if xmlName == "" {
		return nil, "", "no top-level .xml file found in index.json for accession " + accession
	}
	xmlURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/%s", cikInt, noDashes, xmlName)
	body, _, ferr := fetchAbsoluteRaw(ctx, c, xmlURL)
	if ferr != nil {
		return nil, "", "fallback XML fetch failed (" + xmlURL + "): " + ferr.Error()
	}
	if !tryParse(body) {
		return nil, "", "fallback XML at " + xmlURL + " is not a Form 4 ownershipDocument"
	}
	return body, xmlURL, ""
}

// min — Go's built-in stdlib min requires 1.21+; provide local for clarity.
// (No-op if generated code already has min; Go 1.26 ships it as builtin so
// this is redundant but harmless under -lint.)

// tickerCacheTTL — 24h per brief.
const tickerCacheTTL = 24 * time.Hour

// resolveTickerToCIK returns the 10-digit zero-padded CIK for a ticker. Looks
// in the local edgar_companies cache first (24h TTL); on miss, fetches the
// full company_tickers.json file from SEC, populates the cache, and returns.
func resolveTickerToCIK(ctx context.Context, c *client.Client, db *store.Store, ticker string) (store.EdgarCompany, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		return store.EdgarCompany{}, fmt.Errorf("empty ticker")
	}
	// Cache lookup
	cached, err := db.LookupEdgarCompanyByTicker(ctx, ticker)
	if err == nil && time.Since(time.Unix(cached.CachedAt, 0)) < tickerCacheTTL {
		return cached, nil
	}
	// PATCH: dropped dead `if err == nil { _ = cached }` block — it was nested
	// inside `if err != nil` and therefore unreachable. Original intent was
	// the stale-fallback below; that is preserved.
	if err := refreshTickerCache(ctx, c, db); err != nil {
		// Fall back to stale entry if we have one.
		if cached.CIK != "" {
			return cached, nil
		}
		return store.EdgarCompany{}, err
	}
	cached, err = db.LookupEdgarCompanyByTicker(ctx, ticker)
	if err != nil {
		return store.EdgarCompany{}, fmt.Errorf("ticker %q not found in SEC company_tickers index", ticker)
	}
	return cached, nil
}

// refreshTickerCache fetches the full company_tickers.json and populates the cache.
func refreshTickerCache(ctx context.Context, c *client.Client, db *store.Store) error {
	if err := requireEdgarUA(c); err != nil {
		return err
	}
	body, _, err := fetchAbsoluteRaw(ctx, c, "https://www.sec.gov/files/company_tickers.json")
	if err != nil {
		return fmt.Errorf("fetching company_tickers.json: %w", err)
	}
	// Shape: {"0":{"cik_str":320193,"ticker":"AAPL","title":"Apple Inc."},...}
	var rows map[string]struct {
		CIKStr int    `json:"cik_str"`
		Ticker string `json:"ticker"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return fmt.Errorf("parsing company_tickers.json: %w", err)
	}
	now := time.Now().Unix()
	for _, r := range rows {
		if r.Ticker == "" {
			continue
		}
		ec := store.EdgarCompany{
			CIK:      fmt.Sprintf("%010d", r.CIKStr),
			Ticker:   strings.ToUpper(r.Ticker),
			Name:     r.Title,
			CachedAt: now,
		}
		_ = db.UpsertEdgarCompany(ctx, ec)
	}
	return nil
}

// resolveCIKOrTicker returns a normalized 10-digit CIK from either a ticker
// or a CIK string. Use this in commands that accept either form.
func resolveCIKOrTicker(ctx context.Context, c *client.Client, db *store.Store, input string) (store.EdgarCompany, error) {
	input = strings.TrimSpace(input)
	// Heuristic: if input is all digits (with optional CIK prefix), treat as CIK
	probe := strings.TrimPrefix(strings.ToUpper(input), "CIK")
	allDigits := true
	for _, r := range probe {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && probe != "" {
		cik, err := normalizeCIK(input)
		if err != nil {
			return store.EdgarCompany{}, err
		}
		// Try cache for name
		rows, _ := db.Query(`SELECT cik, ticker, name, COALESCE(sic,''), cached_at FROM edgar_companies WHERE cik = ? LIMIT 1`, cik)
		if rows != nil {
			defer rows.Close()
			if rows.Next() {
				var ec store.EdgarCompany
				if err := rows.Scan(&ec.CIK, &ec.Ticker, &ec.Name, &ec.SIC, &ec.CachedAt); err == nil {
					return ec, nil
				}
			}
		}
		return store.EdgarCompany{CIK: cik, Ticker: ""}, nil
	}
	return resolveTickerToCIK(ctx, c, db, input)
}

// SubmissionsResponse models the recent-filings slice of data.sec.gov/submissions/CIK*.json.
type SubmissionsResponse struct {
	CIK     string `json:"cik"`
	Name    string `json:"name"`
	Filings struct {
		Recent struct {
			AccessionNumber []string `json:"accessionNumber"`
			FilingDate      []string `json:"filingDate"`
			Form            []string `json:"form"`
			PrimaryDocument []string `json:"primaryDocument"`
			ReportDate      []string `json:"reportDate"`
			Items           []string `json:"items"`
		} `json:"recent"`
	} `json:"filings"`
	SIC string `json:"sic"`
}

// fetchSubmissions returns the parsed submissions index for a CIK. Also
// upserts each row into edgar_filings (without body_text) for downstream use.
func fetchSubmissions(ctx context.Context, c *client.Client, db *store.Store, cik string) (*SubmissionsResponse, error) {
	if err := requireEdgarUA(c); err != nil {
		return nil, err
	}
	body, _, err := fetchAbsoluteRaw(ctx, c, fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik))
	if err != nil {
		return nil, fmt.Errorf("fetching submissions: %w", err)
	}
	var sub SubmissionsResponse
	if err := json.Unmarshal(body, &sub); err != nil {
		return nil, fmt.Errorf("parsing submissions: %w", err)
	}
	// Upsert filings into edgar_filings (no body). Best-effort.
	if db != nil {
		now := time.Now().Unix()
		recent := sub.Filings.Recent
		for i, acc := range recent.AccessionNumber {
			if i >= len(recent.Form) || i >= len(recent.FilingDate) {
				break
			}
			withDashes, noDashes, accErr := normalizeAccession(acc)
			if accErr != nil {
				continue
			}
			primary := ""
			if i < len(recent.PrimaryDocument) {
				primary = recent.PrimaryDocument[i]
			}
			var primaryURL string
			if primary != "" {
				cikInt, _ := strconv.ParseInt(strings.TrimLeft(cik, "0"), 10, 64)
				primaryURL = fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/%s", cikInt, noDashes, primary)
			}
			f := store.EdgarFiling{
				Accession:     withDashes,
				CIK:           cik,
				FormType:      recent.Form[i],
				FiledAt:       recent.FilingDate[i],
				PrimaryDocURL: primaryURL,
				CachedAt:      now,
			}
			_ = db.UpsertEdgarFiling(ctx, f)
		}
	}
	return &sub, nil
}

// fetchFilingBody downloads + caches the primary document text for a filing.
// strip-tags only; the caller should treat it as semi-structured plain text.
func fetchFilingBody(ctx context.Context, c *client.Client, db *store.Store, f *store.EdgarFiling) (string, error) {
	if f.BodyText != "" {
		return f.BodyText, nil
	}
	if f.PrimaryDocURL == "" {
		return "", fmt.Errorf("no primary_doc_url for accession %s", f.Accession)
	}
	body, _, err := fetchAbsoluteRaw(ctx, c, f.PrimaryDocURL)
	if err != nil {
		return "", err
	}
	text := stripHTMLTags(string(body))
	f.BodyText = text
	f.BodyCachedAt = time.Now().Unix()
	_ = db.UpsertEdgarFiling(ctx, *f)
	return text, nil
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)
var multiWSRE = regexp.MustCompile(`[ \t]+`)
var multiNLRE = regexp.MustCompile(`\n{3,}`)

func stripHTMLTags(s string) string {
	s = htmlTagRE.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&#160;", " ")
	s = strings.ReplaceAll(s, "&#8217;", "'")
	s = strings.ReplaceAll(s, "&#8220;", "\"")
	s = strings.ReplaceAll(s, "&#8221;", "\"")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = multiWSRE.ReplaceAllString(s, " ")
	s = multiNLRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// --- Form 4 XML parsing ---

type form4OwnershipDoc struct {
	XMLName xml.Name `xml:"ownershipDocument"`
	Issuer  struct {
		IssuerCIK  string `xml:"issuerCik"`
		IssuerName string `xml:"issuerName"`
	} `xml:"issuer"`
	ReportingOwner []struct {
		ReportingOwnerID struct {
			RptOwnerCIK  string `xml:"rptOwnerCik"`
			RptOwnerName string `xml:"rptOwnerName"`
		} `xml:"reportingOwnerId"`
		ReportingOwnerRelationship struct {
			IsDirector   string `xml:"isDirector"`
			IsOfficer    string `xml:"isOfficer"`
			IsTenPercent string `xml:"isTenPercentOwner"`
			OfficerTitle string `xml:"officerTitle"`
		} `xml:"reportingOwnerRelationship"`
	} `xml:"reportingOwner"`
	NonDerivativeTable struct {
		Transactions []form4Tx `xml:"nonDerivativeTransaction"`
	} `xml:"nonDerivativeTable"`
	DerivativeTable struct {
		Transactions []form4Tx `xml:"derivativeTransaction"`
	} `xml:"derivativeTable"`
}

type form4Tx struct {
	TransactionDate struct {
		Value string `xml:"value"`
	} `xml:"transactionDate"`
	TransactionCoding struct {
		TransactionCode string `xml:"transactionCode"`
	} `xml:"transactionCoding"`
	TransactionAmounts struct {
		Shares struct {
			Value string `xml:"value"`
		} `xml:"transactionShares"`
		Price struct {
			Value string `xml:"value"`
		} `xml:"transactionPricePerShare"`
		AcquiredDisposed struct {
			Value string `xml:"value"`
		} `xml:"transactionAcquiredDisposedCode"`
	} `xml:"transactionAmounts"`
	PostTransactionAmounts struct {
		SharesOwned struct {
			Value string `xml:"value"`
		} `xml:"sharesOwnedFollowingTransaction"`
	} `xml:"postTransactionAmounts"`
}

var seniorOfficerRE = regexp.MustCompile(
	`(?i)\b(chief executive officer|ceo|chief financial officer|cfo|chief operating officer|coo|chief technology officer|cto|chairman|chair of the board|president)\b`)

// parseForm4 parses an ownershipDocument XML body into a slice of typed
// transactions. The issuer CIK is the cik arg (zero-padded). Returns rows
// not yet persisted; caller upserts.
func parseForm4(accession, issuerCIK string, body []byte) ([]store.EdgarInsiderTransaction, error) {
	var doc form4OwnershipDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parsing Form 4 XML: %w", err)
	}
	if len(doc.ReportingOwner) == 0 {
		return nil, nil
	}
	var out []store.EdgarInsiderTransaction
	for _, owner := range doc.ReportingOwner {
		title := owner.ReportingOwnerRelationship.OfficerTitle
		isDirector := owner.ReportingOwnerRelationship.IsDirector == "1" || strings.EqualFold(owner.ReportingOwnerRelationship.IsDirector, "true")
		isSenior := seniorOfficerRE.MatchString(title)

		appendTx := func(txs []form4Tx) {
			for _, tx := range txs {
				code := strings.ToUpper(strings.TrimSpace(tx.TransactionCoding.TransactionCode))
				if code == "" {
					continue
				}
				shares, _ := strconv.ParseFloat(strings.TrimSpace(tx.TransactionAmounts.Shares.Value), 64)
				price, _ := strconv.ParseFloat(strings.TrimSpace(tx.TransactionAmounts.Price.Value), 64)
				owned, _ := strconv.ParseFloat(strings.TrimSpace(tx.PostTransactionAmounts.SharesOwned.Value), 64)
				value := 0.0
				if shares > 0 && price > 0 {
					value = shares * price
				}
				out = append(out, store.EdgarInsiderTransaction{
					Accession:        accession,
					CIK:              issuerCIK,
					ReporterCIK:      owner.ReportingOwnerID.RptOwnerCIK,
					ReporterName:     owner.ReportingOwnerID.RptOwnerName,
					ReporterTitle:    title,
					IsSeniorOfficer:  isSenior,
					IsDirector:       isDirector,
					TransactionDate:  strings.TrimSpace(tx.TransactionDate.Value),
					TransactionCode:  code,
					IsDiscretionary:  code == "S",
					Shares:           shares,
					PricePerShare:    price,
					ValueUSD:         value,
					AcquiredDisposed: strings.TrimSpace(tx.TransactionAmounts.AcquiredDisposed.Value),
					SharesOwnedAfter: owned,
				})
			}
		}
		appendTx(doc.NonDerivativeTable.Transactions)
		appendTx(doc.DerivativeTable.Transactions)
	}
	return out, nil
}

// --- 8-K Item parsing ---

var eightKItemRE = regexp.MustCompile(`(?i)\bITEM\s+(\d+\.\d+)\b`)

// parseEightKItems extracts the list of distinct Item codes appearing in
// an 8-K body. Order-preserving, deduplicated.
func parseEightKItems(body string) []string {
	matches := eightKItemRE.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		code := m[1]
		if !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	return out
}

// validEightKItems is the canonical 8-K Item taxonomy. Used to filter noise.
var validEightKItems = map[string]bool{
	"1.01": true, "1.02": true, "1.03": true, "1.04": true,
	"2.01": true, "2.02": true, "2.03": true, "2.04": true, "2.05": true, "2.06": true,
	"3.01": true, "3.02": true, "3.03": true,
	"4.01": true, "4.02": true,
	"5.01": true, "5.02": true, "5.03": true, "5.04": true, "5.05": true, "5.06": true, "5.07": true, "5.08": true,
	"6.01": true, "6.02": true, "6.03": true, "6.04": true, "6.05": true,
	"7.01": true, "8.01": true, "9.01": true,
}

// extractFirstSentence returns the first sentence after the Item header,
// capped at maxLen characters. For best-effort 8-K summaries.
func extractFirstSentenceAfterItem(body, item string, maxLen int) string {
	re := regexp.MustCompile(`(?i)\bITEM\s+` + regexp.QuoteMeta(item) + `\b`)
	loc := re.FindStringIndex(body)
	if loc == nil {
		return ""
	}
	tail := body[loc[1]:]
	tail = strings.TrimSpace(tail)
	// Cut at first period or 200 chars
	if idx := strings.IndexAny(tail, ".\n"); idx > 0 && idx < maxLen {
		tail = tail[:idx]
	} else if len(tail) > maxLen {
		tail = tail[:maxLen]
	}
	return strings.TrimSpace(tail)
}

// --- sections parser (10-K/10-Q Item extraction) ---

// itemHeaderRE matches "ITEM 1A.", "Item 7.", "ITEM 9B.", etc. Anchored on
// the start of a line OR preceded by whitespace, because SEC bodies are HTML
// and after tag-stripping the ITEM header lands mid-line of normalized text.
var itemHeaderRE = regexp.MustCompile(`(?im)(?:^|\s)(?:PART\s+[IVX]+[\.\s]*)?ITEM\s+(\d+[A-Z]?)\b\.?\s*([^\n]{0,200})`)

// boundaryWSRE collapses any run of whitespace into a single space, so
// ITEM headers buried in nested HTML markup land on a normalized text
// stream for the boundary regex below. (htmlTagRE + stripHTMLTags helpers
// live earlier in this file.)
var boundaryWSRE = regexp.MustCompile(`\s+`)

// stripHTMLForBoundary turns a raw 10-K/10-Q HTML body into a plain-text
// stream suitable for ITEM-header regex scanning. Returns the normalized
// body. Byte offsets reported by extractSections are into this stripped
// form, not the original HTML; v1 acceptable since the offsets are useful
// to LODESTAR as positional pointers within the agent-readable text.
func stripHTMLForBoundary(body string) string {
	s := stripHTMLTags(body)
	return boundaryWSRE.ReplaceAllString(s, " ")
}

// SectionResult is either a parsed section text or a boundary_unverifiable error.
type SectionResult struct {
	Item       string `json:"item"`
	TextOffset int    `json:"text_offset,omitempty"`
	TextLength int    `json:"text_length,omitempty"`
	Text       string `json:"text,omitempty"`
	Error      string `json:"error,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Candidates []int  `json:"candidates,omitempty"`
}

// extractSections parses requested items from body. Applies the boundary
// safety contract — never best-effort. Returns one SectionResult per requested
// item, plus a bool indicating whether ANY item failed (so callers can set
// exit code 2).
func extractSections(body string, items []string) ([]SectionResult, bool) {
	// PATCH(phase5: sections-html-strip-pre-pass): Pre-strip HTML — SEC 10-K
	// bodies embed ITEM headers inside font/span/b tags, so the regex can't
	// match raw HTML. After this step, all offsets and emitted text are
	// against the stripped form.
	body = stripHTMLForBoundary(body)
	// Build map of itemID → list of (start, end) byte ranges.
	allMatches := itemHeaderRE.FindAllStringSubmatchIndex(body, -1)
	if len(allMatches) == 0 {
		// No item headers at all
		var out []SectionResult
		for _, it := range items {
			out = append(out, SectionResult{Item: it, Error: "boundary_unverifiable",
				Reason: "no ITEM headers found in body"})
		}
		return out, true
	}
	type hit struct {
		item  string
		start int
		end   int // byte after the header line; section content starts here
	}
	var hits []hit
	for _, m := range allMatches {
		// m[0], m[1] = full match; m[2], m[3] = first captured group (item id)
		itemID := strings.ToUpper(body[m[2]:m[3]])
		hits = append(hits, hit{item: itemID, start: m[0], end: m[1]})
	}
	// For each requested item, find unambiguous boundary.
	results := make([]SectionResult, 0, len(items))
	anyFailed := false
	for _, requested := range items {
		req := strings.ToUpper(strings.TrimSpace(requested))
		var matchIdxs []int
		for i, h := range hits {
			if h.item == req {
				matchIdxs = append(matchIdxs, i)
			}
		}
		if len(matchIdxs) == 0 {
			results = append(results, SectionResult{Item: req, Error: "boundary_unverifiable",
				Reason: "item header not found in body"})
			anyFailed = true
			continue
		}
		// Compute substantive content size for each candidate.
		type cand struct {
			idx   int
			start int
			end   int
			size  int
		}
		var cands []cand
		for _, i := range matchIdxs {
			start := hits[i].end
			end := len(body)
			if i+1 < len(hits) {
				end = hits[i+1].start
			}
			cands = append(cands, cand{idx: i, start: start, end: end, size: end - start})
		}
		// Filter to "substantial" candidates (>2KB body)
		var substantial []cand
		for _, c := range cands {
			if c.size > 2048 {
				substantial = append(substantial, c)
			}
		}
		var chosen cand
		switch len(substantial) {
		case 0:
			// No substantial candidate — only TOC entries, ambiguous.
			var offsets []int
			for _, c := range cands {
				offsets = append(offsets, c.start)
			}
			results = append(results, SectionResult{Item: req, Error: "boundary_unverifiable",
				Reason: "no candidate has substantial body content (>2KB)", Candidates: offsets})
			anyFailed = true
			continue
		case 1:
			chosen = substantial[0]
		default:
			// PATCH(phase5: sections-v1.1-disambiguation): multiple substantial
			// candidates. The modern SEC 10-K shape is TOC-entry-for-Item-X
			// (early in document) followed by body-Item-X (later in document).
			// v1.1 heuristic: if the LATEST candidate is
			// >3x larger than every earlier substantial candidate, prefer it.
			// Boundary safety still holds — when the ratio test fails we fall
			// back to boundary_unverifiable with the candidate offsets.
			sort.Slice(substantial, func(i, j int) bool { return substantial[i].start < substantial[j].start })
			last := substantial[len(substantial)-1]
			var maxOther int
			for _, c := range substantial[:len(substantial)-1] {
				if c.size > maxOther {
					maxOther = c.size
				}
			}
			if maxOther > 0 && last.size > 3*maxOther {
				chosen = last
				break
			}
			var offsets []int
			for _, c := range substantial {
				offsets = append(offsets, c.start)
			}
			results = append(results, SectionResult{Item: req, Error: "boundary_unverifiable",
				Reason:     "multiple candidate boundaries with substantial content; could not disambiguate (later-candidate not >3x larger than earlier)",
				Candidates: offsets})
			anyFailed = true
			continue
		}
		// Extract the chosen range
		text := body[chosen.start:chosen.end]
		text = strings.TrimSpace(text)
		if len(text) < 200 {
			results = append(results, SectionResult{Item: req, Error: "boundary_unverifiable",
				Reason: "extracted section is too small (<200 bytes) to be a real Item body"})
			anyFailed = true
			continue
		}
		results = append(results, SectionResult{
			Item:       req,
			TextOffset: chosen.start,
			TextLength: len(text),
			Text:       text,
		})
	}
	return results, anyFailed
}

// --- date helpers ---

// parseSinceDate accepts ISO8601 date or "12mo"/"90d" relative shorthand and
// returns the ISO date string suitable for >= comparisons.
func parseSinceDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	// ISO date
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02"), nil
	}
	// Relative: 90d, 12mo, 1y
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil {
			return time.Now().AddDate(0, 0, -n).Format("2006-01-02"), nil
		}
	}
	if strings.HasSuffix(s, "mo") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "mo"))
		if err == nil {
			return time.Now().AddDate(0, -n, 0).Format("2006-01-02"), nil
		}
	}
	if strings.HasSuffix(s, "y") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "y"))
		if err == nil {
			return time.Now().AddDate(-n, 0, 0).Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unrecognized --since value: %q (use ISO date, e.g., 2024-01-15, or 90d/12mo/1y)", s)
}

// _ = url to keep import stable if added later.
var _ = url.QueryEscape
