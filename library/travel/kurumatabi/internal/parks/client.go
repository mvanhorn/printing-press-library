package parks

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func NewClient(base string, rate float64) *Client {
	if base == "" {
		base = Origin
	}
	jar, _ := cookiejar.New(nil)
	if rate <= 0 || rate > 2 {
		rate = 1
	}
	return &Client{BaseURL: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 25 * time.Second, Jar: jar}, limiter: cliutil.NewAdaptiveLimiter(rate)}
}
func (c *Client) fetch(ctx context.Context, method, path string, body url.Values) ([]byte, error) {
	if e := c.limiter.Wait(ctx); e != nil {
		return nil, e
	}
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(body.Encode())
	}
	req, e := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "kurumatabi-pp-cli/0.1 (read-only planning)")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: req.URL.String(), RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from %s; confirm the source page directly", resp.StatusCode, path)
	}
	c.limiter.OnSuccess()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 4*1024*1024 {
		return nil, fmt.Errorf("response exceeded 4 MiB bound")
	}
	return b, nil
}
func (c *Client) Detail(ctx context.Context, id string) (Park, error) {
	id, e := NormalizeID(id)
	if e != nil {
		return Park{}, e
	}
	b, e := c.fetch(ctx, "GET", "/park/"+id+".html", nil)
	if e != nil {
		return Park{}, e
	}
	return ParseDetail(id, b, time.Now())
}
func (c *Client) Search(ctx context.Context, q Query) (SearchResult, error) {
	params, e := QueryValues(q)
	if e != nil {
		return SearchResult{}, e
	}
	all := SearchResult{Results: []Park{}}
	seen := map[string]bool{}
	for page := 0; page < q.MaxPages; page++ {
		method, path, body := "GET", "/park/search.php?start_num="+fmt.Sprint(page*20), url.Values(nil)
		if page == 0 {
			method, path, body = "POST", "/park/search.php", params
		}
		b, e := c.fetch(ctx, method, path, body)
		if e != nil {
			return SearchResult{}, fmt.Errorf("search page %d: %w", page+1, e)
		}
		one, e := ParseSearch(b, time.Now())
		if e != nil {
			return SearchResult{}, e
		}
		if page == 0 {
			all.Meta = one.Meta
			all.Meta.ScannedRecords = 0
			all.Meta.ScannedPages = 0
		}
		all.Meta.ScannedPages++
		all.Meta.ScannedRecords += len(one.Results)
		all.Meta.ProviderPagesComplete = one.Meta.ProviderPagesComplete
		for _, p := range one.Results {
			if seen[p.ID] {
				return SearchResult{}, fmt.Errorf("pagination repeated park %s; refusing misleading coverage", p.ID)
			}
			seen[p.ID] = true
			all.Results = append(all.Results, p)
		}
		if one.Meta.ProviderPagesComplete {
			break
		}
	}
	all.Observations = append([]Park{}, all.Results...)
	all.Meta.OutputTruncated = len(all.Results) > q.Limit
	if all.Meta.OutputTruncated {
		all.Results = all.Results[:q.Limit]
	}
	all.Meta.ReturnedRecords = len(all.Results)
	if !all.Meta.ProviderPagesComplete {
		all.Meta.Note = "Provider pages incomplete; raise --max-scan-pages (maximum 5) to widen the observed search."
	}
	if len(all.Results) == 0 {
		all.Meta.Note = "No matches in the observed provider pages. This is not vacancy evidence."
	}
	return all, nil
}
