package smartex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const MaxHTMLBytes = 512 << 10

type Client struct {
	HTTP     *http.Client
	limiter  *cliutil.AdaptiveLimiter
	requests int
	endpoint string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 4 {
			return fmt.Errorf("upstream redirects exceed four")
		}
		if r.URL.Scheme != "https" {
			return fmt.Errorf("upstream redirect must stay HTTPS")
		}
		return nil
	}}, limiter: cliutil.NewAdaptiveLimiter(2), endpoint: FareURL}
}
func (c *Client) Requests() int { return c.requests }
func (c *Client) SetRateLimit(rate float64) {
	if rate > 0 && rate < 2 {
		c.limiter = cliutil.NewAdaptiveLimiter(rate)
	}
}
func (c *Client) fetch(ctx context.Context, method, address, body string) ([]byte, error) {
	if e := c.limiter.Wait(ctx); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, method, address, strings.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "smartex-pp-cli/0.1 (public read-only planning)")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	c.requests++
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, fmt.Errorf("public source request failed: %w; use official handoff", e)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: address, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("public source HTTP%d; use official handoff and retry later", resp.StatusCode)
	}
	c.limiter.OnSuccess()
	data, e := io.ReadAll(io.LimitReader(resp.Body, MaxHTMLBytes+1))
	if e != nil {
		return nil, e
	}
	if len(data) > MaxHTMLBytes {
		return nil, fmt.Errorf("public HTML exceeds %d bytes; refusing truncated data", MaxHTMLBytes)
	}
	return data, nil
}

func encodeEUC(fields url.Values) (string, error) {
	out := url.Values{}
	for k, vals := range fields {
		for _, v := range vals {
			b, _, e := transform.String(japanese.EUCJP.NewEncoder(), v)
			if e != nil {
				return "", fmt.Errorf("cannot encode station value in EUC-JP: %w", e)
			}
			out.Add(k, b)
		}
	}
	return out.Encode(), nil
}
func parseEUC(data []byte) (*html.Node, error) {
	b, _, e := transform.Bytes(japanese.EUCJP.NewDecoder(), data)
	if e != nil {
		return nil, e
	}
	return html.Parse(strings.NewReader(string(b)))
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func find(n *html.Node, p func(*html.Node) bool) *html.Node {
	if p(n) {
		return n
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if got := find(ch, p); got != nil {
			return got
		}
	}
	return nil
}
func all(n *html.Node, p func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	if n == nil {
		return out
	}
	if p(n) {
		out = append(out, n)
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		out = append(out, all(ch, p)...)
	}
	return out
}
func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Data == "script" || n.Data == "style" {
		return ""
	}
	var b strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		b.WriteString(nodeText(ch))
		b.WriteByte(' ')
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
func idNode(n *html.Node, id string) *html.Node {
	return find(n, func(x *html.Node) bool { return attr(x, "id") == id })
}
func classNode(n *html.Node, class string) *html.Node {
	return find(n, func(x *html.Node) bool { return strings.Contains(" "+attr(x, "class")+" ", " "+class+" ") })
}

type Quote struct {
	Class                    string   `json:"class"`
	SeasonJapanese           string   `json:"season_japanese"`
	TrainBasis               []string `json:"fare_calculation_train_basis"`
	RegularJPY               int      `json:"regular_adult_one_way_jpy"`
	SmartEXJPY               int      `json:"smartex_adult_one_way_jpy"`
	EXMemberJPY              int      `json:"ex_reservation_member_adult_one_way_jpy"`
	EXRequiresPaidMembership bool     `json:"ex_reservation_requires_paid_membership"`
	AdultSubtotalJPY         int      `json:"smartex_adult_subtotal_jpy"`
	PartyTotalJPY            any      `json:"smartex_party_total_jpy"`
	ChildFareJPY             any      `json:"child_fare_jpy"`
}
type Failure struct {
	Class  string `json:"class,omitempty"`
	Source string `json:"source,omitempty"`
	Error  string `json:"error"`
}
type FareResult struct {
	From             Station   `json:"from"`
	To               Station   `json:"to"`
	Date             string    `json:"date_jst"`
	Adults           int       `json:"adults"`
	Children         int       `json:"children"`
	Currency         string    `json:"currency"`
	Quotes           []Quote   `json:"quotes"`
	FetchFailures    []Failure `json:"fetch_failures"`
	UpstreamRequests int       `json:"upstream_requests"`
	RetrievedAt      string    `json:"retrieved_at"`
	SourceURL        string    `json:"source_url"`
	BookingURL       string    `json:"booking_url"`
	Inventory        any       `json:"inventory"`
	Notes            []string  `json:"notes"`
}

func ValidateFareDate(date string, now time.Time) (time.Time, error) {
	d, e := ParseDate(date)
	if e != nil {
		return d, e
	}
	n := now.In(JST)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, JST)
	start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, JST)
	end := start.AddDate(0, 3, 0)
	if d.Before(today) || !d.Before(end) {
		return d, fmt.Errorf("public fare navigator supports today through the end of the next two JST calendar months; requested %s, today %s", date, today.Format("2006-01-02"))
	}
	return d, nil
}

func (c *Client) Fares(ctx context.Context, from, to, date, class string, adults, children int, now time.Time) (FareResult, error) {
	var r FareResult
	f, e := Resolve(from)
	if e != nil {
		return r, e
	}
	t, e := Resolve(to)
	if e != nil {
		return r, e
	}
	if f.ID == t.ID {
		return r, fmt.Errorf("--from and --to must be different")
	}
	d, e := ValidateFareDate(date, now)
	if e != nil {
		return r, e
	}
	if e = ValidateParty(adults, children); e != nil {
		return r, e
	}
	classes := []string{class}
	if class == "all" {
		classes = []string{"reserved", "unreserved", "green"}
	} else if !ValidClass(class) {
		return r, fmt.Errorf("--class must be reserved, unreserved, green or all")
	}
	r = FareResult{From: f, To: t, Date: date, Adults: adults, Children: children, Currency: "JPY", Quotes: []Quote{}, FetchFailures: []Failure{}, SourceURL: "https://unchin-navi.jp/", BookingURL: BookingURL, Notes: []string{"Source quotes one adult one way; adult subtotal is arithmetic only and does not prove party seat availability", "Child fares are not provided by this calculator; party total is null when children are requested", "EX Reservation fares require separate paid EX membership; smartEX membership has different conditions", "This is a date/class/season fare calculation, not an actual train departure or available seat", "smartEX has no city-zone extension; local/conventional connection fares are separate", "Discount Hayatoku fares, route eligibility and excluded days require product-source/booking confirmation"}}
	fields := url.Values{"MODE": {"0"}, "ETOK": {"0"}, "YEAR": {strconv.Itoa(d.Year())}, "MONH": {strconv.Itoa(int(d.Month()))}, "DATE": {strconv.Itoa(d.Day())}, "INES": {""}, "INEE": {""}, "SHIF": {""}, "SHIT": {""}, "SEAT": {""}, "InsPos": {"0"}, "EKIS": {f.Japanese}, "EKIE": {t.Japanese}, "ym": {d.Format("200601")}}
	body, e := encodeEUC(fields)
	if e != nil {
		return r, e
	}
	data, e := c.fetch(ctx, http.MethodPost, c.endpoint, body)
	if e != nil {
		return r, e
	}
	doc, e := parseEUC(data)
	if e != nil {
		return r, e
	}
	normalized := map[string]string{}
	for _, n := range all(doc, func(x *html.Node) bool { return x.Data == "input" }) {
		if name := attr(n, "name"); name == "INES" || name == "INEE" {
			normalized[name] = attr(n, "value")
		}
	}
	if normalized["INES"] != f.Japanese || normalized["INEE"] != t.Japanese {
		return r, fmt.Errorf("source did not resolve the exact requested stations; use official navigator")
	}
	expected := f.Japanese + "-" + t.Japanese
	option := find(doc, func(x *html.Node) bool { return x.Data == "option" && attr(x, "value") == expected })
	if option == nil {
		return r, fmt.Errorf("source has no exact Shinkansen segment %s; no substitute route was quoted", expected)
	}
	fields.Set("INES", f.Japanese)
	fields.Set("INEE", t.Japanese)
	fields.Set("SHIF", f.Japanese)
	fields.Set("SHIT", t.Japanese)
	var rateError *cliutil.RateLimitError
	for _, cl := range classes {
		fields.Set("SEAT", map[string]string{"reserved": "0", "unreserved": "1", "green": "2"}[cl])
		body, e = encodeEUC(fields)
		if e != nil {
			return r, e
		}
		data, e = c.fetch(ctx, http.MethodPost, c.endpoint, body)
		if e == nil {
			var q Quote
			q, e = ParseQuote(data, f, t, date, cl)
			if e == nil {
				q.AdultSubtotalJPY = q.SmartEXJPY * adults
				if children == 0 {
					q.PartyTotalJPY = q.AdultSubtotalJPY
				}
				r.Quotes = append(r.Quotes, q)
			}
		}
		if e != nil {
			var rate *cliutil.RateLimitError
			if rateError == nil && errors.As(e, &rate) {
				rateError = rate
			}
			r.FetchFailures = append(r.FetchFailures, Failure{Class: cl, Error: e.Error()})
		}
	}
	r.UpstreamRequests = c.requests
	r.RetrievedAt = ts(time.Now())
	if len(r.Quotes) == 0 {
		if rateError != nil {
			return r, rateError
		}
		return r, fmt.Errorf("all fare classes failed; inspect fetch_failures and use booking handoff")
	}
	return r, nil
}

var yenPattern = regexp.MustCompile(`([0-9][0-9,]*)\s*円`)

func rowYen(doc *html.Node, id string) (int, error) {
	n := idNode(doc, id)
	m := yenPattern.FindAllStringSubmatch(nodeText(n), -1)
	if len(m) != 1 {
		return 0, fmt.Errorf("source drift: expected one active adult fare in %s", id)
	}
	v, e := strconv.Atoi(strings.ReplaceAll(m[0][1], ",", ""))
	if e != nil || v <= 0 || v > 1000000 {
		return 0, fmt.Errorf("source adult fare is invalid")
	}
	return v, nil
}
func ParseQuote(data []byte, f, t Station, date, class string) (Quote, error) {
	var q Quote
	doc, e := parseEUC(data)
	if e != nil {
		return q, e
	}
	header := nodeText(classNode(doc, "frtoeki_rosen"))
	header = strings.ReplaceAll(header, "～", "〜")
	d, e := ParseDate(date)
	if e != nil {
		return q, e
	}
	expectedDate := fmt.Sprintf("%d年%d月%d日", d.Year(), d.Month(), d.Day())
	clJP := map[string]string{"reserved": "普通車指定席", "unreserved": "普通車自由席", "green": "グリーン車"}[class]
	if !strings.Contains(header, f.Japanese+" 〜 "+t.Japanese) || !strings.Contains(header, expectedDate) || !strings.Contains(header, clJP) || !strings.Contains(header, "大人1名") || !strings.Contains(header, "片道") {
		return q, fmt.Errorf("source drift: result route/date/class/adult assumptions do not match request")
	}
	parts := strings.Split(header, "／")
	if len(parts) < 3 {
		return q, fmt.Errorf("source season is missing")
	}
	season := parts[0]
	if i := strings.LastIndex(season, "（"); i >= 0 {
		season = season[i+len("（"):]
	} else {
		return q, fmt.Errorf("source season format changed")
	}
	q = Quote{Class: class, SeasonJapanese: season, TrainBasis: []string{}, EXRequiresPaidMembership: true}
	if q.RegularJPY, e = rowYen(doc, "resultHyouka_t2"); e != nil {
		return q, e
	}
	if q.SmartEXJPY, e = rowYen(doc, "resultHyouka_t5"); e != nil {
		return q, e
	}
	if q.EXMemberJPY, e = rowYen(doc, "resultHyouka_t1"); e != nil {
		return q, e
	}
	for _, n := range all(idNode(doc, "resultDetail_2"), func(x *html.Node) bool { return strings.Contains(" "+attr(x, "class")+" ", " rosen_name_kek ") }) {
		q.TrainBasis = appendUnique(q.TrainBasis, nodeText(n))
	}
	return q, nil
}

type Link struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}
type SourceCheck struct {
	Source      Source `json:"source"`
	Status      string `json:"status"`
	RetrievedAt string `json:"retrieved_at"`
	Bytes       int    `json:"bytes"`
	Links       []Link `json:"links,omitempty"`
	Error       string `json:"error,omitempty"`
	Err         error  `json:"-"`
}

// AllSourceFailures retains typed throttling/cancellation across serialization.
func AllSourceFailures(checks []SourceCheck) error {
	errs := []error{}
	for _, check := range checks {
		if check.Err != nil {
			errs = append(errs, check.Err)
		}
	}
	if len(errs) == 0 {
		return errors.New("all requested public sources failed")
	}
	return fmt.Errorf("all requested public sources failed: %w", errors.Join(errs...))
}

func (c *Client) Source(ctx context.Context, s Source, links bool) SourceCheck {
	result := SourceCheck{Source: s, Status: "failed", RetrievedAt: ts(time.Now()), Links: []Link{}}
	data, e := c.fetch(ctx, http.MethodGet, s.URL, "")
	if e != nil {
		result.Error = e.Error()
		result.Err = e
		return result
	}
	result.Bytes = len(data)
	doc, e := html.Parse(strings.NewReader(string(data)))
	if e != nil {
		result.Error = e.Error()
		result.Err = e
		return result
	}
	if find(doc, func(n *html.Node) bool { return n.Data == "body" }) == nil {
		result.Error = "source HTML body missing"
		result.Err = errors.New(result.Error)
		return result
	}
	result.Status = "http_200_public_page"
	if links {
		base, _ := url.Parse(s.URL)
		for _, a := range all(doc, func(n *html.Node) bool { return n.Data == "a" }) {
			u, e := url.Parse(attr(a, "href"))
			if e != nil {
				continue
			}
			u = base.ResolveReference(u)
			if u.Scheme != "https" {
				continue
			}
			if strings.HasSuffix(strings.ToLower(u.Path), ".pdf") {
				result.Links = append(result.Links, Link{Text: nodeText(a), URL: u.String()})
			}
			if len(result.Links) >= 20 {
				break
			}
		}
	}
	return result
}
