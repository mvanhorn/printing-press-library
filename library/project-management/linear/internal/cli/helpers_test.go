package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestLooksLikeGraphQLGETError(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "real CSRF body",
			msg:  `GET /graphql returned HTTP 400: {"errors":[{"message":"This operation has been blocked as a potential Cross-Site Request Forgery"}]}`,
			want: true,
		},
		{
			name: "content-type hint",
			msg:  `HTTP 400: Please either specify a 'content-type' header`,
			want: true,
		},
		{
			name: "unrelated 400",
			msg:  `HTTP 400: invalid input: missing field 'id'`,
			want: false,
		},
		{
			name: "auth error",
			msg:  `HTTP 401: invalid api key`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeGraphQLGETError(tc.msg); got != tc.want {
				t.Fatalf("looksLikeGraphQLGETError(%q) = %v; want %v", tc.msg, got, tc.want)
			}
		})
	}
}

func TestClassifyAPIError_CSRFGetsClassifiedHint(t *testing.T) {
	raw := errors.New(`GET /graphql returned HTTP 400: {"errors":[{"message":"This operation has been blocked as a potential Cross-Site Request Forgery"}]}`)
	got := classifyAPIError(raw)
	if got == nil {
		t.Fatal("classifyAPIError returned nil")
	}
	msg := got.Error()
	if !strings.Contains(msg, "issues list") {
		t.Errorf("expected hint to mention 'issues list'; got: %s", msg)
	}
	if !strings.Contains(msg, "rejects GET") {
		t.Errorf("expected hint to mention 'rejects GET'; got: %s", msg)
	}
}

func TestClassifyAPIError_PlainAuth400StillRoutes(t *testing.T) {
	raw := errors.New(`POST /graphql returned HTTP 400: api_key is required`)
	got := classifyAPIError(raw)
	if got == nil {
		t.Fatal("classifyAPIError returned nil")
	}
	if !strings.Contains(got.Error(), "LINEAR_API_KEY") {
		t.Errorf("expected auth hint; got: %s", got.Error())
	}
}

func TestClassifyAPIError_Unrelated400PassesThrough(t *testing.T) {
	raw := errors.New(`POST /graphql returned HTTP 400: invalid field`)
	got := classifyAPIError(raw)
	if got == nil {
		t.Fatal("classifyAPIError returned nil")
	}
	if strings.Contains(got.Error(), "issues list") {
		t.Errorf("unrelated 400 should not suggest issues list; got: %s", got.Error())
	}
}
