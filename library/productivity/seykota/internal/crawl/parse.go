// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

// Package crawl fetches and parses pages from seykota.com into corpus.Doc
// records. seykota.com is flat 1990s static HTML, so the parser is
// deliberately regex-based (no DOM dependency) and tolerant of broken markup.
package crawl

import (
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/corpus"
)

const base = "https://www.seykota.com"

var (
	reScriptStyle = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>|<style\b[^>]*>.*?</style\s*>|<head\b[^>]*>.*?</head\s*>`)
	reComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	reTag         = regexp.MustCompile(`(?s)<[^>]+>`)
	reTitle       = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</\s*title\s*>`)
	reHref        = regexp.MustCompile(`(?is)href\s*=\s*["']([^"']+)["']`)
	reWS          = regexp.MustCompile(`[ \t\r\f\v]+`)
	reBlankLines  = regexp.MustCompile(`\n{3,}`)
	reNBSP        = regexp.MustCompile(`\x{00a0}+`)
	// FAQ month path: /tt/2023/JAN/01-31/default.html  or  /tribe/FAQ/2006_Apr/22/index.htm
	reFAQMonthTT  = regexp.MustCompile(`(?i)/tt/(\d{4})/([A-Za-z]+)/([\d-]+)/default\.html?$`)
	reFAQDayOld   = regexp.MustCompile(`(?i)/tribe/FAQ/(\d{4})_([A-Za-z]+)/(\d+)/index\.html?$`)
)

var monthNum = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var reBlockTag = regexp.MustCompile(`(?is)<\s*(br|/p|/div|/tr|/td|/li|/h[1-6]|hr)\s*/?\s*>`)

// navLines is the recurring chrome that wraps every seykota.com page: the
// left-nav link list and the FAQ header/footer boilerplate. Lines whose
// trimmed text exactly matches one of these (case-insensitive) are dropped
// from the cleaned body so they don't pollute search or the printed text.
var navLines = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range []string{
		"ed seykota's", "frequently asked questions", "faq index & ground rules",
		"faq index", "ground rules", "tribe directory - how to join", "site search",
		"ttp - the trading tribe process", "the trading tribe process", "glossary",
		"ttp workshop", "resources", "reader feedback", "the trading tribe book",
		"tsp: trading system project", "trading system project", "breathwork",
		"tt associate program", "chart server", "send mail to faq", "workshops",
		"home", "directory", "rocks", "the trading tribe", "epidemic lab",
		"mail", "contact", "essentials", "econowmics", "levitator",
		"...", "==>", "<==", "back | down | forward",
		"view and buy ed's books", "view and buy ed's books.",
	} {
		m[s] = true
	}
	return m
}()

// reFwdNav matches the FAQ "<-- back | down | forward -->" navigation line
// that sits between every FAQ page's header chrome and its content.
var reFwdNav = regexp.MustCompile(`(?i)back\s*\|\s*(down|forward)\b`)

// chromeContains: a line containing any of these substrings is dropped.
var chromeContains = []string{
	"write for permission to reprint", "send mail to faq", "tt_chartbook",
	"tt chartbook", "view and buy ed's books", "faq archives:", "faq archives :",
}

// CleanText strips tags, comments, scripts and styles, decodes HTML
// entities, drops the recurring page chrome (the left-nav links, the FAQ
// header/footer boilerplate, and — for FAQ pages — everything above the
// "back | down | forward" navigation line), and collapses whitespace into
// a readable plain-text body.
func CleanText(htmlStr string) string {
	s := reComment.ReplaceAllString(htmlStr, " ")
	s = reScriptStyle.ReplaceAllString(s, " ")
	// turn block-ish tags into newlines so paragraphs survive
	s = reBlockTag.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = reNBSP.ReplaceAllString(s, " ")
	// drop invalid UTF-8 (raw latin-1 bytes leak from these 1990s pages) and
	// any literal U+FFFD replacement characters
	s = strings.ToValidUTF8(s, "")
	s = strings.ReplaceAll(s, "�", "")
	// normalize line whitespace, drop nav/footer chrome lines, collapse blanks
	in := strings.Split(s, "\n")
	out := make([]string, 0, len(in))
	cutToHere := -1 // index of the FAQ "back | down | forward" line, if seen
	for _, ln := range in {
		ln = strings.TrimSpace(reWS.ReplaceAllString(ln, " "))
		key := strings.ToLower(strings.Trim(ln, " .|<>="))
		if key == "" {
			out = append(out, "")
			continue
		}
		if navLines[key] {
			continue
		}
		dropped := false
		for _, c := range chromeContains {
			if strings.Contains(key, c) {
				dropped = true
				break
			}
		}
		if dropped {
			continue
		}
		if reFwdNav.MatchString(ln) {
			// the FIRST "back | down | forward" line marks the end of a FAQ
			// page's header chrome; later occurrences are the footer nav and
			// must not truncate the content, so only record the first.
			if cutToHere < 0 {
				cutToHere = len(out)
			}
			continue
		}
		// on FAQ pages the header chrome (which is split across many short
		// lines, defeating line-by-line matching) ends right before the
		// "Contributors Say" / "Contributors" lead-in to the first Q&A.
		if cutToHere < 0 && (key == "contributors say" || key == "contributors say:" || key == "contributors" || strings.HasPrefix(key, "contributors say")) {
			cutToHere = len(out)
		}
		out = append(out, ln)
	}
	if cutToHere > 0 && cutToHere < len(out) {
		out = out[cutToHere:]
	}
	s = strings.Join(out, "\n")
	s = reBlankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Title extracts the <title> text, falling back to an empty string.
func Title(htmlStr string) string {
	m := reTitle.FindStringSubmatch(htmlStr)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(reTag.ReplaceAllString(m[1], " ")))
}

// AbsLinks returns the absolute hrefs found in htmlStr, resolved against
// pageURL (which must be an absolute https://www.seykota.com/... URL).
func AbsLinks(htmlStr, pageURL string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range reHref.FindAllStringSubmatch(htmlStr, -1) {
		u := resolveURL(strings.TrimSpace(m[1]), pageURL)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func resolveURL(href, pageURL string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "javascript:") {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		// only keep seykota.com links
		if strings.Contains(href, "seykota.com") {
			return strings.Replace(strings.Replace(href, "http://", "https://", 1), "//www.seykota.com.com", "//www.seykota.com", 1)
		}
		return ""
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	// relative — resolve against pageURL's directory
	dir := pageURL
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		dir = dir[:i+1]
	}
	if strings.HasPrefix(href, "/") {
		return base + href
	}
	for strings.HasPrefix(href, "../") {
		href = href[3:]
		// pop one dir from `dir`
		dir = strings.TrimSuffix(dir, "/")
		if i := strings.LastIndex(dir, "/"); i >= 0 {
			dir = dir[:i+1]
		} else {
			dir = base + "/"
		}
	}
	href = strings.TrimPrefix(href, "./")
	return dir + href
}

// urlPath returns the path portion of a seykota.com URL with the leading
// slash stripped — used as the corpus Doc ID.
func urlPath(u string) string {
	u = strings.TrimPrefix(u, base)
	u = strings.TrimPrefix(u, "/")
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return u
}

// ---- FAQ ----

// FAQMonthLinks extracts the monthly FAQ page URLs from the /tt/FAQ_Index/
// page (and, for the legacy era, the /tribe/FAQ/index.htm page).
func FAQMonthLinks(indexHTML, indexURL string) []string {
	var out []string
	seen := map[string]bool{}
	for _, u := range AbsLinks(indexHTML, indexURL) {
		if reFAQMonthTT.MatchString(u) || reFAQDayOld.MatchString(u) {
			if !seen[u] {
				seen[u] = true
				out = append(out, u)
			}
		}
	}
	return out
}

// ParseFAQMonth turns a monthly FAQ page into a corpus.Doc.
func ParseFAQMonth(pageURL, htmlStr string) corpus.Doc {
	d := corpus.Doc{
		ID: urlPath(pageURL), Source: corpus.SourceFAQ, URL: pageURL,
		Title: Title(htmlStr), Body: CleanText(htmlStr),
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if m := reFAQMonthTT.FindStringSubmatch(pageURL); m != nil {
		d.Year, d.Month, d.Range = m[1], m[2], m[3]
	} else if m := reFAQDayOld.FindStringSubmatch(pageURL); m != nil {
		d.Year, d.Month, d.Range = m[1], m[2], m[3]
	}
	if d.Month != "" {
		d.MonthN = monthNum[strings.ToLower(d.Month[:min3(len(d.Month))])]
	}
	if d.Title == "" {
		d.Title = "Ed's FAQ " + d.Month + " " + d.Year
	}
	d.Contributors = parseContributors(d.Body)
	return d
}

func min3(n int) int {
	if n < 3 {
		return n
	}
	return 3
}

// contribBlockWord is the word/phrases used by the legacy reader-mail era to
// label a list of who wrote in. It is matched at the start of a line.
var reContribLabel = regexp.MustCompile(`(?im)^\s*contributors?\s*:?\s*$`)

// contribStopWords are tokens that, if present in a candidate, disqualify it
// as a person's name — covers nav chrome, TTP meeting structure, weekday and
// month words, and salutations.
var contribStopWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range []string{
		"say", "says", "dear", "sir", "ed", "chief", "hello", "hi", "reply", "clip",
		"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
		"january", "february", "march", "april", "may", "june", "july",
		"august", "september", "october", "november", "december",
		"jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "sept", "oct", "nov", "dec",
		"check", "in", "order", "speaking", "song", "cotton", "fred", "father", "mother",
		"relationship", "rock", "rocks", "process", "workshop", "tribe", "ttp", "faq",
		"home", "mail", "resources", "glossary", "directory", "late", "music",
		"contributors", "contributor", "feedback", "reader", "readers", "archives",
		"the", "and", "with", "from", "about", "this", "that", "page", "site", "search",
	} {
		m[w] = true
	}
	return m
}()

// parseContributors does a best-effort scrape of the "Contributors" block the
// legacy reader-mail era carries near the top of a month-page: a
// comma/newline separated list of who wrote in. It is conservative — a
// candidate must look like a person's name (2–4 words, each starting with an
// uppercase letter, no digits, no stop-words, ≤ 35 chars) — so it under-
// reports rather than emit chrome/garbage. Returns nil when the label or any
// plausible name is absent; callers must tolerate an empty list.
func parseContributors(body string) []string {
	loc := reContribLabel.FindStringIndex(body)
	if loc == nil {
		return nil
	}
	rest := body[loc[1]:]
	// stop at the first strong signal that the mailbag has started
	stopRE := regexp.MustCompile(`(?i)\b(dear ed|dear chief|dear sir|hi ed|hello ed|ed says|reply\s*:|clip\s*:)\b`)
	if l := stopRE.FindStringIndex(rest); l != nil {
		rest = rest[:l[0]]
	}
	if len(rest) > 800 {
		rest = rest[:800]
	}
	var names []string
	seen := map[string]bool{}
	for _, part := range regexp.MustCompile(`[,\n;/|]+`).Split(rest, -1) {
		n := strings.TrimSpace(part)
		if !plausibleName(n) {
			continue
		}
		key := strings.ToLower(n)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, n)
		if len(names) >= 20 {
			break
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func plausibleName(n string) bool {
	n = strings.TrimSpace(n)
	if n == "" || len(n) > 35 {
		return false
	}
	for _, r := range n {
		if r >= '0' && r <= '9' {
			return false
		}
	}
	if strings.ContainsAny(n, ".?!:()\"") {
		return false
	}
	words := strings.Fields(n)
	if len(words) < 2 || len(words) > 4 {
		return false
	}
	for _, w := range words {
		lw := strings.ToLower(strings.Trim(w, "-'"))
		if lw == "" || contribStopWords[lw] {
			return false
		}
		// each word must start with an uppercase letter and be mostly letters
		r := []rune(w)
		if !(r[0] >= 'A' && r[0] <= 'Z') {
			return false
		}
		letters := 0
		for _, c := range w {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				letters++
			}
		}
		if letters*2 < len(w) {
			return false
		}
	}
	return true
}

// ---- TSP ----

// TSPSectionLinks extracts the section page URLs from /tribe/TSP/index.htm.
// It keeps only links under /tribe/TSP/<slug>/ (one level deep), excluding
// the index page itself.
func TSPSectionLinks(indexHTML, indexURL string) []string {
	var out []string
	seen := map[string]bool{}
	reTSP := regexp.MustCompile(`(?i)/tribe/TSP/([A-Za-z0-9_]+)/[Ii]ndex\.html?$`)
	for _, u := range AbsLinks(indexHTML, indexURL) {
		if m := reTSP.FindStringSubmatch(u); m != nil && !strings.EqualFold(m[1], "TSP") {
			if !seen[u] {
				seen[u] = true
				out = append(out, u)
			}
		}
	}
	return out
}

var reUpdated = regexp.MustCompile(`(?i)\bupdated?:?\s+([A-Z][a-z]+\.?\s+\d{1,2},?\s+\d{4}|\d{1,2}\s+[A-Z][a-z]+\.?\s+\d{4})`)

// ParseTSPSection turns a TSP section page into a corpus.Doc.
func ParseTSPSection(pageURL, htmlStr string, ord int) corpus.Doc {
	body := CleanText(htmlStr)
	slug := ""
	if m := regexp.MustCompile(`(?i)/tribe/TSP/([A-Za-z0-9_]+)/`).FindStringSubmatch(pageURL); m != nil {
		slug = m[1]
	}
	title := Title(htmlStr)
	if title == "" || strings.EqualFold(title, "New Page 1") || title == "?" {
		title = slug
	}
	updated := ""
	if m := reUpdated.FindStringSubmatch(body); m != nil {
		updated = strings.TrimSpace(m[1])
	}
	return corpus.Doc{
		ID: urlPath(pageURL), Source: corpus.SourceTSP, URL: pageURL,
		Title: title, Slug: slug, Updated: updated, Ord: ord, Body: body,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// ---- Risk essay ----

// ParseRiskEssay turns the single /tribe/risk/index.htm page into one
// corpus.Doc with the full cleaned body.
func ParseRiskEssay(pageURL, htmlStr string) corpus.Doc {
	body := CleanText(htmlStr)
	title := Title(htmlStr)
	if title == "" {
		title = "Risk Management"
	}
	return corpus.Doc{
		ID: urlPath(pageURL), Source: corpus.SourceRisk, URL: pageURL,
		Title: title, Body: body,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// flexRE turns a heading string into a whitespace-flexible, case-insensitive
// regexp (so a heading split across a line break in the source still matches).
func flexRE(heading string) *regexp.Regexp {
	parts := strings.Fields(strings.TrimSpace(heading))
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	if len(parts) == 0 {
		return nil
	}
	return regexp.MustCompile(`(?is)` + strings.Join(parts, `\s+`))
}

// RiskSectionWindow returns the slice of body text starting at the named
// heading (whitespace-flexible, case-insensitive) up to the next known
// section heading, or the whole body when name is empty (ok=true) or the
// heading isn't found (ok=false).
func RiskSectionWindow(body, name string) (string, bool) {
	if strings.TrimSpace(name) == "" {
		return body, true
	}
	re := flexRE(name)
	if re == nil {
		return body, false
	}
	loc := re.FindStringIndex(body)
	if loc == nil {
		return body, false
	}
	rest := body[loc[0]:]
	headLen := loc[1] - loc[0] // bytes the target heading itself occupies in rest
	end := len(rest)
	for _, h := range riskHeadings {
		if strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(name)) {
			continue
		}
		hre := flexRE(h)
		if hre == nil {
			continue
		}
		// look past the target heading itself, then stop at the next section heading
		if m := hre.FindStringIndex(rest); m != nil && m[0] >= headLen && m[0] < end {
			end = m[0]
		}
	}
	return strings.TrimSpace(rest[:end]), true
}

// RiskHeadings is the curated list of section headings in the risk essay,
// in document order — used for `risk show --list` and `risk show --section`.
var riskHeadings = []string{
	"Risk Management",
	"The Coin Toss Example",
	"Optimal Betting",
	"Hunches and Systems",
	"Fixed Bet and Fixed-Fraction Bet",
	"Simulations",
	"Pyramiding and Martingale",
	"Optimizing - Using Simulation",
	"The Timid Trader Rule",
	"The Bold Trader Rule",
	"Calculus Methods",
	"The Kelly Formula",
	"Graphic Relationships",
	"Diversification",
	"The Uncle Point",
	"Portfolio Selection",
	"Position Sizing",
	"Psychological Considerations",
	"Sharpe Ratio",
	"Lake Ratio",
	"Stress Testing",
}

// RiskHeadings returns a copy of the curated section-heading list.
func RiskHeadings() []string {
	out := make([]string, len(riskHeadings))
	copy(out, riskHeadings)
	return out
}
