package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPinterestAuthorizationCodeTokenRequest(t *testing.T) {
	const clientID, secret = "test-client", "test-secret"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, gotSecret, ok := r.BasicAuth()
		if !ok || id != clientID || gotSecret != secret {
			t.Errorf("Basic auth = %q, %q, %v", id, gotSecret, ok)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if r.Form.Has("client_secret") {
			t.Error("client_secret leaked into token form")
		}
		if r.Form.Get("continuous_refresh") != "true" {
			t.Errorf("continuous_refresh = %q", r.Form.Get("continuous_refresh"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ok"}`))
	})
	var requestURL string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestURL = r.URL.String()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.Result(), nil
	})}

	got, err := exchangeAuthorizationCode(context.Background(), httpClient, "https://api.example/oauth/token", clientID, secret, url.Values{
		"grant_type": {"authorization_code"}, "continuous_refresh": {"true"},
	})
	if err != nil || got.AccessToken != "ok" {
		t.Fatalf("exchangeAuthorizationCode() = %#v, %v", got, err)
	}
	if requestURL != "https://api.example/oauth/token" {
		t.Fatalf("request URL = %q", requestURL)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPinterestOAuthRedirectURI(t *testing.T) {
	if got := pinterestOAuthRedirectURI(8875); got != "http://localhost:8875/callback" {
		t.Fatalf("redirect URI = %q", got)
	}
}
