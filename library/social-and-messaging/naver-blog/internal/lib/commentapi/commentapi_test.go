package commentapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetCommentsResolvesBlogNoAndSetsReferer(t *testing.T) {
	var sawInfo, sawCbox atomic.Bool
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/blogs/perfect62/posts/224286416663/comments-info":
			sawInfo.Store(true)
			if got := r.Header.Get("User-Agent"); !strings.Contains(got, "iPhone") {
				t.Errorf("comments-info User-Agent = %q, want mobile browser", got)
			}
			writeJSON(t, w, map[string]any{
				"isSuccess": true,
				"result": map[string]any{
					"totalCount": 1,
					"blogNo":     4636571,
				},
			})
		case "/web_naver_list_jsonp.json":
			sawCbox.Store(true)
			if got := r.Header.Get("Referer"); got != "https://m.blog.naver.com/perfect62/224286416663" {
				t.Errorf("Referer = %q, want mobile post URL", got)
			}
			if got := r.Header.Get("User-Agent"); !strings.Contains(got, "iPhone") {
				t.Errorf("cbox User-Agent = %q, want mobile browser", got)
			}
			q := r.URL.Query()
			if got := q.Get("objectId"); got != "4636571_201_224286416663" {
				t.Errorf("objectId = %q, want numeric blogNo object id", got)
			}
			for k, want := range map[string]string{
				"ticket":     "blog",
				"templateId": "default",
				"pool":       "blogid",
				"lang":       "ko",
				"pageSize":   "100",
				"listType":   "OBJECT",
				"pageType":   "more",
				"page":       "1",
				"initialize": "true",
			} {
				if got := q.Get(k); got != want {
					t.Errorf("%s = %q, want %q", k, got, want)
				}
			}
			writeCboxPage(t, w, 1, []map[string]any{
				commentJSON("1", "1", 1, "hello<br>world &amp; done", nil),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	withEndpoints(t, "https://cbox.test", "https://mobile.test/api/blogs")

	got, total, err := GetComments(context.Background(), client, "perfect62", "224286416663", GetOptions{})
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if !sawInfo.Load() || !sawCbox.Load() {
		t.Fatalf("expected both comments-info and cbox endpoints to be called; info=%v cbox=%v", sawInfo.Load(), sawCbox.Load())
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(got) != 1 {
		t.Fatalf("len(comments) = %d, want 1", len(got))
	}
	if got[0].Contents != "hello\nworld & done" {
		t.Errorf("clean contents = %q", got[0].Contents)
	}
	if got[0].RegTimeUTC.IsZero() {
		t.Error("RegTimeUTC should be parsed from regTimeGmt")
	}
}

func TestGetCommentsPaginationStopsAtTotal(t *testing.T) {
	var requests atomic.Int32
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Query().Get("page") {
		case "1":
			writeCboxPage(t, w, 2, []map[string]any{
				commentJSON("1", "1", 1, "one", nil),
			})
		case "2":
			writeCboxPage(t, w, 2, []map[string]any{
				commentJSON("2", "2", 1, "two", nil),
			})
		default:
			t.Fatalf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	withEndpoints(t, "https://cbox.test", "")

	got, total, err := GetCommentsByObjectID(context.Background(), client, "blog", "99", 12, "99", GetOptions{All: true})
	if err != nil {
		t.Fatalf("GetCommentsByObjectID: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(got) != 2 {
		t.Fatalf("len(comments) = %d, want 2", len(got))
	}
	if requests.Load() != 2 {
		t.Errorf("requests = %d, want 2", requests.Load())
	}
}

func TestGetCommentsPaginationStopsOnEmptyPage(t *testing.T) {
	var requests atomic.Int32
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		requests.Add(1)
		if page == "1" {
			writeCboxPage(t, w, 99, []map[string]any{
				commentJSON("1", "1", 1, "one", nil),
			})
			return
		}
		if page == "2" {
			writeCboxPage(t, w, 99, nil)
			return
		}
		t.Fatalf("unexpected page %q", page)
	}))
	withEndpoints(t, "https://cbox.test", "")

	got, total, err := GetCommentsByObjectID(context.Background(), client, "blog", "99", 12, "99", GetOptions{All: true})
	if err != nil {
		t.Fatalf("GetCommentsByObjectID: %v", err)
	}
	if total != 99 {
		t.Errorf("total = %d, want 99", total)
	}
	if len(got) != 1 {
		t.Fatalf("len(comments) = %d, want 1", len(got))
	}
	if requests.Load() != 2 {
		t.Errorf("requests = %d, want 2", requests.Load())
	}
}

func TestGetCommentsFlattensReplyList(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCboxPage(t, w, 2, []map[string]any{
			commentJSON("parent", "parent", 1, "parent body", []map[string]any{
				commentJSON("reply", "parent", 2, "reply body", nil),
			}),
		})
	}))
	withEndpoints(t, "https://cbox.test", "")

	got, total, err := GetCommentsByObjectID(context.Background(), client, "blog", "99", 12, "99", GetOptions{})
	if err != nil {
		t.Fatalf("GetCommentsByObjectID: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(got) != 2 {
		t.Fatalf("len(comments) = %d, want 2", len(got))
	}
	if got[0].CommentNo != "parent" || got[0].ReplyLevel != 1 {
		t.Errorf("parent row = %+v", got[0])
	}
	if got[1].CommentNo != "reply" || got[1].ParentCommentNo != "parent" || got[1].ReplyLevel != 2 {
		t.Errorf("reply row = %+v", got[1])
	}
}

func TestGetCommentsSurfacesMediaAndHomepageFields(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imageComment := commentJSON("image", "image", 1, "", nil)
		imageComment["commentType"] = "img"
		imageComment["imageList"] = []string{
			"https://comment.example/one.jpg",
			"https://comment.example/one.jpg",
			" ",
		}
		imageComment["userHomepageUrl"] = "https://blog.naver.com/commenter"

		stickerComment := commentJSON("sticker", "sticker", 1, "", nil)
		stickerComment["commentType"] = "sticker"
		stickerComment["stickerId"] = "line-123"

		writeCboxPage(t, w, 2, []map[string]any{imageComment, stickerComment})
	}))
	withEndpoints(t, "https://cbox.test", "")

	got, _, err := GetCommentsByObjectID(context.Background(), client, "blog", "99", 12, "99", GetOptions{})
	if err != nil {
		t.Fatalf("GetCommentsByObjectID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(comments) = %d, want 2", len(got))
	}
	if len(got[0].ImageURLs) != 1 || got[0].ImageURLs[0] != "https://comment.example/one.jpg" {
		t.Errorf("image_urls = %#v", got[0].ImageURLs)
	}
	if got[0].UserHomepageURL != "https://blog.naver.com/commenter" {
		t.Errorf("user_homepage_url = %q", got[0].UserHomepageURL)
	}
	if got[1].StickerID != "line-123" {
		t.Errorf("sticker_id = %q", got[1].StickerID)
	}
}

func TestGetCommentsRateLimitError(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	withEndpoints(t, "https://cbox.test", "")

	_, _, err := GetCommentsByObjectID(context.Background(), client, "blog", "99", 12, "99", GetOptions{})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	rle, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("err = %T, want *RateLimitError", err)
	}
	if rle.RetryAfter.String() != "2s" {
		t.Errorf("RetryAfter = %s, want 2s", rle.RetryAfter)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockClient(handler http.Handler) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			resp := rr.Result()
			if resp.Body == nil {
				resp.Body = io.NopCloser(strings.NewReader(""))
			}
			return resp, nil
		}),
	}
}

func withEndpoints(t *testing.T, cboxBase, infoBase string) {
	t.Helper()
	oldEndpointBase := endpointBase
	oldCommentsInfoBase := commentsInfoBase
	endpointBase = cboxBase
	if infoBase != "" {
		commentsInfoBase = infoBase
	}
	t.Cleanup(func() {
		endpointBase = oldEndpointBase
		commentsInfoBase = oldCommentsInfoBase
	})
}

func writeCboxPage(t *testing.T, w http.ResponseWriter, total int, comments []map[string]any) {
	t.Helper()
	if comments == nil {
		comments = []map[string]any{}
	}
	writeJSON(t, w, map[string]any{
		"success": true,
		"code":    "1000",
		"result": map[string]any{
			"commentList": comments,
			"count": map[string]any{
				"total": total,
			},
		},
	})
}

func commentJSON(commentNo, parentNo string, level int, contents string, replies []map[string]any) map[string]any {
	return map[string]any{
		"commentNo":        commentNo,
		"parentCommentNo":  parentNo,
		"replyLevel":       level,
		"replyCount":       len(replies),
		"replyAllCount":    len(replies),
		"contents":         contents,
		"userName":         "윤하",
		"userProfileImage": "https://blogimgs.pstatic.net/imgs/emot/emo12.gif",
		"regTime":          "2026-05-17T02:55:02+0900",
		"regTimeGmt":       "2026-05-16T17:55:02+0000",
		"modTime":          "",
		"modTimeGmt":       "",
		"sympathyCount":    3,
		"antipathyCount":   0,
		"commentType":      "txt",
		"secret":           false,
		"visible":          true,
		"hiddenByCleanbot": false,
		"replyList":        replies,
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encoding fixture: %v", err)
	}
}
