package drudge

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	topLeftStartRE = regexp.MustCompile(`(?is)<!\s*TOP\s+LEFT\s+STARTS\s+HERE\s*>`)
	topLeftEndRE   = regexp.MustCompile(`(?is)<!\s*TOP\s+LEFT\s+HEADLINES\s+END\s+HERE\s*>`)
	mainStartRE    = regexp.MustCompile(`(?is)<!\s*MAIN\s+HEADLINE\s*>`)
	mainEndRE      = regexp.MustCompile(`(?is)<!\s*MAIN\s+HEADLINE\s+END\s+HERE\s*>`)
	firstColumnRE  = regexp.MustCompile(`(?is)<!\s*LINKS\s+FIRST\s+C\s*O\s*L\s*U\s*M\s*N\s*>`)
	secondColumnRE = regexp.MustCompile(`(?is)<!\s*LINKS\s+SECOND\s+C\s*O\s*L\s*U\s*M\s*N\s*>`)

	anchorRE      = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*['"]([^'"]+)['"][^>]*>(.*?)</a>`)
	tagRE         = regexp.MustCompile(`(?is)<[^>]+>`)
	redRE         = regexp.MustCompile(`(?is)color\s*=\s*['"]?red['"]?|color\s*:\s*red\b`)
	imgRE         = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc\s*=\s*['"]([^'"]+)['"][^>]*>`)
	rssPositionRE = regexp.MustCompile(`(?is)\(([A-Za-z ]+),\s*(\d+)(?:st|nd|rd|th)\s+story,`)
	// Go's RE2 doesn't support Perl-style lookahead, so we match an <img> tag
	// with class="story-image" by capturing the full tag and then locating the
	// src attribute inside it. Two alternatives cover class-before-src and
	// src-before-class orderings.
	rssImageRE   = regexp.MustCompile(`(?is)<img\b[^>]*\bclass\s*=\s*['"][^'"]*\bstory-image\b[^'"]*['"][^>]*\bsrc\s*=\s*['"]([^'"]+)['"][^>]*>|<img\b[^>]*\bsrc\s*=\s*['"]([^'"]+)['"][^>]*\bclass\s*=\s*['"][^'"]*\bstory-image\b[^'"]*['"][^>]*>`)
	allCapsShort = regexp.MustCompile(`^[A-Z]{1,5}$`)
)

// ParseHTML parses Drudge Report HTML into story records.
func ParseHTML(body []byte) ([]Story, error) {
	zones := []struct {
		slot       Slot
		startRE    *regexp.Regexp
		endRE      *regexp.Regexp
		afterStart bool
	}{
		{slot: SlotSplash, startRE: mainStartRE, endRE: mainEndRE},
		{slot: SlotTopLeft, startRE: topLeftStartRE, endRE: topLeftEndRE},
		{slot: SlotColumn1, startRE: firstColumnRE, endRE: secondColumnRE},
		{slot: SlotColumn2, startRE: secondColumnRE, afterStart: true},
	}

	foundZone := false
	var stories []Story
	for _, z := range zones {
		zone, ok := extractZone(body, z.startRE, z.endRE, z.afterStart)
		if !ok {
			continue
		}
		foundZone = true
		stories = append(stories, parseHTMLZone(zone, z.slot)...)
	}
	if !foundZone {
		return nil, fmt.Errorf("no recognized Drudge layout zones found")
	}
	return stories, nil
}

// ParseRSS parses Drudge Report RSS XML into story records.
func ParseRSS(body []byte) ([]Story, error) {
	var feed struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			GUID        string `xml:"guid"`
			Description string `xml:"description"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode RSS: %w", err)
	}

	stories := make([]Story, 0, len(feed.Items))
	for _, item := range feed.Items {
		title := cleanText(item.Title)
		storyURL := strings.TrimSpace(item.GUID)
		if storyURL == "" || strings.Contains(strings.ToLower(storyURL), "feedpress.me") {
			storyURL = strings.TrimSpace(item.Link)
		}
		if title == "" || storyURL == "" {
			continue
		}

		capturedAt := time.Now().UTC()
		if item.PubDate != "" {
			if parsed, err := time.Parse(time.RFC1123, strings.TrimSpace(item.PubDate)); err == nil {
				capturedAt = parsed.UTC()
			}
		}

		slot, slotIndex := rssSlot(item.Description)
		imageURL := ""
		if m := rssImageRE.FindStringSubmatch(item.Description); len(m) >= 3 {
			// Alternation: capture group 1 = class-before-src, group 2 = src-before-class.
			cap := m[1]
			if cap == "" {
				cap = m[2]
			}
			imageURL = html.UnescapeString(strings.TrimSpace(cap))
		}

		stories = append(stories, Story{
			StoryID:        StoryIDFromTitleURL(title, storyURL),
			Title:          title,
			URL:            storyURL,
			Slot:           slot,
			SlotIndex:      slotIndex,
			IsRed:          redRE.MatchString(item.Description),
			HasImage:       imageURL != "",
			ImageURL:       imageURL,
			OutboundDomain: outboundDomain(storyURL),
			CapturedAt:     capturedAt,
		})
	}
	return stories, nil
}

func extractZone(body []byte, startRE, endRE *regexp.Regexp, afterStart bool) ([]byte, bool) {
	start := startRE.FindIndex(body)
	if start == nil {
		return nil, false
	}
	zoneStart := start[0]
	if afterStart {
		zoneStart = start[1]
	}
	zoneEnd := len(body)
	if endRE != nil {
		if end := endRE.FindIndex(body[zoneStart:]); end != nil {
			zoneEnd = zoneStart + end[0]
		}
	}
	return body[zoneStart:zoneEnd], true
}

func parseHTMLZone(zone []byte, slot Slot) []Story {
	matches := anchorRE.FindAllSubmatchIndex(zone, -1)
	stories := make([]Story, 0, len(matches))
	capturedAt := time.Now().UTC()
	for _, match := range matches {
		rawURL := html.UnescapeString(strings.TrimSpace(string(zone[match[2]:match[3]])))
		inner := zone[match[4]:match[5]]
		title := cleanText(string(inner))
		if title == "" || skipAnchor(rawURL, title) {
			continue
		}

		beforeStart := match[0] - 300
		if beforeStart < 0 {
			beforeStart = 0
		}
		imageURL := ""
		beforeImage := zone[beforeStart:match[0]]
		imgs := imgRE.FindAllSubmatch(beforeImage, -1)
		if len(imgs) > 0 && len(imgs[len(imgs)-1]) == 2 {
			imageURL = html.UnescapeString(strings.TrimSpace(string(imgs[len(imgs)-1][1])))
		}

		redStart := match[0] - 200
		if redStart < 0 {
			redStart = 0
		}
		redContext := append(bytes.Clone(zone[redStart:match[0]]), inner...)

		stories = append(stories, Story{
			StoryID:        StoryIDFromTitleURL(title, rawURL),
			Title:          title,
			URL:            rawURL,
			Slot:           slot,
			SlotIndex:      len(stories),
			IsRed:          redRE.Match(redContext),
			HasImage:       imageURL != "",
			ImageURL:       imageURL,
			OutboundDomain: outboundDomain(rawURL),
			CapturedAt:     capturedAt,
		})
	}
	return stories
}

func cleanText(s string) string {
	stripped := tagRE.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(html.UnescapeString(stripped)), " ")
}

func skipAnchor(rawURL, title string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	host := ""
	if err == nil {
		host = strings.ToLower(u.Hostname())
	}
	if strings.HasSuffix(host, "drudgereport.com") || strings.HasSuffix(host, "drudgereportarchives.com") {
		return true
	}
	return allCapsShort.MatchString(strings.TrimSpace(title))
}

func outboundDomain(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func rssSlot(description string) (Slot, int) {
	m := rssPositionRE.FindStringSubmatch(description)
	if len(m) != 3 {
		return SlotColumn2, 0
	}
	ordinal, err := strconv.Atoi(m[2])
	if err != nil || ordinal < 1 {
		ordinal = 1
	}
	switch strings.ToLower(strings.Join(strings.Fields(m[1]), " ")) {
	case "main headline":
		return SlotSplash, ordinal - 1
	case "first column":
		return SlotColumn1, ordinal - 1
	case "second column":
		return SlotColumn2, ordinal - 1
	case "top left":
		return SlotTopLeft, ordinal - 1
	default:
		return SlotColumn2, ordinal - 1
	}
}
