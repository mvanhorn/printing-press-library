// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestCampaignCreateTranslatesLegacyRequestBody(t *testing.T) {
	expected := map[string]any{
		"data": map[string]any{
			"type": "campaign",
			"attributes": map[string]any{
				"definition": map[string]any{"name": "Contract test"},
				"audiences":  []any{map[string]any{"temporary_id": "legacy-audience", "name": "Contract test audience", "included": []any{"list-1"}, "excluded": []any{}}},
				"messages":   []any{map[string]any{"name": "Contract test", "audience_temporary_id": "legacy-audience", "variations": []any{map[string]any{"definition": map[string]any{"name": "Contract test", "details": map[string]any{"channel": "email"}}}}}},
			},
		},
	}

	server := requestBodyServer(t, "/api/campaigns", expected)
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")

	previousRegistration := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previousRegistration })
	registeredPlatformSource = nil // wire-body contract only; tenant gate is covered by platform tests

	flags := rootFlags{asJSON: true}
	cmd := newCampaignsCreateCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--data-type", "campaign",
		"--data-attributes-name", "Contract test",
		"--data-attributes-audiences", `{"included":["list-1"],"excluded":[]}`,
		"--data-attributes-campaign-messages", `{"data":[{"type":"campaign-message","attributes":{"definition":{"channel":"email"}}}]}`,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute campaigns create: %v", err)
	}
}

func TestCampaignCreateRejectsMixedStdinAndBodyFlags(t *testing.T) {
	cmd := newCampaignsCreateCmd(&rootFlags{})
	cmd.SetArgs([]string{"--stdin", "--data-attributes-name", "ignored"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--stdin cannot be combined") {
		t.Fatalf("error = %v, want mixed-input rejection", err)
	}
}

func TestParseCampaignObjectOrNull(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "object", value: `{"method":"immediate"}`},
		{name: "null", value: `null`},
		{name: "string", value: `"immediate"`, wantErr: true},
		{name: "array", value: `[]`, wantErr: true},
		{name: "invalid JSON", value: `immediate`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCampaignObjectOrNull("data-attributes-send-strategy", tt.value)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestCampaignMessageAssignTemplatePreservesNestedRequestBody(t *testing.T) {
	expected := map[string]any{
		"data": map[string]any{
			"id":   "message-1",
			"type": "campaign-message",
			"relationships": map[string]any{
				"template": map[string]any{
					"data": map[string]any{
						"id":   "template-1",
						"type": "template",
					},
				},
			},
		},
	}

	server := requestBodyServer(t, "/api/campaign-message-assign-template", expected)
	defer server.Close()
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")

	previousRegistration := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previousRegistration })
	registeredPlatformSource = nil // wire-body contract only; tenant gate is covered by platform tests

	flags := rootFlags{asJSON: true}
	cmd := newCampaignMessageAssignTemplatePromotedCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--data-id", "message-1",
		"--data-type", "campaign-message",
		"--data-relationships-template", `{"data":{"id":"template-1","type":"template"}}`,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute campaign-message-assign-template: %v", err)
	}
}

func requestBodyServer(t *testing.T, expectedPath string, expectedBody map[string]any, queryKey ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("revision"); got != "2026-10-15" {
			t.Errorf("revision = %q, want 2026-10-15", got)
		}
		if got := r.Header.Get("Authorization"); got != "Klaviyo-API-Key test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if expectedPath == "/api/campaign-message-assign-template" && r.Method == http.MethodGet && r.URL.Path == "/api/templates/template-1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"id":"template-1","type":"template","attributes":{"html":"<a href=\"{% unsubscribe_link %}\">Unsubscribe</a>"}}}`))
			return
		}
		if expectedPath == "/api/campaign-message-assign-template" && r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/campaign-variations/") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":{"id":%q,"type":"campaign-variation"}}`, strings.TrimPrefix(r.URL.Path, "/api/campaign-variations/"))
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != expectedPath {
			t.Errorf("path = %s, want %s", r.URL.Path, expectedPath)
		}
		if len(queryKey) > 0 && r.URL.Query().Get(queryKey[0]) != "id" {
			t.Errorf("sparse field query lost: %s", r.URL.RawQuery)
		}
		var got map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&got); err != nil {
			t.Errorf("decode request body: %v", err)
		} else if !reflect.DeepEqual(got, expectedBody) {
			t.Errorf("request body mismatch\ngot:  %s\nwant: %s", mustJSON(got), mustJSON(expectedBody))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"ok","type":"campaign"}}`))
	}))
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(data)
}

func TestTemplateShortcutsReadStdinRequestBody(t *testing.T) {
	for _, endpoint := range []string{"template-render", "campaign-message-assign-template"} {
		t.Run(endpoint, func(t *testing.T) {
			expected := map[string]any{"data": map[string]any{
				"type": "campaign-message", "id": "message-1",
				"relationships": map[string]any{"template": map[string]any{"data": map[string]any{"type": "template", "id": "template-1"}}},
			}}
			if endpoint == "template-render" {
				expected = map[string]any{"data": map[string]any{
					"type": "template", "id": "template-1",
					"attributes": map[string]any{"context": map[string]any{"order_id": json.Number("9007199254740993")}},
				}}
			}
			fieldFlag, queryKey := "--fields-template", "fields[template]"
			if endpoint == "campaign-message-assign-template" {
				fieldFlag, queryKey = "--fields-campaign-message", "fields[campaign-message]"
			}
			server := requestBodyServer(t, "/api/"+endpoint, expected, queryKey)
			defer server.Close()
			t.Setenv("HOME", t.TempDir())
			t.Setenv("KLAVIYO_BASE_URL", server.URL)
			t.Setenv("KLAVIYO_API_KEY", "test-key")
			previousRegistration := registeredPlatformSource
			t.Cleanup(func() { registeredPlatformSource = previousRegistration })
			registeredPlatformSource = nil
			flags := &rootFlags{asJSON: true}
			cmd := newTemplateRenderPromotedCmd(flags)
			if endpoint == "campaign-message-assign-template" {
				cmd = newCampaignMessageAssignTemplatePromotedCmd(flags)
			}
			cmd.SetIn(strings.NewReader(mustJSON(expected)))
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--stdin", fieldFlag, "id"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTemplateShortcutRejectsMissingOrMalformedBody(t *testing.T) {
	for _, body := range []string{"", "null", "{}", "[]", `{"data":{}} trailing`, `{"data":{}} {"extra":1}`} {
		cmd := newTemplateRenderPromotedCmd(&rootFlags{})
		cmd.SetIn(strings.NewReader(body))
		cmd.SetArgs([]string{"--stdin"})
		if err := cmd.Execute(); err == nil {
			t.Errorf("body %q should fail before creating a client", body)
		}
	}
	cmd := newTemplateRenderPromotedCmd(&rootFlags{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("bodyless template-render must fail")
	}
	assign := newCampaignMessageAssignTemplatePromotedCmd(&rootFlags{})
	assign.SetArgs([]string{"--stdin", "--data-id", "ignored"})
	if err := assign.Execute(); err == nil || !strings.Contains(err.Error(), "--stdin cannot be combined") {
		t.Fatalf("mixed stdin and flags error = %v", err)
	}
}

func TestMergedClientDoesNotReplayUnprotectedWrites(t *testing.T) {
	for _, failure := range []string{"server", "transport"} {
		for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, "multipart"} {
			t.Run(failure+"/"+method, func(t *testing.T) {
				var calls atomic.Int32
				requestMethod := method
				if method == "multipart" {
					requestMethod = http.MethodPost
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if r.Method != requestMethod || r.URL.RequestURI() != "/api/write" {
						t.Errorf("request = %s %s", r.Method, r.URL.RequestURI())
					}
					if got := r.Header.Get("revision"); got != "2026-10-15" {
						t.Errorf("revision = %q", got)
					}
					_, _ = io.Copy(io.Discard, r.Body)
					if failure == "transport" {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, `{"errors":[{"detail":"ambiguous"}]}`)
				}))
				defer server.Close()
				c := mergedRetryClient(t, server, true)
				_, status, err := mergedWriteRequest(t, c, method, nil)
				if err == nil || calls.Load() != 1 {
					t.Fatalf("calls = %d, error = %v; want one failed write", calls.Load(), err)
				}
				if failure == "transport" {
					verb := map[string]string{"POST": "Post", "PATCH": "Patch", "PUT": "Put", "DELETE": "Delete"}[requestMethod]
					want := fmt.Sprintf("%s /api/write: %s %q: EOF", requestMethod, verb, server.URL+"/api/write")
					if status != 0 || !errors.Is(err, io.EOF) || err.Error() != want {
						t.Fatalf("status = %d, error = %v, want %q", status, err, want)
					}
				} else {
					want := requestMethod + ` /api/write returned HTTP 500: {"errors":[{"detail":"ambiguous"}]}`
					if status != http.StatusInternalServerError || err.Error() != want {
						t.Fatalf("status = %d, error = %v, want %q", status, err, want)
					}
				}
			})
		}
	}
}

func TestMergedClientPreservesBoundedRecovery(t *testing.T) {
	for _, tt := range []struct {
		name       string
		method     string
		status     int
		tenant     bool
		key        string
		wantCalls  int32
		alwaysFail bool
	}{
		{name: "legacy POST rate limit", method: "POST", status: 429, wantCalls: 2},
		{name: "legacy multipart rate limit", method: "multipart", status: 429, wantCalls: 2},
		{name: "legacy rate limit bound", method: "POST", status: 429, wantCalls: 4, alwaysFail: true},
		{name: "legacy safe server recovery", method: "GET", status: 500, wantCalls: 2},
		{name: "tenant unprotected rate limit", method: "POST", status: 429, tenant: true, wantCalls: 1, alwaysFail: true},
		{name: "tenant protected POST", method: "POST", status: 500, tenant: true, key: "synthetic-key", wantCalls: 2},
		{name: "tenant protected multipart", method: "multipart", status: 500, tenant: true, key: "synthetic-key", wantCalls: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			var firstBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				attempt := calls.Add(1)
				if attempt == 1 {
					firstBody = body
				} else if !bytes.Equal(body, firstBody) {
					t.Error("retry changed request body")
				}
				if r.Header.Get("Idempotency-Key") != tt.key {
					t.Errorf("Idempotency-Key = %q, want %q", r.Header.Get("Idempotency-Key"), tt.key)
				}
				if attempt == 1 || tt.alwaysFail {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(tt.status)
					fmt.Fprint(w, `{"errors":[{"detail":"retry"}]}`)
					return
				}
				fmt.Fprint(w, `{"data":{"id":"ok"}}`)
			}))
			defer server.Close()
			c := mergedRetryClient(t, server, tt.tenant)
			var data json.RawMessage
			var err error
			if tt.method == http.MethodGet {
				data, err = c.Get(klaviyoRequestContext(c), "/api/write", nil)
			} else {
				data, _, err = mergedWriteRequest(t, c, tt.method, map[string]string{"Idempotency-Key": tt.key})
			}
			if calls.Load() != tt.wantCalls {
				t.Fatalf("calls = %d, want %d (error %v)", calls.Load(), tt.wantCalls, err)
			}
			if tt.alwaysFail {
				want := `POST /api/write returned HTTP 429: {"errors":[{"detail":"retry"}]}`
				var apiErr *client.APIError
				if !errors.As(err, &apiErr) || apiErr.Error() != want {
					t.Fatalf("error = %v, want APIError %q", err, want)
				}
			} else if err != nil || string(data) != `{"data":{"id":"ok"}}` {
				t.Fatalf("response = %s, error = %v", data, err)
			}
		})
	}
}

func mergedRetryClient(t *testing.T, server *httptest.Server, tenant bool) *client.Client {
	t.Helper()
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	c := client.New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, 5*time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true
	if tenant {
		if err := c.BindPlatformSession(platformCommandTestSession(t)); err != nil {
			t.Fatal(err)
		}
		if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{
			Class: "*", Steady: 100, Interval: time.Second, Burst: 10, MaxAttempts: 2, RetryBudget: 3 * time.Second,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func mergedWriteRequest(t *testing.T, c *client.Client, method string, headers map[string]string) (json.RawMessage, int, error) {
	t.Helper()
	body := map[string]any{"data": map[string]any{"type": "synthetic"}}
	switch method {
	case http.MethodPost:
		return c.PostWithHeaders(context.Background(), "/api/write", body, headers)
	case http.MethodPatch:
		return c.PatchWithHeaders(context.Background(), "/api/write", body, headers)
	case http.MethodPut:
		return c.PutWithHeaders(context.Background(), "/api/write", body, headers)
	case http.MethodDelete:
		return c.DeleteWithHeaders(context.Background(), "/api/write", headers)
	default:
		file, err := os.CreateTemp(t.TempDir(), "*.png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if _, err := file.WriteString("synthetic upload"); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		return c.PostMultipartWithHeaders(context.Background(), "/api/write", map[string]string{"name": "Synthetic"}, map[string]string{"file": file.Name()}, headers)
	}
}

func TestPathParametersRemainSingleSegmentsOnWire(t *testing.T) {
	for _, tt := range []struct{ id, encoded string }{
		{".", "%2E"},
		{"..", "%2E%2E"},
		{"a/b?#% space", "a%2Fb%3F%23%25%20space"},
	} {
		t.Run(tt.id, func(t *testing.T) {
			want := "/api/segments/" + tt.encoded
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.RequestURI() != want {
					t.Errorf("request = %s %s, want GET %s", r.Method, r.URL.RequestURI(), want)
				}
				fmt.Fprint(w, `{"data":{"id":"ok"}}`)
			}))
			defer server.Close()
			c := mergedRetryClient(t, server, false)
			if _, err := c.Get(klaviyoRequestContext(c), replacePathParam("/api/segments/{id}", "id", tt.id), nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestListQualityScoreRequestsListProfileCounts(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/api/lists":
			if r.URL.Query().Get("fields[list]") != "name,created,updated" {
				t.Errorf("collection query = %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"data":[{"id":"list-1","attributes":{"name":"Main"}}],"links":{"next":null}}`)
		case "/api/lists/list-1/":
			if r.URL.RawQuery != "additional-fields%5Blist%5D=profile_count" {
				t.Errorf("profile count query = %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"data":{"id":"list-1","attributes":{"profile_count":100}}}`)
		default:
			fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
		}
	}))
	defer server.Close()
	c := mergedRetryClient(t, server, false)
	result, err := listQualityScore(context.Background(), c, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	lists := result["lists"].([]map[string]any)
	if len(lists) != 1 || lists[0]["size"] != 100 {
		t.Fatalf("list quality rows = %#v", lists)
	}
	if len(paths) < 2 || paths[0] != "/api/lists" || paths[1] != "/api/lists/list-1/" {
		t.Fatalf("request paths = %#v", paths)
	}
}
