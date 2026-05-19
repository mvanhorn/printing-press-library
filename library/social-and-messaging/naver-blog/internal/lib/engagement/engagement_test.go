package engagement

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchPopulatesAllSources(t *testing.T) {
	c := newEngagementTestClient(t, testTransport{
		likes:    map[string]int{"blog_1": 7},
		comments: map[string]int{"blog_1": 12},
		dates:    map[string]string{"blog_1": "2026. 3. 30. 15:35"},
	})

	got := Fetch(context.Background(), c, "blog", "1")

	if len(got.Errors) != 0 {
		t.Fatalf("Errors = %v, want none", got.Errors)
	}
	if got.Likes == nil || *got.Likes != 7 || got.LikesSource != sourceReactionAPI {
		t.Fatalf("Likes = %v source=%q, want 7 from reaction-api", got.Likes, got.LikesSource)
	}
	if got.Comments != 12 || got.CommentsSource != sourceCbox {
		t.Fatalf("Comments = %d source=%q, want 12 from cbox", got.Comments, got.CommentsSource)
	}
	if got.PublishDateStr != "2026. 3. 30. 15:35" || got.DateSource != sourcePostViewHTML {
		t.Fatalf("PublishDateStr = %q source=%q", got.PublishDateStr, got.DateSource)
	}
	if got.PublishedAtUTC.IsZero() {
		t.Fatal("PublishedAtUTC is zero")
	}
}

func TestFetchPartialFailureKeepsOtherSources(t *testing.T) {
	c := newEngagementTestClient(t, testTransport{
		likes:          map[string]int{"blog_1": 9},
		commentsStatus: http.StatusTooManyRequests,
		dates:          map[string]string{"blog_1": "2026. 4. 1. 09:00"},
	})

	got := Fetch(context.Background(), c, "blog", "1")

	if got.Likes == nil || *got.Likes != 9 {
		t.Fatalf("Likes = %v, want 9", got.Likes)
	}
	if got.PublishedAtUTC.IsZero() || got.DateSource != sourcePostViewHTML {
		t.Fatalf("PublishedAtUTC = %v source=%q, want post-view date", got.PublishedAtUTC, got.DateSource)
	}
	if got.Comments != 0 || got.CommentsSource != "" {
		t.Fatalf("Comments = %d source=%q, want no comment source", got.Comments, got.CommentsSource)
	}
	if !hasSourceError(got.Errors, sourceCbox) {
		t.Fatalf("Errors = %v, want cbox error", got.Errors)
	}
}

func TestFetchBatchPreservesInputOrder(t *testing.T) {
	c := newEngagementTestClient(t, testTransport{
		likes: map[string]int{
			"blog_3": 30,
			"blog_1": 10,
			"blog_2": 20,
		},
		comments: map[string]int{
			"blog_3": 300,
			"blog_1": 100,
			"blog_2": 200,
		},
		dates: map[string]string{
			"blog_3": "2026. 3. 3. 03:00",
			"blog_1": "2026. 3. 1. 01:00",
			"blog_2": "2026. 3. 2. 02:00",
		},
	})
	keys := []BatchKey{
		{BlogID: "blog", LogNo: "3"},
		{BlogID: "blog", LogNo: "1"},
		{BlogID: "blog", LogNo: "2"},
	}

	got := FetchBatch(context.Background(), c, keys, 2)

	if len(got) != len(keys) {
		t.Fatalf("len = %d, want %d", len(got), len(keys))
	}
	wantComments := []int{300, 100, 200}
	wantLikes := []int{30, 10, 20}
	for i := range got {
		if got[i].Comments != wantComments[i] {
			t.Fatalf("snapshot %d comments = %d, want %d", i, got[i].Comments, wantComments[i])
		}
		if got[i].Likes == nil || *got[i].Likes != wantLikes[i] {
			t.Fatalf("snapshot %d likes = %v, want %d", i, got[i].Likes, wantLikes[i])
		}
	}
}

func TestFetchBatchBatchesReactionAPI(t *testing.T) {
	var reactionCalls atomic.Int32
	c := newEngagementTestClient(t, testTransport{
		likes:         generatedLikes(45),
		comments:      generatedComments(45),
		dates:         generatedDates(45),
		reactionCalls: &reactionCalls,
	})
	keys := make([]BatchKey, 45)
	for i := range keys {
		keys[i] = BatchKey{BlogID: "blog", LogNo: strconv.Itoa(i + 1)}
	}

	got := FetchBatch(context.Background(), c, keys, 8)

	if len(got) != 45 {
		t.Fatalf("len = %d, want 45", len(got))
	}
	if reactionCalls.Load() != 3 {
		t.Fatalf("reaction calls = %d, want 3", reactionCalls.Load())
	}
}

func newEngagementTestClient(t *testing.T, tt testTransport) *client.Client {
	t.Helper()
	c := client.New(&config.Config{BaseURL: "https://m.blog.naver.com"}, time.Second, 0)
	c.NoCache = true
	c.HTTPClient = &http.Client{Transport: tt.roundTripper()}
	return c
}

type testTransport struct {
	likes          map[string]int
	comments       map[string]int
	dates          map[string]string
	commentsStatus int
	reactionCalls  *atomic.Int32
}

func (tt testTransport) roundTripper() http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Path, "/blogserver/like/v1/search/contents"):
			if tt.reactionCalls != nil {
				tt.reactionCalls.Add(1)
			}
			return jsonResponse(http.StatusOK, reactionBody(req.URL.Query().Get("q"), tt.likes)), nil
		case strings.Contains(req.URL.Path, "/comments-info"):
			if tt.commentsStatus != 0 && tt.commentsStatus >= 400 {
				return jsonResponse(tt.commentsStatus, `{"isSuccess":false,"message":"rate limited"}`), nil
			}
			_, logNo := blogAndLogFromCommentsInfo(req.URL.Path)
			return jsonResponse(http.StatusOK, fmt.Sprintf(`{"isSuccess":true,"result":{"totalCount":0,"blogNo":%d}}`, 1000+atoi(logNo))), nil
		case strings.Contains(req.URL.Path, "/web_naver_list_jsonp.json"):
			if tt.commentsStatus != 0 && tt.commentsStatus >= 400 {
				return jsonResponse(tt.commentsStatus, `{"success":false,"message":"rate limited"}`), nil
			}
			logNo := logNoFromObjectID(req.URL.Query().Get("objectId"))
			key := "blog_" + logNo
			return jsonResponse(http.StatusOK, fmt.Sprintf(`{"success":true,"result":{"commentList":[],"count":{"total":%d}}}`, tt.comments[key])), nil
		case req.URL.Host == "blog.naver.com" && req.URL.Path == "/PostView.naver":
			key := req.URL.Query().Get("blogId") + "_" + req.URL.Query().Get("logNo")
			return htmlResponse(http.StatusOK, postViewBody(tt.dates[key])), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s", req.URL.String())
		}
	})
}

func reactionBody(q string, likes map[string]int) string {
	ids := idsFromReactionQ(q)
	var b strings.Builder
	b.WriteString(`{"contents":[`)
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"contentsId":%q,"reactions":[{"reactionType":"like","count":%d}]}`, id, likes[id])
	}
	b.WriteString(`]}`)
	return b.String()
}

func idsFromReactionQ(q string) []string {
	q = strings.TrimSpace(q)
	q = strings.TrimPrefix(q, "BLOG[")
	q = strings.TrimSuffix(q, "]")
	if q == "" {
		return nil
	}
	return strings.Split(q, ",")
}

func blogAndLogFromCommentsInfo(path string) (string, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+4 < len(parts); i++ {
		if parts[i] == "api" && parts[i+1] == "blogs" && parts[i+3] == "posts" {
			blogID, _ := url.PathUnescape(parts[i+2])
			logNo, _ := url.PathUnescape(parts[i+4])
			return blogID, logNo
		}
	}
	return "", ""
}

func logNoFromObjectID(objectID string) string {
	parts := strings.Split(objectID, "_")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func postViewBody(date string) string {
	if date == "" {
		return `<html><body><em id="commentCount" class="num_cmt"></em></body></html>`
	}
	return fmt.Sprintf(`<html><body><span class="se_publishDate pcol2">%s</span><em id="commentCount" class="num_cmt"></em></body></html>`, date)
}

func generatedLikes(n int) map[string]int {
	out := make(map[string]int, n)
	for i := 1; i <= n; i++ {
		out[fmt.Sprintf("blog_%d", i)] = i
	}
	return out
}

func generatedComments(n int) map[string]int {
	out := make(map[string]int, n)
	for i := 1; i <= n; i++ {
		out[fmt.Sprintf("blog_%d", i)] = i * 10
	}
	return out
}

func generatedDates(n int) map[string]string {
	out := make(map[string]string, n)
	for i := 1; i <= n; i++ {
		out[fmt.Sprintf("blog_%d", i)] = "2026. 3. 1. 01:00"
	}
	return out
}

func jsonResponse(status int, body string) *http.Response {
	return response(status, "application/json", body)
}

func htmlResponse(status int, body string) *http.Response {
	return response(status, "text/html", body)
}

func response(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func hasSourceError(errs []error, source string) bool {
	for _, err := range errs {
		if strings.HasPrefix(err.Error(), source+":") {
			return true
		}
	}
	return false
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
