package thparse

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"strings"
)

type rssDoc struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

var rssImgRE = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc\s*=\s*["']([^"']*)["']`)

func ParseRSS(body []byte) ([]Trend, error) {
	var doc rssDoc
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&doc); err != nil {
		return nil, err
	}
	out := make([]Trend, 0, len(doc.Channel.Items))
	for _, item := range doc.Channel.Items {
		title := strings.TrimSuffix(strings.TrimSpace(item.Title), " (TrendHunter.com)")
		trendName := title
		desc := ""
		if before, after, ok := strings.Cut(title, " - "); ok {
			trendName = strings.TrimSpace(before)
			desc = strings.TrimSpace(after)
		}
		if desc == "" {
			desc = cleanText(item.Description)
		}
		img := ""
		if m := rssImgRE.FindStringSubmatch(item.Description); len(m) >= 2 {
			img = m[1]
		}
		out = append(out, Trend{
			Slug:        slugFromURL(item.Link),
			Title:       trendName,
			Description: desc,
			ImageURL:    img,
			PubDate:     strings.TrimSpace(item.PubDate),
			SourceURL:   strings.TrimSpace(item.Link),
			Source:      "rss",
		})
	}
	return out, nil
}
