package craigslist

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// SitemapURL is one <url><loc> entry from a date+cat sitemap. Each entry is a single
// listing detail URL.
type SitemapURL struct {
	Loc string `xml:"loc"`
}

type sitemapURLSet struct {
	URLs []SitemapURL `xml:"url"`
}

// FreshListings reads `/sitemap/date/<YYYY-MM-DD>/cat/<cat>/sitemap.xml` for the
// given site (e.g. "sfbay") and returns every listing URL posted that day in that
// top-level meta-category. Use category abbrs `bbb` (community/services), `ccc`,
// `eee`, `ggg`, `hhh` (housing), `jjj` (jobs), `rrr` (resumes), `sss` (for sale).
func (c *Client) FreshListings(ctx context.Context, site, dateYMD, metaCat string) ([]SitemapURL, error) {
	if site == "" || dateYMD == "" || metaCat == "" {
		return nil, fmt.Errorf("FreshListings: site, dateYMD, metaCat all required")
	}
	host := fmt.Sprintf("https://%s.craigslist.org", site)
	path := fmt.Sprintf("/sitemap/date/%s/cat/%s/sitemap.xml", dateYMD, metaCat)
	body, err := c.RawGet(ctx, host, path, nil)
	if err != nil {
		return nil, err
	}
	var urlset sitemapURLSet
	if err := xml.Unmarshal(body, &urlset); err != nil {
		return nil, fmt.Errorf("decode sitemap: %w", err)
	}
	return urlset.URLs, nil
}

// FreshListingsWindow walks back N days from `since` and aggregates listings.
// Caller is responsible for deduping by URL across days.
func (c *Client) FreshListingsWindow(ctx context.Context, site, metaCat string, since time.Duration) ([]SitemapURL, error) {
	now := time.Now().UTC()
	startDay := now.Add(-since)
	var all []SitemapURL
	seen := make(map[string]bool)
	for d := startDay; !d.After(now); d = d.Add(24 * time.Hour) {
		ymd := d.Format("2006-01-02")
		urls, err := c.FreshListings(ctx, site, ymd, metaCat)
		if err != nil {
			return all, err
		}
		for _, u := range urls {
			if !seen[u.Loc] {
				seen[u.Loc] = true
				all = append(all, u)
			}
		}
	}
	return all, nil
}

// MetaCategoryFromAbbr maps a fine-grained category abbreviation (e.g. "apa", "sof")
// to its top-level meta-category sitemap key (e.g. "hhh", "jjj"). The mapping mirrors
// Craigslist's own type-letter convention from reference.craigslist.org/Categories.
func MetaCategoryFromAbbr(abbr string) string {
	abbr = strings.ToLower(strings.TrimSpace(abbr))
	switch {
	case abbr == "" || abbr == "sss" || abbr == "for":
		return "sss"
	case abbr == "apa" || abbr == "hou" || abbr == "roo" || abbr == "sha" ||
		abbr == "sub" || abbr == "sbw" || abbr == "off" || abbr == "prk" || abbr == "swp":
		return "hhh"
	case abbr == "sof" || abbr == "web" || abbr == "bus" || abbr == "mar" ||
		abbr == "acc" || abbr == "ofc" || abbr == "med" || abbr == "hea" ||
		abbr == "ret" || abbr == "npo" || abbr == "lgl" || abbr == "egr" ||
		abbr == "sls" || abbr == "sad" || abbr == "tfr" || abbr == "hum" ||
		abbr == "tch" || abbr == "edu" || abbr == "trd" || abbr == "gov" ||
		abbr == "sci":
		return "jjj"
	case abbr == "res":
		return "rrr"
	case abbr == "eve" || abbr == "cls":
		return "eee"
	case abbr == "com" || abbr == "vol" || abbr == "act" || abbr == "rid" ||
		abbr == "pet" || abbr == "kid" || abbr == "mis" || abbr == "ats" ||
		abbr == "muc":
		return "ccc"
	case abbr == "cps" || abbr == "crs" || abbr == "evs" || abbr == "hss" ||
		abbr == "lss" || abbr == "lbs" || abbr == "biz":
		return "bbb"
	default:
		// Most for-sale subcategories (cta, ele, fua, mob, etc.) roll up to sss.
		return "sss"
	}
}
