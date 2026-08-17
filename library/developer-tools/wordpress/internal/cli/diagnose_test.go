package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
)

func TestClassifyDiagnose(t *testing.T) {
	probe := func(status int, code, contentType, body string) diagnoseProbe {
		return diagnoseProbe{Status: status, Code: code, contentType: contentType, body: []byte(body)}
	}

	tests := []struct {
		name   string
		input  diagnoseEvidence
		want   string
		plugin string
	}{
		{
			name: "both collection forms work",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusOK, "", "application/json", `[{"id":1}]`),
				RoutePosts:  probe(http.StatusOK, "", "application/json", `[{"id":1}]`),
			},
			want: "ok",
		},
		{
			name: "settings require valid credentials",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusOK, "", "application/json", `[]`),
				RoutePosts:  probe(http.StatusOK, "", "application/json", `[]`),
				AnonSettings: probe(http.StatusUnauthorized, "rest_cannot_view", "application/json",
					`{"code":"rest_cannot_view"}`),
				AuthSettings: probe(http.StatusOK, "", "application/json", `{}`),
				HasAuth:      true,
			},
			want: "ok-auth-required",
		},
		{
			name: "pretty path blocked",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusForbidden, "", "text/html", `<html>blocked</html>`),
				RoutePosts:  probe(http.StatusOK, "", "application/json", `[]`),
			},
			want: "path-blocked",
		},
		{
			name: "rate limiting takes precedence over a working fallback",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusTooManyRequests, "", "text/html", `<html>slow down</html>`),
				RoutePosts:  probe(http.StatusOK, "", "application/json", `[]`),
			},
			want: "rate-limited",
		},
		{
			name: "known app layer plugin",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusUnauthorized, "rest_login_required", "application/json", `{"code":"rest_login_required"}`),
				RoutePosts:  probe(http.StatusUnauthorized, "rest_login_required", "application/json", `{"code":"rest_login_required"}`),
			},
			want:   "app-layer-block",
			plugin: "Disable WP REST API",
		},
		{
			name: "cloudflare challenge",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusForbidden, "", "text/html", `Just a moment...`),
				RoutePosts:  probe(http.StatusForbidden, "", "text/html", `Attention Required`),
			},
			want: "bot-challenge",
		},
		{
			name: "challenge header wins without html",
			input: diagnoseEvidence{
				PrettyPosts: diagnoseProbe{Status: http.StatusForbidden, headers: http.Header{"Cf-Mitigated": []string{"challenge"}}},
				RoutePosts:  probe(http.StatusForbidden, "", "application/octet-stream", "blocked"),
			},
			want: "bot-challenge",
		},
		{
			name: "rate limit wins even with html",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusTooManyRequests, "", "text/html", `<html>slow down</html>`),
				RoutePosts:  probe(http.StatusTooManyRequests, "", "text/html", `<html>slow down</html>`),
			},
			want: "rate-limited",
		},
		{
			name: "collection route missing while root works",
			input: diagnoseEvidence{
				PrettyRoot:  probe(http.StatusOK, "", "application/json", `{"routes":{}}`),
				RouteRoot:   probe(http.StatusOK, "", "application/json", `{"routes":{}}`),
				PrettyPosts: probe(http.StatusNotFound, "rest_no_route", "application/json", `{"code":"rest_no_route"}`),
				RoutePosts:  probe(http.StatusNotFound, "rest_no_route", "application/json", `{"code":"rest_no_route"}`),
			},
			want: "route-missing",
		},
		{
			name: "rest root absent",
			input: diagnoseEvidence{
				PrettyRoot:  probe(http.StatusNotFound, "", "text/html", `not found`),
				RouteRoot:   probe(http.StatusNotFound, "", "text/html", `not found`),
				PrettyPosts: probe(http.StatusNotFound, "", "text/html", `not found`),
				RoutePosts:  probe(http.StatusNotFound, "", "text/html", `not found`),
			},
			want: "no-rest-api",
		},
		{
			name: "authorization header stripped",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusOK, "", "application/json", `[]`),
				RoutePosts:  probe(http.StatusOK, "", "application/json", `[]`),
				AnonSettings: probe(http.StatusUnauthorized, "rest_cannot_view", "application/json",
					`{"code":"rest_cannot_view"}`),
				AuthSettings: probe(http.StatusForbidden, "rest_cannot_view", "application/json",
					`{"code":"rest_cannot_view"}`),
				AuthHeaderTest: probe(http.StatusOK, "", "application/json",
					`{"status":"critical","description":"The Authorization header is missing."}`),
				HasAuth: true,
			},
			want: "auth-header-stripped",
		},
		{
			name: "credentials rejected when header arrives",
			input: diagnoseEvidence{
				PrettyPosts: probe(http.StatusOK, "", "application/json", `[]`),
				RoutePosts:  probe(http.StatusOK, "", "application/json", `[]`),
				AnonSettings: probe(http.StatusUnauthorized, "rest_cannot_view", "application/json",
					`{"code":"rest_cannot_view"}`),
				AuthSettings: probe(http.StatusUnauthorized, "rest_cannot_view", "application/json",
					`{"code":"rest_cannot_view"}`),
				AuthHeaderTest: probe(http.StatusOK, "", "application/json", `{"success":true,"status":"good"}`),
				HasAuth:        true,
			},
			want: "credentials-rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyDiagnose(tt.input)
			if got.Verdict != tt.want {
				t.Fatalf("verdict = %q, want %q", got.Verdict, tt.want)
			}
			if got.Plugin != tt.plugin {
				t.Fatalf("plugin = %q, want %q", got.Plugin, tt.plugin)
			}
		})
	}
}

func TestParseWordPressErrorIncludesInvalidParameterDetails(t *testing.T) {
	code, details := parseWordPressError([]byte(`{
		"code":"rest_invalid_param",
		"message":"Invalid parameter(s): per_page",
		"data":{"details":{"per_page":{"code":"rest_out_of_bounds","message":"Must be between 1 and 100"}}}
	}`))
	if code != "rest_invalid_param" {
		t.Fatalf("code = %q", code)
	}
	parameterDetails, ok := details["parameter_details"].(map[string]any)
	if !ok || parameterDetails["per_page"] == nil {
		t.Fatalf("parameter details were not preserved: %#v", details)
	}
}

func TestKnownRESTBlocker(t *testing.T) {
	tests := map[string]string{
		"rest_cannot_access":               "Disable REST API",
		"rest_not_logged_in":               "WordPress handbook login-required filter",
		"rest_login_required":              "Disable WP REST API",
		"itsec_rest_api_access_restricted": "Solid/Kadence Security Restricted Access",
		"aios_user_lists_forbidden":        "All-In-One Security",
		"aios_user_details_forbidden":      "All-In-One Security",
		"rest_authentication_error":        "Perfmatters",
	}
	for code, want := range tests {
		if got := knownRESTBlocker(code); got != want {
			t.Errorf("knownRESTBlocker(%q) = %q, want %q", code, got, want)
		}
	}
	if got := knownRESTBlocker("unknown"); got != "" {
		t.Errorf("unknown code mapped to %q", got)
	}
}

func TestParseWordPressAPILink(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		want   string
	}{
		{name: "missing", header: http.Header{}, want: ""},
		{
			name:   "quoted rel among links",
			header: http.Header{"Link": []string{`<https://example.com/feed/>; rel="alternate", <https://example.com/custom-api/>; rel="https://api.w.org/"`}},
			want:   "https://example.com/custom-api/",
		},
		{
			name:   "unquoted rel",
			header: http.Header{"Link": []string{`<https://example.com/wp-json/>; rel=https://api.w.org/`}},
			want:   "https://example.com/wp-json/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseWordPressAPILink(tt.header); got != tt.want {
				t.Fatalf("parseWordPressAPILink() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSameWordPressSite(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		want        bool
	}{
		{name: "same origin and install", left: "https://example.com/blog", right: "https://EXAMPLE.com/blog/", want: true},
		{name: "different host", left: "https://example.com", right: "https://attacker.example", want: false},
		{name: "different scheme", left: "https://example.com", right: "http://example.com", want: false},
		{name: "different install path", left: "https://example.com/blog", right: "https://example.com/shop", want: false},
		{name: "missing configured site", left: "https://example.com", right: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameWordPressSite(tt.left, tt.right); got != tt.want {
				t.Fatalf("sameWordPressSite(%q, %q) = %v, want %v", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestRuntimeRESTURLUsesRegisteredFallback(t *testing.T) {
	runtime := wordpressRuntime{Origin: "https://example.com", RestRouteFallback: true}
	got := runtimeRESTURL(runtime, "/wp/v2/posts", url.Values{"per_page": []string{"1"}})
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("rest_route") != "/wp/v2/posts" || parsed.Query().Get("per_page") != "1" {
		t.Fatalf("fallback URL = %q", got)
	}
}

func TestRunDiagnoseKeepsProbeOrder(t *testing.T) {
	requests := make([]string, 0)
	var requestsMu sync.Mutex
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		label := r.Method + " " + r.URL.Path
		if restRoute := r.URL.Query().Get("rest_route"); restRoute != "" {
			label += " rest_route=" + restRoute
		}
		requestsMu.Lock()
		requests = append(requests, label)
		requestsMu.Unlock()

		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/":
			w.Header().Set("Link", "<"+server.URL+"/wp-json/>; rel=\"https://api.w.org/\"")
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/wp-json/wp/v2/posts" || r.URL.Query().Get("rest_route") == "/wp/v2/posts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"id":1}]`)
		case r.URL.Path == "/wp-json/wp/v2/settings":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"code":"rest_forbidden","message":"login required"}`)
		case r.URL.Path == "/wp-json/" || r.URL.Query().Get("rest_route") == "/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"routes":{"/wp/v2/posts":{}},"namespaces":["wp/v2"]}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	out, classification := runDiagnose(context.Background(), server.Client(), wordpressRuntime{Origin: server.URL, AuthHeaders: map[string]string{}})
	if classification.Verdict != "ok" || out.Verdict != "ok" {
		t.Fatalf("verdict = %q / %q", classification.Verdict, out.Verdict)
	}
	want := []string{
		"HEAD /",
		"GET /wp-json/wp/v2/posts",
		"GET / rest_route=/wp/v2/posts",
		"GET /wp-json/wp/v2/settings",
		"GET /wp-json/",
		"GET / rest_route=/",
	}
	requestsMu.Lock()
	got := append([]string(nil), requests...)
	requestsMu.Unlock()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("probe order = %#v, want %#v", got, want)
	}
}
