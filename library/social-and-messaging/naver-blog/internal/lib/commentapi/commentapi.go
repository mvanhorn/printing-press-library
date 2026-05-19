// Package commentapi calls Naver Blog's public cbox comment endpoint.
//
// Endpoint:
// https://apis.naver.com/commentBox/cbox/web_naver_list_jsonp.json
//
// Authentication: none. The endpoint does require a mobile-blog Referer
// matching the post, and uses the numeric blogNo in objectId:
//
//	<objectId> = <blogNo>_201_<logNo>
//
// GetComments resolves blogNo from the public comments-info endpoint before
// fetching comments. GetCommentsByObjectID skips that lookup when callers
// already have blogNo.
package commentapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1"

// Comment is one flat comment row. Replies are inlined with ReplyLevel >= 2.
type Comment struct {
	CommentNo        string    `json:"comment_no"`
	ParentCommentNo  string    `json:"parent_comment_no"`
	ReplyLevel       int       `json:"reply_level"`
	ReplyCount       int       `json:"reply_count"`
	ReplyAllCount    int       `json:"reply_all_count"`
	Contents         string    `json:"contents"`
	ContentsRaw      string    `json:"contents_raw,omitempty"`
	ImageURLs        []string  `json:"image_urls,omitempty"`
	StickerID        string    `json:"sticker_id,omitempty"`
	UserName         string    `json:"user_name"`
	UserProfileURL   string    `json:"user_profile_url,omitempty"`
	UserHomepageURL  string    `json:"user_homepage_url,omitempty"`
	RegTimeUTC       time.Time `json:"reg_time_utc"`
	ModTimeUTC       time.Time `json:"mod_time_utc,omitempty"`
	SympathyCount    int       `json:"sympathy_count"`
	AntipathyCount   int       `json:"antipathy_count"`
	CommentType      string    `json:"comment_type"`
	Secret           bool      `json:"secret,omitempty"`
	Visible          bool      `json:"visible"`
	HiddenByCleanbot bool      `json:"hidden_by_cleanbot,omitempty"`
}

// GetOptions controls comment retrieval.
type GetOptions struct {
	Page     int
	PageSize int
	All      bool
	Limiter  Limiter
	Pacing   time.Duration
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
		return fmt.Sprintf("comment API rate limited (HTTP %d), retry after %s", e.StatusCode, e.RetryAfter)
	}
	return fmt.Sprintf("comment API rate limited (HTTP %d)", e.StatusCode)
}

// UnsuccessfulError is returned when the cbox API responds with success:false.
type UnsuccessfulError struct {
	Code    string
	Message string
	Body    string
}

func (e *UnsuccessfulError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "comment API returned success=false"
	}
	if e.Code != "" {
		return fmt.Sprintf("comment API returned success=false (code %s): %s", e.Code, msg)
	}
	return msg
}

var (
	endpointBase         = "https://apis.naver.com/commentBox/cbox"
	commentsInfoBase     = "https://m.blog.naver.com/api/blogs"
	brTagRe              = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe            = regexp.MustCompile(`(?s)<[^>]+>`)
	cboxTimestampLayouts = []string{
		"2006-01-02T15:04:05-0700",
		time.RFC3339,
	}
)

type commentsInfoResponse struct {
	IsSuccess bool `json:"isSuccess"`
	Result    struct {
		TotalCount int `json:"totalCount"`
		BlogNo     int `json:"blogNo"`
	} `json:"result"`
	Message string `json:"message"`
}

type cboxResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  struct {
		CommentList []apiComment `json:"commentList"`
		Count       struct {
			Total int `json:"total"`
		} `json:"count"`
	} `json:"result"`
}

type apiComment struct {
	CommentNo        string       `json:"commentNo"`
	ParentCommentNo  string       `json:"parentCommentNo"`
	ReplyLevel       int          `json:"replyLevel"`
	ReplyCount       int          `json:"replyCount"`
	ReplyAllCount    int          `json:"replyAllCount"`
	Contents         string       `json:"contents"`
	ImageList        []string     `json:"imageList"`
	ImagePathList    []string     `json:"imagePathList"`
	StickerID        string       `json:"stickerId"`
	UserName         string       `json:"userName"`
	UserProfileImage string       `json:"userProfileImage"`
	UserHomepageURL  string       `json:"userHomepageUrl"`
	RegTime          string       `json:"regTime"`
	RegTimeGmt       string       `json:"regTimeGmt"`
	ModTime          string       `json:"modTime"`
	ModTimeGmt       string       `json:"modTimeGmt"`
	SympathyCount    int          `json:"sympathyCount"`
	AntipathyCount   int          `json:"antipathyCount"`
	CommentType      string       `json:"commentType"`
	Secret           bool         `json:"secret"`
	Visible          bool         `json:"visible"`
	HiddenByCleanbot bool         `json:"hiddenByCleanbot"`
	ReplyList        []apiComment `json:"replyList"`
}

// GetComments fetches comments for a single Naver Blog post.
// blogID is the slug (e.g., "perfect62"); the function internally
// fetches comments-info to obtain the numeric blogNo and constructs
// the objectId.
func GetComments(ctx context.Context, httpClient *http.Client, blogID, logNo string, opts GetOptions) (comments []Comment, total int, err error) {
	if strings.TrimSpace(blogID) == "" {
		return nil, 0, fmt.Errorf("blog_id is required")
	}
	if strings.TrimSpace(logNo) == "" {
		return nil, 0, fmt.Errorf("log_no is required")
	}
	blogNo, _, err := fetchBlogNo(ctx, httpClient, blogID, logNo, opts.Limiter)
	if err != nil {
		return nil, 0, err
	}
	return GetCommentsByObjectID(ctx, httpClient, blogID, logNo, blogNo, logNo, opts)
}

// GetCommentsByObjectID fetches comments when caller already has blogNo.
func GetCommentsByObjectID(ctx context.Context, httpClient *http.Client, refererBlogID, refererLogNo string, blogNo int, logNo string, opts GetOptions) (comments []Comment, total int, err error) {
	if httpClient == nil {
		return nil, 0, fmt.Errorf("nil http client")
	}
	if strings.TrimSpace(refererBlogID) == "" {
		return nil, 0, fmt.Errorf("referer blog_id is required")
	}
	if strings.TrimSpace(refererLogNo) == "" {
		return nil, 0, fmt.Errorf("referer log_no is required")
	}
	if blogNo <= 0 {
		return nil, 0, fmt.Errorf("blogNo must be positive")
	}
	if strings.TrimSpace(logNo) == "" {
		return nil, 0, fmt.Errorf("log_no is required")
	}

	page := opts.Page
	if page <= 0 {
		page = 1
	}
	pageSize := opts.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}

	out := make([]Comment, 0)
	for {
		pageComments, pageTotal, err := fetchCommentPage(ctx, httpClient, refererBlogID, refererLogNo, blogNo, logNo, page, pageSize, opts.Limiter)
		if err != nil {
			return nil, 0, err
		}
		if pageTotal > 0 || total == 0 {
			total = pageTotal
		}
		out = append(out, pageComments...)
		if !opts.All {
			break
		}
		if len(pageComments) == 0 {
			break
		}
		if total > 0 && len(out) >= total {
			break
		}
		page++
		if opts.Pacing > 0 {
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(opts.Pacing):
			}
		}
	}
	return out, total, nil
}

func fetchBlogNo(ctx context.Context, httpClient *http.Client, blogID, logNo string, limiter Limiter) (blogNo int, total int, err error) {
	if httpClient == nil {
		return 0, 0, fmt.Errorf("nil http client")
	}
	reqURL := strings.TrimRight(commentsInfoBase, "/") + "/" + url.PathEscape(blogID) + "/posts/" + url.PathEscape(logNo) + "/comments-info"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("building comments-info request: %w", err)
	}
	setCommonHeaders(req)
	// Naver's m.blog.naver.com /api/* endpoints reject anonymous requests
	// without a same-host Referer (verified via direct curl A/B). Without
	// this header the response is HTTP 403 with isSuccess=false.
	req.Header.Set("Referer", fmt.Sprintf("https://m.blog.naver.com/%s/%s", url.PathEscape(blogID), url.PathEscape(logNo)))
	if limiter != nil {
		limiter.Wait()
	}
	body, resp, err := doRead(httpClient, req)
	if err != nil {
		return 0, 0, fmt.Errorf("calling comments-info API: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, 0, &RateLimitError{StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")), Body: truncate(string(body), 512)}
	}
	if resp.StatusCode >= 400 {
		return 0, 0, fmt.Errorf("comments-info API HTTP %d: %s", resp.StatusCode, truncate(string(body), 512))
	}
	var parsed commentsInfoResponse
	if err := unmarshalMaybeJSONP(body, &parsed); err != nil {
		return 0, 0, fmt.Errorf("decoding comments-info response: %w (body: %s)", err, truncate(string(body), 512))
	}
	if !parsed.IsSuccess {
		msg := strings.TrimSpace(parsed.Message)
		if msg == "" {
			msg = "comments-info API returned isSuccess=false"
		}
		return 0, 0, fmt.Errorf("%s", msg)
	}
	if parsed.Result.BlogNo <= 0 {
		return 0, 0, fmt.Errorf("comments-info response missing blogNo")
	}
	return parsed.Result.BlogNo, parsed.Result.TotalCount, nil
}

func fetchCommentPage(ctx context.Context, httpClient *http.Client, refererBlogID, refererLogNo string, blogNo int, logNo string, page, pageSize int, limiter Limiter) ([]Comment, int, error) {
	q := url.Values{}
	q.Set("ticket", "blog")
	q.Set("templateId", "default")
	q.Set("pool", "blogid")
	q.Set("lang", "ko")
	q.Set("objectId", fmt.Sprintf("%d_201_%s", blogNo, logNo))
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("listType", "OBJECT")
	q.Set("pageType", "more")
	q.Set("page", strconv.Itoa(page))
	q.Set("initialize", "true")
	reqURL := strings.TrimRight(endpointBase, "/") + "/web_naver_list_jsonp.json?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("building comment API request: %w", err)
	}
	setCommonHeaders(req)
	req.Header.Set("Referer", fmt.Sprintf("https://m.blog.naver.com/%s/%s", url.PathEscape(refererBlogID), url.PathEscape(refererLogNo)))
	if limiter != nil {
		limiter.Wait()
	}

	body, resp, err := doRead(httpClient, req)
	if err != nil {
		return nil, 0, fmt.Errorf("calling comment API: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, 0, &RateLimitError{StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")), Body: truncate(string(body), 512)}
	}
	if resp.StatusCode >= 400 {
		return nil, 0, fmt.Errorf("comment API HTTP %d: %s", resp.StatusCode, truncate(string(body), 512))
	}

	var parsed cboxResponse
	if err := unmarshalMaybeJSONP(body, &parsed); err != nil {
		return nil, 0, fmt.Errorf("decoding comment response: %w (body: %s)", err, truncate(string(body), 512))
	}
	if !parsed.Success {
		return nil, 0, &UnsuccessfulError{
			Code:    parsed.Code,
			Message: parsed.Message,
			Body:    truncate(string(body), 512),
		}
	}
	out := make([]Comment, 0, len(parsed.Result.CommentList))
	for _, c := range parsed.Result.CommentList {
		out = appendFlattened(out, c)
	}
	return out, parsed.Result.Count.Total, nil
}

func appendFlattened(out []Comment, c apiComment) []Comment {
	out = append(out, convertComment(c))
	for _, reply := range c.ReplyList {
		out = appendFlattened(out, reply)
	}
	return out
}

func convertComment(c apiComment) Comment {
	return Comment{
		CommentNo:        c.CommentNo,
		ParentCommentNo:  c.ParentCommentNo,
		ReplyLevel:       c.ReplyLevel,
		ReplyCount:       c.ReplyCount,
		ReplyAllCount:    c.ReplyAllCount,
		Contents:         cleanContents(c.Contents),
		ContentsRaw:      c.Contents,
		ImageURLs:        cleanStringSlice(firstNonEmptyStrings(c.ImageList, c.ImagePathList)),
		StickerID:        strings.TrimSpace(c.StickerID),
		UserName:         html.UnescapeString(strings.TrimSpace(c.UserName)),
		UserProfileURL:   strings.TrimSpace(c.UserProfileImage),
		UserHomepageURL:  strings.TrimSpace(c.UserHomepageURL),
		RegTimeUTC:       parseCboxTime(c.RegTimeGmt, c.RegTime),
		ModTimeUTC:       parseCboxTime(c.ModTimeGmt, c.ModTime),
		SympathyCount:    c.SympathyCount,
		AntipathyCount:   c.AntipathyCount,
		CommentType:      strings.TrimSpace(c.CommentType),
		Secret:           c.Secret,
		Visible:          c.Visible,
		HiddenByCleanbot: c.HiddenByCleanbot,
	}
}

func firstNonEmptyStrings(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func cleanStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
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

func cleanContents(raw string) string {
	s := brTagRe.ReplaceAllString(raw, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}

func parseCboxTime(values ...string) time.Time {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		for _, layout := range cboxTimestampLayouts {
			if t, err := time.Parse(layout, value); err == nil {
				return t.UTC()
			}
		}
	}
	return time.Time{}
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
