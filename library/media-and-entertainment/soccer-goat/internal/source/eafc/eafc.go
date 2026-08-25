package eafc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
)

const (
	baseURL       = "https://drop-api.ea.com/rating/ea-sports-fc"
	desktopChrome = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	maxRetries    = 2
)

// Player is the EA Sports FC rating and attribute record used by reports.
type Player struct {
	ID                                  int            `json:"id"`
	FirstName, LastName, CommonName     string         `json:"-"`
	Overall                             int            `json:"overall"`
	Team, League, Nationality, Position string         `json:"-"`
	Pace, Shooting, Passing, Dribbling  int            `json:"-"`
	Defending, Physical                 int            `json:"-"`
	Stats                               map[string]int `json:"stats"`
}

// DisplayName returns the name EA expects users to recognize.
func (p Player) DisplayName() string {
	if strings.TrimSpace(p.CommonName) != "" {
		return strings.TrimSpace(p.CommonName)
	}
	return strings.TrimSpace(strings.TrimSpace(p.FirstName) + " " + strings.TrimSpace(p.LastName))
}

// Client queries EA's public ratings endpoint.
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: 12 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(8),
	}
}

// Search finds players by name. The returned slice is always non-nil.
func (c *Client) Search(ctx context.Context, name string, limit int) ([]Player, error) {
	players := make([]Player, 0)
	if cliutil.IsVerifyEnv() {
		return players, nil
	}
	if limit <= 0 {
		limit = 10
	}

	query := url.Values{}
	query.Set("locale", "en")
	query.Set("limit", strconv.Itoa(limit))
	query.Set("search", name)
	target := baseURL + "?" + query.Encode()

	body, err := c.get(ctx, target)
	if err != nil {
		return players, fmt.Errorf("eafc search %q: %w", name, err)
	}

	var response struct {
		Items []struct {
			ID         int    `json:"id"`
			FirstName  string `json:"firstName"`
			LastName   string `json:"lastName"`
			CommonName string `json:"commonName"`
			Overall    int    `json:"overallRating"`
			League     string `json:"leagueName"`
			Team       struct {
				Label string `json:"label"`
			} `json:"team"`
			Nationality struct {
				Label string `json:"label"`
			} `json:"nationality"`
			Position struct {
				ShortLabel string `json:"shortLabel"`
			} `json:"position"`
			Stats map[string]json.RawMessage `json:"stats"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return players, fmt.Errorf("eafc search %q: decode response: %w", name, err)
	}

	players = make([]Player, 0, len(response.Items))
	for _, item := range response.Items {
		stats := make(map[string]int, len(item.Stats))
		for key, raw := range item.Stats {
			if value, ok := ratingValue(raw); ok {
				stats[key] = value
			}
		}
		players = append(players, Player{
			ID:          item.ID,
			FirstName:   item.FirstName,
			LastName:    item.LastName,
			CommonName:  item.CommonName,
			Overall:     item.Overall,
			Team:        item.Team.Label,
			League:      item.League,
			Nationality: item.Nationality.Label,
			Position:    item.Position.ShortLabel,
			Pace:        stats["pac"],
			Shooting:    stats["sho"],
			Passing:     stats["pas"],
			Dribbling:   stats["dri"],
			Defending:   stats["def"],
			Physical:    stats["phy"],
			Stats:       stats,
		})
	}
	return players, nil
}

// Best returns the first search result.
func (c *Client) Best(ctx context.Context, name string) (*Player, bool, error) {
	players, err := c.Search(ctx, name, 1)
	if err != nil {
		return nil, false, err
	}
	if len(players) == 0 {
		return nil, false, nil
	}
	return &players[0], true, nil
}

func ratingValue(raw json.RawMessage) (int, bool) {
	var value int
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	var wrapped struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		return wrapped.Value, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value, err := strconv.Atoi(text)
		return value, err == nil
	}
	return 0, false
}

func (c *Client) get(ctx context.Context, target string) ([]byte, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Origin", "https://www.ea.com")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", desktopChrome)

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
				return nil, &cliutil.RateLimitError{URL: target, RetryAfter: retryAfter, Body: bodySnippet(body)}
			}
			if err := wait(ctx, retryAfter); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s returned HTTP %d: %s", target, resp.StatusCode, bodySnippet(body))
		}
		c.limiter.OnSuccess()
		return body, nil
	}
	return nil, fmt.Errorf("GET %s failed", target)
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

func bodySnippet(body []byte) string {
	const max = 512
	text := strings.TrimSpace(string(body))
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}
