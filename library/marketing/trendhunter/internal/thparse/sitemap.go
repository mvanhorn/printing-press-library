package thparse

import (
	"bytes"
	"encoding/xml"
	"net/url"
	"strings"
)

type sitemapDoc struct {
	URLs []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

func ParseSitemap(body []byte) ([]SitemapEntry, error) {
	var doc sitemapDoc
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&doc); err != nil {
		return nil, err
	}
	out := make([]SitemapEntry, 0, len(doc.URLs))
	for _, item := range doc.URLs {
		loc := strings.TrimSpace(item.Loc)
		out = append(out, SitemapEntry{
			URL:     loc,
			LastMod: strings.TrimSpace(item.LastMod),
			Kind:    sitemapKind(loc),
		})
	}
	return out, nil
}

func sitemapKind(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "other"
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 2 {
		switch parts[0] {
		case "trends":
			return "trend"
		case "megatrend":
			return "megatrend"
		case "pattern":
			return "pattern"
		case "futurist":
			return "futurist"
		case "protrends":
			return "protrends"
		}
	}
	if len(parts) == 1 && parts[0] != "" {
		if _, ok := categorySet[parts[0]]; ok {
			return "category"
		}
	}
	return "other"
}
