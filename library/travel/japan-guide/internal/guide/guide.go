// Package guide extracts bounded planning facts from public Japan Guide HTML.
package guide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-guide/internal/cliutil"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const Origin = "https://www.japan-guide.com"
const maxBody = 4 << 20
const maxFactsCache = 128 << 10

var pageID = regexp.MustCompile(`^e[0-9]+[a-z]?(?:_[a-zA-Z0-9_-]+)?$`)
var sourcePath = regexp.MustCompile(`^/(?:e|list)/e[0-9]+[a-z]?(?:_[a-zA-Z0-9_-]+)?\.html$`)
var seasonal = regexp.MustCompile(`(?i)\b(?:january|february|march|april|may|june|july|august|september|october|november|december|summer|winter|spring|autumn|season)\b`)
var datedEvent = regexp.MustCompile(`(?i)^(?:The )?([0-9]{4})\b.*\b(?:will be held|is held|takes place|scheduled)\b`)
var annualEvent = regexp.MustCompile(`(?i)\b(?:held|takes place)\b.*\b(?:every year|annually|(?:first|second|third|fourth|last)\s+(?:full\s+)?(?:monday|tuesday|wednesday|thursday|friday|saturday|sunday|weekend))\b`)
var sourceYear = regexp.MustCompile(`\b20[0-9]{2}\b`)
var yenFee = regexp.MustCompile(`(?i)\b[0-9][0-9,]*(?:\s*[-–]\s*[0-9][0-9,]*)?\s*yen\b`)

type Recommendation struct {
	Dots  int    `json:"dots"`
	Scale int    `json:"scale"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
type Item struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Name           string          `json:"name"`
	URL            string          `json:"url"`
	Region         string          `json:"region,omitempty"`
	Category       string          `json:"category,omitempty"`
	Interests      []string        `json:"interests,omitempty"`
	Recommendation *Recommendation `json:"editorial_recommendation,omitempty"`
	SourceURL      string          `json:"source_url"`
	Duration       string          `json:"duration,omitempty"`
}
type Visit struct {
	Facility  string   `json:"facility"`
	Hours     *string  `json:"hours"`
	Closed    *string  `json:"closed_days"`
	Admission *string  `json:"admission"`
	Currency  string   `json:"currency,omitempty"`
	SourceURL string   `json:"source_url"`
	Notes     []string `json:"notes,omitempty"`
}
type CalendarFact struct {
	Statement    string `json:"statement"`
	Kind         string `json:"kind"`
	ExplicitYear *int   `json:"explicit_year"`
	SourceURL    string `json:"source_url"`
}
type Detail struct {
	Item
	JapaneseName          *string        `json:"japanese_name"`
	Visits                []Visit        `json:"visit_information"`
	Access                *string        `json:"access"`
	AccessURL             string         `json:"access_url"`
	SeasonalNotes         []string       `json:"seasonal_notes"`
	PlanningNotices       []string       `json:"planning_notices"`
	DatedNotes            []string       `json:"dated_notes"`
	EventCalendar         []CalendarFact `json:"event_calendar"`
	ExtractionLimitations []string       `json:"extraction_limitations"`
	SourceUpdated         *string        `json:"source_updated"`
	RetrievedAt           string         `json:"retrieved_at"`
	Freshness             string         `json:"freshness"`
	OpenNow               *bool          `json:"open_now"`
	OpeningUncertainty    string         `json:"opening_uncertainty"`
	Truncated             bool           `json:"facts_truncated"`
}
type Metrics struct {
	Requests  int   `json:"upstream_requests"`
	Bytes     int64 `json:"upstream_bytes"`
	ElapsedMS int64 `json:"elapsed_ms"`
}

// SnapshotWriteError means live acquisition succeeded but its requested local save failed.
type SnapshotWriteError struct {
	SourceID string
	Err      error
}

func (e *SnapshotWriteError) Error() string {
	return fmt.Sprintf("save extracted facts for %s: %v", e.SourceID, e.Err)
}
func (e *SnapshotWriteError) Unwrap() error { return e.Err }

type Client struct {
	HTTP     *http.Client
	Limiter  *cliutil.AdaptiveLimiter
	Metrics  Metrics
	CacheDir string
	Offline  bool
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 4 {
			return fmt.Errorf("too many source redirects")
		}
		if req.URL.Scheme != "https" || req.URL.Host != "www.japan-guide.com" || req.URL.User != nil {
			return fmt.Errorf("source redirected outside Japan Guide")
		}
		if _, err := Canonical(req.URL.String()); err != nil || req.URL.Path != via[0].URL.Path {
			return fmt.Errorf("source redirected to another page; use the published canonical source URL")
		}
		return nil
	}}, Limiter: cliutil.NewAdaptiveLimiter(1)}
}
func Canonical(input string) (string, error) {
	input = strings.TrimSpace(input)
	if pageID.MatchString(input) {
		return Origin + "/e/" + input + ".html", nil
	}
	u, e := url.Parse(input)
	if e != nil {
		return "", fmt.Errorf("invalid source ID or URL")
	}
	if u.IsAbs() {
		if u.Scheme != "https" || u.Host != "www.japan-guide.com" || u.User != nil {
			return "", fmt.Errorf("use an HTTPS www.japan-guide.com source URL")
		}
	} else {
		if u.Host != "" || u.User != nil {
			return "", fmt.Errorf("use a source page ID or a canonical HTTPS source URL")
		}
		if !strings.HasPrefix(u.Path, "/") {
			u.Path = "/e/" + u.Path
		}
	}
	if !sourcePath.MatchString(u.Path) {
		return "", fmt.Errorf("use a source page ID such as e3001 or a canonical /e/e3001.html URL")
	}
	return Origin + u.Path, nil
}
func ID(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(u.Path), ".html")
}
func link(base, ref string) string {
	if strings.TrimSpace(ref) == "" {
		return ""
	}
	b, e := url.Parse(base)
	if e != nil {
		return ""
	}
	r, e := url.Parse(ref)
	if e != nil {
		return ""
	}
	v := b.ResolveReference(r)
	s, e := Canonical(v.String())
	if e != nil {
		return ""
	}
	return s
}
func (c *Client) Fetch(ctx context.Context, raw string) (*html.Node, error) {
	if c.Offline {
		return nil, fmt.Errorf("offline mode supports cached guide inspect only")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	u, e := Canonical(raw)
	if e != nil {
		return nil, e
	}
	if e = c.Limiter.Wait(ctx); e != nil {
		return nil, e
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "japan-guide-pp-cli/0.1 (+read-only travel planning)")
	t := time.Now()
	c.Metrics.Requests++
	res, e := c.HTTP.Do(req)
	c.Metrics.ElapsedMS += time.Since(t).Milliseconds()
	if e != nil {
		return nil, fmt.Errorf("fetch %s: %w; check network access or retry later", u, e)
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		c.Limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(res)}
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("source %s returned HTTP %d; open the canonical page or retry later", u, res.StatusCode)
	}
	c.Limiter.OnSuccess()
	data, e := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	c.Metrics.Bytes += int64(len(data))
	if e != nil {
		return nil, e
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("source page exceeds %d byte download budget", maxBody)
	}
	r, e := charset.NewReader(bytes.NewReader(data), res.Header.Get("Content-Type"))
	if e != nil {
		return nil, e
	}
	doc, e := html.Parse(r)
	if e != nil {
		return nil, e
	}
	if first(doc, func(n *html.Node) bool { return n.Data == "h1" }) == nil {
		return nil, fmt.Errorf("source page has no guide heading; may be an access challenge or changed layout")
	}
	return doc, nil
}
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func has(n *html.Node, cls string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == cls {
			return true
		}
	}
	return false
}
func all(n *html.Node, p func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x == nil {
			return
		}
		if x.Type == html.ElementNode && p(x) {
			out = append(out, x)
		}
		for q := x.FirstChild; q != nil; q = q.NextSibling {
			walk(q)
		}
	}
	walk(n)
	return out
}
func first(n *html.Node, p func(*html.Node) bool) *html.Node {
	a := all(n, p)
	if len(a) > 0 {
		return a[0]
	}
	return nil
}
func cls(n *html.Node, c string) *html.Node {
	return first(n, func(x *html.Node) bool { return has(x, c) })
}
func byID(n *html.Node, id string) *html.Node {
	return first(n, func(x *html.Node) bool { return attr(x, "id") == id })
}
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x == nil {
			return
		}
		if x.Type == html.ElementNode {
			if x.Data == "script" || x.Data == "style" || x.Data == "svg" || has(x, "dot_rating") {
				return
			}
			if x.Data == "br" {
				b.WriteString("; ")
			}
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for q := x.FirstChild; q != nil; q = q.NextSibling {
			walk(q)
		}
		if x.Type == html.ElementNode && (x.Data == "p" || x.Data == "div" || x.Data == "h3" || x.Data == "li") {
			b.WriteByte(' ')
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
func bounded(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
func str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func rec(n *html.Node) *Recommendation {
	x := cls(n, "dot_rating__dots")
	if x == nil {
		return nil
	}
	v, e := strconv.Atoi(attr(x, "data-dots"))
	if e != nil || v < 0 || v > 3 {
		return nil
	}
	return &Recommendation{Dots: v, Scale: 3, Label: attr(x, "data-tooltip-label"), Kind: "Japan Guide editorial recommendation; not a visitor rating"}
}
func ParseDestinations(doc *html.Node, source string) ([]Item, error) {
	out := []Item{}
	for _, r := range all(doc, func(n *html.Node) bool { return has(n, "dest_top_destinations__region") }) {
		region := text(cls(r, "dest_top_destinations__region_name"))
		for _, a := range all(r, func(n *html.Node) bool { return has(n, "dest_top_destinations__destination") }) {
			u := link(source, attr(a, "href"))
			if u == "" {
				continue
			}
			out = append(out, Item{ID: ID(u), Kind: "destination", Name: text(cls(a, "dest_top_destinations__destination_name_text")), URL: u, Region: region, Recommendation: rec(a), SourceURL: source})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("destination directory layout changed; no region destination records found")
	}
	return out, nil
}
func (c *Client) Destinations(ctx context.Context) ([]Item, error) {
	u := Origin + "/e/e623a.html"
	d, e := c.Fetch(ctx, u)
	if e != nil {
		return nil, e
	}
	return ParseDestinations(d, u)
}
func ParseInterests(doc *html.Node, u string) ([]Item, error) {
	out := []Item{}
	for _, cat := range all(doc, func(n *html.Node) bool { return has(n, "interests_top_page__category") }) {
		category := text(cls(cat, "interests_top_page__category_title"))
		for _, a := range all(cat, func(n *html.Node) bool { return has(n, "link_gallery__link") }) {
			v := link(u, attr(a, "href"))
			if v == "" {
				continue
			}
			out = append(out, Item{ID: ID(v), Kind: "interest", Name: text(cls(a, "link_gallery__link__label")), URL: v, Category: category, SourceURL: u})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("interest directory layout changed")
	}
	return out, nil
}
func (c *Client) Interests(ctx context.Context) ([]Item, error) {
	u := Origin + "/e/e623.html"
	d, e := c.Fetch(ctx, u)
	if e != nil {
		return nil, e
	}
	return ParseInterests(d, u)
}
func ParseAttractions(doc *html.Node, u string) ([]Item, error) {
	out := []Item{}
	root := byID(doc, "section_spot_list")
	if root == nil {
		return nil, fmt.Errorf("page is not a destination attraction directory; use a destination ID from guide destinations")
	}
	for _, cat := range all(root, func(n *html.Node) bool { return has(n, "spot_list__category") }) {
		category := text(cls(cat, "spot_list__category__label"))
		if category == "" {
			category = text(cls(cat, "spot_list__category_name"))
		}
		for _, n := range all(cat, func(n *html.Node) bool { return has(n, "spot_list__spot") }) {
			a := cls(n, "spot_list__spot__name")
			v := link(u, attr(a, "href"))
			if v == "" {
				continue
			}
			tags := []string{}
			for _, t := range all(n, func(x *html.Node) bool { return x.Data == "a" && has(x, "icon_wrap") && attr(x, "aria-label") != "" }) {
				tags = append(tags, attr(t, "aria-label"))
			}
			kind := "attraction"
			if strings.Contains(strings.ToLower(category), "side trip") {
				kind = "destination"
			}
			if strings.EqualFold(category, "Events") {
				kind = "event"
			}
			out = append(out, Item{ID: ID(v), Kind: kind, Name: text(a), URL: v, Category: category, Interests: tags, Recommendation: rec(n), SourceURL: u})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("attraction directory layout changed; no source records found")
	}
	return out, nil
}
func (c *Client) Attractions(ctx context.Context, input string) ([]Item, error) {
	u, e := Canonical(input)
	if e != nil {
		return nil, e
	}
	d, e := c.Fetch(ctx, u)
	if e != nil {
		return nil, e
	}
	return ParseAttractions(d, u)
}
func ParseItineraries(doc *html.Node, u string) ([]Item, error) {
	root := byID(doc, "section_itinerary_teasers")
	if root == nil {
		if ID(u) != "e2400" && !strings.Contains(strings.ToLower(text(first(doc, func(n *html.Node) bool { return n.Data == "h1" }))), "itinerar") {
			return nil, fmt.Errorf("page is not a source itinerary index; use guide itineraries or a destination with itinerary teasers")
		}
		root = cls(doc, "page_body")
	}
	out := []Item{}
	seen := map[string]bool{}
	for _, a := range all(root, func(n *html.Node) bool { return has(n, "link_gallery__link") }) {
		v := link(u, attr(a, "href"))
		label := text(cls(a, "link_gallery__link__label"))
		if v == "" || label == "" || seen[v] || !strings.Contains(ID(v), "_") {
			continue
		}
		seen[v] = true
		out = append(out, Item{ID: ID(v), Kind: "source_itinerary", Name: label, URL: v, Duration: text(cls(a, "link_gallery__link__type")), SourceURL: u})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no source itinerary links found; use guide itineraries for the index")
	}
	return out, nil
}
func (c *Client) Itineraries(ctx context.Context, input string) ([]Item, error) {
	if input == "" {
		input = "e2400"
	}
	u, e := Canonical(input)
	if e != nil {
		return nil, e
	}
	d, e := c.Fetch(ctx, u)
	if e != nil {
		return nil, e
	}
	return ParseItineraries(d, u)
}

// currentSourceCategory reads the category around the current page's sidebar self-link.
func currentSourceCategory(doc *html.Node, u string) string {
	for _, a := range all(doc, func(n *html.Node) bool { return has(n, "related_links__section_link__text") }) {
		if link(u, attr(a, "href")) != u {
			continue
		}
		for p := a.Parent; p != nil; p = p.Parent {
			if !has(p, "related_links__sub_section") {
				continue
			}
			for prev := p.PrevSibling; prev != nil; prev = prev.PrevSibling {
				if prev.Type != html.ElementNode {
					continue
				}
				if has(prev, "accordion__trigger") {
					return bounded(text(prev), 100)
				}
				break
			}
		}
	}
	return ""
}

func introductoryParagraphs(doc *html.Node) []*html.Node {
	paragraphs := all(byID(doc, "section_main_content"), func(n *html.Node) bool { return n.Data == "p" && text(n) != "" })
	if len(paragraphs) > 6 {
		paragraphs = paragraphs[:6]
	}
	return paragraphs
}

func japaneseName(title string, paragraphs []*html.Node) *string {
	titleBase := strings.TrimSpace(strings.Split(title, "(")[0])
	nameWords := strings.Fields(titleBase)
	for _, p := range paragraphs {
		s := text(p)
		for _, match := range regexp.MustCompile(`[(（]([^()（）]+)[)）]`).FindAllStringSubmatchIndex(s, -1) {
			before := strings.ToLower(strings.TrimSpace(s[:match[0]]))
			parts := strings.FieldsFunc(s[match[2]:match[3]], func(r rune) bool { return r == ',' || r == '，' })
			belongs := false
			for i := len(nameWords); i > 0; i-- {
				candidate := strings.ToLower(strings.Join(nameWords[:i], " "))
				if before == candidate || strings.HasSuffix(before, " "+candidate) {
					belongs = true
					break
				}
			}
			for _, part := range parts {
				if strings.EqualFold(strings.TrimSpace(part), titleBase) {
					belongs = true
				}
			}
			if !belongs {
				continue
			}
			for _, part := range parts {
				for _, r := range part {
					if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) {
						return str(bounded(strings.TrimSpace(part), 120))
					}
				}
			}
		}
	}
	return nil
}

func ParseDetail(doc *html.Node, u string) Detail {
	title := text(first(doc, func(n *html.Node) bool { return n.Data == "h1" }))
	kind := "guide_page"
	if byID(doc, "section_get_there") != nil || byID(doc, "section_admission") != nil {
		kind = "attraction"
	}
	if byID(doc, "section_spot_list") != nil {
		kind = "destination"
	}
	for _, a := range all(cls(doc, "breadcrumbs"), func(n *html.Node) bool { return n.Data == "a" }) {
		if ID(link(u, attr(a, "href"))) == "e623" {
			kind = "interest"
		}
	}
	if cls(doc, "itinerary") != nil || strings.HasPrefix(ID(u), "e2400_") {
		kind = "source_itinerary"
	}
	switch ID(u) {
	case "e623a":
		kind = "destination_directory"
	case "e623":
		kind = "interest_directory"
	case "e2400":
		kind = "itinerary_directory"
	}
	category := currentSourceCategory(doc, u)
	if strings.EqualFold(category, "Events") && kind != "interest" && !strings.Contains(kind, "itinerary") && !strings.Contains(kind, "directory") {
		kind = "event"
	}
	d := Detail{Item: Item{ID: ID(u), Kind: kind, Name: title, URL: u, Category: category, SourceURL: u, Recommendation: rec(cls(doc, "page_title"))}, Visits: []Visit{}, SeasonalNotes: []string{}, PlanningNotices: []string{}, DatedNotes: []string{}, EventCalendar: []CalendarFact{}, ExtractionLimitations: []string{}, RetrievedAt: time.Now().UTC().Format(time.RFC3339), Freshness: "live", AccessURL: u + "#section_get_there", OpeningUncertainty: "Source schedules are reference facts in Japan local time (JST); open_now is unknown. Seasonal exceptions, construction, weather and one-off closures require checking the source or operator."}
	intro := byID(doc, "section_main_content")
	paragraphs := introductoryParagraphs(doc)
	d.JapaneseName = japaneseName(title, paragraphs)
	admission := byID(doc, "section_admission")
	for i, v := range all(admission, func(n *html.Node) bool { return has(n, "page_admission") }) {
		if i >= 8 {
			d.Truncated = true
			break
		}
		scope := text(cls(v, "page_admission__title"))
		if scope == "" {
			scope = title
			for p := v.PrevSibling; p != nil; p = p.PrevSibling {
				if p.Data == "h3" {
					scope = text(p)
					break
				}
			}
		}
		f := Visit{Facility: scope, SourceURL: u + "#section_admission"}
		for _, n := range all(v, func(x *html.Node) bool { return has(x, "page_admission__item") }) {
			label := text(cls(n, "page_admission__item_label"))
			raw := text(cls(n, "page_admission__item_content"))
			value := bounded(raw, 360)
			d.Truncated = d.Truncated || len([]rune(raw)) > 360
			switch label {
			case "Hours":
				f.Hours = str(value)
			case "Closed":
				f.Closed = str(value)
			case "Admission":
				f.Admission = str(value)
				if strings.Contains(strings.ToLower(value), "yen") {
					f.Currency = "JPY"
				}
			}
			if seasonal.MatchString(value) {
				d.SeasonalNotes = append(d.SeasonalNotes, scope+": "+value)
			}
		}
		d.Visits = append(d.Visits, f)
	}
	if admission != nil && len(d.Visits) == 0 {
		scope := title
		for _, n := range all(admission, func(x *html.Node) bool { return x.Data == "h3" || x.Data == "p" }) {
			if n.Data == "h3" {
				scope = bounded(text(n), 120)
				continue
			}
			raw := text(n)
			fee := yenFee.FindString(raw)
			if fee == "" {
				continue
			}
			visit := Visit{Facility: scope, Admission: str(fee), Currency: "JPY", SourceURL: u + "#section_admission", Notes: []string{bounded(raw, 360)}}
			d.Truncated = d.Truncated || len([]rune(raw)) > 360
			d.Visits = append(d.Visits, visit)
			if sourceYear.MatchString(raw) {
				d.DatedNotes = append(d.DatedNotes, scope+": "+bounded(raw, 360))
			}
			if len(d.Visits) == 8 {
				d.Truncated = true
				break
			}
		}
		if len(d.Visits) == 0 {
			d.ExtractionLimitations = append(d.ExtractionLimitations, "Source Hours and Fees uses an unrecognized narrative layout; follow the canonical admission citation for complete facts.")
		}
	}
	access := byID(doc, "section_get_there")
	ap := first(access, func(n *html.Node) bool { return n.Data == "p" })
	if ap != nil {
		raw := text(ap)
		d.Access = str(bounded(raw, 300))
		d.Truncated = d.Truncated || len([]rune(raw)) > 300
	}
	for _, a := range all(access, func(n *html.Node) bool { return n.Data == "a" }) {
		if strings.Contains(strings.ToLower(text(a)), "getting to") || strings.Contains(strings.ToLower(text(a)), "access") {
			if v := link(u, attr(a, "href")); v != "" {
				d.AccessURL = v
				break
			}
		}
	}
	for _, n := range all(doc, func(x *html.Node) bool {
		return has(x, "page_last_updated") || has(x, "page_update") || has(x, "page_updated")
	}) {
		if stamp := attr(first(n, func(x *html.Node) bool { return x.Data == "time" }), "datetime"); stamp != "" {
			d.SourceUpdated = str(stamp)
		} else {
			d.SourceUpdated = str(text(n))
		}
	}
	if d.SourceUpdated == nil {
		for _, n := range all(doc, func(x *html.Node) bool { return x.Data == "div" || x.Data == "p" }) {
			t := text(n)
			if strings.HasPrefix(t, "Page last updated:") && len(t) < 100 {
				d.SourceUpdated = str(strings.TrimSpace(strings.TrimPrefix(t, "Page last updated:")))
				break
			}
		}
	}
	if d.SourceUpdated != nil {
		raw := strings.TrimSpace(strings.TrimPrefix(*d.SourceUpdated, "Page last updated:"))
		if t, e := time.Parse("January 2, 2006", raw); e == nil {
			raw = t.Format("2006-01-02")
		}
		d.SourceUpdated = &raw
	}
	for _, n := range all(cls(doc, "page_body"), func(x *html.Node) bool {
		return has(x, "notice") || has(x, "alert") || has(x, "box--warning") || has(x, "notice_box")
	}) {
		raw := text(n)
		t := bounded(raw, 320)
		d.Truncated = d.Truncated || len([]rune(raw)) > 320
		if t != "" {
			d.PlanningNotices = append(d.PlanningNotices, t)
		}
		if len(d.PlanningNotices) == 3 {
			d.Truncated = true
			break
		}
	}
	for _, n := range all(intro, func(x *html.Node) bool { return x.Data == "b" || x.Data == "strong" }) {
		raw := text(n)
		dateMatch := datedEvent.FindStringSubmatch(raw)
		if d.Kind == "event" && len(dateMatch) > 1 {
			d.DatedNotes = append(d.DatedNotes, bounded(raw, 280))
			year, _ := strconv.Atoi(dateMatch[1])
			d.EventCalendar = append(d.EventCalendar, CalendarFact{Statement: bounded(raw, 280), Kind: "dated_notice", ExplicitYear: &year, SourceURL: u + "#section_main_content"})
			d.Truncated = d.Truncated || len([]rune(raw)) > 280
			if len(d.DatedNotes) == 3 {
				break
			}
		}
	}
	if d.Kind == "event" {
		for _, p := range paragraphs {
			for _, sentence := range strings.Split(text(p), ". ") {
				if !annualEvent.MatchString(sentence) {
					continue
				}
				statement := bounded(sentence, 280)
				d.EventCalendar = append(d.EventCalendar, CalendarFact{Statement: statement, Kind: "annual_recurrence", SourceURL: u + "#section_main_content"})
				d.SeasonalNotes = append(d.SeasonalNotes, statement)
				d.Truncated = d.Truncated || len([]rune(sentence)) > 280
				if len(d.EventCalendar) >= 4 {
					break
				}
			}
			if len(d.EventCalendar) >= 4 {
				break
			}
		}
	}
	return d
}
func (c *Client) Inspect(ctx context.Context, input string) (Detail, error) {
	u, e := Canonical(input)
	if e != nil {
		return Detail{}, e
	}
	path := filepath.Join(c.CacheDir, ID(u)+".json")
	if c.Offline {
		if c.CacheDir == "" {
			return Detail{}, fmt.Errorf("offline requires a cache directory")
		}
		// #nosec G304 -- CacheDir is explicitly selected; the filename is a canonical ID with no separators and cached source identity is checked below.
		f, e := os.Open(path)
		if e != nil {
			return Detail{}, fmt.Errorf("read cached facts for %s: %w; first run guide inspect %s --cache", ID(u), e, ID(u))
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, maxFactsCache+1))
		if e != nil || len(b) > maxFactsCache {
			return Detail{}, fmt.Errorf("cached facts are unreadable or exceed %d byte budget", maxFactsCache)
		}
		var d Detail
		if e = json.Unmarshal(b, &d); e != nil {
			return Detail{}, fmt.Errorf("invalid cached facts: %w", e)
		}
		if d.URL != u || d.ID != ID(u) || d.RetrievedAt == "" {
			return Detail{}, fmt.Errorf("cached facts do not match source %s; refresh with guide inspect %s --cache", u, ID(u))
		}
		d.OpenNow = nil
		d.Freshness = "offline_snapshot"
		return d, nil
	}
	doc, e := c.Fetch(ctx, u)
	if e != nil {
		return Detail{}, e
	}
	d := ParseDetail(doc, u)
	if c.CacheDir != "" {
		if e = os.MkdirAll(c.CacheDir, 0700); e != nil {
			return Detail{}, &SnapshotWriteError{SourceID: ID(u), Err: e}
		}
		b, e := json.Marshal(d)
		if e != nil {
			return Detail{}, &SnapshotWriteError{SourceID: ID(u), Err: e}
		}
		f, e := os.CreateTemp(c.CacheDir, "facts-*.tmp")
		if e != nil {
			return Detail{}, &SnapshotWriteError{SourceID: ID(u), Err: e}
		}
		name := f.Name()
		defer os.Remove(name)
		if _, e = f.Write(b); e != nil {
			return Detail{}, &SnapshotWriteError{SourceID: ID(u), Err: errors.Join(e, f.Close())}
		}
		if e = f.Close(); e != nil {
			return Detail{}, &SnapshotWriteError{SourceID: ID(u), Err: e}
		}
		if e = os.Rename(name, path); e != nil {
			return Detail{}, &SnapshotWriteError{SourceID: ID(u), Err: e}
		}
	}
	return d, nil
}

// Itinerary returns factual day or stop labels and canonical links, never article paragraphs.
func (c *Client) Itinerary(ctx context.Context, input string) (map[string]any, error) {
	u, e := Canonical(input)
	if e != nil {
		return nil, e
	}
	doc, e := c.Fetch(ctx, u)
	if e != nil {
		return nil, e
	}
	return ParseItinerary(doc, u)
}

// ParseItinerary extracts source stop labels without republishing the plan's prose.
func ParseItinerary(doc *html.Node, u string) (map[string]any, error) {
	stops := []map[string]any{}
	local := cls(doc, "itinerary")
	var nodes []*html.Node
	if local != nil {
		nodes = all(local, func(n *html.Node) bool { return has(n, "itinerary__node--item") })
	} else if strings.HasPrefix(ID(u), "e2400_") {
		nodes = all(byID(doc, "section_main_content"), func(n *html.Node) bool { return has(n, "spot_list__spot") })
	} else {
		return nil, fmt.Errorf("page is not a source itinerary; choose a plan with guide itineraries")
	}
	for _, n := range nodes {
		title := cls(n, "spot_list__spot__name")
		if local != nil {
			title = cls(n, "itinerary__node__name")
		}
		name := text(title)
		if name == "" {
			continue
		}
		links := []string{}
		seen := map[string]bool{}
		linkRoot := title
		if local != nil {
			linkRoot = n
		}
		for _, a := range all(linkRoot, func(x *html.Node) bool { return x.Data == "a" }) {
			if v := link(u, attr(a, "href")); v != "" && !seen[v] {
				seen[v] = true
				links = append(links, v)
				if len(links) == 5 {
					break
				}
			}
		}
		stop := map[string]any{"label": bounded(name, 100), "source_links": links}
		if local != nil {
			stop["source_visit_duration"] = str(text(cls(n, "itinerary__node__duration")))
		}
		notes := []string{}
		for _, sentence := range strings.Split(text(n), ". ") {
			if seasonal.MatchString(sentence) && strings.Contains(strings.ToLower(sentence), "closed") {
				notes = append(notes, bounded(sentence, 220))
				if len(notes) == 2 {
					break
				}
			}
		}
		stop["planning_notes"] = notes
		stops = append(stops, stop)
		if len(stops) == 30 {
			break
		}
	}
	if len(stops) == 0 {
		return nil, fmt.Errorf("no source day/stop labels found; open %s for the complete source plan", u)
	}
	detail := ParseDetail(doc, u)
	return map[string]any{"id": ID(u), "kind": "source_itinerary", "name": detail.Name, "url": u, "stops": stops, "source_url": u, "source_updated": detail.SourceUpdated, "retrieved_at": detail.RetrievedAt, "editorial_plan": true, "truncated": len(nodes) > 30, "scope": "Source day/stop labels and stated visit durations; timings and suitability are not verified"}, nil
}
