// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/xml"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// collectRow is one headline as `collect` prints it. url is always the
// publisher's own link: a Techmeme redirect (/goto/, /r/ and the like) or
// any other techmeme.com link is never used, and a row without a publisher
// link is dropped. published_at is null on dated pages, which show no
// times; date (the day in the item's Techmeme id) is always set.
type collectRow struct {
	ID                string  `json:"id"`
	Title             string  `json:"title"`
	Publication       string  `json:"publication"`
	URL               string  `json:"url"`
	TechmemePermalink string  `json:"techmeme_permalink"`
	PublishedAt       *string `json:"published_at"`
	Date              string  `json:"date"`
}

var (
	// An item id such as 261121p9: the day (YYMMDD), then p and a number.
	itemIDRe = regexp.MustCompile(`^(\d{6})(p\d+)$`)
	// The publication link: <A HREF="https://www.ft.com/">Financial Times</A>:
	pubLinkRe = regexp.MustCompile(`(?i)<a href="[^"]*">([^<]+)</a>\s*:`)
	// A <cite> with no link: <cite>Bloomberg:</cite>
	plainCiteRe = regexp.MustCompile(`(?is)<cite>\s*([^<]+?)\s*:?\s*</cite>`)

	riverDateRe = regexp.MustCompile(`<H2>([^<]+)</H2>`)
	riverRowRe  = regexp.MustCompile(`(?s)<tr class="ritem"><td>([^<]*?)&nbsp;.*?</tr>`)
	riverPmlRe  = regexp.MustCompile(`pml="([0-9a-z]+)"`)
	riverCiteRe = regexp.MustCompile(`(?is)<cite>.*?</cite>`)
	riverLinkRe = regexp.MustCompile(`(?is)</cite>&nbsp;\s*<a href="([^"]+)"[^>]*>(.*?)</a>`)

	dayItemRe  = regexp.MustCompile(`<DIV CLASS="itc2" ID="([0-9a-z]+)">`)
	dayHeadRe  = regexp.MustCompile(`(?is)<A CLASS="ourh" HREF="([^"]+)">(.*?)</A>`)
	rssBoldRe  = regexp.MustCompile(`(?is)<B><A HREF="([^"]+)">`)
	rssSuffix  = regexp.MustCompile(`\s*\(([^()]*)\)\s*$`)
	tagRe      = regexp.MustCompile(`<[^>]+>`)
	techmemeET = loadEastern()
)

// Techmeme dates its pages in US Eastern time.
func loadEastern() *time.Location {
	if loc, err := time.LoadLocation("America/New_York"); err == nil {
		return loc
	}
	return time.FixedZone("EST", -5*3600)
}

// itemDay turns an item id into its ISO date and its permalink.
func itemDay(id string) (date, permalink string, ok bool) {
	m := itemIDRe.FindStringSubmatch(id)
	if m == nil {
		return "", "", false
	}
	d, err := time.Parse("060102", m[1])
	if err != nil {
		return "", "", false
	}
	return d.Format("2006-01-02"), fmt.Sprintf("https://www.techmeme.com/%s/%s#a%s", m[1], m[2], id), true
}

// publisherURL returns u when it is an absolute http(s) link off
// techmeme.com, else "".
func publisherURL(u string) string {
	u = html.UnescapeString(strings.TrimSpace(u))
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
		return ""
	}
	host := strings.ToLower(p.Hostname())
	if host == "techmeme.com" || strings.HasSuffix(host, ".techmeme.com") {
		return ""
	}
	return u
}

func cleanText(s string) string {
	s = html.UnescapeString(tagRe.ReplaceAllString(s, ""))
	return strings.Join(strings.Fields(s), " ")
}

// publicationIn reads the publication name from a cite or RSS description.
func publicationIn(fragment string) string {
	if m := pubLinkRe.FindStringSubmatch(fragment); m != nil {
		return cleanText(m[1])
	}
	if m := plainCiteRe.FindStringSubmatch(fragment); m != nil {
		name := cleanText(m[1])
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = strings.TrimSpace(name[i+1:])
		}
		return name
	}
	return ""
}

// newRow fills a row's id-derived fields; ok is false for a malformed id.
func newRow(id, title, pub, link string, published *string) (collectRow, bool) {
	date, permalink, ok := itemDay(id)
	link = publisherURL(link)
	title = cleanText(title)
	if !ok || link == "" || title == "" {
		return collectRow{}, false
	}
	return collectRow{ID: id, Title: title, Publication: pub, URL: link, TechmemePermalink: permalink, PublishedAt: published, Date: date}, true
}

// parseRiver reads /river: date headers, then one row per headline with
// its Eastern time of day.
func parseRiver(page []byte) []collectRow {
	s := string(page)
	dates := riverDateRe.FindAllStringSubmatchIndex(s, -1)
	var rows []collectRow
	for _, m := range riverRowRe.FindAllStringSubmatchIndex(s, -1) {
		row := s[m[0]:m[1]]
		day := ""
		for _, d := range dates {
			if d[0] < m[0] {
				day = s[d[2]:d[3]]
			}
		}
		pml := riverPmlRe.FindStringSubmatch(row)
		link := riverLinkRe.FindStringSubmatch(row)
		if pml == nil || link == nil {
			continue
		}
		var published *string
		if t, err := time.ParseInLocation("January 2, 2006 3:04 PM", strings.TrimSpace(day)+" "+strings.TrimSpace(s[m[2]:m[3]]), techmemeET); err == nil {
			v := t.UTC().Format(time.RFC3339)
			published = &v
		}
		pub := publicationIn(riverCiteRe.FindString(row))
		if r, ok := newRow(pml[1], link[2], pub, link[1], published); ok {
			rows = append(rows, r)
		}
	}
	return rows
}

// parseDay reads a dated snapshot page (/YYMMDD/h2000). It shows no times,
// so published_at is null.
func parseDay(page []byte) []collectRow {
	s := string(page)
	starts := dayItemRe.FindAllStringSubmatchIndex(s, -1)
	var rows []collectRow
	for i, m := range starts {
		end := len(s)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := s[m[1]:end]
		head := dayHeadRe.FindStringSubmatchIndex(block)
		if head == nil {
			continue
		}
		pub := publicationIn(riverCiteRe.FindString(block[:head[0]]))
		if r, ok := newRow(s[m[2]:m[3]], block[head[4]:head[5]], pub, block[head[2]:head[3]], nil); ok {
			rows = append(rows, r)
		}
	}
	return rows
}

type rssDoc struct {
	Items []struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		PubDate     string `xml:"pubDate"`
		GUID        string `xml:"guid"`
	} `xml:"channel>item"`
}

// parseRSS reads /feed.xml. Its titles end in "(Publication)" or
// "(Author/Publication)", which is stripped; the publication and the
// publisher link come from the item's description.
func parseRSS(page []byte) ([]collectRow, error) {
	var doc rssDoc
	dec := xml.NewDecoder(strings.NewReader(string(page)))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("reading the RSS feed: %w", err)
	}
	var rows []collectRow
	for _, it := range doc.Items {
		permalink := strings.TrimSpace(firstNonEmptyStr(it.GUID, it.Link))
		id := ""
		if i := strings.LastIndex(permalink, "#a"); i >= 0 {
			id = permalink[i+2:]
		}
		title := strings.TrimSpace(it.Title)
		suffix := ""
		if m := rssSuffix.FindStringSubmatchIndex(title); m != nil {
			suffix = title[m[2]:m[3]]
			title = strings.TrimSpace(title[:m[0]])
		}
		pub := publicationIn(it.Description)
		if pub == "" {
			pub = strings.TrimSpace(suffix[strings.LastIndex(suffix, "/")+1:])
		}
		link := ""
		if m := rssBoldRe.FindStringSubmatch(it.Description); m != nil {
			link = m[1]
		}
		var published *string
		if t, err := time.Parse(time.RFC1123Z, strings.TrimSpace(it.PubDate)); err == nil {
			v := t.UTC().Format(time.RFC3339)
			published = &v
		}
		if r, ok := newRow(id, title, pub, link, published); ok {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
