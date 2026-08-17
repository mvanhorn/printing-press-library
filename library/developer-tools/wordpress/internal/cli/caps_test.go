package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseAllowHeader(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "normalizes sorts and deduplicates", value: "POST, GET, post, DELETE", want: []string{"DELETE", "GET", "POST"}},
		{name: "empty", value: "", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAllowHeader(tt.value); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseAllowHeader() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMethodDifference(t *testing.T) {
	if got, want := methodDifference([]string{"DELETE", "GET", "POST"}, []string{"GET"}), []string{"DELETE", "POST"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("methodDifference() = %#v, want %#v", got, want)
	}
}

func TestRunCapsPreservesPerRouteErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wp-json/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"routes":{"/wp/v2/a":{},"/wp/v2/b":{},"/wp/v2/a/(?P<id>[\\d]+)":{}}}`)
		case r.Method == http.MethodOptions && r.URL.Path == "/wp-json/wp/v2/a":
			if r.Header.Get("Authorization") == "Basic configured" {
				w.Header().Set("Allow", "GET, POST")
			} else {
				w.Header().Set("Allow", "GET")
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodOptions && r.URL.Path == "/wp-json/wp/v2/b":
			if r.Header.Get("Authorization") == "Basic configured" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, `{"code":"upstream_failed","message":"edge failure"}`)
				return
			}
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	runtime := wordpressRuntime{
		Origin:      server.URL,
		AuthHeaders: map[string]string{"Authorization": "Basic configured"},
		HasAuth:     true,
	}
	out, err := runCaps(context.Background(), server.Client(), runtime, "wp/v2", 200)
	if err != nil {
		t.Fatal(err)
	}
	if out.ScannedRoutes != 2 || out.FailedRoutes != 1 {
		t.Fatalf("scan counts = %d scanned, %d failed", out.ScannedRoutes, out.FailedRoutes)
	}
	if got, want := out.Routes[0].Delta, []string{"POST"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("route delta = %#v, want %#v", got, want)
	}
	if out.Routes[1].Error == "" {
		t.Fatal("failed credential probe was silently reported as no permissions")
	}
}
