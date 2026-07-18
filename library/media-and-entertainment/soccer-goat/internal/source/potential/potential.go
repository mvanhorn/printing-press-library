package potential

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
)

const (
	fifacmBase    = "https://www.fifacm.com/player"
	sofifaBase    = "https://sofifa.com/player"
	desktopChrome = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	maxRetries    = 2
)

var challengeMarkers = []string{"cf-chl", "challenge-platform", "just a moment", "cloudflare ray id"}

// Rating is a current/potential pair scraped from a public player page.
type Rating struct {
	Overall   int    `json:"overall"`
	Potential int    `json:"potential"`
	Source    string `json:"source"`
}

// Client performs best-effort potential lookups. Its public lookup methods do
// not return upstream failures because potential must never block a report.
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 12 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(1),
	}
}

// ByEAID looks up fifacm using the EA player id shared by both sites.
//
// fifacm sits behind Cloudflare and returns 403 to every request that lacks a
// cleared-session cookie. Without SOCCER_GOAT_FIFACM_COOKIE set we skip the
// network entirely: the call is guaranteed to fail, and hitting it once per
// player (rate-limited to ~1/sec) is what previously made team-wide commands
// time out. Potential is best-effort, so a fast "unavailable" is correct.
func (c *Client) ByEAID(ctx context.Context, eaID int) (Rating, bool, error) {
	cookie := os.Getenv("SOCCER_GOAT_FIFACM_COOKIE")
	if cliutil.IsVerifyEnv() || eaID <= 0 || strings.TrimSpace(cookie) == "" {
		return Rating{}, false, nil
	}
	target := fmt.Sprintf("%s/%d/x", fifacmBase, eaID)
	return c.lookup(ctx, target, cookie, "fifacm")
}

// BySofifaID supports an explicit Sofifa id without attempting unreliable
// name-based Sofifa search.
func (c *Client) BySofifaID(ctx context.Context, sofifaID int) (Rating, bool, error) {
	cookie := os.Getenv("SOCCER_GOAT_SOFIFA_COOKIE")
	if cliutil.IsVerifyEnv() || sofifaID <= 0 || strings.TrimSpace(cookie) == "" {
		return Rating{}, false, nil
	}
	target := fmt.Sprintf("%s/%d", sofifaBase, sofifaID)
	return c.lookup(ctx, target, cookie, "sofifa")
}

func (c *Client) lookup(ctx context.Context, target, cookie, source string) (Rating, bool, error) {
	body, err := c.get(ctx, target, cookie)
	if err != nil || isChallenge(body) {
		return Rating{}, false, nil
	}
	overall, potential, ok := parsePotential(string(body))
	if !ok {
		return Rating{}, false, nil
	}
	return Rating{Overall: overall, Potential: potential, Source: source}, true, nil
}

func (c *Client) get(ctx context.Context, target, cookie string) ([]byte, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", desktopChrome)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		if strings.TrimSpace(cookie) != "" {
			req.Header.Set("Cookie", cookie)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			retryAfter := cliutil.RetryAfter(resp)
			if attempt == maxRetries {
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: retryAfter, Body: snippet(body)}
			}
			if err := wait(ctx, retryAfter); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s returned HTTP %d", target, resp.StatusCode)
		}
		c.limiter.OnSuccess()
		return body, nil
	}
	return nil, fmt.Errorf("GET %s failed", target)
}

func parsePotential(page string) (overall, potential int, ok bool) {
	potential, potentialOK := labelledRating(page, "potential")
	if !potentialOK {
		return 0, 0, false
	}
	overall, _ = labelledRating(page, "overall")
	return overall, potential, true
}

func labelledRating(page, label string) (int, bool) {
	quoted := regexp.QuoteMeta(label)
	patterns := []string{
		`(?i)(?:["']?` + quoted + `["']?|data-` + quoted + `)\s*[:=]\s*["']?([0-9]{2})`,
		`(?is)>\s*` + quoted + `\s*<.{0,240}?\b([0-9]{2})\b`,
		`(?is)\b` + quoted + `\b.{0,120}?\b([0-9]{2})\b`,
	}
	for _, pattern := range patterns {
		match := regexp.MustCompile(pattern).FindStringSubmatch(page)
		if len(match) != 2 {
			continue
		}
		value, err := strconv.Atoi(match[1])
		if err == nil && value > 0 && value <= 99 {
			return value, true
		}
	}
	return 0, false
}

func isChallenge(body []byte) bool {
	lower := strings.ToLower(string(body))
	for _, marker := range challengeMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func snippet(body []byte) string {
	const max = 512
	text := cliutil.CleanText(string(body))
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
