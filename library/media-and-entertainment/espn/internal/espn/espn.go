package espn

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const userAgent = "espn-pp-cli/1.0.0"

type ESPN struct {
	httpClient *http.Client
	cacheDir   string
	noCache    bool
	dryRun     bool

	mu       sync.RWMutex
	memCache map[string]cacheEntry
}

type cacheEntry struct {
	body      json.RawMessage
	expiresAt time.Time
}

type apiError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s %s returned HTTP %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

func New() *ESPN {
	return NewWithTimeout(30 * time.Second)
}

func NewWithTimeout(timeout time.Duration) *ESPN {
	homeDir, _ := os.UserHomeDir()
	return &ESPN{
		httpClient: &http.Client{Timeout: timeout},
		cacheDir:   filepath.Join(homeDir, ".cache", "espn-pp-cli", "espn"),
		memCache:   make(map[string]cacheEntry),
	}
}

func (e *ESPN) SetDryRun(v bool) { e.dryRun = v }

func (e *ESPN) SetNoCache(v bool) { e.noCache = v }

func (e *ESPN) Scoreboard(sport, league, date string) (json.RawMessage, error) {
	u := siteAPIURL(sport, league, "scoreboard")
	if date != "" {
		u = withQuery(u, map[string]string{"dates": date})
	}
	return e.get(u, 5*time.Minute)
}

func (e *ESPN) Standings(sport, league string) (json.RawMessage, error) {
	// The correct standings endpoint uses /apis/v2/sports/ (not /apis/site/v2/sports/)
	u := "https://site.api.espn.com/apis/v2/sports/" + sport + "/" + league + "/standings"
	return e.get(u, time.Hour)
}

func (e *ESPN) Teams(sport, league string) (json.RawMessage, error) {
	return e.get(siteAPIURL(sport, league, "teams"), time.Hour)
}

func (e *ESPN) Team(sport, league, teamID string) (json.RawMessage, error) {
	return e.get(siteAPIURL(sport, league, "teams", teamID), time.Hour)
}

func (e *ESPN) TeamRoster(sport, league, teamID string) (json.RawMessage, error) {
	return e.get(siteAPIURL(sport, league, "teams", teamID, "roster"), time.Hour)
}

func (e *ESPN) Schedule(sport, league string, dates string) (json.RawMessage, error) {
	u := siteAPIURL(sport, league, "schedule")
	if dates != "" {
		u = withQuery(u, map[string]string{"dates": dates})
	}
	return e.get(u, 5*time.Minute)
}

func (e *ESPN) News(sport, league string) (json.RawMessage, error) {
	return e.get(withQuery("https://now.core.api.espn.com/v1/sports/news", map[string]string{
		"sport":  sport,
		"league": league,
	}), 5*time.Minute)
}

func (e *ESPN) Rankings(sport, league string) (json.RawMessage, error) {
	return e.get(siteAPIURL(sport, league, "rankings"), time.Hour)
}

func (e *ESPN) Injuries(sport, league string) (json.RawMessage, error) {
	return e.get(siteAPIURL(sport, league, "injuries"), 5*time.Minute)
}

func (e *ESPN) Transactions(sport, league string) (json.RawMessage, error) {
	return e.get(siteAPIURL(sport, league, "transactions"), 5*time.Minute)
}

func (e *ESPN) Summary(sport, league, eventID string) (json.RawMessage, error) {
	return e.get(withQuery(siteAPIURL(sport, league, "summary"), map[string]string{
		"event": eventID,
	}), 5*time.Minute)
}

func (e *ESPN) Boxscore(sport, league, eventID string) (json.RawMessage, error) {
	return e.get(withQuery(siteAPIURL(sport, league, "summary"), map[string]string{
		"event": eventID,
	}), 5*time.Minute)
}

func (e *ESPN) PlayByPlay(sport, league, eventID string) (json.RawMessage, error) {
	return e.get(withQuery(siteAPIURL(sport, league, "summary"), map[string]string{
		"event": eventID,
	}), 5*time.Minute)
}

func (e *ESPN) Athletes(sport, league string, limit int) (json.RawMessage, error) {
	params := map[string]string{}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}
	return e.get(withQuery(coreAPIURL(sport, league, "athletes"), params), time.Hour)
}

func (e *ESPN) Athlete(sport, league, athleteID string) (json.RawMessage, error) {
	return e.get(webAPIURL("common", "v3", "sports", sport, league, "athletes", athleteID, "overview"), time.Hour)
}

func (e *ESPN) AthleteGamelog(sport, league, athleteID string) (json.RawMessage, error) {
	return e.get(webAPIURL("common", "v3", "sports", sport, league, "athletes", athleteID, "gamelog"), 5*time.Minute)
}

func (e *ESPN) AthleteSplits(sport, league, athleteID string) (json.RawMessage, error) {
	return e.get(webAPIURL("common", "v3", "sports", sport, league, "athletes", athleteID, "splits"), time.Hour)
}

func (e *ESPN) AthleteStats(sport, league, athleteID string) (json.RawMessage, error) {
	return e.get(webAPIURL("common", "v3", "sports", sport, league, "athletes", athleteID, "stats"), 5*time.Minute)
}

func (e *ESPN) Search(query string) (json.RawMessage, error) {
	return e.get(withQuery("https://site.web.api.espn.com/apis/search/v2", map[string]string{
		"query": query,
		"limit": "10",
	}), 5*time.Minute)
}

func (e *ESPN) Leaders(sport, league string, category string) (json.RawMessage, error) {
	u := siteAPIURL(sport, league, "statistics")
	if category != "" {
		u = withQuery(u, map[string]string{"category": category})
	}
	return e.get(u, 5*time.Minute)
}

func (e *ESPN) Odds(sport, league, eventID string) (json.RawMessage, error) {
	return e.get(withQuery(siteAPIURL(sport, league, "odds"), map[string]string{
		"event": eventID,
	}), 5*time.Minute)
}

func (e *ESPN) Calendar(sport, league string) (json.RawMessage, error) {
	return e.get(siteAPIURL(sport, league, "calendar"), time.Hour)
}

func (e *ESPN) get(rawURL string, ttl time.Duration) (json.RawMessage, error) {
	if e.dryRun {
		fmt.Fprintln(os.Stderr, rawURL)
		return nil, nil
	}
	if !e.noCache {
		if body, ok := e.readCache(rawURL); ok {
			return body, nil
		}
	}

	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := e.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("GET %s: %w", rawURL, err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode < 400 {
			body := json.RawMessage(respBody)
			if !e.noCache {
				e.writeCache(rawURL, body, ttl)
			}
			return body, nil
		}

		apiErr := &apiError{
			Method:     http.MethodGet,
			URL:        rawURL,
			StatusCode: resp.StatusCode,
			Body:       truncateBody(respBody),
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			wait := retryAfter(resp)
			time.Sleep(wait)
			lastErr = apiErr
			continue
		}
		if resp.StatusCode >= 500 && attempt < maxRetries {
			wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(wait)
			lastErr = apiErr
			continue
		}
		return nil, apiErr
	}
	return nil, lastErr
}

func (e *ESPN) readCache(rawURL string) (json.RawMessage, bool) {
	now := time.Now()

	e.mu.RLock()
	entry, ok := e.memCache[rawURL]
	e.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.body, true
	}

	cacheFile := filepath.Join(e.cacheDir, cacheKey(rawURL)+".json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}

	var disk struct {
		ExpiresAt time.Time       `json:"expires_at"`
		Body      json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &disk); err != nil || now.After(disk.ExpiresAt) {
		return nil, false
	}

	e.mu.Lock()
	e.memCache[rawURL] = cacheEntry{body: disk.Body, expiresAt: disk.ExpiresAt}
	e.mu.Unlock()
	return disk.Body, true
}

func (e *ESPN) writeCache(rawURL string, body json.RawMessage, ttl time.Duration) {
	expiresAt := time.Now().Add(ttl)

	e.mu.Lock()
	e.memCache[rawURL] = cacheEntry{body: body, expiresAt: expiresAt}
	e.mu.Unlock()

	_ = os.MkdirAll(e.cacheDir, 0o755)
	payload, err := json.Marshal(struct {
		ExpiresAt time.Time       `json:"expires_at"`
		Body      json.RawMessage `json:"body"`
	}{
		ExpiresAt: expiresAt,
		Body:      body,
	})
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(e.cacheDir, cacheKey(rawURL)+".json"), payload, 0o644)
}

func siteAPIURL(sport, league string, segments ...string) string {
	parts := append([]string{"https://site.api.espn.com/apis/site/v2/sports", sport, league}, segments...)
	return joinURL(parts...)
}

func coreAPIURL(sport, league string, segments ...string) string {
	parts := append([]string{"https://sports.core.api.espn.com/v2/sports", sport, "leagues", league}, segments...)
	return joinURL(parts...)
}

func webAPIURL(segments ...string) string {
	parts := append([]string{"https://site.web.api.espn.com/apis"}, segments...)
	return joinURL(parts...)
}


func joinURL(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i == 0 {
			clean = append(clean, strings.TrimRight(part, "/"))
			continue
		}
		clean = append(clean, strings.Trim(part, "/"))
	}
	return strings.Join(clean, "/")
}

func withQuery(rawURL string, params map[string]string) string {
	if len(params) == 0 {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := parsed.Query()
	for k, v := range params {
		if strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func cacheKey(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(h[:8])
}

func truncateBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 512 {
		return s[:512] + "..."
	}
	return s
}

func retryAfter(resp *http.Response) time.Duration {
	if value := resp.Header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if when, err := http.ParseTime(value); err == nil {
			wait := time.Until(when)
			if wait > 0 {
				return wait
			}
		}
	}
	return 2 * time.Second
}
