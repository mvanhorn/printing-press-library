package activityjapan

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/cliutil"
)

const sitemapMaxBody = 2 << 20
const sitemapMaxXML = 16 << 20
const sitemapTTL = 24 * time.Hour

type LanguagePresence struct {
	PlanID          string         `json:"plan_id"`
	EnglishIndexed  bool           `json:"english_indexed"`
	JapaneseIndexed bool           `json:"japanese_indexed"`
	EnglishChecked  bool           `json:"english_checked"`
	JapaneseChecked bool           `json:"japanese_checked"`
	EnglishCount    int            `json:"english_plan_url_count"`
	JapaneseCount   int            `json:"japanese_plan_url_count"`
	EnglishURL      *string        `json:"english_url"`
	JapaneseURL     *string        `json:"japanese_url"`
	Basis           string         `json:"basis"`
	ObservedAt      string         `json:"observed_at"`
	Cache           map[string]any `json:"cache"`
	Partial         bool           `json:"partial"`
	Errors          []string       `json:"errors"`
}
type sitemapClient struct {
	http     *http.Client
	limiter  *cliutil.AdaptiveLimiter
	cacheDir string
	noCache  bool
	refresh  bool
	requests int
}

func newSitemapClient(cacheDir string, noCache, refresh bool) *sitemapClient {
	c := &http.Client{Timeout: 10 * time.Second}
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 2 || (req.URL.Host != "en.activityjapan.com" && req.URL.Host != "activityjapan.com") {
			return errors.New("Activity Japan sitemap redirect refused")
		}
		return nil
	}
	return &sitemapClient{http: c, limiter: cliutil.NewAdaptiveLimiter(2), cacheDir: cacheDir, noCache: noCache, refresh: refresh}
}
func (c *sitemapClient) fetch(ctx context.Context, lang string) ([]byte, string, time.Duration, error) {
	if err := ValidLang(lang); err != nil {
		return nil, "", 0, err
	}
	path := filepath.Join(c.cacheDir, "sitemap-plan-"+lang+".xml.gz")
	if !c.noCache && !c.refresh {
		if st, e := os.Stat(path); e == nil && st.Size() <= sitemapMaxBody && time.Since(st.ModTime()) < sitemapTTL {
			// #nosec G304 -- locale is restricted to en/ja; the filename is fixed within the CLI cache root.
			b, e := os.ReadFile(path)
			if e == nil {
				if _, valid := parseSitemap(b); valid != nil {
					_ = os.Remove(path)
				} else {
					return b, "hit", time.Since(st.ModTime()), nil
				}
			}
		}
	}
	host := "activityjapan.com"
	if lang == "en" {
		host = "en.activityjapan.com"
	}
	endpoint := "https://" + host + "/sitemap/" + lang + "/sitemap_plan1.xml.gz"
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, "", 0, ctx.Err()
			case <-timer.C:
			}
		}
		if e := c.limiter.Wait(ctx); e != nil {
			return nil, "", 0, e
		}
		req, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if e != nil {
			return nil, "", 0, e
		}
		req.Header.Set("User-Agent", "activity-japan-pp-cli/1.0 (read-only sitemap coverage)")
		c.requests++
		resp, e := c.http.Do(req)
		if e != nil {
			last = e
			continue
		}
		if resp.StatusCode == 429 {
			c.limiter.OnRateLimit()
			last = &cliutil.RateLimitError{URL: endpoint, RetryAfter: cliutil.RetryAfter(resp)}
			if e := resp.Body.Close(); e != nil {
				return nil, "", 0, errors.Join(last, fmt.Errorf("close Activity Japan sitemap response: %w", e))
			}
			continue
		}
		if resp.StatusCode >= 500 {
			last = fmt.Errorf("Activity Japan sitemap HTTP %d", resp.StatusCode)
			if e := resp.Body.Close(); e != nil {
				return nil, "", 0, fmt.Errorf("close Activity Japan sitemap response: %w", e)
			}
			continue
		}
		if resp.StatusCode != 200 {
			last = fmt.Errorf("Activity Japan sitemap HTTP %d", resp.StatusCode)
			if e := resp.Body.Close(); e != nil {
				return nil, "", 0, fmt.Errorf("close Activity Japan sitemap response: %w", e)
			}
			break
		}
		c.limiter.OnSuccess()
		body, e := io.ReadAll(io.LimitReader(resp.Body, sitemapMaxBody+1))
		closeErr := resp.Body.Close()
		if e != nil {
			return nil, "", 0, e
		}
		if closeErr != nil {
			return nil, "", 0, fmt.Errorf("close Activity Japan sitemap response: %w", closeErr)
		}
		if len(body) > sitemapMaxBody {
			return nil, "", 0, errors.New("Activity Japan sitemap exceeds 2 MiB compressed limit")
		}
		if _, e := parseSitemap(body); e != nil {
			return nil, "", 0, fmt.Errorf("Activity Japan sitemap invalid XML: %w", e)
		}
		if !c.noCache && c.cacheDir != "" {
			if e := os.MkdirAll(c.cacheDir, 0700); e == nil {
				tmp := path + ".tmp"
				if e := os.WriteFile(tmp, body, 0600); e == nil {
					_ = os.Rename(tmp, path)
				} else {
					_ = os.Remove(tmp)
				}
			}
		}
		return body, "miss", 0, nil
	}
	return nil, "", 0, last
}
func parseSitemap(data []byte) (map[string]struct{}, error) {
	var reader io.Reader = bytes.NewReader(data)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, e := gzip.NewReader(reader)
		if e != nil {
			return nil, e
		}
		defer zr.Close()
		reader = zr
	}
	body, e := io.ReadAll(io.LimitReader(reader, sitemapMaxXML+1))
	if e != nil {
		return nil, e
	}
	if len(body) > sitemapMaxXML {
		return nil, errors.New("Activity Japan sitemap XML exceeds 16 MiB limit")
	}
	dec := xml.NewDecoder(bytes.NewReader(body))
	out := map[string]struct{}{}
	for {
		tok, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "loc" {
			continue
		}
		var loc string
		if e := dec.DecodeElement(&loc, &start); e != nil {
			return nil, e
		}
		parts := strings.Split(strings.TrimSuffix(loc, "/"), "/")
		id := parts[len(parts)-1]
		if len(parts) >= 3 && parts[len(parts)-2] == "plan" && digits.MatchString(id) {
			out[id] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("Activity Japan plan sitemap contains no plan IDs")
	}
	return out, nil
}
func CheckLanguages(ctx context.Context, id, cacheDir string, noCache, refresh bool) (LanguagePresence, int, error) {
	if e := ValidateID(id); e != nil {
		return LanguagePresence{}, 0, e
	}
	c := newSitemapClient(cacheDir, noCache, refresh)
	return checkLanguagesWithClient(ctx, id, c)
}
func checkLanguagesWithClient(ctx context.Context, id string, c *sitemapClient) (LanguagePresence, int, error) {
	out := LanguagePresence{PlanID: id, Basis: "public plan sitemap index only; not proof of bookability or instructor language", ObservedAt: ObservedAt(), Cache: map[string]any{}, Errors: []string{}}
	checked := 0
	failures := []error{}
	for _, lang := range []string{"en", "ja"} {
		body, status, age, e := c.fetch(ctx, lang)
		if e != nil {
			var rate *cliutil.RateLimitError
			if errors.As(e, &rate) {
				return out, c.requests, fmt.Errorf("%s sitemap: %w", lang, e)
			}
			out.Errors = append(out.Errors, lang+" sitemap: "+e.Error())
			failures = append(failures, e)
			out.Partial = true
			continue
		}
		ids, e := parseSitemap(body)
		if e != nil {
			out.Errors = append(out.Errors, lang+" sitemap: "+e.Error())
			failures = append(failures, e)
			out.Partial = true
			continue
		}
		checked++
		out.Cache[lang] = map[string]any{"status": status, "age_seconds": int(age.Seconds()), "ttl_seconds": int(sitemapTTL.Seconds()), "fetched_at": time.Now().Add(-age).In(tokyo).Format(time.RFC3339)}
		_, present := ids[id]
		if lang == "en" {
			out.EnglishChecked = true
			out.EnglishIndexed = present
			out.EnglishCount = len(ids)
			if present {
				u := PlanURL(id, "en")
				out.EnglishURL = &u
			}
		} else {
			out.JapaneseChecked = true
			out.JapaneseIndexed = present
			out.JapaneseCount = len(ids)
			if present {
				u := PlanURL(id, "ja")
				out.JapaneseURL = &u
			}
		}
	}
	if checked == 0 {
		return out, c.requests, fmt.Errorf("both Activity Japan plan sitemaps failed: %w", errors.Join(failures...))
	}
	return out, c.requests, nil
}
