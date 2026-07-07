// Copyright 2026 ChrisDrit and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ownership pulls a company's latest DEF 14A proxy statement and extracts the
// "Security Ownership of Certain Beneficial Owners" section as readable text.
// It is the one disclosure every proxy carries under a near-identical heading,
// and the only one that requires chaining submissions -> document fetch ->
// HTML section extraction, which no single SEC endpoint provides.

// ---- HTML -> text -----------------------------------------------------------

var (
	// Separate script/style regexes: RE2 has no backreferences, so a single
	// `<(script|style)>...</(script|style)>` could match a `<script>` open
	// against an unrelated `</style>` close. Matching each tag independently
	// keeps the open/close pinned to the same element.
	ownReScript     = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	ownReStyle      = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	ownReBlockTag   = regexp.MustCompile(`(?is)</?(p|div|tr|br|h[1-6]|li|table|thead|tbody)\b[^>]*>`)
	ownReTag        = regexp.MustCompile(`(?s)<[^>]+>`)
	ownReWS         = regexp.MustCompile(`[ \t]+`)
	ownReBlankLines = regexp.MustCompile(`\n{3,}`)
)

// ownershipHTMLToText converts an HTML document to readable plain text:
// scripts/styles removed, block-level tags turned into line breaks, remaining
// tags stripped, HTML entities decoded (named, decimal, and hex — via the
// stdlib, so e.g. "&#37;" becomes "%" rather than being dropped), and
// whitespace collapsed.
func ownershipHTMLToText(doc string) string {
	s := ownReScript.ReplaceAllString(doc, " ")
	s = ownReStyle.ReplaceAllString(s, " ")
	s = ownReBlockTag.ReplaceAllString(s, "\n")
	s = ownReTag.ReplaceAllString(s, " ")
	// Decode entities with the stdlib unescaper, then normalize non-breaking
	// spaces (which decode to U+00A0) to plain spaces so the collapse below
	// catches them.
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ownReWS.ReplaceAllString(ln, " "))
	}
	s = strings.Join(lines, "\n")
	s = ownReBlankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// ---- Ownership section extraction ------------------------------------------

// ownershipHeadings are the canonical and common alternate section titles for
// the beneficial-ownership disclosure across issuers. The first entry is the
// canonical phrase; the rest are observed variants (e.g. some issuers use
// "Principal Shareholders").
var ownershipHeadings = []string{
	"security ownership of certain beneficial owners",
	"security ownership of certain beneficial owners and management",
	"security ownership of management and certain beneficial owners",
	"security ownership",
	"beneficial ownership of",
	"ownership of securities",
	"principal shareholders",
	"principal stockholders",
	"beneficial owners",
}

// ownershipNextHeadingRe matches the start of a plausible following section,
// used to bound the extracted ownership text. It covers both numbered headings
// ("Item 12", "Proposal 3") and the bare narrative headings ("Director
// Compensation", "Board of Directors") that commonly follow the ownership
// table without an "Item"/"Proposal" prefix.
var ownershipNextHeadingRe = regexp.MustCompile(`(?im)^\s*(item\s+\d+|proposal\s+\d+|equity compensation plan|executive compensation|compensation discussion|director compensation|named executive|board of directors|corporate governance|section 16|certain relationships|related party transactions|audit committee|report of the|annex [a-z]|general information)\b`)

// ownershipTokenRe scores how "ownership-ish" a span of text is. Alongside
// generic terms it names a few recurring institutional-holder words (the big
// index funds plus the common entity suffixes fund/corp/llc/trust) so the
// scorer still discriminates for filers whose holders aren't the big three.
var ownershipTokenRe = regexp.MustCompile(`(?i)\b(shares?|beneficial|percent|stock|vanguard|blackrock|state street|fund|corp|llc|trust|holdings?|trustee)\b|%|\d{3,}`)

const (
	ownershipMaxSectionLen  = 18000
	ownershipScoreWindow    = 2500
	ownershipFallbackWindow = 4000
)

// OwnershipResult is the outcome of extracting the ownership section.
type OwnershipResult struct {
	Text     string // extracted readable text
	Heading  string // the heading phrase that matched
	Fallback bool   // true if no heading matched and a fuzzy fallback was used
}

// ExtractOwnershipSection finds and returns the beneficial-ownership section of
// a proxy statement's plain text. It scores every occurrence of a known heading
// by the density of ownership tokens in the bounded span that follows (up to
// the next major heading, capped by ownershipScoreWindow) and returns the
// densest one. Scoring the bounded span — not a fixed forward window — keeps a
// table-of-contents mention of the heading from spuriously winning over the
// real, token-dense section. If no heading matches, it falls back to a window
// around the densest ownership-token cluster and sets Fallback=true. It never
// returns empty text when the input is non-empty.
func ExtractOwnershipSection(text string) OwnershipResult {
	low := strings.ToLower(text)

	bestScore := -1
	bestStart := -1
	bestHeading := ""
	for _, hd := range ownershipHeadings {
		from := 0
		for {
			idx := strings.Index(low[from:], hd)
			if idx < 0 {
				break
			}
			pos := from + idx
			from = pos + len(hd)
			spanEnd := pos + ownershipScoreWindow
			if spanEnd > len(text) {
				spanEnd = len(text)
			}
			searchFrom := pos + len(hd)
			if searchFrom < spanEnd {
				if loc := ownershipNextHeadingRe.FindStringIndex(text[searchFrom:spanEnd]); loc != nil {
					spanEnd = searchFrom + loc[0]
				}
			}
			score := len(ownershipTokenRe.FindAllStringIndex(text[pos:spanEnd], -1))
			if score > bestScore {
				bestScore = score
				bestStart = pos
				bestHeading = hd
			}
		}
	}

	if bestStart >= 0 {
		end := bestStart + ownershipMaxSectionLen
		if end > len(text) {
			end = len(text)
		}
		searchFrom := bestStart + len(bestHeading)
		if loc := ownershipNextHeadingRe.FindStringIndex(text[searchFrom:end]); loc != nil {
			end = searchFrom + loc[0]
		}
		section := strings.TrimSpace(text[bestStart:end])
		if section != "" {
			return OwnershipResult{Text: section, Heading: bestHeading}
		}
	}

	matches := ownershipTokenRe.FindAllStringIndex(text, -1)
	if len(matches) > 0 {
		// Pick the densest window: for each token start, count how many token
		// matches fall within an ownershipFallbackWindow-sized window beginning
		// there and keep the start with the most. A two-pointer sweep over the
		// already-sorted match starts keeps this linear. This replaces a naive
		// "median match" center, which could land in a sparse region when
		// tokens are spread across several distant sections.
		bestStartIdx, bestCount, j := 0, 0, 0
		for i := range matches {
			if j < i {
				j = i
			}
			for j < len(matches) && matches[j][0] < matches[i][0]+ownershipFallbackWindow {
				j++
			}
			if j-i > bestCount {
				bestCount = j - i
				bestStartIdx = i
			}
		}
		start := matches[bestStartIdx][0]
		end := start + ownershipFallbackWindow
		if end > len(text) {
			end = len(text)
		}
		return OwnershipResult{Text: strings.TrimSpace(text[start:end]), Fallback: true}
	}

	end := ownershipFallbackWindow
	if end > len(text) {
		end = len(text)
	}
	return OwnershipResult{Text: strings.TrimSpace(text[:end]), Fallback: true}
}

// ---- command ----------------------------------------------------------------

// resolveOwnershipCIK turns a ticker, company name fragment, or raw CIK into a
// 10-digit zero-padded CIK. Raw all-digit input is padded directly; otherwise
// the ticker map is consulted (exact ticker match first, then a case-insensitive
// title substring match).
func resolveOwnershipCIK(c clientLike, input string) (cik, title string, err error) {
	in := strings.TrimSpace(input)
	if in == "" {
		return "", "", fmt.Errorf("empty company reference")
	}
	allDigits := true
	for _, r := range in {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		p, perr := padCIK(in)
		if perr != nil {
			return "", "", perr
		}
		return p, "", nil
	}
	rows, ferr := fetchTickerMap(c)
	if ferr != nil {
		return "", "", ferr
	}
	upper := strings.ToUpper(in)
	for _, r := range rows {
		if strings.ToUpper(r.Ticker) == upper {
			return r.Padded, r.Title, nil
		}
	}
	low := strings.ToLower(in)
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Title), low) {
			return r.Padded, r.Title, nil
		}
	}
	return "", "", fmt.Errorf("no company found matching %q (try an exact ticker like AAPL, a 10-digit CIK, or a name fragment)", input)
}

func newOwnershipCmd(flags *rootFlags) *cobra.Command {
	var savePath string

	cmd := &cobra.Command{
		Use:   "ownership <ticker-or-cik>",
		Short: "Extract the Security Ownership of Certain Beneficial Owners section from a company's latest DEF 14A.",
		Long: "Resolve a ticker, name, or CIK to a company, find its most recent DEF 14A\n" +
			"proxy statement, fetch the document, and extract the \"Security Ownership of\n" +
			"Certain Beneficial Owners\" section as readable text. This is the disclosure\n" +
			"that lists who beneficially owns the company, present under a near-identical\n" +
			"heading in every proxy statement filed before an annual shareholder meeting.",
		Example: "  sec-edgar-pp-cli ownership MSFT\n" +
			"  sec-edgar-pp-cli ownership AAPL --json\n" +
			"  sec-edgar-pp-cli ownership 0000320193 --save apple-ownership.txt",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("ownership requires a ticker, name, or CIK argument"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			cik, title, err := resolveOwnershipCIK(c, args[0])
			if err != nil {
				return notFoundErr(err)
			}

			subs, err := fetchSubmissions(c, cik)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if title == "" {
				title = subs.Name
			}

			// Find the most recent DEF 14A. filings.recent is reverse-chronological,
			// but sort defensively in case ordering ever changes.
			type proxyRef struct {
				accession string
				date      string
				doc       string
			}
			var proxies []proxyRef
			rec := subs.Filings.Recent
			n := len(rec.Form)
			for i := 0; i < n; i++ {
				if !strings.EqualFold(strings.TrimSpace(rec.Form[i]), "DEF 14A") {
					continue
				}
				p := proxyRef{}
				if i < len(rec.AccessionNumber) {
					p.accession = rec.AccessionNumber[i]
				}
				if i < len(rec.FilingDate) {
					p.date = rec.FilingDate[i]
				}
				if i < len(rec.PrimaryDocument) {
					p.doc = rec.PrimaryDocument[i]
				}
				// Skip entries missing the fields needed to build a document
				// URL — a short or mismatched parallel array would otherwise
				// yield an invalid archive URL that fails opaquely downstream.
				if p.accession == "" || p.doc == "" {
					continue
				}
				proxies = append(proxies, p)
			}
			if len(proxies) == 0 {
				msg := fmt.Sprintf("no DEF 14A proxy statement found for %s (CIK %s) in the recent-filings window", title, cik)
				// filings.recent holds only ~400 of the most recent filings; a
				// high-frequency filer's DEF 14A can sit in the older
				// filings.files pages. Surface that so the result isn't
				// mistaken for "this company never filed a proxy".
				if older := len(subs.Filings.Files); older > 0 {
					msg += fmt.Sprintf(" (%d older filing page(s) exist beyond the recent window and were not searched)", older)
				}
				return notFoundErr(fmt.Errorf("%s", msg))
			}
			sort.SliceStable(proxies, func(i, j int) bool { return proxies[i].date > proxies[j].date })
			latest := proxies[0]

			docURL := archiveBase(cik, latest.accession) + latest.doc
			body, err := fetchSECRaw(c, docURL)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			text := ownershipHTMLToText(string(body))
			res := ExtractOwnershipSection(text)

			if savePath != "" {
				if werr := os.WriteFile(savePath, []byte(res.Text), 0o644); werr != nil {
					return fmt.Errorf("writing --save file: %w", werr)
				}
			}

			if flags.asJSON || !wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := map[string]any{
					"company":     title,
					"cik":         cik,
					"accession":   latest.accession,
					"filing_date": latest.date,
					"source_url":  docURL,
					"heading":     res.Heading,
					"fallback":    res.Fallback,
					"section":     res.Text,
				}
				if savePath != "" {
					out["saved_to"] = savePath
				}
				return flags.printJSON(cmd, out)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Security Ownership — %s (CIK %s)\n", title, cik)
			fmt.Fprintf(w, "DEF 14A filed %s · %s\n", latest.date, docURL)
			if res.Fallback {
				fmt.Fprintln(w, "(heading not matched exactly — showing the densest ownership passage)")
			}
			fmt.Fprintln(w)
			fmt.Fprintln(w, res.Text)
			if savePath != "" {
				fmt.Fprintf(w, "\n(saved to %s)\n", savePath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&savePath, "save", "", "Write the extracted section to this file path")
	return cmd
}
