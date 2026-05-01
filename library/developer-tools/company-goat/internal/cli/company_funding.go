// Hand-written: funding, funding-trend, and funding --who commands.
// SEC EDGAR Form D extraction is the killer feature.

package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/sec"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/yc"
	"github.com/spf13/cobra"
)

// fundingResult is the JSON shape for `funding <co>`.
type fundingResult struct {
	Domain       string           `json:"domain,omitempty"`
	Query        string           `json:"query,omitempty"`
	Filings      []fundingFiling  `json:"form_d_filings"`
	CIKSummaries []cikSummary     `json:"cik_summaries,omitempty"`
	// IsAmbiguous is true when the name search matched filings spanning
	// more than one SEC CIK. Common false-positive sources: VC funds and
	// long-dormant Delaware shell corps that share a stem with the
	// company you actually meant. Agents should consult cik_summaries
	// and re-call with --cik <id> to disambiguate.
	IsAmbiguous bool             `json:"is_ambiguous,omitempty"`
	YCEntry     *yc.Company      `json:"yc_entry,omitempty"`
	Coverage    string           `json:"coverage_note,omitempty"`
	StemsTried  []string         `json:"stems_tried,omitempty"`
	Mentions    *fundingMentions `json:"mentions,omitempty"`
}

type fundingFiling struct {
	CIK            string            `json:"cik"`
	FilingDate     string            `json:"filing_date"`
	Accession      string            `json:"accession"`
	EntityName     string            `json:"entity_name"`
	State          string            `json:"state_of_inc,omitempty"`
	IndustryGroup  string            `json:"industry_group,omitempty"`
	OfferingAmount int64             `json:"offering_amount,omitempty"` // -1 means "Indefinite"
	AmountSold     int64             `json:"amount_sold,omitempty"`
	Exemptions     []string          `json:"exemptions_claimed,omitempty"`
	RelatedPersons []sec.FormDPerson `json:"related_persons,omitempty"`
}

// cikSummary aggregates filings by CIK so callers can disambiguate when
// a name search returns hits across unrelated entities. The default
// EDGAR full-text search is name-based, and "Notion" matches both
// Notion Labs and Notion Capital VC. Without a per-CIK summary the
// caller has no signal that the result mixes entities.
type cikSummary struct {
	CIK              string `json:"cik"`
	EntityName       string `json:"entity_name"`
	State            string `json:"state_of_inc,omitempty"`
	YearOfInc        string `json:"year_of_inc,omitempty"`
	FilingCount      int    `json:"filing_count"`
	LatestFilingDate string `json:"latest_filing_date,omitempty"`
}

// fundingMentions groups EDGAR hits by signal class when Form D is empty.
// Subsidiary = parent's 10-K mentions (e.g. EX-21 subsidiary list).
// Debt       = venture-debt holder's 10-Q/10-K portfolio mentions.
// Acquisition = parent's 8-K announcing the deal.
// Other      = anything else that mentions the subject.
type fundingMentions struct {
	Subsidiary  []mentionRow `json:"subsidiary,omitempty"`
	Debt        []mentionRow `json:"debt,omitempty"`
	Acquisition []mentionRow `json:"acquisition,omitempty"`
	Other       []mentionRow `json:"other,omitempty"`
	Total       int          `json:"total"`
}

type mentionRow struct {
	Form         string `json:"form"`
	Filer        string `json:"filer"`
	FileDate     string `json:"file_date"`
	AccessionURL string `json:"accession_url"`
}

func newFundingCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags
	var who string
	var maxFilings int
	var sinceYear int
	var cikFilter string

	cmd := &cobra.Command{
		Use:   "funding [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "SEC EDGAR Form D filings + YC batch lookup. The killer feature for US private fundraising.",
		Long: `funding fetches every Form D filing the SEC has for a company name, parses the structured XML, and reports offering amount, filing date, exemption claimed, and related persons.

Form D is filed by US private companies raising capital under Reg D (506(b) or 506(c)). The data is free and public — Crunchbase Pro charges thousands/year for what's essentially a wrapper around this same source.

With --who <person>, lists every Form D filing where the named person appears as a related party (officer, director, promoter). Useful for mapping serial founders.

Exit codes:
  0  at least one filing found (or candidate list rendered)
  2  ambiguous — rerun with --pick or --domain
  4  no candidates found
  5  no filings found for resolved company`,
		Example: strings.Trim(`
  company-goat-pp-cli funding anthropic
  company-goat-pp-cli funding stripe --json
  company-goat-pp-cli funding --domain anthropic.com --max 3
  company-goat-pp-cli funding --who "Patrick Collison" --json
  company-goat-pp-cli funding ramp --since 2020
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if who == "" && t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			if maxFilings <= 0 {
				maxFilings = 5
			}

			secCli := sec.NewClient(getContactEmail(flags))

			// --who path: show every Form D filing for a named person.
			if who != "" {
				return runFundingWho(cmd, flags, secCli, who, maxFilings, sinceYear)
			}

			// Standard path: resolve company → search Form D.
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}

			// Use a small set of stem variants (e.g. junelife, "june life",
			// "junelife inc") rather than only the bare domain stem, since
			// EDGAR indexes legal names like "June Life Inc." as multi-token
			// phrases. Variants run sequentially with early exit on the
			// first non-empty result to stay polite under EDGAR fair-access.
			variants := stemVariants(domain)

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			var filings []sec.FormD
			var lastErr error
			for _, q := range variants {
				filings, lastErr = secCli.SearchAndFetchAll(ctx, q, maxFilings)
				if lastErr != nil {
					return classifyAPIError(fmt.Errorf("sec edgar: %w", lastErr))
				}
				if len(filings) > 0 {
					break
				}
			}
			if sinceYear > 0 {
				filings = filterByYear(filings, sinceYear)
			}
			if cikFilter != "" {
				filings = filterFilingsByCIK(filings, cikFilter)
			}
			ycCli := yc.NewClient()
			ycCtx, ycCancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer ycCancel()
			ycEntry, _ := ycCli.FindByDomain(ycCtx, domain)

			out := buildFundingResult(domain, filings, ycEntry)
			out.StemsTried = variants

			// Form D + YC both empty: fall back to broader EDGAR search
			// and surface mentions binned by signal class. This is the
			// path that lights up acquired companies (Weber 10-K EX-21),
			// venture-debt portfolio companies (Venture Lending & Leasing
			// 10-Q mentions), and 8-K-only acquisition announcements.
			if len(out.Filings) == 0 && ycEntry == nil {
				if mentions := searchMentions(ctx, secCli, variants); mentions != nil && mentions.Total > 0 {
					out.Mentions = mentions
					out.Coverage = fmt.Sprintf("No Form D filings; surfaced %d EDGAR mentions across other filings (Form D coverage note still applies: US-only, pre-priced-round startups absent).", mentions.Total)
					renderFunding(cmd, flags, out)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "no SEC filings found for %q (tried stems: %s)\n", domain, strings.Join(variants, ", "))
				fmt.Fprintf(cmd.OutOrStdout(), "Try: company-goat-pp-cli funding --domain %s with an exact issuer name, or browse https://efts.sec.gov/LATEST/search-index?q=%%22%s%%22\n", domain, urlEscape(variants[0]))
				fmt.Fprintf(cmd.OutOrStdout(), "Coverage note: Form D is US-only. Non-US companies and pre-priced-round startups won't appear.\n")
				os.Exit(5)
			}
			renderFunding(cmd, flags, out)
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	cmd.Flags().StringVar(&who, "who", "", "Show every SEC filing naming this person: Form D related-person matches (officer, director, promoter) plus EDGAR mentions across S-1, 10-K, DEF 14A, etc. (e.g. \"Patrick Collison\")")
	cmd.Flags().IntVar(&maxFilings, "max", 5, "Maximum filings to fetch and parse")
	cmd.Flags().IntVar(&sinceYear, "since", 0, "Filter to filings on or after this year")
	cmd.Flags().StringVar(&cikFilter, "cik", "", "Filter results to a specific SEC CIK (e.g. 0001999999). Use after running funding without --cik to disambiguate when multiple entities matched the name.")
	return cmd
}

func runFundingWho(cmd *cobra.Command, flags *rootFlags, secCli *sec.Client, who string, maxFilings, sinceYear int) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	// Pass 1: Form D filings naming the person as a related party
	// (officer, director, promoter). High confidence: the XML parser
	// extracts structured related-person fields.
	formDFilings, err := secCli.SearchAndFetchAll(ctx, who, maxFilings*2)
	if err != nil {
		return classifyAPIError(fmt.Errorf("sec edgar: %w", err))
	}
	matchedFormD := formDFilings[:0]
	for _, fd := range formDFilings {
		for _, p := range fd.RelatedPersons {
			if nameMatchesAtWordBoundary(p.Name, who) {
				matchedFormD = append(matchedFormD, fd)
				break
			}
		}
	}
	if sinceYear > 0 {
		matchedFormD = filterByYear(matchedFormD, sinceYear)
	}

	// Pass 2: broader EDGAR mentions of the name across S-1, 10-K,
	// DEF 14A, 8-K, etc. Lower confidence: matches on the EFTS hit's
	// display_names, not on parsed related-person fields. This catches
	// officers and named executives of companies that have not filed
	// Form D (typical for SAFE-funded or acquired startups).
	mentions := searchMentionsForPerson(ctx, secCli, who, sinceYear)

	if len(matchedFormD) == 0 && (mentions == nil || mentions.Total == 0) {
		fmt.Fprintf(cmd.OutOrStdout(), "no SEC filings found naming %q\n", who)
		fmt.Fprintf(cmd.OutOrStdout(), "Try: company-goat-pp-cli funding --who %q with a different spelling, or browse https://efts.sec.gov/LATEST/search-index?q=%%22%s%%22\n", who, urlEscape(who))
		os.Exit(5)
	}

	type whoOut struct {
		Person   string           `json:"person"`
		Filings  []fundingFiling  `json:"form_d_filings"`
		Mentions *fundingMentions `json:"mentions,omitempty"`
		Coverage string           `json:"coverage_note"`
	}
	out := whoOut{
		Person:   who,
		Filings:  fundingFilingsFromSEC(matchedFormD),
		Mentions: mentions,
		Coverage: fmt.Sprintf("Form D filings: %d. EDGAR mentions: %d. Form D is US-only; mentions cover all form types.",
			len(matchedFormD), mentionTotal(mentions)),
	}
	w := cmd.OutOrStdout()
	asJSON := flags.asJSON || !isTerminal(w)
	if asJSON {
		return flags.printJSON(cmd, out)
	}
	if len(matchedFormD) > 0 {
		fmt.Fprintf(w, "Form D filings naming %q:\n\n", who)
		for _, f := range out.Filings {
			fmt.Fprintf(w, "  %s  %-40s  %s\n", f.FilingDate, f.EntityName, formatAmount(f.OfferingAmount))
		}
	}
	if mentions != nil && mentions.Total > 0 {
		fmt.Fprintf(w, "\nEDGAR mentions of %q (%d total):\n", who, mentions.Total)
		renderMentionGroup(w, "Subsidiary signal", mentions.Subsidiary)
		renderMentionGroup(w, "Venture-debt signal", mentions.Debt)
		renderMentionGroup(w, "Acquisition signal", mentions.Acquisition)
		renderMentionGroup(w, "Other mentions", mentions.Other)
	}
	fmt.Fprintf(w, "\n%s\n", out.Coverage)
	return nil
}

// searchMentionsForPerson runs SearchAnyForm and filters hits to those
// where the person's name appears in the display_names at a word
// boundary. Form D hits are excluded since pass 1 covers them with
// parsed-XML high-confidence matching.
func searchMentionsForPerson(ctx context.Context, secCli *sec.Client, person string, sinceYear int) *fundingMentions {
	resp, err := secCli.SearchAnyForm(ctx, person, 25)
	if err != nil || resp == nil || len(resp.Hits) == 0 {
		return nil
	}
	out := &fundingMentions{}
	prefix := ""
	if sinceYear > 0 {
		prefix = fmt.Sprintf("%04d", sinceYear)
	}
	seen := map[string]bool{}
	for _, hit := range resp.Hits {
		if strings.EqualFold(hit.Form, "D") {
			continue
		}
		if prefix != "" && hit.FileDate < prefix {
			continue
		}
		filer := firstDisplayName(hit.DisplayNames)
		if !nameMatchesAtWordBoundary(filer, person) {
			continue
		}
		row := mentionRow{
			Form:         hit.Form,
			Filer:        filer,
			FileDate:     hit.FileDate,
			AccessionURL: accessionURL(hit),
		}
		if row.AccessionURL != "" {
			if seen[row.AccessionURL] {
				continue
			}
			seen[row.AccessionURL] = true
		}
		switch binMention(hit, person) {
		case "subsidiary":
			out.Subsidiary = append(out.Subsidiary, row)
		case "debt":
			out.Debt = append(out.Debt, row)
		case "acquisition":
			out.Acquisition = append(out.Acquisition, row)
		default:
			out.Other = append(out.Other, row)
		}
		out.Total++
	}
	if out.Total == 0 {
		return nil
	}
	return out
}

func mentionTotal(m *fundingMentions) int {
	if m == nil {
		return 0
	}
	return m.Total
}

// nameMatchesAtWordBoundary returns true when needle appears in haystack
// as a complete word phrase (case-insensitive). "Patrick Collins" matches
// "Patrick Collins" but not "Patrick Collinsworth".
func nameMatchesAtWordBoundary(haystack, needle string) bool {
	parts := strings.Fields(needle)
	if len(parts) == 0 {
		return false
	}
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = regexp.QuoteMeta(p)
	}
	pattern := `(?i)\b` + strings.Join(quoted, `\s+`) + `\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(haystack)
}

func filterByYear(in []sec.FormD, year int) []sec.FormD {
	var out []sec.FormD
	prefix := fmt.Sprintf("%04d", year)
	for _, f := range in {
		if f.FilingDate >= prefix {
			out = append(out, f)
		}
	}
	return out
}

func buildFundingResult(domain string, filings []sec.FormD, ycEntry *yc.Company) fundingResult {
	summaries := summarizeByCIK(filings)
	r := fundingResult{
		Domain:       domain,
		Filings:      fundingFilingsFromSEC(filings),
		CIKSummaries: summaries,
		IsAmbiguous:  len(summaries) > 1,
		YCEntry:      ycEntry,
		Coverage:     "Form D is US-only. Non-US companies and pre-priced-round startups won't appear.",
	}
	return r
}

func fundingFilingsFromSEC(in []sec.FormD) []fundingFiling {
	out := make([]fundingFiling, 0, len(in))
	for _, fd := range in {
		out = append(out, fundingFiling{
			CIK:            fd.CIK,
			FilingDate:     fd.FilingDate,
			Accession:      fd.Accession,
			EntityName:     fd.EntityName,
			State:          fd.State,
			IndustryGroup:  fd.IndustryGroup,
			OfferingAmount: fd.OfferingAmount,
			AmountSold:     fd.AmountSold,
			Exemptions:     fd.ExemptionsClaimed,
			RelatedPersons: fd.RelatedPersons,
		})
	}
	// Sort by filing date descending.
	sort.SliceStable(out, func(i, j int) bool { return out[i].FilingDate > out[j].FilingDate })
	return out
}

// summarizeByCIK groups filings by issuer CIK so the caller can spot
// when a name search dragged in unrelated entities. Sorted by latest
// filing date descending so the most-recently-active entity appears
// first (commonly the one the agent meant).
func summarizeByCIK(filings []sec.FormD) []cikSummary {
	type acc struct {
		EntityName string
		State      string
		YearOfInc  string
		Count      int
		LatestDate string
	}
	by := map[string]*acc{}
	for _, fd := range filings {
		a, ok := by[fd.CIK]
		if !ok {
			a = &acc{EntityName: fd.EntityName, State: fd.State, YearOfInc: fd.YearOfInc}
			by[fd.CIK] = a
		}
		a.Count++
		if fd.FilingDate > a.LatestDate {
			a.LatestDate = fd.FilingDate
		}
	}
	out := make([]cikSummary, 0, len(by))
	for cik, a := range by {
		out = append(out, cikSummary{
			CIK:              cik,
			EntityName:       a.EntityName,
			State:            a.State,
			YearOfInc:        a.YearOfInc,
			FilingCount:      a.Count,
			LatestFilingDate: a.LatestDate,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LatestFilingDate > out[j].LatestFilingDate })
	return out
}

// filterFilingsByCIK returns only the filings whose CIK matches the
// requested value. Leading zeros are tolerated on both sides — SEC EDGAR
// pads CIKs to 10 digits, but humans copy-pasting may drop them.
func filterFilingsByCIK(filings []sec.FormD, cik string) []sec.FormD {
	want := strings.TrimLeft(cik, "0")
	out := make([]sec.FormD, 0, len(filings))
	for _, fd := range filings {
		got := strings.TrimLeft(fd.CIK, "0")
		if got == want {
			out = append(out, fd)
		}
	}
	return out
}

func renderFunding(cmd *cobra.Command, flags *rootFlags, r fundingResult) {
	w := cmd.OutOrStdout()
	asJSON := flags.asJSON || !isTerminal(w)
	if asJSON {
		_ = flags.printJSON(cmd, r)
		return
	}
	fmt.Fprintf(w, "Domain: %s\n", r.Domain)
	if r.YCEntry != nil {
		fmt.Fprintf(w, "YC: %s (batch %s, status %s)\n", r.YCEntry.Name, r.YCEntry.Batch, r.YCEntry.Status)
	}
	if r.IsAmbiguous {
		fmt.Fprintf(w, "\n⚠ Ambiguous: %d distinct SEC entities matched. The Form D rows below may mix unrelated companies. Use --cik <id> to disambiguate.\n", len(r.CIKSummaries))
		fmt.Fprintf(w, "  Matched entities (most-recently-active first):\n")
		for _, s := range r.CIKSummaries {
			yr := ""
			if s.YearOfInc != "" {
				yr = " inc:" + s.YearOfInc
			}
			fmt.Fprintf(w, "    CIK %s  %-40s  state:%s%s  filings:%d  latest:%s\n",
				s.CIK, fundingTruncate(s.EntityName, 40), s.State, yr, s.FilingCount, s.LatestFilingDate)
		}
	}
	if len(r.Filings) > 0 {
		fmt.Fprintf(w, "\nForm D filings (%d):\n", len(r.Filings))
		for _, f := range r.Filings {
			fmt.Fprintf(w, "  %s  CIK %s  %-40s  %s  exempt:%v  state:%s  industry:%s\n",
				f.FilingDate, f.CIK, fundingTruncate(f.EntityName, 40), formatAmount(f.OfferingAmount),
				f.Exemptions, f.State, f.IndustryGroup)
		}
	} else {
		fmt.Fprintf(w, "Form D: no filings found\n")
	}
	if r.Mentions != nil && r.Mentions.Total > 0 {
		fmt.Fprintf(w, "\nEDGAR mentions (%d total, broader fallback):\n", r.Mentions.Total)
		renderMentionGroup(w, "Subsidiary signal", r.Mentions.Subsidiary)
		renderMentionGroup(w, "Venture-debt signal", r.Mentions.Debt)
		renderMentionGroup(w, "Acquisition signal", r.Mentions.Acquisition)
		renderMentionGroup(w, "Other mentions", r.Mentions.Other)
	}
	if r.Coverage != "" {
		fmt.Fprintf(w, "\n%s\n", r.Coverage)
	}
}

func renderMentionGroup(w interface{ Write([]byte) (int, error) }, label string, rows []mentionRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(w, "  %s (%d):\n", label, len(rows))
	for _, row := range rows {
		fmt.Fprintf(w, "    %s  %-8s  %-40s  %s\n",
			row.FileDate, row.Form, fundingTruncate(row.Filer, 40), row.AccessionURL)
	}
}

func formatAmount(amt int64) string {
	if amt == -1 {
		return "$Indefinite"
	}
	if amt == 0 {
		return "$0"
	}
	switch {
	case amt >= 1_000_000_000:
		return fmt.Sprintf("$%.1fB", float64(amt)/1_000_000_000)
	case amt >= 1_000_000:
		return fmt.Sprintf("$%.1fM", float64(amt)/1_000_000)
	case amt >= 1_000:
		return fmt.Sprintf("$%.0fK", float64(amt)/1_000)
	default:
		return fmt.Sprintf("$%d", amt)
	}
}

// stemVariants returns the small set of EFTS query strings to try in
// order. The bare stem matches camel-case-tokenized companies (anthropic,
// stripe). The space-split variant matches issuer-name phrases EDGAR
// indexes as separate tokens (June Life). The "<stem> inc" variant
// catches unsuffixed domains whose legal name carries Inc.
//
// Hyphenated stems (acme-corp) get the hyphen replaced with a space.
// Concatenated stems (junelife) are split at the first vowel-consonant
// boundary where both halves are at least 3 characters; this is a small
// heuristic, not a full tokenizer, and skipping it for ambiguous cases is
// fine because variants only run sequentially when prior ones return
// empty.
func stemVariants(domain string) []string {
	stem := strings.SplitN(domain, ".", 2)[0]
	if stem == "" {
		return nil
	}
	stem = strings.ToLower(stem)
	out := []string{stem}
	seen := map[string]bool{stem: true}
	add := func(v string) {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}

	if strings.Contains(stem, "-") {
		add(strings.ReplaceAll(stem, "-", " "))
	} else {
		if split := splitAtVowelConsonantBoundary(stem); split != "" {
			add(split)
		}
	}
	add(stem + " inc")
	return out
}

func splitAtVowelConsonantBoundary(stem string) string {
	if len(stem) < 6 {
		return ""
	}
	const vowels = "aeiouy"
	for i := 3; i <= len(stem)-3; i++ {
		prev := stem[i-1]
		curr := stem[i]
		if strings.ContainsRune(vowels, rune(prev)) && !strings.ContainsRune(vowels, rune(curr)) {
			return stem[:i] + " " + stem[i:]
		}
	}
	return ""
}

// searchMentions runs the broad-EDGAR fallback and bins hits by signal
// class. Picks the most distinctive variant as the query (the bigram if
// present, else the last variant which is "<stem> inc"). Returns nil
// when the search yields no results so callers can fall through to the
// regular empty-state path.
func searchMentions(ctx context.Context, secCli *sec.Client, variants []string) *fundingMentions {
	if len(variants) == 0 {
		return nil
	}
	query := pickMentionQuery(variants)
	resp, err := secCli.SearchAnyForm(ctx, query, 25)
	if err != nil || resp == nil || len(resp.Hits) == 0 {
		return nil
	}
	out := &fundingMentions{}
	seen := map[string]bool{}
	for _, hit := range resp.Hits {
		row := mentionRow{
			Form:         hit.Form,
			Filer:        firstDisplayName(hit.DisplayNames),
			FileDate:     hit.FileDate,
			AccessionURL: accessionURL(hit),
		}
		// EFTS returns one hit per matched document/exhibit, so the same
		// underlying filing can show up several times across exhibits.
		// Dedupe by accession URL so the user sees each filing once.
		if row.AccessionURL != "" {
			if seen[row.AccessionURL] {
				continue
			}
			seen[row.AccessionURL] = true
		}
		switch binMention(hit, query) {
		case "subsidiary":
			out.Subsidiary = append(out.Subsidiary, row)
		case "debt":
			out.Debt = append(out.Debt, row)
		case "acquisition":
			out.Acquisition = append(out.Acquisition, row)
		default:
			out.Other = append(out.Other, row)
		}
		out.Total++
	}
	return out
}

func pickMentionQuery(variants []string) string {
	for _, v := range variants {
		if strings.Contains(v, " ") && !strings.HasSuffix(v, " inc") {
			return v
		}
	}
	return variants[len(variants)-1]
}

func firstDisplayName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	// EFTS display names look like "Weber Inc.  (CIK 0001890586)" —
	// strip the trailing CIK annotation and trailing whitespace.
	name := names[0]
	if idx := strings.Index(name, "(CIK"); idx > 0 {
		name = name[:idx]
	}
	return strings.TrimSpace(name)
}

func accessionURL(hit sec.SearchHit) string {
	if len(hit.CIKs) == 0 || hit.Accession == "" {
		return ""
	}
	cik := strings.TrimLeft(hit.CIKs[0], "0")
	if cik == "" {
		cik = "0"
	}
	dashless := strings.ReplaceAll(hit.Accession, "-", "")
	return fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/", cik, dashless)
}

// binMention classifies an EDGAR hit by signal class. The subject query
// is the search term that produced the hit; when the filer's display name
// contains the subject, the hit is the company filing under itself and
// classifies as "other".
func binMention(hit sec.SearchHit, subjectQuery string) string {
	filer := strings.ToLower(firstDisplayName(hit.DisplayNames))
	subject := strings.ToLower(subjectQuery)

	// Venture Lending & Leasing portfolio reports.
	if strings.HasPrefix(filer, "venture lending & leasing") ||
		strings.HasPrefix(filer, "venture lending and leasing") {
		return "debt"
	}

	// Form-code prefix: "10-K", "10-K/A", "8-K", "8-K/A", etc.
	formCode := strings.ToUpper(strings.SplitN(hit.Form, "/", 2)[0])
	formCode = strings.SplitN(formCode, " ", 2)[0]

	selfFiling := filer != "" && strings.Contains(filer, subject)
	if !selfFiling {
		if strings.HasPrefix(formCode, "10-K") {
			return "subsidiary"
		}
		if strings.HasPrefix(formCode, "8-K") {
			return "acquisition"
		}
	}
	return "other"
}

func urlEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "&", "%26")
}

// fundingTruncate is a local helper. The generated helpers.go already has
// a truncate(...) but its semantics differ; we use this variant only inside
// company_funding.go (and other novel commands) for consistency.
func fundingTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func newFundingTrendCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags
	var sinceYear int
	var maxFilings int

	cmd := &cobra.Command{
		Use:   "funding-trend [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Time series of Form D filings showing fundraising cadence over years.",
		Long: `funding-trend renders a year-by-year count of Form D filings for a company. Useful for spotting fundraising gaps or a startup that quietly stopped raising.

Output bins by filing year and shows offering amount totals per year.`,
		Example: strings.Trim(`
  company-goat-pp-cli funding-trend stripe
  company-goat-pp-cli funding-trend anthropic --since 2020 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			if maxFilings <= 0 {
				maxFilings = 25
			}

			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			variants := stemVariants(domain)

			secCli := sec.NewClient(getContactEmail(flags))
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()

			var filings []sec.FormD
			for _, q := range variants {
				filings, err = secCli.SearchAndFetchAll(ctx, q, maxFilings)
				if err != nil {
					return classifyAPIError(fmt.Errorf("sec edgar: %w", err))
				}
				if len(filings) > 0 {
					break
				}
			}
			if sinceYear > 0 {
				filings = filterByYear(filings, sinceYear)
			}

			type yearBucket struct {
				Year         int   `json:"year"`
				FilingCount  int   `json:"filing_count"`
				TotalOffered int64 `json:"total_offered_usd"`
			}
			buckets := map[int]*yearBucket{}
			for _, f := range filings {
				if len(f.FilingDate) < 4 {
					continue
				}
				yr := 0
				_, err := fmt.Sscanf(f.FilingDate[:4], "%d", &yr)
				if err != nil {
					continue
				}
				b, ok := buckets[yr]
				if !ok {
					b = &yearBucket{Year: yr}
					buckets[yr] = b
				}
				b.FilingCount++
				if f.OfferingAmount > 0 {
					b.TotalOffered += f.OfferingAmount
				}
			}
			years := make([]int, 0, len(buckets))
			for y := range buckets {
				years = append(years, y)
			}
			sort.Ints(years)
			out := make([]yearBucket, 0, len(years))
			for _, y := range years {
				out = append(out, *buckets[y])
			}

			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			if asJSON {
				return flags.printJSON(cmd, map[string]any{
					"domain":        domain,
					"buckets":       out,
					"total_filings": len(filings),
				})
			}
			if len(out) == 0 {
				fmt.Fprintf(w, "no Form D filings found for %q\n", domain)
				return nil
			}
			fmt.Fprintf(w, "Form D fundraising trend for %s:\n\n", domain)
			fmt.Fprintf(w, "  YEAR  FILINGS  TOTAL OFFERED\n")
			for _, b := range out {
				fmt.Fprintf(w, "  %d  %5d    %s\n", b.Year, b.FilingCount, formatAmount(b.TotalOffered))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	cmd.Flags().IntVar(&sinceYear, "since", 0, "Only include filings on or after this year")
	cmd.Flags().IntVar(&maxFilings, "max", 25, "Maximum filings to fetch")
	return cmd
}

// getContactEmail reads the SEC fair-access contact email from
// COMPANY_PP_CONTACT_EMAIL. Empty falls back to the generic User-Agent.
func getContactEmail(flags *rootFlags) string {
	return strings.TrimSpace(os.Getenv("COMPANY_PP_CONTACT_EMAIL"))
}
