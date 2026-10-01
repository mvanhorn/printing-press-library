package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/config"
)

func TestCookieConfiguredHealthProbeReportsServerStatus(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/gateway/csrf" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := New(&config.Config{BaseURL: srv.URL, RapidapiCookie: "test-session"}, time.Second, 0)
	c.NoCache = true
	c.HTTPClient.Transport = srv.Client().Transport
	_, err := c.Get(context.Background(), "/gateway/csrf", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("health probe error = %v, want HTTP 503 APIError", err)
	}
	if requests != 1 {
		t.Fatalf("health probe sent %d requests, want one", requests)
	}
}

func TestCookieBootstrapFailureStopsGraphQLRequest(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"server error", 503, `{}`, "503"},
		{"malformed response", 200, `{`, "decode csrf"},
		{"missing token", 200, `{}`, "missing csrfToken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/gateway/csrf" {
					requests++
					w.WriteHeader(401)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := New(&config.Config{BaseURL: srv.URL, RapidapiCookie: "test-session"}, time.Second, 0)
			c.HTTPClient = srv.Client()
			_, _, err := c.Post(context.Background(), "/gateway/graphql", map[string]any{"query": "query { viewer { id } }"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if requests != 0 {
				t.Fatalf("sent %d GraphQL requests after failed bootstrap", requests)
			}
		})
	}
}

func TestCookieBootstrapAndGraphQLRedirectsDoNotRebootstrap(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	bootstrapCalls, graphqlCalls := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gateway/csrf":
			bootstrapCalls++
			if bootstrapCalls > 1 {
				http.Error(w, "unexpected rebootstrap", 503)
				return
			}
			http.Redirect(w, r, "/gateway/csrf-final", http.StatusTemporaryRedirect)
		case "/gateway/csrf-final":
			_, _ = w.Write([]byte(`{"csrfToken":"session-csrf"}`))
		case "/gateway/graphql":
			http.Redirect(w, r, "/gateway/graphql-final", http.StatusTemporaryRedirect)
		case "/gateway/graphql-final":
			graphqlCalls++
			if r.Header.Get("x-csrf-token") != "session-csrf" {
				t.Errorf("lost CSRF token across redirect")
			}
			_, _ = w.Write([]byte(`{"data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(&config.Config{BaseURL: srv.URL, RapidapiCookie: "test-session"}, time.Second, 0)
	// Keep the production redirect handler and cookie jar, while allowing
	// the test's plain HTTP server instead of the provider's HTTP/2 transport.
	c.HTTPClient.Transport = srv.Client().Transport
	if _, _, err := c.Post(context.Background(), "/gateway/graphql", map[string]any{"query": "query { viewer { id } }"}); err != nil {
		t.Fatal(err)
	}
	if bootstrapCalls != 1 || graphqlCalls != 1 {
		t.Fatalf("bootstrap=%d GraphQL=%d, want one each", bootstrapCalls, graphqlCalls)
	}
}
