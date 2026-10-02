// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package limousine

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func CleanHTML(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return ""
	}
	parts := []string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			parts = append(parts, n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
func SourceLinks(input string) ([]map[string]string, error) {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return nil, err
	}
	links := []map[string]string{}
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := ""
			for _, a := range n.Attr {
				if a.Key == "href" {
					href = a.Val
				}
			}
			u, e := url.Parse(href)
			if e == nil {
				base, _ := url.Parse(Origin)
				u = base.ResolveReference(u)
				if u.Scheme == "https" && (u.Hostname() == "www.limousinebus.co.jp" || strings.HasSuffix(u.Hostname(), ".limousinebus.co.jp")) && !seen[u.String()] {
					parts := []string{}
					var text func(*html.Node)
					text = func(c *html.Node) {
						if c.Type == html.TextNode {
							parts = append(parts, c.Data)
						}
						for x := c.FirstChild; x != nil; x = x.NextSibling {
							text(x)
						}
					}
					text(n)
					seen[u.String()] = true
					links = append(links, map[string]string{"url": u.String(), "title": strings.Join(strings.Fields(strings.Join(parts, " ")), " ")})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links, nil
}

type Condition struct {
	SourceLanguage  string   `json:"source_language"`
	Scope           string   `json:"scope"`
	RouteIDs        []string `json:"route_ids"`
	Topic           string   `json:"topic"`
	Key             string   `json:"key"`
	Summary         string   `json:"summary"`
	Value           any      `json:"value"`
	Unit            string   `json:"unit"`
	SourceURL       string   `json:"source_url"`
	SourceUpdatedAt string   `json:"source_updated_at"`
}

var checkedCount = regexp.MustCompile(`(?i)(?:maximum number[^.]*?is|up to)\s+(one|two|three|\d+)\s+per person`)
var sizePattern = regexp.MustCompile(`(?i)(\d+)\s*[x×]\s*(\d+)\s*[x×]\s*(\d+)\s*cm`)
var weightPattern = regexp.MustCompile(`(?i)weigh up to\s+(\d+)\s*kg`)

func Conditions(data map[string]any, topic string) ([]Condition, error) {
	content := M(data["content"])
	if content == nil || S(content["content"]) == "" {
		return nil, fmt.Errorf("provider conditions response has no content")
	}
	text := CleanHTML(S(content["content"]))
	source := PageURL("/en/guide/terms/caution", nil)
	if topic == "baggage" {
		source = PageURL("/en/guide/terms/baggage", nil)
	}
	updated := S(content["updatedAt"])
	out := []Condition{}
	add := func(key, summary string, value any, unit string) {
		scope := ""
		if topic == "baggage" {
			scope = "General English guide; route-specific and partner limits may be lower."
		}
		out = append(out, Condition{SourceLanguage: "en", Scope: scope, Topic: topic, Key: key, Summary: summary, Value: value, Unit: unit, SourceURL: source, SourceUpdatedAt: updated})
	}
	if topic == "baggage" {
		if m := checkedCount.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[1])
			if m[1] == "one" {
				n = 1
			}
			if m[1] == "two" {
				n = 2
			}
			if m[1] == "three" {
				n = 3
			}
			add("checked_bag_count", fmt.Sprintf("The general English guide lists at most %d checked pieces per person; this is not a route-specific allowance.", n), n, "pieces per person")
		}
		if m := sizePattern.FindStringSubmatch(text); m != nil {
			dims := []int{}
			for _, n := range m[1:] {
				v, _ := strconv.Atoi(n)
				dims = append(dims, v)
			}
			add("checked_bag_dimensions", fmt.Sprintf("Each checked piece must fit within %d × %d × %d cm.", dims[0], dims[1], dims[2]), dims, "cm")
		}
		if m := weightPattern.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[1])
			add("checked_bag_weight", fmt.Sprintf("Each checked piece may weigh up to %d kg.", n), n, "kg per piece")
		}
		if strings.Contains(strings.ToLower(text), "passport") {
			add("valuables", "Keep passports, valuables, fragile items and precision instruments out of checked baggage.", nil, "")
		}
		if strings.Contains(strings.ToLower(text), "jointly operated") {
			add("operator_scope", "Handling can differ on jointly operated routes; check the operating company and linked notices.", nil, "")
		}
		if strings.Contains(strings.ToLower(text), "decline to accept") {
			add("acceptance", "The operator may refuse baggage whose size or quantity disrupts service.", nil, "")
		}
	} else {
		low := strings.ToLower(text)
		if strings.Contains(low, "delays") {
			add("travel_time", "Allow extra time for traffic and road conditions; arrival times are estimates.", nil, "")
		}
		if strings.Contains(low, "standing passengers") {
			add("seated_only", "Standing passengers are not accepted; a full bus cannot take additional passengers.", nil, "")
		}
		if strings.Contains(low, "first-come") {
			add("unreserved_boarding", "Unreserved boarding is generally first come, first served; a timetable does not secure a seat.", nil, "")
		}
		if strings.Contains(low, "elementary school") {
			add("child_category", "The child category covers elementary-school students; the guide describes ages 6–12 and a school-status exception at age 12.", nil, "")
		}
		if strings.Contains(low, "under 6") && strings.Contains(low, "free of charge") {
			add("preschool_seats", "Preschoolers under six using no seat are free; those occupying a seat generally pay child fare.", nil, "")
		}
		if strings.Contains(low, "fractions less than 10") {
			add("child_rounding", "The guide describes half the adult fare, rounding fractional amounts upward to 10 yen; fare uses the actual published child unit instead.", 10, "JPY rounding unit")
		}
		if strings.Contains(low, "schedules and fares may change") {
			add("changes", "Published schedules and fares can change; confirm the dated canonical page before travel.", nil, "")
		}
		if strings.Contains(low, "other companies") {
			add("operator_scope", "These guide rules apply to this operator; jointly operated services can differ.", nil, "")
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("provider guide wording changed; no supported planning facts were recognized")
	}
	return out, nil
}

// JapaneseBaggage reads the route exception from the current Japanese guide,
// rather than treating the general English allowance as valid on every route.
func JapaneseBaggage(body string) []Condition {
	text := CleanHTML(body)
	pattern := regexp.MustCompile(`渋谷[～〜\-－]成田空港[^。]{0,120}LCB[^。]{0,120}1名様につき、\s*(\d+)個`)
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return []Condition{{SourceLanguage: "ja", Topic: "baggage", Key: "route_specific_allowance", Summary: "The Japanese route exception was not recognized; exact route baggage allowance remains unknown. Consult the linked Japanese guide.", Value: nil, Scope: "Route-specific rules unknown", SourceURL: Origin + "/ja/guide/terms/baggage/"}}
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return nil
	}
	return []Condition{{SourceLanguage: "ja", Topic: "baggage", Key: "lcb_checked_bag_count", Summary: fmt.Sprintf("The current Japanese guide limits the Shibuya–Narita LCB route to %d checked piece(s) per person.", n), Value: n, Unit: "pieces per person", Scope: "Shibuya–Narita LCB route; this overrides the general English count", RouteIDs: []string{"Narita-ShibuyaLCB"}, SourceURL: Origin + "/ja/guide/terms/baggage/"}}
}
