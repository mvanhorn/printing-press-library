// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/unicode/norm"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxOperatorBody = 512 * 1024
const MaxOperatorEvidence = 8

var operatorHosts = map[string]bool{"railway.jr-central.co.jp": true, "www.jrhokkaido.co.jp": true, "www.jreast.co.jp": true, "tickets.jr-odekake.net": true, "www.jrkyushu-kippu.jp": true, "www.jr-eki.com": true}

type RuleCheck struct {
	Field     string  `json:"field"`
	State     string  `json:"state"`
	Publisher *string `json:"publisher_value"`
	Operator  *string `json:"operator_value"`
}
type OperatorEvidence struct {
	Freshness             string         `json:"freshness"`
	ScopeStatus           string         `json:"scope_status"`
	ScopeSelector         *string        `json:"scope_selector"`
	Transport             string         `json:"transport"`
	ObservationAgeSeconds int64          `json:"observation_age_seconds"`
	Stale                 bool           `json:"stale"`
	Validity              Evidence       `json:"validity"`
	ValidityDays          *int           `json:"validity_days"`
	URL                   string         `json:"source_url"`
	EffectiveURL          *string        `json:"effective_url"`
	ObservedAt            *string        `json:"observed_at"`
	HTTPStatus            *int           `json:"http_status"`
	Format                string         `json:"format"`
	Status                string         `json:"status"`
	Reason                string         `json:"reason"`
	NameEvidence          Evidence       `json:"name_evidence"`
	NameMatch             string         `json:"name_match"`
	EditionState          string         `json:"edition_state"`
	AsOf                  string         `json:"as_of"`
	Sales                 Period         `json:"sales"`
	Use                   Period         `json:"use"`
	Price                 Price          `json:"price"`
	PurchaseChannels      []Evidence     `json:"purchase_channels"`
	Eligibility           []Evidence     `json:"eligibility"`
	Supplements           []Evidence     `json:"supplements"`
	Exceptions            []Evidence     `json:"exceptions"`
	BlackoutSpans         []MonthDaySpan `json:"blackout_spans"`
	RuleChecks            []RuleCheck    `json:"rule_checks"`
	PDFReferences         []string       `json:"pdf_references"`
	Unresolved            []string       `json:"unresolved"`
	EvidenceTruncated     bool           `json:"evidence_truncated"`
	AllRulesVerified      bool           `json:"all_rules_verified"`
}

func emptyOperator(raw, asOf string) OperatorEvidence {
	return OperatorEvidence{Freshness: "unknown_clock", ScopeStatus: "not_checked", Transport: "live", URL: raw, AsOf: asOf, Format: "unknown", Status: "not_checked", NameMatch: "unknown", EditionState: "unknown", Sales: period(""), Use: period(""), Price: Price{Status: "unknown", Amounts: []Money{}, Reason: "No labelled operator fare evidence extracted."}, PurchaseChannels: []Evidence{}, Eligibility: []Evidence{}, Supplements: []Evidence{}, Exceptions: []Evidence{}, BlackoutSpans: []MonthDaySpan{}, RuleChecks: []RuleCheck{}, PDFReferences: []string{}, Unresolved: []string{"calendar_exceptions", "traveler_eligibility", "complete_train_coverage", "referenced_documents", "operator_inventory"}}
}
func safeOperatorURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && operatorHosts[u.Hostname()] && len(raw) < 2048
}

// ReadOperator follows exactly one publisher-linked official document. A successful
// fetch is never a blanket confirmation; parsed rules and unresolved fields remain separate.
func (c *Client) ReadOperator(ctx context.Context, t Ticket, asOf string) (OperatorEvidence, error) {
	if asOf == "" {
		asOf = time.Now().In(time.FixedZone("Asia/Tokyo", 9*3600)).Format("2006-01-02")
	}
	if len(t.OperatorURLs) == 0 {
		o := emptyOperator("", asOf)
		o.Status = "no_operator_link"
		o.Reason = "Publisher supplied no usable HTTPS operator detail link."
		return o, nil
	}
	raw := t.OperatorURLs[0]
	o := emptyOperator(raw, asOf)
	if !safeOperatorURL(raw) {
		o.Status = "unsupported_operator"
		o.Reason = "Linked host is outside the supported official JR document hosts; no request made."
		return o, nil
	}
	if e := c.limiter.Wait(ctx); e != nil {
		return o, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return o, e
	}
	req.Header.Set("User-Agent", "toretabi-pp-cli/0.0.0-dev")
	req.Header.Set("Accept", "text/html,application/pdf")
	hc := *c.HTTP
	if hc.Timeout == 0 {
		hc.Timeout = 20 * time.Second
	}
	hc.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !safeOperatorURL(r.URL.String()) {
			return fmt.Errorf("operator redirect leaves supported public HTTPS hosts")
		}
		return nil
	}
	resp, e := hc.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return o, ctx.Err()
		}
		o.Status = "access_error"
		o.Reason = e.Error()
		return o, nil
	}
	defer resp.Body.Close()
	at := time.Now().UTC().Format(time.RFC3339Nano)
	o.ObservedAt = &at
	o.Freshness = "live_observation"
	o.EffectiveURL = strptr(resp.Request.URL.String())
	code := resp.StatusCode
	o.HTTPStatus = &code
	if code == 429 {
		c.limiter.OnRateLimit()
		return o, &cliutil.RateLimitError{URL: raw, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if code != 200 {
		o.Status = "http_error"
		o.Reason = fmt.Sprintf("Linked operator returned HTTP%d; rules remain unverified.", code)
		return o, nil
	}
	c.limiter.OnSuccess()
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "application/pdf") {
		o.Format = "pdf"
		o.Status = "unsupported_pdf"
		o.Reason = "PDF linked document was identified, but PDF rule extraction is unsupported; no rules confirmed."
		return o, nil
	}
	if !strings.Contains(ct, "text/html") {
		o.Status = "unsupported_format"
		o.Reason = "Operator document is not supported HTML; rules remain unverified."
		return o, nil
	}
	o.Format = "html"
	b, e := io.ReadAll(io.LimitReader(resp.Body, MaxOperatorBody+1))
	if e != nil {
		return o, e
	}
	if len(b) > MaxOperatorBody {
		o.Status = "response_limit"
		o.Reason = "Operator HTML exceeded the 512 KiB bound; no partial rules accepted."
		return o, nil
	}
	reader, e := charset.NewReader(strings.NewReader(string(b)), ct)
	if e != nil {
		o.Status = "encoding_error"
		o.Reason = e.Error()
		return o, nil
	}
	decoded, e := io.ReadAll(io.LimitReader(reader, MaxOperatorBody*2+1))
	if e != nil {
		return o, e
	}
	if len(decoded) > MaxOperatorBody*2 {
		o.Status = "response_limit"
		o.Reason = "Decoded operator HTML exceeded its bound."
		return o, nil
	}
	operatorTicket := t
	operatorTicket.OperatorURLs = []string{resp.Request.URL.String()}
	parsed, e := parseOperator(decoded, operatorTicket, asOf)
	if e != nil {
		o.Status = "parse_unknown"
		o.Reason = e.Error()
		return o, nil
	}
	parsed.URL = raw
	parsed.EffectiveURL = o.EffectiveURL
	parsed.ObservedAt = o.ObservedAt
	parsed.Freshness = "live_observation"
	parsed.HTTPStatus = o.HTTPStatus
	parsed.Format = "html"
	return parsed, nil
}
func operatorAdd(dst *[]Evidence, raw string, tr *bool) {
	if len(*dst) >= MaxOperatorEvidence {
		*tr = true
		return
	}
	addEvidence(dst, raw, tr)
}
func normName(s string) string { s = norm.NFKC.String(s); return strings.Join(strings.Fields(s), "") }
func nextElement(n *html.Node) *html.Node {
	for x := n.NextSibling; x != nil; x = x.NextSibling {
		if x.Type == html.ElementNode {
			return x
		}
	}
	return nil
}
func operatorLabel(s string) string {
	s = norm.NFKC.String(s)
	switch {
	case strings.Contains(s, "有効期間"):
		return "validity"
	case strings.Contains(s, "発売期間"):
		return "sales"
	case strings.Contains(s, "利用期間") && !strings.Contains(s, "制限"):
		return "use"
	case strings.Contains(s, "発売条件") || strings.Contains(s, "前売り期間"):
		return "purchase_rule"
	case strings.Contains(s, "利用制限期間"):
		return "blackout"
	default:
		return ""
	}
}
func parseOperator(body []byte, t Ticket, asOf string) (OperatorEvidence, error) {
	o := emptyOperator("", asOf)
	doc, e := html.Parse(strings.NewReader(string(body)))
	if e != nil {
		return o, e
	}
	identity := text(first(doc, func(n *html.Node) bool { return n.Data == "title" }))
	if t.NameJA != "" && !strings.Contains(normName(identity), normName(t.NameJA)) {
		h := first(doc, func(n *html.Node) bool {
			return (n.Data == "h1" || n.Data == "h2") && normName(text(n)) == normName(t.NameJA)
		})
		if h != nil {
			identity = text(h)
		}
	}
	o.NameEvidence = clip(identity)
	if t.NameJA == "" || !strings.Contains(normName(identity), normName(t.NameJA)) {
		o.Status = "identity_unverified"
		o.Reason = "Operator document title/heading does not match the publisher ticket identity; no rule extraction accepted."
		return o, nil
	}
	o.NameMatch = "matched"
	scope, selector, identity, reason := operatorContentScope(doc, t)
	if scope == nil {
		o.ScopeStatus = "unverified"
		o.Status = "ambiguous_scope"
		o.Reason = reason
		return o, nil
	}
	o.ScopeStatus = "verified_product_container"
	o.ScopeSelector = strptr(selector)
	o.NameEvidence = clip(identity)
	fields := map[string][]string{}
	add := func(label, value string) {
		k := operatorLabel(label)
		if k != "" && value != "" {
			fields[k] = append(fields[k], value)
		}
	}
	walk(scope, func(n *html.Node) {
		if n.Data == "tr" {
			cells := []*html.Node{}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "th" || c.Data == "td" {
					cells = append(cells, c)
				}
			}
			if len(cells) == 2 && operatorLabel(text(cells[0])) != "" {
				add(text(cells[0]), text(cells[1]))
			}
		}
		if n.Data == "dt" {
			if d := nextElement(n); d != nil && d.Data == "dd" {
				add(text(n), text(d))
			}
		}
		if n.Data == "section" {
			h := first(n, func(x *html.Node) bool { return x.Data == "h1" || x.Data == "h2" })
			if h != nil && (h.Parent == n || (h.Parent != nil && h.Parent.Data == "header" && h.Parent.Parent == n)) && operatorLabel(text(h)) != "" {
				value := strings.TrimPrefix(text(n), text(h))
				add(text(h), value)
			}
		}
	})
	o.Validity = clip(strings.Join(unique(fields["validity"]), " "))
	if m := regexp.MustCompile(`([0-9]+)日間`).FindStringSubmatch(norm.NFKC.String(o.Validity.TextJA)); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		if n > 0 {
			o.ValidityDays = &n
		}
	}
	o.Sales = period(strings.Join(unique(fields["sales"]), " "))
	o.Use = period(strings.Join(unique(fields["use"]), " "))
	rules := strings.Join(unique(fields["purchase_rule"]), " ")
	if rules != "" {
		o.Sales.Conditional = true
		o.Use.Conditional = true
		operatorAdd(&o.PurchaseChannels, rules, &o.EvidenceTruncated)
	}
	blackout := strings.Join(unique(fields["blackout"]), " ")
	if blackout != "" {
		o.Use.Conditional = true
		operatorAdd(&o.Exceptions, blackout, &o.EvidenceTruncated)
	}
	// Fare tables must explicitly identify a passenger category. Voucher/benefit
	// amounts are deliberately excluded, and train matrices are not fare tables.
	walk(scope, func(n *html.Node) {
		if n.Data != "table" {
			return
		}
		all := norm.NFKC.String(text(n))
		if !strings.Contains(all, "発売額") || strings.Contains(all, "×") || strings.Contains(all, "倍") || strings.Contains(all, "例)") || strings.Contains(all, "ご利用区間") || strings.Contains(t.NameJA, "回数券") || strings.Contains(t.NameJA, "枚きっぷ") || (!strings.Contains(all, "円") && !strings.Contains(all, "発売額")) || (!strings.Contains(all, "おとな") && !strings.Contains(all, "大人") && !strings.Contains(all, "こども") && !strings.Contains(all, "U25")) || strings.Contains(all, "ご利用券") || strings.Contains(all, "円分") {
			return
		}
		var headers []string
		walk(n, func(row *html.Node) {
			if row.Data != "tr" {
				return
			}
			cells := []string{}
			for cell := row.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Data == "td" || cell.Data == "th" {
					cells = append(cells, text(cell))
				}
			}
			hasMoney := false
			for _, v := range cells {
				if amountRE.MatchString(norm.NFKC.String(v)) || (strings.Contains(all, "発売額") && regexp.MustCompile(`^[0-9][0-9,]{3,}$`).MatchString(norm.NFKC.String(v))) {
					hasMoney = true
				}
			}
			if !hasMoney {
				if len(cells) > 1 {
					headers = cells
				}
				return
			}
			for i, v := range cells {
				numeric := norm.NFKC.String(v)
				if i < len(headers) && strings.Contains(normName(headers[i]), "有効期間") {
					if m := regexp.MustCompile(`^([0-9]+)日(?:間)?$`).FindStringSubmatch(numeric); len(m) > 1 {
						d, _ := strconv.Atoi(m[1])
						if d > 0 {
							o.ValidityDays = &d
							o.Validity = clip(headers[i] + " / " + v)
						}
					}
				}
				currencyBasis := "explicit_yen_cell"
				if regexp.MustCompile(`^[0-9][0-9,]{3,}$`).MatchString(numeric) && strings.Contains(all, "発売額") {
					numeric += "円"
					currencyBasis = "JR_Japan_fare_table_context_cell_unit_omitted"
				}
				for _, m := range money(numeric, "per_ticket") {
					if len(o.Price.Amounts) >= 4 {
						o.EvidenceTruncated = true
						return
					}
					label := cells[0]
					if i < len(headers) {
						label += " / " + headers[i]
					}
					category := label
					if strings.Contains(all, "大人") && !strings.Contains(norm.NFKC.String(label), "U25") {
						category += " / 大人"
					}
					m.CategoryJA = &category
					m.CurrencyBasis = &currencyBasis
					m.Evidence = clip(label + " / " + v)
					o.Price.Amounts = append(o.Price.Amounts, m)
				}
			}
		})
	})
	if len(o.Price.Amounts) > 0 {
		o.Price.Status = "published"
		o.Price.Reason = "Amounts extracted from a labelled operator ticket-fare table; category/basis evidence preserved."
	}
	walk(scope, func(n *html.Node) {
		if n.Data == "a" {
			raw := attr(n, "href")
			if strings.Contains(strings.ToLower(raw), ".pdf") {
				if u, e := url.Parse(raw); e == nil {
					if len(t.OperatorURLs) == 0 {
						return
					}
					base, _ := url.Parse(t.OperatorURLs[0])
					u = base.ResolveReference(u)
					if safeOperatorURL(u.String()) && len(o.PDFReferences) < 6 {
						o.PDFReferences = append(o.PDFReferences, u.String())
					}
				}
			}
		}
		if n.Data != "p" && n.Data != "li" && n.Data != "h2" {
			return
		}
		if first(n, func(x *html.Node) bool { return x.Data == "img" }) != nil {
			return
		}
		for _, v := range lines(n) {
			k := norm.NFKC.String(v)
			if strings.Contains(k, "e5489") || strings.Contains(k, "受取") || strings.Contains(k, "受け取り") || strings.Contains(k, "券売機") || strings.Contains(k, "発売箇所") {
				operatorAdd(&o.PurchaseChannels, v, &o.EvidenceTruncated)
			}
			if strings.Contains(k, "25歳") || strings.Contains(k, "購入時点") || strings.Contains(k, "公的証明") {
				operatorAdd(&o.Eligibility, v, &o.EvidenceTruncated)
			}
			if strings.Contains(k, "別途") || strings.Contains(k, "別に") || strings.Contains(k, "特急券") {
				operatorAdd(&o.Supplements, v, &o.EvidenceTruncated)
			}
			if strings.Contains(k, "購入日時点") || strings.Contains(k, "購入時点") || strings.Contains(k, "25歳") {
				operatorAdd(&o.Eligibility, v, &o.EvidenceTruncated)
			}
			if strings.Contains(k, "観光列車") || strings.Contains(k, "受取できません") || strings.Contains(k, "受け取りできません") || strings.Contains(k, "トーマス") || strings.Contains(k, "利用いただけません") || strings.Contains(k, "利用できません") {
				operatorAdd(&o.Exceptions, v, &o.EvidenceTruncated)
			}
		}
	})
	for i := range o.Eligibility {
		scope := "purchase_age"
		o.Eligibility[i].Scope = &scope
	}
	o.PDFReferences = unique(o.PDFReferences)
	for _, v := range spanRE.FindAllStringSubmatch(norm.NFKC.String(blackout), -1) {
		sm, _ := strconv.Atoi(v[1])
		sd, _ := strconv.Atoi(v[2])
		em := sm
		if v[3] != "" {
			em, _ = strconv.Atoi(v[3])
		}
		ed, _ := strconv.Atoi(v[4])
		o.BlackoutSpans = append(o.BlackoutSpans, MonthDaySpan{StartMonth: sm, StartDay: sd, EndMonth: em, EndDay: ed, Year: nil, Evidence: clip(blackout)})
	}
	o.EditionState = operatorEdition(o, asOf)
	if t.ValidityDays != nil || o.ValidityDays != nil {
		var a, b *string
		if t.ValidityDays != nil {
			a = strptr(strconv.Itoa(*t.ValidityDays))
		}
		if o.ValidityDays != nil {
			b = strptr(strconv.Itoa(*o.ValidityDays))
		}
		o.RuleChecks = append(o.RuleChecks, compareRule("validity_days", a, b))
	}
	if o.Price.Status == "published" {
		state := "operator_only"
		if t.Price.Status == "published" {
			state = "category_comparison_required"
		}
		values := []string{}
		for _, m := range o.Price.Amounts {
			category := "unknown"
			if m.CategoryJA != nil {
				category = *m.CategoryJA
			}
			values = append(values, category+":"+m.Amount+" "+m.Currency+"/"+m.Unit)
		}
		o.RuleChecks = append(o.RuleChecks, RuleCheck{Field: "ticket_fares", State: state, Operator: strptr(strings.Join(values, "; "))})
	}
	for _, p := range []struct {
		name string
		a, b *string
	}{{"sales_start", t.Sales.Start, o.Sales.Start}, {"sales_end", t.Sales.End, o.Sales.End}, {"use_start", t.Use.Start, o.Use.Start}, {"use_end", t.Use.End, o.Use.End}} {
		o.RuleChecks = append(o.RuleChecks, compareRule(p.name, p.a, p.b))
	}
	o.Status = "partial_rule_evidence"
	o.Reason = "Ticket identity and labelled rule/price evidence were inspected; exact traveler eligibility, all train/calendar exceptions and referenced documents are not certified."
	if o.Sales.Evidence.TextJA == "" && o.Use.Evidence.TextJA == "" && o.Price.Status != "published" {
		o.Status = "ambiguous_rules"
		o.Reason = "Matched title, but no sufficient labelled operator rule/fare evidence; no current rules confirmed."
	}
	return o, nil
}
func operatorEdition(o OperatorEvidence, asOf string) string {
	if o.Use.End != nil {
		if asOf > *o.Use.End {
			return "archived_use_window"
		}
		return "published_dated_window"
	}
	if o.Use.Start != nil {
		return "published_start_year"
	}
	if len(o.Use.Intervals) > 0 {
		for _, v := range o.Use.Intervals {
			if asOf <= v.End {
				return "published_dated_window"
			}
		}
		return "archived_use_window"
	}
	return "unknown"
}

func compareRule(field string, a, b *string) RuleCheck {
	state := "unknown"
	if a != nil && b != nil {
		state = "corroborated"
		if *a != *b {
			state = "conflict"
		}
	} else if b != nil {
		state = "operator_only"
	}
	return RuleCheck{Field: field, State: state, Publisher: a, Operator: b}
}
