package cli

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestNormalizeSiteURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "bare host", input: "example.com", want: "https://example.com"},
		{name: "http trailing slash", input: "http://example.com/", want: "http://example.com"},
		{name: "https", input: "https://example.com", want: "https://example.com"},
		{name: "subdirectory", input: "https://example.com/blog/", want: "https://example.com/blog"},
		{name: "unsupported scheme", input: "ftp://example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSiteURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizeSiteURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWordPressRESTRootFromLinkHeaders(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "multiple relations",
			values: []string{`<https://example.com/feed>; rel="alternate", <https://example.com/wp-json/>; rel="https://api.w.org/", <https://wp.me/a,b>; rel=shortlink`},
			want:   "https://example.com/wp-json/",
		},
		{
			name:   "several header lines",
			values: []string{`<https://example.com/a>; rel=alternate`, `<https://example.com/?rest_route=/>; rel='https://api.w.org/'`},
			want:   "https://example.com/?rest_route=/",
		},
		{name: "missing", values: []string{`<https://example.com/>; rel=shortlink`}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wordpressRESTRootFromLinkHeaders(tt.values); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWordPressRESTRootFromHTML(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		base string
		want string
	}{
		{name: "rel before href", doc: `<html><head><link rel="https://api.w.org/" href="/wp-json/"></head></html>`, base: "https://example.com/blog", want: "https://example.com/wp-json/"},
		{name: "href before rel", doc: `<head><link href='https://example.com/?rest_route=/' rel='alternate https://api.w.org/'></head>`, base: "https://example.com", want: "https://example.com/?rest_route=/"},
		{name: "ignore body", doc: `<head></head><body><link rel="https://api.w.org/" href="/wrong"></body>`, base: "https://example.com", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wordpressRESTRootFromHTML(tt.doc, tt.base); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWordPressSitePureHelpers(t *testing.T) {
	tests := []struct {
		name     string
		siteName string
		root     string
		wantName string
		fallback bool
	}{
		{name: "host name", siteName: "Client.Example.COM", root: "https://example.com/wp-json/", wantName: "client-example-com"},
		{name: "rest route", siteName: "Client Blog!", root: "https://example.com/?rest_route=/", wantName: "client-blog", fallback: true},
		{name: "index rest route", siteName: "x", root: "https://example.com/index.php?rest_route=/", wantName: "x", fallback: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeWordPressSiteName(tt.siteName); got != tt.wantName {
				t.Fatalf("sanitize = %q, want %q", got, tt.wantName)
			}
			if got := usesRestRouteFallback(tt.root); got != tt.fallback {
				t.Fatalf("usesRestRouteFallback = %v, want %v", got, tt.fallback)
			}
		})
	}
	if got := siteNameFromURL("https://Client.Example.COM:8443/blog"); got != "client-example-com" {
		t.Fatalf("siteNameFromURL = %q, want client-example-com", got)
	}
}

func TestWordPressAuthorizeURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "application passwords", raw: `{"application-passwords":{"endpoints":{"authorization":"https://example.com/wp-admin/authorize-application.php"}}}`, want: "https://example.com/wp-admin/authorize-application.php"},
		{name: "missing", raw: `{}`, want: ""},
		{name: "invalid", raw: `{`, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wordpressAuthorizeURL(json.RawMessage(tt.raw)); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDiscoveryURL(t *testing.T) {
	base, err := normalizeSiteURL("https://example.com/blog/")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(base)
	tests := []struct{ input, want string }{
		{input: "/wp-json/", want: "https://example.com/wp-json/"},
		{input: "https://api.example.net/wp-json/", want: "https://api.example.net/wp-json/"},
	}
	for _, tt := range tests {
		if got := resolveDiscoveryURL(parsed, tt.input); got != tt.want {
			t.Fatalf("resolveDiscoveryURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
