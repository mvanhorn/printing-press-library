// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package keywordapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCloudProjectOAuthWithoutDeveloperToken(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{"ideas", "/v25/customers/1234567890:generateKeywordIdeas", "{}"},
		{"historical", "/v25/customers/1234567890:generateKeywordHistoricalMetrics", "{}"},
		{"account", "/v25/customers/1234567890/googleAds:search", `{"query":"` + accountQuery + `"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			const body = `{"results":[]}`
			client, _, tokenCalls, adsCalls := newTestClient(t, testHandlers{
				ads: func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != test.path || r.Header.Get("Authorization") != "Bearer access-token" {
						t.Error("request did not use the expected endpoint and OAuth authorization")
					}
					if _, present := r.Header[http.CanonicalHeaderKey("developer-token")]; present {
						t.Error("retired developer-token header was sent")
					}
					_, _ = io.WriteString(w, body)
				},
			})
			client.cfg.MaxAttempts = 1
			var receipts []Response
			response, err := client.Call(context.Background(), http.MethodPost, test.path, []byte(test.body), func(receipt Response) error {
				receipts = append(receipts, receipt)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if tokenCalls.Load() != 1 || adsCalls.Load() != 1 || response.StatusCode != http.StatusOK {
				t.Fatalf("token calls=%d Ads calls=%d status=%d", tokenCalls.Load(), adsCalls.Load(), response.StatusCode)
			}
			if len(receipts) != 1 || !bytes.Equal(receipts[0].Body, []byte(body)) || !bytes.Equal(response.Body, []byte(body)) {
				t.Fatal("the authenticated response was not preserved in the receipt")
			}
		})
	}
}

func TestCloudProjectOAuthStillRequiresRefreshCredentials(t *testing.T) {
	for _, test := range []struct {
		name  string
		clear func(*Config)
	}{
		{"client ID", func(cfg *Config) { cfg.ClientID = "" }},
		{"client secret", func(cfg *Config) { cfg.ClientSecret = "" }},
		{"refresh token", func(cfg *Config) { cfg.RefreshToken = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, _, tokenCalls, adsCalls := newTestClient(t, testHandlers{})
			test.clear(&client.cfg)
			client.cfg.MaxAttempts = 1
			_, err := client.Call(context.Background(), http.MethodPost,
				"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
			if CodeOf(err) != CodeCredentialsMissing || tokenCalls.Load() != 0 || adsCalls.Load() != 0 {
				t.Fatalf("missing %s: code=%q token calls=%d Ads calls=%d", test.name, CodeOf(err), tokenCalls.Load(), adsCalls.Load())
			}
		})
	}
}

func TestCloudProjectAccessFailurePreservesReceiptAndExplainsRemedy(t *testing.T) {
	const body = `{"error":{"status":"PERMISSION_DENIED","details":[{"errors":[{"errorCode":{"authorizationError":"CLOUD_PROJECT_NOT_APPROVED_FOR_PRODUCTION"}}]}]}}`
	client, _, _, adsCalls := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, body)
		},
	})
	client.cfg.MaxAttempts = 1
	var receipts []Response
	_, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordHistoricalMetrics", []byte("{}"), func(receipt Response) error {
			receipts = append(receipts, receipt)
			return nil
		})
	if CodeOf(err) != CodeCloudProjectNotApproved || ExitCodeOf(err) != 6 || adsCalls.Load() != 1 {
		t.Fatalf("code=%q exit=%d calls=%d", CodeOf(err), ExitCodeOf(err), adsCalls.Load())
	}
	if len(receipts) != 1 || string(receipts[0].Body) != body || receipts[0].StatusCode != http.StatusForbidden {
		t.Fatal("Cloud project access failure was not preserved in its receipt")
	}
	for _, fragment := range []string{"Google Cloud project", "OAuth client", "Basic or Standard"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("diagnostic does not explain %q: %v", fragment, err)
		}
	}
}
