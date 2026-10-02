// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package yamato

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/cliutil"
	"golang.org/x/net/html"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Airport struct {
	ID      string   `json:"id"`
	NameJP  string   `json:"name_jp"`
	Aliases []string `json:"aliases,omitempty"`
}

var airportAliases = map[string][]string{"001": {"New Chitose", "CTS"}, "003": {"Sendai", "SDJ"}, "002": {"Fukushima", "FKS"}, "004": {"Niigata", "KIJ"}, "005": {"Narita terminal1 north", "NRT"}, "022": {"Narita terminal1 south", "NRT"}, "006": {"Narita terminal2", "NRT"}, "034": {"Narita terminal3", "NRT"}, "008": {"Haneda terminal1 domestic", "HND"}, "019": {"Haneda terminal2 domestic", "HND"}, "038": {"Haneda terminal2 international", "HND"}, "007": {"Haneda terminal3 international", "HND"}, "009": {"Komatsu", "KMQ"}, "010": {"Chubu terminal1", "NGO"}, "035": {"Chubu terminal2", "NGO"}, "011": {"Itami", "ITM"}, "012": {"Kansai terminal1", "KIX"}, "039": {"Kobe terminal1", "UKB"}, "040": {"Kobe terminal2", "UKB"}, "020": {"Okayama", "OKJ"}, "013": {"Hiroshima", "HIJ"}, "014": {"Fukuoka international", "FUK"}, "015": {"Fukuoka domestic", "FUK"}, "018": {"Kitakyushu", "KKJ"}, "016": {"Nagasaki", "NGS"}, "017": {"Kagoshima", "KOJ"}}

func (c *Client) Airports(ctx context.Context) ([]Airport, error) {
	b, e := c.Fetch(ctx, Date+"MainSmp?LINK=KT", nil, 2<<20)
	if e != nil {
		return nil, e
	}
	doc, e := Parse(b)
	if e != nil {
		return nil, e
	}
	var aa []Airport
	seen := map[string]bool{}
	for _, s := range Nodes(doc, "select") {
		if Attr(s, "name") != "PARA_END" {
			continue
		}
		for _, o := range Nodes(s, "option") {
			id := Attr(o, "value")
			if id == "" {
				continue
			}
			if !regexp.MustCompile(`^\d{3}$`).MatchString(id) || seen[id] {
				return nil, fmt.Errorf("source airport IDs changed; open %s", Date+"MainSmp?LINK=KT")
			}
			seen[id] = true
			aa = append(aa, Airport{id, Text(o), airportAliases[id]})
		}
	}
	if len(aa) == 0 {
		return nil, fmt.Errorf("live airport selector absent; source changed")
	}
	return aa, nil
}
func Matches(q string, values ...string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	for _, s := range values {
		if strings.Contains(strings.ToLower(s), q) {
			return true
		}
	}
	return false
}

type Hours struct {
	Open        string `json:"open_jst"`
	Close       string `json:"close_jst"`
	ObservedAt  string `json:"observed_at"`
	EvidenceURL string `json:"evidence_url"`
	Status      string `json:"status"`
}
type Counter struct {
	ID                 string `json:"id"`
	Prefecture         string `json:"prefecture"`
	Name               string `json:"name"`
	Service            string `json:"service"`
	Floor              string `json:"floor"`
	MapURL             string `json:"map_url"`
	Hours              *Hours `json:"hours"`
	HoursStatus        string `json:"hours_status"`
	DispatchCutoffTime any    `json:"dispatch_cutoff_time"`
	Acceptance         string `json:"acceptance_evidence"`
}

func (c *Client) Counters(ctx context.Context, q string, limit, offset int) ([]Counter, int, error) {
	b, e := c.Fetch(ctx, CounterList, nil, 2<<20)
	if e != nil {
		return nil, 0, e
	}
	doc, e := Parse(b)
	if e != nil {
		return nil, 0, e
	}
	all := []Counter{}
	parsed := 0
	for _, table := range Nodes(doc, "table") {
		if !strings.Contains(Text(table), "Prefecture") || !strings.Contains(Text(table), "Service") {
			continue
		}
		for _, row := range expandedRows(table) {
			if len(row) != 5 || Text(row[0]) == "Prefecture" {
				continue
			}
			href := ""
			for _, a := range Nodes(row[4], "a") {
				if strings.Contains(Attr(a, "href"), "/services/airport/") {
					href = Attr(a, "href")
					break
				}
			}
			if href == "" {
				continue
			}
			u, e := url.Parse(href)
			if e != nil {
				return nil, 0, e
			}
			base, _ := url.Parse(Main)
			href = base.ResolveReference(u).String()
			id := strings.TrimSuffix(path.Base(u.Path), ".html")
			name := Text(row[1])
			parsed++
			if !Matches(q, name, id, Text(row[0]), Text(row[2])) {
				continue
			}
			all = append(all, Counter{id, Text(row[0]), name, Text(row[2]), Text(row[3]), href, nil, "unknown: image-only map; use source map", nil, "listed public service/floor, not confirmation of current parcel acceptance"})
		}
	}
	if parsed == 0 {
		return nil, 0, fmt.Errorf("airport directory table changed; no guessed counters returned")
	}
	total := len(all)
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	out := all[offset:end]
	for i := range out {
		if out[i].ID != "narita2_send" {
			continue
		}
		raw := Main + "/ytc/en/send/services/airport/image/narita2_send_img.gif"
		_, e := c.Fetch(ctx, raw, nil, 2<<20)
		if e != nil {
			var rl *cliutil.RateLimitError
			if errors.As(e, &rl) {
				return nil, 0, e
			}
			c.Failures = append(c.Failures, FetchFailure{raw, e.Error()})
			out[i].HoursStatus = "unknown: could not check image freshness: " + e.Error()
			continue
		}
		if c.Sources[len(c.Sources)-1].SHA256 == "8a0e0f87a57a9db84d2ff7e3dc64882c5db96223e081fbba598a738a2381f1f6" {
			out[i].Hours = &Hours{"06:30", "22:30", ObservedAt, raw, "current bytes match visually verified snapshot"}
			out[i].HoursStatus = "verified-image-snapshot"
		} else {
			out[i].HoursStatus = "unknown: source image changed; inspect map before visiting"
		}
	}
	return out, total, nil
}
func expandedRows(table *html.Node) [][]*html.Node {
	type span struct {
		node      *html.Node
		remaining int
	}
	carry := map[int]span{}
	var out [][]*html.Node
	for _, tr := range Nodes(table, "tr") {
		cells := map[int]*html.Node{}
		for col, s := range carry {
			cells[col] = s.node
			s.remaining--
			if s.remaining <= 0 {
				delete(carry, col)
			} else {
				carry[col] = s
			}
		}
		col := 0
		for ch := tr.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode || (ch.Data != "td" && ch.Data != "th") {
				continue
			}
			for cells[col] != nil {
				col++
			}
			cs, _ := strconv.Atoi(Attr(ch, "colspan"))
			if cs < 1 {
				cs = 1
			}
			rs, _ := strconv.Atoi(Attr(ch, "rowspan"))
			for j := 0; j < cs; j++ {
				cells[col+j] = ch
				if rs > 1 {
					carry[col+j] = span{ch, rs - 1}
				}
			}
			col += cs
		}
		var row []*html.Node
		for i := 0; i < len(cells); i++ {
			if cells[i] == nil {
				break
			}
			row = append(row, cells[i])
		}
		out = append(out, row)
	}
	return out
}
