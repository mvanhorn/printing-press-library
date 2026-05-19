package naverurl

import (
	"net/url"
	"testing"
)

func TestCanonicalKey(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantBlog string
		wantLog  string
		wantOK   bool
	}{
		{
			name:     "desktop short form",
			input:    "https://blog.naver.com/selly9401/224234460263",
			wantBlog: "selly9401",
			wantLog:  "224234460263",
			wantOK:   true,
		},
		{
			name:     "mobile short form",
			input:    "https://m.blog.naver.com/selly9401/224234460263",
			wantBlog: "selly9401",
			wantLog:  "224234460263",
			wantOK:   true,
		},
		{
			name:     "PostView with full query",
			input:    "https://blog.naver.com/PostView.naver?blogId=selly9401&logNo=224234460263&redirect=Dlog",
			wantBlog: "selly9401",
			wantLog:  "224234460263",
			wantOK:   true,
		},
		{
			name:     "PostView minimal",
			input:    "https://blog.naver.com/PostView.naver?blogId=foo&logNo=123",
			wantBlog: "foo",
			wantLog:  "123",
			wantOK:   true,
		},
		{
			name:     "no scheme accepted",
			input:    "blog.naver.com/selly9401/224234460263",
			wantBlog: "selly9401",
			wantLog:  "224234460263",
			wantOK:   true,
		},
		{
			name:     "trailing slash",
			input:    "https://m.blog.naver.com/selly9401/224234460263/",
			wantBlog: "selly9401",
			wantLog:  "224234460263",
			wantOK:   true,
		},
		{
			name:   "wrong host",
			input:  "https://example.com/selly9401/224234460263",
			wantOK: false,
		},
		{
			name:   "naver.me shortener — not supported",
			input:  "https://naver.me/abc123",
			wantOK: false,
		},
		{
			name:   "blog.naver.com profile (no log_no)",
			input:  "https://blog.naver.com/selly9401",
			wantOK: false,
		},
		{
			name:   "non-digit log_no",
			input:  "https://m.blog.naver.com/selly9401/categoryList",
			wantOK: false,
		},
		{
			name:   "PostView missing logNo",
			input:  "https://blog.naver.com/PostView.naver?blogId=selly9401",
			wantOK: false,
		},
		{
			name:   "PostView non-digit logNo",
			input:  "https://blog.naver.com/PostView.naver?blogId=selly9401&logNo=abc",
			wantOK: false,
		},
		{
			name:   "empty",
			input:  "",
			wantOK: false,
		},
		{
			name:   "whitespace-only",
			input:  "   ",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBlog, gotLog, gotOK := CanonicalKey(tc.input)
			if gotOK != tc.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if gotBlog != tc.wantBlog {
				t.Errorf("blogID = %q, want %q", gotBlog, tc.wantBlog)
			}
			if gotLog != tc.wantLog {
				t.Errorf("logNo = %q, want %q", gotLog, tc.wantLog)
			}
		})
	}
}

func TestMobileURL(t *testing.T) {
	got := MobileURL("selly9401", "224234460263")
	want := "https://m.blog.naver.com/selly9401/224234460263"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPostViewURL(t *testing.T) {
	got := PostViewURL("selly9401", "224234460263")
	// Parse back so we don't depend on a particular query-param order.
	blogID, logNo, ok := CanonicalKey(got)
	if !ok {
		t.Fatalf("PostViewURL produced an URL CanonicalKey couldn't parse: %s", got)
	}
	if blogID != "selly9401" || logNo != "224234460263" {
		t.Errorf("round-trip failed: blogID=%q logNo=%q", blogID, logNo)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", got, err)
	}
	q := parsed.Query()
	if len(q) != 2 || q.Get("blogId") != "selly9401" || q.Get("logNo") != "224234460263" {
		t.Fatalf("PostViewURL query = %v, want only blogId and logNo", q)
	}
}
