// Package ytanalytics provides a thin client for the YouTube Analytics API v2.
// The base URL differs from the YouTube Data API, so this client lives in its
// own package to avoid plumbing a multi-base client through the generated
// internal/client package.
package ytanalytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/cliutil"
)

const baseURL = "https://youtubeanalytics.googleapis.com"

// Client wraps an HTTP client with the bearer-token Authorization header.
type Client struct {
	HTTP        *http.Client
	AccessToken string
	// Limiter paces outbound calls to stay under YouTube Analytics quotas.
	// A nil limiter is the zero-value no-op; production constructs one via New.
	Limiter *cliutil.AdaptiveLimiter
}

// New constructs a Client. The access token is required.
func New(token string) *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: 30 * time.Second},
		AccessToken: token,
		Limiter:     cliutil.NewAdaptiveLimiter(2.0),
	}
}

// Report is a partial decode of the Analytics API reports.query response.
type Report struct {
	Kind          string          `json:"kind"`
	ColumnHeaders []ColumnHeader  `json:"columnHeaders"`
	Rows          [][]interface{} `json:"rows"`
}

// ColumnHeader describes one column in a Report.
type ColumnHeader struct {
	Name       string `json:"name"`
	ColumnType string `json:"columnType"`
	DataType   string `json:"dataType"`
}

// QueryParams holds the inputs to a reports.query call.
type QueryParams struct {
	IDs        string // typically "channel==MINE"
	StartDate  string // YYYY-MM-DD
	EndDate    string // YYYY-MM-DD
	Metrics    string // comma-separated
	Dimensions string // comma-separated
	Filters    string // semicolon-separated (e.g. "video==XXXX")
}

// Query calls reports.query and returns the decoded Report.
func (c *Client) Query(ctx context.Context, p QueryParams) (*Report, error) {
	if p.IDs == "" {
		p.IDs = "channel==MINE"
	}
	v := url.Values{}
	v.Set("ids", p.IDs)
	if p.StartDate != "" {
		v.Set("startDate", p.StartDate)
	}
	if p.EndDate != "" {
		v.Set("endDate", p.EndDate)
	}
	if p.Metrics != "" {
		v.Set("metrics", p.Metrics)
	}
	if p.Dimensions != "" {
		v.Set("dimensions", p.Dimensions)
	}
	if p.Filters != "" {
		v.Set("filters", p.Filters)
	}

	endpoint := baseURL + "/v2/reports?" + v.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Accept", "application/json")

	c.Limiter.Wait()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("analytics query: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &Error{StatusCode: resp.StatusCode, Body: string(body), Kind: KindAuth}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.Limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: endpoint, RetryAfter: cliutil.RetryAfter(resp), Body: string(body)}
	}
	if resp.StatusCode >= 400 {
		return nil, &Error{StatusCode: resp.StatusCode, Body: string(body), Kind: KindOther}
	}
	c.Limiter.OnSuccess()

	var r Report
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decoding analytics response: %w (body: %s)", err, truncate(string(body), 200))
	}
	return &r, nil
}

// RetentionCurve returns the 100-bucket retention curve for a single video.
// Convention: 100 rows with elapsedVideoTimeRatio from 0.01 to 1.0 in 0.01
// increments, with audienceWatchRatio as the y-axis.
func (c *Client) RetentionCurve(ctx context.Context, videoID, startDate, endDate string) ([]float64, error) {
	r, err := c.Query(ctx, QueryParams{
		Metrics:    "audienceWatchRatio",
		Dimensions: "elapsedVideoTimeRatio",
		Filters:    fmt.Sprintf("video==%s", videoID),
		StartDate:  startDate,
		EndDate:    endDate,
	})
	if err != nil {
		return nil, err
	}
	points := make([]float64, 0, len(r.Rows))
	for _, row := range r.Rows {
		if len(row) < 2 {
			continue
		}
		if v, ok := toFloat(row[1]); ok {
			points = append(points, v)
		}
	}
	return points, nil
}

// DailyMetrics returns the day-by-day metrics for a single video. Returns a
// list of (day, views, watchTime, avgViewPct, ctr, impressions) tuples.
type DailyRow struct {
	Day                     string
	Views                   int64
	EstimatedMinutesWatched int64
	AverageViewDuration     int64
	AverageViewPercentage   float64
	CTR                     float64
	ThumbnailImpressions    int64
	SubscribersGained       int64
}

// VideoDailyMetrics queries the standard daily metrics for a video.
func (c *Client) VideoDailyMetrics(ctx context.Context, videoID, startDate, endDate string) ([]DailyRow, error) {
	r, err := c.Query(ctx, QueryParams{
		Metrics:    "views,estimatedMinutesWatched,averageViewDuration,averageViewPercentage,videoThumbnailImpressionsClickRate,videoThumbnailImpressions,subscribersGained",
		Dimensions: "day",
		Filters:    fmt.Sprintf("video==%s", videoID),
		StartDate:  startDate,
		EndDate:    endDate,
	})
	if err != nil {
		return nil, err
	}
	var out []DailyRow
	for _, row := range r.Rows {
		if len(row) < 8 {
			continue
		}
		day, _ := row[0].(string)
		var d DailyRow
		d.Day = day
		d.Views = toInt64(row[1])
		d.EstimatedMinutesWatched = toInt64(row[2])
		d.AverageViewDuration = toInt64(row[3])
		d.AverageViewPercentage, _ = toFloat(row[4])
		d.CTR, _ = toFloat(row[5])
		d.ThumbnailImpressions = toInt64(row[6])
		d.SubscribersGained = toInt64(row[7])
		out = append(out, d)
	}
	return out, nil
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

func toInt64(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		i, _ := x.Int64()
		return i
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ErrorKind is the typed classification of an Analytics API error.
type ErrorKind string

const (
	KindAuth      ErrorKind = "auth"
	KindRateLimit ErrorKind = "rate_limit"
	KindOther     ErrorKind = "other"
)

// Error is an analytics-specific typed error.
type Error struct {
	StatusCode int
	Body       string
	Kind       ErrorKind
}

func (e *Error) Error() string {
	return fmt.Sprintf("analytics API error (kind=%s, http=%d): %s", e.Kind, e.StatusCode, truncate(e.Body, 200))
}

// EnsureScopes is a syntactic helper: returns a string explaining required scopes.
func EnsureScopes() string {
	return strings.Join([]string{
		"https://www.googleapis.com/auth/yt-analytics.readonly",
		"https://www.googleapis.com/auth/youtube.readonly",
	}, " ")
}
