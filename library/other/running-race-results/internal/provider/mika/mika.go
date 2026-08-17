// internal/provider/mika/mika.go
package mika

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
)

// userAgent is a browser-like UA. Mika's host returns 403 to the default
// Go-http-client UA, so a realistic UA is required.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Client is the Mika Timing provider adapter.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a Client configured for the Mika Timing production host.
func New() *Client {
	return &Client{
		BaseURL: "https://berlin.r.mikatiming.com",
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Name implements provider.Provider.
func (c *Client) Name() string { return "mika" }

// Lookup implements provider.Provider.
func (c *Client) Lookup(ctx context.Context, ev domain.Event, bib string) (domain.Result, error) {
	base := c.BaseURL
	if ev.BaseURL != "" {
		base = ev.BaseURL
	}

	// When the event carries a year, include it in the path so the request
	// lands directly on the year-scoped URL (e.g. /2025/) instead of hitting
	// the root which issues a 301 redirect and causes Go's http.Client to
	// convert the POST to a GET, dropping the form body.
	yearBase := base
	if ev.Year != 0 {
		yearBase = fmt.Sprintf("%s/%d", strings.TrimRight(base, "/"), ev.Year)
	}

	// Step 1: POST search by bib.
	searchURL := fmt.Sprintf("%s/?event=%s&pid=search", yearBase, url.QueryEscape(ev.ID))
	body := url.Values{
		"search[name]":     {""},
		"search[start_no]": {bib},
		"search[nation]":   {""},
		"Search":           {"Search"},
	}
	searchReq, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, strings.NewReader(body.Encode()))
	if err != nil {
		return domain.Result{}, fmt.Errorf("mika: create search request: %w", err)
	}
	searchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	searchReq.Header.Set("User-Agent", userAgent)

	searchResp, err := c.HTTP.Do(searchReq)
	if err != nil {
		return domain.Result{}, fmt.Errorf("mika: search request: %w", err)
	}
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		return domain.Result{}, fmt.Errorf("mika: search status %d", searchResp.StatusCode)
	}

	searchDoc, err := html.Parse(searchResp.Body)
	if err != nil {
		return domain.Result{}, fmt.Errorf("mika: parse search HTML: %w", err)
	}

	// Step 2: Extract idp from a content=detail link inside the result list.
	// The results are in a <ul> whose class contains "list-group". Nav links
	// live in <ul class="nav ..."> and must be ignored.
	idp, ok := idpFromResultList(searchDoc)
	if !ok {
		return domain.Result{}, provider.ErrBibNotFound
	}

	// Step 3: GET detail page.
	detailURL := fmt.Sprintf("%s/?content=detail&fpid=search&pid=search&idp=%s&lang=EN_CAP&event=%s",
		yearBase, url.QueryEscape(idp), url.QueryEscape(ev.ID))
	detailReq, err := http.NewRequestWithContext(ctx, http.MethodGet, detailURL, nil)
	if err != nil {
		return domain.Result{}, fmt.Errorf("mika: create detail request: %w", err)
	}
	detailReq.Header.Set("User-Agent", userAgent)

	detailResp, err := c.HTTP.Do(detailReq)
	if err != nil {
		return domain.Result{}, fmt.Errorf("mika: detail request: %w", err)
	}
	defer detailResp.Body.Close()
	if detailResp.StatusCode != http.StatusOK {
		return domain.Result{}, fmt.Errorf("mika: detail status %d", detailResp.StatusCode)
	}

	doc, err := html.Parse(detailResp.Body)
	if err != nil {
		return domain.Result{}, fmt.Errorf("mika: parse detail HTML: %w", err)
	}

	// Step 4: Parse detail cells.
	fullName := tdText(doc, "f-__fullname")
	parsedBib := tdText(doc, "f-start_no_text")
	netTime := tdText(doc, "f-time_finish_netto")
	gunTime := tdText(doc, "f-time_finish_brutto")
	// f-place_all  = Place (M/W/D) = gender place
	// f-place_nosex = Place (Total) = overall place
	genderPlaceStr := tdText(doc, "f-place_all")
	overallPlaceStr := tdText(doc, "f-place_nosex")

	// Step 5: Bib guard — prevent returning a wrong runner.
	if parsedBib != bib {
		return domain.Result{}, provider.ErrBibNotFound
	}

	overallPlace := parsePlace(overallPlaceStr)
	genderPlace := parsePlace(genderPlaceStr)

	return domain.Result{
		Provider:     "mika",
		RaceName:     ev.Name,
		Year:         ev.Year,
		Runner:       parseName(fullName),
		Bib:          parsedBib,
		NetTime:      netTime,
		GunTime:      gunTime,
		OverallPlace: overallPlace,
		GenderPlace:  genderPlace,
		SourceURL:    detailURL,
	}, nil
}

// SearchByName implements provider.NameSearcher. It parses the search results
// page for a name query, collecting each result list row's runner name + bib
// without fetching individual detail pages.
func (c *Client) SearchByName(ctx context.Context, ev domain.Event, name string) ([]domain.Result, error) {
	base := c.BaseURL
	if ev.BaseURL != "" {
		base = ev.BaseURL
	}
	yearBase := base
	if ev.Year != 0 {
		yearBase = fmt.Sprintf("%s/%d", strings.TrimRight(base, "/"), ev.Year)
	}

	searchURL := fmt.Sprintf("%s/?event=%s&pid=search", yearBase, url.QueryEscape(ev.ID))
	body := url.Values{
		"search[name]":     {name},
		"search[start_no]": {""},
		"search[nation]":   {""},
		"Search":           {"Search"},
	}
	searchReq, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("mika: create search request: %w", err)
	}
	searchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	searchReq.Header.Set("User-Agent", userAgent)

	searchResp, err := c.HTTP.Do(searchReq)
	if err != nil {
		return nil, fmt.Errorf("mika: search request: %w", err)
	}
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mika: search status %d", searchResp.StatusCode)
	}

	doc, err := html.Parse(searchResp.Body)
	if err != nil {
		return nil, fmt.Errorf("mika: parse search HTML: %w", err)
	}

	return parseSearchRows(doc, ev), nil
}

// parseSearchRows walks the result <ul> and extracts runner name + bib from
// each non-header list-group-item <li>.
func parseSearchRows(doc *html.Node, ev domain.Event) []domain.Result {
	var findResultUL func(*html.Node) *html.Node
	findResultUL = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "ul" {
			cls := ""
			for _, a := range n.Attr {
				if a.Key == "class" {
					cls = a.Val
				}
			}
			if containsToken(cls, "list-group") &&
				!containsToken(cls, "nav") &&
				!containsToken(cls, "list-info") {
				return n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if found := findResultUL(c); found != nil {
				return found
			}
		}
		return nil
	}

	ul := findResultUL(doc)
	if ul == nil {
		return nil
	}

	var out []domain.Result
	for li := ul.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		cls := ""
		for _, a := range li.Attr {
			if a.Key == "class" {
				cls = a.Val
			}
		}
		if !containsToken(cls, "list-group-item") || containsToken(cls, "list-group-header") {
			continue
		}

		runnerName := liRunnerName(li)
		bib := liBib(li)
		if runnerName == "" && bib == "" {
			continue
		}
		out = append(out, domain.Result{
			Provider: "mika",
			RaceName: ev.Name,
			Year:     ev.Year,
			Runner:   parseName(runnerName),
			Bib:      bib,
		})
	}
	return out
}

// liRunnerName finds the text of the <h4 class="...type-fullname..."> anchor in a li.
func liRunnerName(li *html.Node) string {
	var find func(*html.Node) string
	find = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "h4" {
			cls := ""
			for _, a := range n.Attr {
				if a.Key == "class" {
					cls = a.Val
				}
			}
			if containsToken(cls, "type-fullname") {
				// Return text of the first <a> child, or direct text content.
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "a" {
						return strings.TrimSpace(textContent(c))
					}
				}
				return strings.TrimSpace(textContent(n))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := find(c); v != "" {
				return v
			}
		}
		return ""
	}
	return find(li)
}

// liBib finds the bib number from the first type-field div in a li.
// The bib text is a direct text node sibling of the inner label div.
func liBib(li *html.Node) string {
	var find func(*html.Node) string
	find = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "div" {
			cls := ""
			for _, a := range n.Attr {
				if a.Key == "class" {
					cls = a.Val
				}
			}
			if containsToken(cls, "type-field") && !containsToken(cls, "list-group-header") {
				// Collect direct text-node content (skipping child element text).
				var sb strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						sb.WriteString(c.Data)
					}
				}
				v := strings.TrimSpace(sb.String())
				if v != "" {
					return v
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := find(c); v != "" {
				return v
			}
		}
		return ""
	}
	return find(li)
}

// idpFromResultList walks the HTML tree, finds the first <ul> whose class
// contains "list-group" but not "nav" (to skip navigation menus), then
// searches its descendants for the first <a> whose href contains
// "content=detail" and returns the idp query-param value from that href.
func idpFromResultList(doc *html.Node) (string, bool) {
	// findResultUL returns the first <ul> whose class contains "list-group"
	// but is neither a nav menu (class "nav") nor the info bar
	// (class "list-info"). The runner results live in a ul that additionally
	// carries "list-group-multicolumn" or similar, but any ul.list-group
	// that is not nav/list-info and contains a content=detail anchor is
	// acceptable, so we look for the first one that yields a hit.
	var findResultUL func(*html.Node) *html.Node
	findResultUL = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "ul" {
			cls := ""
			for _, a := range n.Attr {
				if a.Key == "class" {
					cls = a.Val
				}
			}
			if containsToken(cls, "list-group") &&
				!containsToken(cls, "nav") &&
				!containsToken(cls, "list-info") {
				return n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if found := findResultUL(c); found != nil {
				return found
			}
		}
		return nil
	}

	ul := findResultUL(doc)
	if ul == nil {
		return "", false
	}

	// findDetailAnchor searches the subtree for the first <a> with content=detail.
	var findDetailAnchor func(*html.Node) string
	findDetailAnchor = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" && strings.Contains(a.Val, "content=detail") {
					// Parse the idp param out of the href.
					// The href may be relative (e.g. "?content=detail&idp=ABC").
					// Use a dummy base so url.Parse handles it.
					u, err := url.Parse("http://x" + a.Val)
					if err == nil {
						if idp := u.Query().Get("idp"); idp != "" {
							return idp
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := findDetailAnchor(c); v != "" {
				return v
			}
		}
		return ""
	}

	idp := findDetailAnchor(ul)
	if idp == "" {
		return "", false
	}
	return idp, true
}

// tdText walks the HTML tree and returns the trimmed text of the first <td>
// whose class attribute contains the given token.
func tdText(n *html.Node, classToken string) string {
	if n.Type == html.ElementNode && n.Data == "td" {
		for _, a := range n.Attr {
			if a.Key == "class" && containsToken(a.Val, classToken) {
				return strings.TrimSpace(textContent(n))
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if v := tdText(c, classToken); v != "" {
			return v
		}
	}
	return ""
}

// textContent returns the concatenated text of all text nodes under n.
func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

// containsToken reports whether s contains tok as a whitespace-separated token.
func containsToken(s, tok string) bool {
	for _, t := range strings.Fields(s) {
		if t == tok {
			return true
		}
	}
	return false
}

// parseName converts "Last, First (Country)" to "First Last".
// Falls back to the raw string if the format is not recognised.
func parseName(raw string) string {
	// Strip parenthetical country suffix, e.g. " (GER)".
	if idx := strings.LastIndex(raw, "("); idx >= 0 {
		raw = strings.TrimSpace(raw[:idx])
	}
	// Split on first comma.
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) == 2 {
		last := strings.TrimSpace(parts[0])
		first := strings.TrimSpace(parts[1])
		return first + " " + last
	}
	return raw
}

// parsePlace strips non-digit characters (commas, spaces) and converts to int.
func parsePlace(s string) int {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return 0
	}
	v, _ := strconv.Atoi(b.String())
	return v
}
