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
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

func fixPassClientEnvironment(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("KLAVIYO_BASE_URL", baseURL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	t.Setenv("KLAVIYO_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	previous := registeredPlatformSource
	registeredPlatformSource = nil
	t.Cleanup(func() { registeredPlatformSource = previous })
}

func fixPassAssertGET(t *testing.T, r *http.Request) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil || r.Method != http.MethodGet || len(body) != 0 {
		t.Errorf("request = %s body %q, error %v; want GET with empty body", r.Method, body, err)
	}
	if r.Header.Get("revision") != "2026-04-15" || r.Header.Get("Authorization") != "Klaviyo-API-Key test-key" {
		t.Error("request must retain revision and test authentication headers")
	}
}

func TestFixPassQAGatePrintsCampaignErrorOnce(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fixPassAssertGET(t, r)
		if r.URL.RequestURI() != "/api/campaigns/missing" {
			t.Errorf("request URI = %q", r.URL.RequestURI())
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errors":[{"detail":"not found"}]}`)
	}))
	defer server.Close()
	fixPassClientEnvironment(t, server.URL)
	cmd := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--campaign-id", "missing"})
	err := cmd.Execute()
	want := "GET /api/campaigns/missing returned HTTP 404: {\"errors\":[{\"detail\":\"not found\"}]}\nhint: resource not found. Run the 'list' command to see available items"
	if err == nil || err.Error() != want || ExitCode(err) != 3 {
		t.Fatalf("error = %v, want exact 404 error and exit code 3", err)
	}
	if stderr.String() != "Error: "+want+"\n" || stdout.Len() != 0 || calls != 1 {
		t.Fatalf("stdout=%q stderr=%q calls=%d", stdout.String(), stderr.String(), calls)
	}
}

func TestFixPassQAGateFailVerdictPrintsOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixPassAssertGET(t, r)
		switch r.URL.RequestURI() {
		case "/api/campaigns/campaign-1":
			fmt.Fprint(w, `{"data":{"id":"campaign-1","type":"campaign"}}`)
		case "/api/campaigns/campaign-1/campaign-messages":
			fmt.Fprint(w, `{"data":[{"id":"message-1","type":"campaign-message"}]}`)
		case "/api/campaign-messages/message-1/template?fields%5Btemplate%5D=html":
			fmt.Fprint(w, `{"data":{"id":"template-1","type":"template","attributes":{"html":"<p>No unsubscribe link</p>"}}}`)
		default:
			t.Errorf("unexpected request URI %q", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	fixPassClientEnvironment(t, server.URL)
	cmd := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--campaign-id", "campaign-1"})
	err := cmd.Execute()
	if err == nil || err.Error() != "campaign QA failed; see findings above" || ExitCode(err) == 0 {
		t.Fatalf("error = %v, want failed verdict and nonzero exit", err)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result["verdict"] != "fail" {
		t.Fatalf("expected exactly one JSON fail verdict, got %q: %v", stdout.String(), err)
	}
	if stderr.String() != "Error: campaign QA failed; see findings above\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestFixPassCompactListFields(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"flat", `[{"id":"1","name":"Welcome","status":"live","created_at":"today","description":"verbose","relationships":{},"attributes":null}]`, `[{"id":"1","name":"Welcome","status":"live","created_at":"today"}]`},
		{"jsonapi", `[{"id":"1","type":"flow","attributes":{"name":"Welcome","status":"live","email":"a@example.test","updated_at":"today","description":"verbose","html":"large"},"relationships":{"tags":{}}}]`, `[{"id":"1","type":"flow","attributes":{"name":"Welcome","status":"live","email":"a@example.test","updated_at":"today"}}]`},
		{"klaviyo timestamps", `[{"id":"1","type":"flow","attributes":{"created":"2026-11-01T10:00:00Z","updated":"2026-11-09T12:00:00Z","description":"verbose"}}]`, `[{"id":"1","type":"flow","attributes":{"created":"2026-11-01T10:00:00Z","updated":"2026-11-09T12:00:00Z"}}]`},
		{"empty attributes", `[{"id":"1","attributes":{"description":"verbose"}}]`, `[{"id":"1","attributes":{}}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got, want any
			if err := json.Unmarshal(compactFields(json.RawMessage(test.input)), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(test.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("compact = %#v, want %#v", got, want)
			}
		})
	}
}

func TestFixPassIdentityProbeRetries(t *testing.T) {
	for _, test := range []struct {
		name       string
		statuses   []int
		retryAfter string
		waits      []time.Duration
		outcome    platform.GateOutcome
		wantError  string
	}{
		{"rate limit recovery", []int{429, 429, 200}, "", []time.Duration{time.Second, 2 * time.Second}, "", ""},
		{"rate limit exhausted", []int{429, 429, 429, 429}, "", []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, platform.GateRateLimited, "Klaviyo identity probe failed with status 429"},
		{"server recovery", []int{500, 503, 200}, "", []time.Duration{time.Second, 2 * time.Second}, "", ""},
		{"server exhausted", []int{503, 503, 503, 503}, "", []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, platform.GateUnavailable, "Klaviyo identity probe failed with status 503"},
		{"retry after", []int{429, 429, 200}, "7", []time.Duration{7 * time.Second, 7 * time.Second}, "", ""},
		{"retry after capped", []int{429, 200}, "300", []time.Duration{30 * time.Second}, "", ""},
		{"retry after zero", []int{429, 200}, "0", []time.Duration{0}, "", ""},
		{"invalid retry after", []int{429, 200}, "invalid", []time.Duration{time.Second}, "", ""},
		{"negative retry after", []int{429, 200}, "-1", []time.Duration{time.Second}, "", ""},
		{"past retry after date", []int{429, 200}, time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), []time.Duration{time.Second}, "", ""},
		{"auth not retried", []int{403}, "", nil, platform.GateInsufficientScope, "Klaviyo identity probe failed with status 403"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var waits []time.Duration
			previous := waitForIdentityProbeRetry
			waitForIdentityProbeRetry = func(_ context.Context, wait time.Duration) error {
				waits = append(waits, wait)
				return nil
			}
			t.Cleanup(func() { waitForIdentityProbeRetry = previous })
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fixPassAssertGET(t, r)
				if r.URL.RequestURI() != "/api/accounts" {
					t.Errorf("request URI = %q", r.URL.RequestURI())
				}
				calls++
				if calls > len(test.statuses) {
					t.Error("too many attempts")
					w.WriteHeader(400)
					return
				}
				w.Header().Set("Retry-After", test.retryAfter)
				w.WriteHeader(test.statuses[calls-1])
				fmt.Fprint(w, `{"data":[{"id":"account-42","attributes":{"contact_information":{"organization_name":"Example Shop"}}}]}`)
			}))
			defer server.Close()
			observed, err := (klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()}).ProbeIdentity(context.Background(), platform.ResolvedCredentials{"credential": []byte("test-key")}, platform.SourceProfile{})
			if test.wantError == "" {
				if err != nil || observed.AccountID != "account-42" || observed.OrgName != "Example Shop" {
					t.Fatalf("observed = %+v, error = %v", observed, err)
				}
			} else {
				var failure *platform.ProbeFailure
				if !errors.As(err, &failure) || failure.Outcome != test.outcome || failure.Err.Error() != test.wantError {
					t.Fatalf("error = %#v, want %s: %s", err, test.outcome, test.wantError)
				}
			}
			if calls != len(test.statuses) || !reflect.DeepEqual(waits, test.waits) {
				t.Fatalf("calls=%d waits=%v, want calls=%d waits=%v", calls, waits, len(test.statuses), test.waits)
			}
		})
	}
}

func TestFixPassIdentityProbeRetryAfterHTTPDate(t *testing.T) {
	for _, test := range []struct {
		name    string
		after   time.Duration
		minWait time.Duration
		maxWait time.Duration
	}{
		{"future date", 20 * time.Second, 18 * time.Second, 20 * time.Second},
		{"far future capped", time.Hour, 30 * time.Second, 30 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			var waits []time.Duration
			previous := waitForIdentityProbeRetry
			waitForIdentityProbeRetry = func(_ context.Context, wait time.Duration) error {
				waits = append(waits, wait)
				return nil
			}
			t.Cleanup(func() { waitForIdentityProbeRetry = previous })
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fixPassAssertGET(t, r)
				if r.URL.RequestURI() != "/api/accounts" {
					t.Errorf("request URI = %q, want /api/accounts", r.URL.RequestURI())
				}
				calls++
				if calls == 1 {
					w.Header().Set("Retry-After", time.Now().Add(test.after).UTC().Format(http.TimeFormat))
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				fmt.Fprint(w, `{"data":[{"id":"account-42","attributes":{"contact_information":{"organization_name":"Example Shop"}}}]}`)
			}))
			defer server.Close()
			observed, err := (klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()}).ProbeIdentity(context.Background(), platform.ResolvedCredentials{"credential": []byte("test-key")}, platform.SourceProfile{})
			if err != nil || observed.AccountID != "account-42" || observed.OrgName != "Example Shop" {
				t.Fatalf("observed = %+v, error = %v", observed, err)
			}
			if calls != 2 || len(waits) != 1 || waits[0] < test.minWait || waits[0] > test.maxWait {
				t.Fatalf("calls=%d waits=%v, want 2 calls and one wait in [%v, %v]", calls, waits, test.minWait, test.maxWait)
			}
		})
	}
}

func TestFixPassIdentityProbeHonorsContextDeadline(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := (klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()}).ProbeIdentity(ctx, platform.ResolvedCredentials{}, platform.SourceProfile{})
	var failure *platform.ProbeFailure
	if !errors.As(err, &failure) || failure.Outcome != platform.GateUnavailable || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %#v, want unavailable deadline error", err)
	}
	if failure.Err.Error() != "context deadline exceeded" || calls != 1 {
		t.Fatalf("error=%v calls=%d, want deadline exceeded after one request", failure.Err, calls)
	}
}

func TestFixPassSegmentVelocityAllTagged(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixPassAssertGET(t, r)
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/api/segments":
			want := url.Values{"include": {"tags"}, "fields[tag]": {"name"}, "page[size]": {"10"}}
			if r.URL.Query().Get("page[cursor]") != "" {
				want.Set("page[cursor]", "second")
			}
			if !reflect.DeepEqual(r.URL.Query(), want) {
				t.Errorf("list query = %v, want %v", r.URL.Query(), want)
			}
			if r.URL.Query().Get("page[cursor]") == "" {
				fmt.Fprint(w, `{"data":[{"id":"seg-a","type":"segment","relationships":{"tags":{"data":[{"type":"tag","id":"tag-first"}]}}},{"id":"vip-untagged","attributes":{"name":"VIP"},"relationships":{"tags":{"data":[{"type":"tag","id":"tag-other"}]}}}],"included":[{"id":"tag-first","type":"tag","attributes":{"name":"VIP"}},{"id":"tag-other","type":"tag","attributes":{"name":"Other"}}],"links":{"next":"/api/segments?page%5Bcursor%5D=second"}}`)
			} else {
				fmt.Fprint(w, `{"data":[{"id":"seg-b","type":"segment","relationships":{"tags":{"data":[{"type":"tag","id":"tag-vip"},{"type":"tag","id":"tag-vip"}]}}}],"included":[{"id":"tag-vip","type":"tag","attributes":{"name":"vIp"}}],"links":{"next":null}}`)
			}
		case "/api/segments/seg-a/", "/api/segments/seg-b/":
			if !reflect.DeepEqual(r.URL.Query(), url.Values{"additional-fields[segment]": {"profile_count"}}) {
				t.Errorf("count query = %v", r.URL.Query())
			}
			fmt.Fprint(w, `{"data":{"attributes":{"name":"Tagged","profile_count":42}}}`)
		default:
			t.Errorf("unexpected request %q", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	previous := waitForProfileCountRequest
	waits := 0
	waitForProfileCountRequest = func(_ context.Context, wait time.Duration) error {
		waits++
		if wait != profileCountRequestSpacing {
			t.Errorf("profile count spacing = %v", wait)
		}
		return nil
	}
	t.Cleanup(func() { waitForProfileCountRequest = previous })
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "snapshots.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := client.New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, 5*time.Second, 0)
	c.NoCache = true
	taggedIDs, err := segmentIDsByTag(c, "VIP")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := segmentVelocity(ctx, c, db, taggedIDs, "90d", "weekly"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(requests, []string{"/api/segments", "/api/segments", "/api/segments/seg-a/", "/api/segments/seg-b/"}) || waits != 1 {
		t.Fatalf("requests=%v waits=%d", requests, waits)
	}
	rows, err := db.DB().Query(`SELECT segment_id, name, count FROM segment_snapshots ORDER BY segment_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, name string
		var count int
		if err := rows.Scan(&id, &name, &count); err != nil {
			t.Fatal(err)
		}
		if name != "Tagged" || count != 42 {
			t.Fatalf("snapshot = %s %s %d", id, name, count)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "seg-a,seg-b" {
		t.Fatalf("snapshotted IDs = %v", ids)
	}
}
