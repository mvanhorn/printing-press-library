package blogapi

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

func TestGetBlogInfoSetsRefererAndProjectsFields(t *testing.T) {
	var sawRequest atomic.Bool
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest.Store(true)
		if r.URL.Path != "/api/blogs/selly9401" {
			t.Fatalf("path = %q, want /api/blogs/selly9401", r.URL.Path)
		}
		if got := r.Header.Get("Referer"); got != "https://m.blog.naver.com/selly9401" {
			t.Errorf("Referer = %q, want mobile blog URL", got)
		}
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "iPhone") {
			t.Errorf("User-Agent = %q, want mobile browser", got)
		}
		writeJSON(t, w, map[string]any{
			"isSuccess": true,
			"result": map[string]any{
				"blogId":            "selly9401",
				"blogNo":            15439060,
				"blogName":          "빡언니의 소소한 일상",
				"nickName":          "빡언니다",
				"displayNickName":   "빡언니다(selly9401)",
				"officialBlog":      false,
				"powerBlog":         true,
				"dayVisitorCount":   4987,
				"subscriberCount":   2504,
				"totalVisitorCount": 13213195,
				"blogDirectoryName": "맛집",
				"profileImagePath":  "https://profile.example/p.jpg",
				"isAiPickBlog":      true,
				"isYearOfBlog":      true,
				"suggestionBlog":    false,
				"mobileTitleImage": map[string]any{
					"originalImage": "https://cover.example/orig.jpg",
					"croppedImage":  "https://cover.example/crop.jpg",
					"previewImage":  "https://cover.example/preview.jpg",
				},
			},
		})
	}))
	withEndpoint(t, "https://mobile.test/api/blogs")

	got, err := GetBlogInfo(context.Background(), client, "selly9401")
	if err != nil {
		t.Fatalf("GetBlogInfo: %v", err)
	}
	if !sawRequest.Load() {
		t.Fatal("expected blog endpoint to be called")
	}
	if got.BlogID != "selly9401" || got.BlogNo != 15439060 {
		t.Errorf("identity fields = %+v", got)
	}
	if got.SubscriberCount != 2504 || got.DayVisitorCount != 4987 || got.TotalVisitorCount != 13213195 {
		t.Errorf("traffic fields = %+v", got)
	}
	if !got.PowerBlog || !got.IsAIPickBlog || !got.IsYearOfBlog {
		t.Errorf("badge fields = %+v", got)
	}
	if got.DirectoryName != "맛집" {
		t.Errorf("directory_name = %q", got.DirectoryName)
	}
	if got.ProfileImageURL != "https://profile.example/p.jpg" {
		t.Errorf("profile_image_url = %q", got.ProfileImageURL)
	}
	if got.CoverImageURL != "https://cover.example/crop.jpg" {
		t.Errorf("cover_image_url = %q", got.CoverImageURL)
	}
}

func TestGetBlogInfoDecodesJSONP(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte(`callback({"isSuccess":true,"result":{"blogId":"blog","blogNo":12}})`))
	}))
	withEndpoint(t, "https://mobile.test/api/blogs")

	got, err := GetBlogInfo(context.Background(), client, "blog")
	if err != nil {
		t.Fatalf("GetBlogInfo: %v", err)
	}
	if got.BlogNo != 12 {
		t.Errorf("blog_no = %d, want 12", got.BlogNo)
	}
}

func TestGetBlogInfoRateLimitError(t *testing.T) {
	client := newMockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	withEndpoint(t, "https://mobile.test/api/blogs")

	_, err := GetBlogInfo(context.Background(), client, "blog")
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

func withEndpoint(t *testing.T, base string) {
	t.Helper()
	oldEndpointBase := endpointBase
	endpointBase = base
	t.Cleanup(func() {
		endpointBase = oldEndpointBase
	})
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encoding fixture: %v", err)
	}
}
