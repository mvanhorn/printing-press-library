// Package blogapi calls Naver Blog's public mobile blog metadata endpoint.
//
// Endpoint:
// https://m.blog.naver.com/api/blogs/<blog_id>
//
// Authentication: none. The endpoint requires a mobile-blog Referer matching
// the requested blog ID; without it Naver returns HTTP 403.
package blogapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1"

// BlogInfo is the stable projection emitted by the blog profile endpoint.
type BlogInfo struct {
	BlogID            string `json:"blog_id"`
	BlogNo            int    `json:"blog_no"`
	BlogName          string `json:"blog_name"`
	Nickname          string `json:"nickname"`
	DisplayNickname   string `json:"display_nickname"`
	OfficialBlog      bool   `json:"official_blog"`
	PowerBlog         bool   `json:"power_blog"`
	DayVisitorCount   int    `json:"day_visitor_count"`
	SubscriberCount   int    `json:"subscriber_count"`
	TotalVisitorCount int    `json:"total_visitor_count"`
	DirectoryName     string `json:"directory_name"`
	ProfileImageURL   string `json:"profile_image_url"`
	CoverImageURL     string `json:"cover_image_url"`
	IsAIPickBlog      bool   `json:"is_ai_pick_blog"`
	IsYearOfBlog      bool   `json:"is_year_of_blog"`
	IsSuggestionBlog  bool   `json:"is_suggestion_blog"`
}

// Limiter paces outbound requests. The press client's AdaptiveLimiter
// satisfies this interface.
type Limiter interface {
	Wait()
}

// RateLimitError is returned when Naver responds with HTTP 429.
type RateLimitError struct {
	StatusCode int
	RetryAfter time.Duration
	Body       string
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("blog API rate limited (HTTP %d), retry after %s", e.StatusCode, e.RetryAfter)
	}
	return fmt.Sprintf("blog API rate limited (HTTP %d)", e.StatusCode)
}

var endpointBase = "https://m.blog.naver.com/api/blogs"

type blogInfoResponse struct {
	IsSuccess bool        `json:"isSuccess"`
	Result    apiBlogInfo `json:"result"`
	Message   string      `json:"message"`
}

type apiBlogInfo struct {
	BlogID           string `json:"blogId"`
	BlogNo           int    `json:"blogNo"`
	BlogName         string `json:"blogName"`
	Nickname         string `json:"nickName"`
	DisplayNickname  string `json:"displayNickName"`
	OfficialBlog     bool   `json:"officialBlog"`
	PowerBlog        bool   `json:"powerBlog"`
	DayVisitors      int    `json:"dayVisitorCount"`
	Subscribers      int    `json:"subscriberCount"`
	TotalVisitors    int    `json:"totalVisitorCount"`
	DirectoryName    string `json:"blogDirectoryName"`
	ProfileImagePath string `json:"profileImagePath"`
	MobileTitleImage struct {
		OriginalImage string `json:"originalImage"`
		CroppedImage  string `json:"croppedImage"`
		PreviewImage  string `json:"previewImage"`
	} `json:"mobileTitleImage"`
	IsAIPickBlog     bool `json:"isAiPickBlog"`
	IsYearOfBlog     bool `json:"isYearOfBlog"`
	IsSuggestionBlog bool `json:"suggestionBlog"`
}

// GetBlogInfo fetches profile metadata for a Naver Blog ID.
func GetBlogInfo(ctx context.Context, httpClient *http.Client, blogID string) (BlogInfo, error) {
	return GetBlogInfoLimited(ctx, httpClient, blogID, nil)
}

// GetBlogInfoLimited is GetBlogInfo with an explicit request limiter.
func GetBlogInfoLimited(ctx context.Context, httpClient *http.Client, blogID string, limiter Limiter) (BlogInfo, error) {
	if httpClient == nil {
		return BlogInfo{}, fmt.Errorf("nil http client")
	}
	blogID = strings.TrimSpace(blogID)
	if blogID == "" {
		return BlogInfo{}, fmt.Errorf("blog_id is required")
	}

	reqURL := strings.TrimRight(endpointBase, "/") + "/" + url.PathEscape(blogID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return BlogInfo{}, fmt.Errorf("building blog API request: %w", err)
	}
	setCommonHeaders(req)
	req.Header.Set("Referer", "https://m.blog.naver.com/"+url.PathEscape(blogID))
	if limiter != nil {
		limiter.Wait()
	}

	body, resp, err := doRead(httpClient, req)
	if err != nil {
		return BlogInfo{}, fmt.Errorf("calling blog API: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return BlogInfo{}, &RateLimitError{StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")), Body: truncate(string(body), 512)}
	}
	if resp.StatusCode >= 400 {
		return BlogInfo{}, fmt.Errorf("blog API HTTP %d: %s", resp.StatusCode, truncate(string(body), 512))
	}

	var parsed blogInfoResponse
	if err := unmarshalMaybeJSONP(body, &parsed); err != nil {
		return BlogInfo{}, fmt.Errorf("decoding blog response: %w (body: %s)", err, truncate(string(body), 512))
	}
	if !parsed.IsSuccess {
		msg := strings.TrimSpace(parsed.Message)
		if msg == "" {
			msg = "blog API returned isSuccess=false"
		}
		return BlogInfo{}, fmt.Errorf("%s", msg)
	}
	return convertBlogInfo(parsed.Result), nil
}

func convertBlogInfo(raw apiBlogInfo) BlogInfo {
	return BlogInfo{
		BlogID:            raw.BlogID,
		BlogNo:            raw.BlogNo,
		BlogName:          raw.BlogName,
		Nickname:          raw.Nickname,
		DisplayNickname:   raw.DisplayNickname,
		OfficialBlog:      raw.OfficialBlog,
		PowerBlog:         raw.PowerBlog,
		DayVisitorCount:   raw.DayVisitors,
		SubscriberCount:   raw.Subscribers,
		TotalVisitorCount: raw.TotalVisitors,
		DirectoryName:     raw.DirectoryName,
		ProfileImageURL:   strings.TrimSpace(raw.ProfileImagePath),
		CoverImageURL:     strings.TrimSpace(raw.MobileTitleImage.CroppedImage),
		IsAIPickBlog:      raw.IsAIPickBlog,
		IsYearOfBlog:      raw.IsYearOfBlog,
		IsSuggestionBlog:  raw.IsSuggestionBlog,
	}
}

func setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", mobileUserAgent)
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,en;q=0.8")
	req.Header.Set("Accept", "application/json")
}

func doRead(httpClient *http.Client, req *http.Request) ([]byte, *http.Response, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}
	return body, resp, nil
}

func unmarshalMaybeJSONP(body []byte, v any) error {
	body = bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF"))
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return fmt.Errorf("empty response body")
	}
	if body[0] == '{' || body[0] == '[' {
		return json.Unmarshal(body, v)
	}
	open := bytes.IndexByte(body, '(')
	close := bytes.LastIndexByte(body, ')')
	if open >= 0 && close > open {
		return json.Unmarshal(bytes.TrimSpace(body[open+1:close]), v)
	}
	return json.Unmarshal(body, v)
}

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
