package naverblog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureSearchHTML = `<html><body>
<ul class="lst_total">
  <li class="bx _bx" id="sp_blog_1">
    <div class="total_wrap">
      <a class="title_link _title" href="https://blog.naver.com/foodie123/22000001">
        성수동 <strong>맛집</strong> 베스트
      </a>
      <a class="name" href="https://blog.naver.com/foodie123">FoodieKim</a>
      <a class="api_txt_lines dsc_txt" href="https://blog.naver.com/foodie123/22000001">
        성수동에서 가장 핫한 맛집 다섯 곳을 정리했어요.
      </a>
      <span class="sub_time sub_txt">2024.03.10.</span>
    </div>
  </li>
  <li class="bx" id="sp_blog_2">
    <div class="total_wrap">
      <a class="title_link" href="https://blog.naver.com/walkerlee/300">
        Seoul Walking Tour Diary
      </a>
      <a class="user_info" href="https://blog.naver.com/walkerlee">walkerlee</a>
      <div class="dsc_txt">A weekend stroll from Hongdae to Yeonnam-dong.</div>
      <span class="sub_date">2024.04.01.</span>
    </div>
  </li>
  <li class="bx-related-search">noise card without title_link</li>
</ul>
</body></html>`

func newTestClient(srvURL string) *Client {
	c := New(nil, "test-ua")
	c.BaseURL = srvURL
	return c
}

func TestSearch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("where") != "blog" {
			t.Errorf("where: %q", r.URL.Query().Get("where"))
		}
		if r.URL.Query().Get("query") != "성수동 맛집" {
			t.Errorf("query: %q", r.URL.Query().Get("query"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(fixtureSearchHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	posts, err := c.Search(context.Background(), "성수동 맛집")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d (%+v)", len(posts), posts)
	}

	first := posts[0]
	if !strings.Contains(first.Title, "맛집") {
		t.Errorf("first title: %q", first.Title)
	}
	if first.URL != "https://blog.naver.com/foodie123/22000001" {
		t.Errorf("first url: %q", first.URL)
	}
	if first.Author != "FoodieKim" {
		t.Errorf("first author: %q", first.Author)
	}
	if !strings.Contains(first.Snippet, "성수동") {
		t.Errorf("first snippet: %q", first.Snippet)
	}
	if first.Posted != "2024.03.10." {
		t.Errorf("first posted: %q", first.Posted)
	}

	second := posts[1]
	if second.Author != "walkerlee" {
		t.Errorf("second author: %q", second.Author)
	}
	if !strings.Contains(second.Snippet, "Hongdae") {
		t.Errorf("second snippet: %q", second.Snippet)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	c := New(nil, "")
	if _, err := c.Search(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.Search(context.Background(), "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSearch_NoCards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>No results</p></body></html>`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	posts, err := c.Search(context.Background(), "no results")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("expected empty, got %+v", posts)
	}
}

func TestNew_DefaultUserAgentIsBrowserShaped(t *testing.T) {
	c := New(nil, "")
	if !strings.Contains(c.UserAgent, "Mozilla/") {
		t.Errorf("default UA should be browser-shaped: %q", c.UserAgent)
	}
}
