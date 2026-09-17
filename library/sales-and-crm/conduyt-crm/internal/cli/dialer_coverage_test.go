// Sweep findings: per-priority fetch/decode failures and queues that reach --limit print partial metadata and exit API-class non-zero.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

func TestNovelDialerCoverageHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"dialer", "coverage", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Usage:") || !strings.Contains(out.String(), "--limit") {
		t.Fatalf("help: %s", out.String())
	}
}

func TestNovelDialerCoverageBehavior(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fail     bool
		failures int
	}{{"complete", false, 0}, {"partial", true, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/smart-views/dial-order":
					fmt.Fprint(w, `{"data":[{"smartViewId":"new","name":"NEW 1","position":1},{"smartViewId":"old","name":"OLD","position":2},{"smartViewId":"bad","name":"BAD","position":3}]}`)
				case "/dialer/queue":
					if tc.fail && r.URL.Query().Get("smartViewId") == "bad" {
						http.Error(w, `{"error":"boom"}`, 500)
						return
					}
					if r.URL.Query().Get("smartViewId") == "new" {
						fmt.Fprint(w, `{"data":{"data":[{"id":"1"},{"id":"2"}],"meta":{"page":1,"per_page":2,"total":7}}}`)
					} else {
						fmt.Fprint(w, `{"data":[]}`)
					}
				case "/dialer/agents-status":
					fmt.Fprint(w, `{"data":[{"status":"Available"},{"status":"Busy"},{"status":"available"}]}`)
				}
			}))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			flags := &rootFlags{asJSON: true, noCache: true}
			cmd := newNovelDialerCoverageCmd(flags)
			cmd.SetArgs([]string{"--limit", "3"})
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			err := cmd.Execute()
			var got dialerCoverageView
			if err := json.NewDecoder(&out).Decode(&got); err != nil {
				t.Fatalf("%q: %v", out.String(), err)
			}
			if tc.fail && ExitCode(err) != 5 {
				t.Fatalf("err=%v", err)
			}
			if !tc.fail && err != nil {
				t.Fatal(err)
			}
			if got.AvailableAgents != 2 || got.Priorities[0].QueueDepth != 7 || got.Priorities[0].Capped || got.FetchFailures != tc.failures {
				t.Fatalf("got %+v", got)
			}
			if !tc.fail && (len(got.EmptyPriorities) != 2 || got.Warning == "") {
				t.Fatalf("empty result %+v", got)
			}
			if tc.fail && !strings.Contains(stderr.String(), "warning: 1 of 3 fetches failed") {
				t.Fatalf("stderr %q", stderr.String())
			}
		})
	}
}

func TestNovelDialerCoverageForbidden(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, `{"error":"forbidden"}`, 403) }))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	flags := &rootFlags{asJSON: true, noCache: true}
	cmd := newNovelDialerCoverageCmd(flags)
	cmd.SetArgs([]string{"--limit", "2"})
	err := cmd.Execute()
	if ExitCode(err) != 4 || !strings.Contains(err.Error(), "dialer.view/admin") {
		t.Fatalf("err=%v code=%d", err, ExitCode(err))
	}
}

func TestNovelDialerCoverageInvalidOrderShapesPrintPartial(t *testing.T) {
	for _, body := range []string{`{`, `{}`, `{"data":{}}`, `{"data":{"priorities":null}}`, `{"data":{"error":"upstream failure"}}`, `{"message":"upstream failure"}`} {
		t.Run(body, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			cmd := newNovelDialerCoverageCmd(&rootFlags{asJSON: true, noCache: true})
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--limit", "3"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			var got dialerCoverageView
			if decodeErr := json.Unmarshal(out.Bytes(), &got); decodeErr != nil {
				t.Fatalf("decode %q: %v", out.String(), decodeErr)
			}
			if ExitCode(err) != 5 || !got.Partial || got.Checked != 0 || got.Total != 0 || got.FetchFailures != 1 || len(got.Failures) != 1 || got.Priorities == nil {
				t.Fatalf("err=%v got=%+v out=%s", err, got, out.String())
			}
		})
	}
}

func TestNovelDialerCoverageAgentStatusFailuresPrintPartial(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{{"fetch", `{"error":"boom"}`, 500}, {"decode", `{`, 200}} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/smart-views/dial-order":
					fmt.Fprint(w, `{"data":[{"smartViewId":"one","name":"ONE","position":1}]}`)
				case "/dialer/queue":
					fmt.Fprint(w, `{"data":[{"id":"lead"}]}`)
				case "/dialer/agents-status":
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.body)
				}
			}))
			defer srv.Close()
			t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
			t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
			cmd := newNovelDialerCoverageCmd(&rootFlags{asJSON: true, noCache: true})
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--limit", "3"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			var got dialerCoverageView
			if decodeErr := json.Unmarshal(out.Bytes(), &got); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !got.Partial || got.Checked != 1 || got.Total != 1 || got.FetchFailures != 1 || len(got.Failures) != 1 || got.Priorities[0].QueueDepth != 1 {
				t.Fatalf("err=%v got=%+v", err, got)
			}
		})
	}
}

func TestNovelDialerCoverageRejectsErrorEnvelopesWithData(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint, body string
	}{
		{"queue top-level error", "queue", `{"error":"unavailable","data":[]}`},
		{"queue nested message", "queue", `{"data":{"message":{"code":"unavailable"}}}`},
		{"agents top-level message", "agents", `{"message":"unavailable","data":[]}`},
		{"agents nested error", "agents", `{"data":{"error":{"message":"unavailable"}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := runNovel(t, []string{"dialer", "coverage", "--limit", "3", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/smart-views/dial-order":
					_, _ = w.Write([]byte(`{"data":[{"smartViewId":"one","name":"ONE","position":1}]}`))
				case "/dialer/queue":
					if tc.endpoint == "queue" {
						_, _ = w.Write([]byte(tc.body))
					} else {
						_, _ = w.Write([]byte(`{"data":[]}`))
					}
				case "/dialer/agents-status":
					if tc.endpoint == "agents" {
						_, _ = w.Write([]byte(tc.body))
					} else {
						_, _ = w.Write([]byte(`{"data":[]}`))
					}
				}
			})
			var got dialerCoverageView
			if decodeErr := json.Unmarshal([]byte(out), &got); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !got.Partial || got.FetchFailures != 1 {
				t.Fatalf("err=%v got=%+v out=%s", err, got, out)
			}
		})
	}
}

func TestNovelDialerCoverageValidatesProviderTotals(t *testing.T) {
	for _, tc := range []struct {
		name, meta string
		wantDepth  int
		wantCode   int
		wantCapped bool
	}{
		{"absent", "", 3, 5, true},
		{"zero with rows", `,"meta":{"total":0}`, 3, 5, false},
		{"below rows", `,"meta":{"total":2}`, 3, 5, false},
		{"consistent limit page", `,"meta":{"total":9}`, 9, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := runNovel(t, []string{"dialer", "coverage", "--limit", "3", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/smart-views/dial-order":
					_, _ = w.Write([]byte(`{"data":[{"smartViewId":"one","name":"ONE","position":1}]}`))
				case "/dialer/queue":
					_, _ = w.Write([]byte(`{"data":[{"id":"1"},{"id":"2"},{"id":"3"}]` + tc.meta + `}`))
				case "/dialer/agents-status":
					_, _ = w.Write([]byte(`{"data":[]}`))
				}
			})
			var got dialerCoverageView
			if decodeErr := json.Unmarshal([]byte(out), &got); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			gotCode := 0
			if err != nil {
				gotCode = ExitCode(err)
			}
			if gotCode != tc.wantCode || got.Priorities[0].QueueDepth != tc.wantDepth || got.Priorities[0].Capped != tc.wantCapped {
				t.Fatalf("err=%v got=%+v", err, got)
			}
		})
	}
}

func TestNovelDialerCoveragePartialFetchFailuresAreTyped(t *testing.T) {
	for _, endpoint := range []string{"queue", "agents"} {
		for _, tc := range []struct{ status, code int }{{401, 4}, {403, 4}, {404, 3}, {500, 5}} {
			t.Run(endpoint+"/"+http.StatusText(tc.status), func(t *testing.T) {
				testenv.Isolate(t)
				out, _, err := runNovel(t, []string{"dialer", "coverage", "--limit", "3", "--json"}, func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/smart-views/dial-order":
						_, _ = w.Write([]byte(`{"data":[{"smartViewId":"one","name":"ONE","position":1}]}`))
					case "/dialer/queue":
						if endpoint == "queue" {
							w.WriteHeader(tc.status)
							_, _ = w.Write([]byte(`{"message":"failure"}`))
							return
						}
						_, _ = w.Write([]byte(`{"data":[]}`))
					case "/dialer/agents-status":
						if endpoint == "agents" {
							w.WriteHeader(tc.status)
							_, _ = w.Write([]byte(`{"message":"failure"}`))
							return
						}
						_, _ = w.Write([]byte(`{"data":[]}`))
					}
				})
				if ExitCode(err) != tc.code || !strings.Contains(compactTestJSON(out), `"partial":true`) {
					t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
				}
			})
		}
	}
}
