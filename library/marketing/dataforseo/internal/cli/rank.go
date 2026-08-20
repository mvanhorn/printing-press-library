// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
	"github.com/spf13/cobra"
)

const rankTrackSchema = `CREATE TABLE IF NOT EXISTS rank_track (
  url TEXT,
  target_keyword TEXT,
  position INTEGER,
  serp_features TEXT,
  checked_at TIMESTAMP,
  PRIMARY KEY (url, checked_at)
)`

type rankRow struct {
	URL           string   `json:"url"`
	TargetKeyword string   `json:"target_keyword"`
	Position      int      `json:"position"`
	Delta         int      `json:"delta"`
	SerpFeatures  []string `json:"serp_features"`
	NewFeatures   []string `json:"new_features"`
	LostFeatures  []string `json:"lost_features"`
}

func newRankCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Track SERP rank + features over a URL set",
	}
	cmd.AddCommand(newRankTrackCmd(flags))
	return cmd
}

func newRankTrackCmd(flags *rootFlags) *cobra.Command {
	var sitemapURL string
	var mapCSV string
	var includeFeatures bool

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Diff position + SERP features against the last run for each URL/keyword pair",
		Long: `Parses a sitemap (or accepts a CSV mapping of url,keyword) and queries
serp/google/organic/live/advanced for each pair. The result row position and
SERP features are diffed against the last stored run in local SQLite.

When no keyword map is supplied, the last path slug is converted to spaces
and used as the target keyword (e.g. /tree-service-orlando/ -> "tree service orlando").`,
		Example: strings.Trim(`
  dataforseo-pp-cli rank track --sitemap https://floridatreemen.com/sitemap.xml
  dataforseo-pp-cli rank track --map /tmp/url-kw.csv --features --json
`, "\n"),
		// PATCH: Keep caller-selected --map file reads off the Cobra-to-MCP mirror.
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would track rank for sitemap/map input")
				return nil
			}
			if sitemapURL == "" && mapCSV == "" {
				return usageErr(fmt.Errorf("either --sitemap or --map is required"))
			}

			pairs, err := loadURLKeywordPairs(sitemapURL, mapCSV)
			if err != nil {
				return err
			}
			if len(pairs) == 0 {
				return usageErr(fmt.Errorf("no url/keyword pairs found"))
			}

			ctx := context.Background()
			s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if _, err := s.DB().Exec(rankTrackSchema); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			out := make([]rankRow, 0, len(pairs))
			for _, p := range pairs {
				body := []map[string]any{{
					"keyword":       p.keyword,
					"language_code": "en",
					"location_code": 2840, // US
				}}
				respRaw, _, err := c.Post("/v3/serp/google/organic/live/advanced", body)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s (%s): %v\n", p.url, p.keyword, err)
					continue
				}
				pos, feats := extractRankAndFeatures(respRaw, p.url)
				prevPos, prevFeats := loadPrevRank(s, p.url)

				row := rankRow{
					URL:           p.url,
					TargetKeyword: p.keyword,
					Position:      pos,
					SerpFeatures:  feats,
				}
				if prevPos > 0 && pos > 0 {
					row.Delta = pos - prevPos
				}
				if includeFeatures {
					row.NewFeatures = diffStrings(feats, prevFeats)
					row.LostFeatures = diffStrings(prevFeats, feats)
				}
				out = append(out, row)

				featsJSON, _ := json.Marshal(feats)
				_, _ = s.DB().Exec(
					`INSERT OR REPLACE INTO rank_track (url, target_keyword, position, serp_features, checked_at) VALUES (?, ?, ?, ?, ?)`,
					p.url, p.keyword, pos, string(featsJSON), time.Now().UTC().Format(time.RFC3339),
				)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&sitemapURL, "sitemap", "", "URL of an XML sitemap to parse")
	cmd.Flags().StringVar(&mapCSV, "map", "", "CSV file of url,keyword pairs (one per line)")
	cmd.Flags().BoolVar(&includeFeatures, "features", false, "Include new/lost SERP features in the diff")
	return cmd
}

type urlKeywordPair struct {
	url     string
	keyword string
}

func loadURLKeywordPairs(sitemap, mapCSV string) ([]urlKeywordPair, error) {
	if mapCSV != "" {
		f, err := os.Open(mapCSV)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var out []urlKeywordPair
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) == 2 {
				out = append(out, urlKeywordPair{
					url:     strings.TrimSpace(parts[0]),
					keyword: strings.TrimSpace(parts[1]),
				})
			}
		}
		return out, scanner.Err()
	}
	return loadSitemapPairs(newSafeSitemapClient(nil, nil), sitemap)
}

type sitemapLookupIP func(context.Context, string) ([]net.IPAddr, error)
type sitemapDialContext func(context.Context, string, string) (net.Conn, error)

var nonPublicSitemapNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

// PATCH: Sitemap HTTP connects only to publicly routable addresses and repeats the check after redirects and DNS changes.
func newSafeSitemapClient(lookup sitemapLookupIP, dial sitemapDialContext) *http.Client {
	if lookup == nil {
		lookup = net.DefaultResolver.LookupIPAddr
	}
	if dial == nil {
		d := &net.Dialer{}
		dial = d.DialContext
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid sitemap address %q: %w", address, err)
			}
			ips, err := resolveSitemapHost(ctx, lookup, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if !isPublicSitemapIP(ip.IP) {
					return nil, fmt.Errorf("sitemap host %q resolved to %s, which is not publicly routable", host, ip.IP)
				}
			}
			var lastErr error
			for _, ip := range ips {
				conn, err := dial(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, fmt.Errorf("connecting to sitemap host %q: %w", host, lastErr)
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many sitemap redirects")
			}
			return validateSitemapURL(req.URL)
		},
	}
}

func resolveSitemapHost(ctx context.Context, lookup sitemapLookupIP, host string) ([]net.IPAddr, error) {
	if parsed := net.ParseIP(host); parsed != nil {
		return []net.IPAddr{{IP: parsed}}, nil
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolving sitemap host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("sitemap host %q resolved to no addresses", host)
	}
	return ips, nil
}

func isPublicSitemapIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() {
		return false
	}
	for _, blocked := range nonPublicSitemapNetworks {
		if blocked.Contains(addr) {
			return false
		}
	}
	return true
}

func validateSitemapURL(target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") {
		return fmt.Errorf("sitemap URL must use http or https")
	}
	if target.Hostname() == "" {
		return fmt.Errorf("sitemap URL must include a host")
	}
	if target.User != nil {
		return fmt.Errorf("sitemap URL must not include credentials")
	}
	return nil
}

func loadSitemapPairs(client *http.Client, sitemap string) ([]urlKeywordPair, error) {
	target, err := url.Parse(sitemap)
	if err != nil {
		return nil, fmt.Errorf("parsing sitemap URL: %w", err)
	}
	if err := validateSitemapURL(target); err != nil {
		return nil, err
	}
	resp, err := client.Get(target.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching sitemap returned HTTP %d", resp.StatusCode)
	}
	dec := xml.NewDecoder(resp.Body)
	var urls []string
	var inLoc bool
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing sitemap XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "loc" {
				inLoc = true
			}
		case xml.EndElement:
			if t.Name.Local == "loc" {
				inLoc = false
			}
		case xml.CharData:
			if inLoc {
				u := strings.TrimSpace(string(t))
				if u != "" {
					urls = append(urls, u)
				}
			}
		}
	}
	out := make([]urlKeywordPair, 0, len(urls))
	for _, u := range urls {
		out = append(out, urlKeywordPair{url: u, keyword: inferKeywordFromURL(u)})
	}
	return out, nil
}

func inferKeywordFromURL(u string) string {
	trimmed := strings.TrimRight(u, "/")
	idx := strings.LastIndex(trimmed, "/")
	slug := trimmed
	if idx >= 0 {
		slug = trimmed[idx+1:]
	}
	slug = strings.ReplaceAll(slug, "-", " ")
	slug = strings.ReplaceAll(slug, "_", " ")
	return strings.TrimSpace(slug)
}

func extractRankAndFeatures(raw json.RawMessage, targetURL string) (int, []string) {
	var resp struct {
		Tasks []struct {
			Result []struct {
				Items []map[string]any `json:"items"`
			} `json:"result"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, nil
	}
	features := map[string]bool{}
	pos := 0
	for _, t := range resp.Tasks {
		for _, r := range t.Result {
			for _, item := range r.Items {
				if typ, ok := item["type"].(string); ok && typ != "" {
					features[typ] = true
				}
				if pos == 0 {
					if u, ok := item["url"].(string); ok && urlMatches(u, targetURL) {
						if rp, ok := item["rank_absolute"].(float64); ok {
							pos = int(rp)
						}
					}
				}
			}
		}
	}
	keys := make([]string, 0, len(features))
	for k := range features {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return pos, keys
}

func urlMatches(a, b string) bool {
	return strings.TrimRight(a, "/") == strings.TrimRight(b, "/")
}

func loadPrevRank(s *store.Store, url string) (int, []string) {
	row := s.DB().QueryRow(
		`SELECT position, serp_features FROM rank_track WHERE url = ? ORDER BY checked_at DESC LIMIT 1`,
		url,
	)
	var pos int
	var featsJSON string
	if err := row.Scan(&pos, &featsJSON); err != nil {
		return 0, nil
	}
	var feats []string
	_ = json.Unmarshal([]byte(featsJSON), &feats)
	return pos, feats
}

func diffStrings(a, b []string) []string {
	bset := map[string]bool{}
	for _, x := range b {
		bset[x] = true
	}
	out := []string{}
	for _, x := range a {
		if !bset[x] {
			out = append(out, x)
		}
	}
	return out
}
