// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

type formPublicationTransport func(*http.Request) (*http.Response, error)

func (f formPublicationTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func executeFormPublicationTest(t *testing.T, handler http.HandlerFunc, args ...string) (string, error) {
	t.Helper()
	root, flags := rcOfflineSyncRoot(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	registeredPlatformSource.BindClient = func(c *client.Client, _ *platform.Session) error {
		c.BaseURL, c.Config.KlaviyoApiKey = "https://klaviyo.test", "fixture-key"
		c.Config.Headers = map[string]string{"Idempotency-Key": "fixture-idempotency"}
		c.HTTPClient.Transport = formPublicationTransport(func(r *http.Request) (*http.Response, error) {
			response := httptest.NewRecorder()
			response.Header().Set("Content-Type", "application/vnd.api+json")
			handler.ServeHTTP(response, r)
			return response.Result(), nil
		})
		return c.SetPlatformEndpointBudget(platform.EndpointBudget{
			Class: "*", Steady: 10000, Interval: time.Second, Burst: 100, MaxAttempts: 4, RetryBudget: time.Second,
		})
	}
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"forms"}, args...))
	err := root.Execute()
	finalizePlatformInvocation(flags, &err)
	return output.String(), err
}

func formPublicationFixture(status string, versions ...map[string]any) string {
	if versions == nil {
		versions = []map[string]any{}
	}
	raw, _ := json.Marshal(map[string]any{"data": map[string]any{
		"type": "form", "id": "form-1", "attributes": map[string]any{
			"name": "Newsletter", "status": status, "definition": map[string]any{"versions": versions},
		},
	}})
	return string(raw)
}

func formPublicationVersion(status, text string) map[string]any {
	return map[string]any{
		"id": status + "-id", "status": status, "ab_test": false,
		"steps": []any{map[string]any{"id": status + "-step", "text": text}},
	}
}

const formPublicationReportFixture = `{"data":{"type":"form-values-report","attributes":{"results":[{"groupings":{"form_id":"form-1"},"statistics":{"viewed_form":100,"submits":12,"submit_rate":0.12}}]}},"links":{"next":null}}`

func TestFormPublicationDiff(t *testing.T) {
	live := formPublicationVersion("live", "Before")
	draft := formPublicationVersion("draft", "Before")
	live["created_at"], draft["created_at"] = "old", "new"
	live["updated_at"], draft["updated_at"] = "old", "new"
	live["steps"].([]any)[0].(map[string]any)["timestamp"] = "old"
	draft["steps"].([]any)[0].(map[string]any)["timestamp"] = "new"
	if changes := diffFormVersions(live, draft); len(changes) != 0 {
		t.Fatalf("metadata-only changes = %#v", changes)
	}
	draft["steps"].([]any)[0].(map[string]any)["text"] = "After"
	live["removed"] = nil
	draft["added"] = strings.Repeat("界", 200)
	live["properties"] = map[string]any{"list_id": "old-list"}
	draft["properties"] = map[string]any{"list_id": "new-list"}
	changes := diffFormVersions(live, draft)
	paths := []string{}
	for _, change := range changes {
		paths = append(paths, change.Path)
		if len([]rune(change.New)) > 160 {
			t.Fatalf("untruncated value: %#v", change)
		}
	}
	want := []string{`$["added"]`, `$["properties"]["list_id"]`, `$["removed"]`, `$["steps"][0]["text"]`}
	if !reflect.DeepEqual(paths, want) || changes[2].Old != "null" || changes[2].New != "<absent>" || changes[3].Old != `"Before"` || changes[3].New != `"After"` {
		t.Fatalf("changes = %#v", changes)
	}
	if live["id"] != "live-id" || draft["id"] != "draft-id" {
		t.Fatal("diff mutated input")
	}
	if changes := diffFormVersions(nil, draft); len(changes) == 0 || changes[0].Old != "<absent>" {
		t.Fatalf("new form changes = %#v", changes)
	}
	arrayChanges := diffFormVersions(map[string]any{"steps": []any{"a", "b"}}, map[string]any{"steps": []any{"c"}})
	if len(arrayChanges) != 2 || arrayChanges[1].New != "<absent>" {
		t.Fatalf("array changes = %#v", arrayChanges)
	}
}

func TestFormPublicationNoOpAndABRefusal(t *testing.T) {
	for _, tc := range []struct {
		name, action, message string
		status                string
		versions              []map[string]any
		wantError             bool
	}{
		{"no draft", "publish", "no draft version", "live", []map[string]any{formPublicationVersion("live", "Before")}, false},
		{"not live", "unpublish", "not live", "draft", []map[string]any{formPublicationVersion("draft", "After")}, false},
		{"AB boolean", "publish", "publish this one in the Klaviyo form editor", "draft", []map[string]any{{"status": "draft", "ab_test": true}}, true},
		{"AB object", "publish", "publish this one in the Klaviyo form editor", "draft", []map[string]any{{"status": "draft", "ab_test": map[string]any{"id": "test-1"}}}, true},
		{"multiple drafts", "publish", "multiple draft versions", "draft", []map[string]any{{"status": "draft"}, {"status": "draft"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gets, posts, reports := 0, 0, 0
			out, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/form-values-reports" {
					reports++
					fmt.Fprint(w, formPublicationReportFixture)
					return
				}
				if r.Method != http.MethodGet {
					posts++
				}
				gets++
				fmt.Fprint(w, formPublicationFixture(tc.status, tc.versions...))
			}, tc.action, "form-1", "--apply", "--json")
			wantReports := 0
			if tc.action == "unpublish" {
				wantReports = 1
			}
			if (err != nil) != tc.wantError || gets != 1 || posts != 0 || reports != wantReports {
				t.Fatalf("gets=%d posts=%d reports=%d err=%v output=%s", gets, posts, reports, err, out)
			}
			if !strings.Contains(out+fmt.Sprint(err), tc.message) {
				t.Fatalf("missing %q: %s %v", tc.message, out, err)
			}
		})
	}
}

func TestFormPublicationPreviewAndReportBody(t *testing.T) {
	for _, action := range []string{"publish", "unpublish"} {
		for _, dryRun := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dry-run=%v", action, dryRun), func(t *testing.T) {
				gets, reports, jobs := 0, 0, 0
				args := []string{action, "form-1", "--json"}
				if dryRun {
					args = append(args, "--apply", "--dry-run")
				}
				out, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
					switch r.Method + " " + r.URL.Path {
					case "GET /api/forms/form-1":
						gets++
						if r.URL.Query().Get("fields[form]") != "definition,name,status,ab_test" {
							t.Errorf("query = %s", r.URL.RawQuery)
						}
						fmt.Fprint(w, formPublicationFixture("live", formPublicationVersion("live", "Before"), formPublicationVersion("draft", "After")))
					case "POST /api/form-values-reports":
						reports++
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Fatal(err)
						}
						want := `{"data":{"type":"form-values-report","attributes":{"statistics":["viewed_form","submits","submit_rate"],"timeframe":{"key":"last_30_days"},"filter":"equals(form_id,\"form-1\")","group_by":["form_id"]}}}`
						var expected map[string]any
						_ = json.Unmarshal([]byte(want), &expected)
						if !reflect.DeepEqual(body, expected) {
							t.Errorf("report body = %#v, want %#v", body, expected)
						}
						fmt.Fprint(w, formPublicationReportFixture)
					default:
						jobs++
						t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					}
				}, args...)
				var envelope struct{ Data formPublicationResult }
				if jsonErr := json.Unmarshal([]byte(out), &envelope); jsonErr != nil {
					t.Fatalf("output=%s json error=%v command error=%v", out, jsonErr, err)
				}
				result := envelope.Data
				if err != nil || gets != 1 || reports != 1 || jobs != 0 || !result.Preview || result.Applied || result.Last30Days == nil || result.Last30Days.Views != 100 || result.Last30Days.Submits != 12 || result.Last30Days.SubmitRate != 0.12 {
					t.Fatalf("gets=%d reports=%d jobs=%d err=%v output=%s", gets, reports, jobs, err, out)
				}
			})
		}
	}
}

func TestFormPublicationApplyAndVerification(t *testing.T) {
	for _, tc := range []struct {
		name, action, after string
		wantError           bool
	}{
		{"publish content", "publish", formPublicationFixture("live", formPublicationVersion("live", "After")), false},
		{"publish status fallback", "publish", `{"data":{"attributes":{"status":"live"}}}`, false},
		{"publish unchanged live", "publish", formPublicationFixture("live", formPublicationVersion("live", "Before"), formPublicationVersion("draft", "After")), true},
		{"publish still draft", "publish", formPublicationFixture("draft", formPublicationVersion("draft", "After")), true},
		{"unpublish", "unpublish", formPublicationFixture("draft", formPublicationVersion("draft", "After")), false},
		{"unpublish still live", "unpublish", formPublicationFixture("live", formPublicationVersion("live", "Before")), true},
		{"unpublish contradictory definition", "unpublish", formPublicationFixture("draft", formPublicationVersion("live", "Before")), true},
		{"unpublish missing state", "unpublish", `{"data":{"attributes":{}}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gets, reports, jobs := 0, 0, 0
			out, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /api/forms/form-1":
					gets++
					if gets == 1 {
						fmt.Fprint(w, formPublicationFixture("live", formPublicationVersion("live", "Before"), formPublicationVersion("draft", "After")))
					} else {
						fmt.Fprint(w, tc.after)
					}
				case "POST /api/form-values-reports":
					reports++
					fmt.Fprint(w, formPublicationReportFixture)
				case "POST /api/form-" + tc.action + "-jobs":
					jobs++
					if gets != 1 || reports != 1 || jobs != 1 {
						t.Errorf("job order gets=%d reports=%d jobs=%d", gets, reports, jobs)
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					want := map[string]any{"data": map[string]any{"type": "form-" + tc.action + "-job", "relationships": map[string]any{"form": map[string]any{"data": map[string]any{"type": "form", "id": "form-1"}}}}}
					if !reflect.DeepEqual(body, want) || r.Header.Get("revision") != "2026-10-15" || r.Header.Get("Content-Type") != "application/vnd.api+json" {
						t.Errorf("job body=%#v headers=%v", body, r.Header)
					}
					w.WriteHeader(http.StatusAccepted)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
			}, tc.action, "form-1", "--apply", "--json")
			var envelope struct{ Data formPublicationResult }
			if jsonErr := json.Unmarshal([]byte(out), &envelope); jsonErr != nil {
				t.Fatalf("output=%s json error=%v command error=%v", out, jsonErr, err)
			}
			result := envelope.Data
			if (err != nil) != tc.wantError || (err != nil && ExitCode(err) == 0) || gets != 2 || reports != 1 || jobs != 1 || !result.Applied || result.Preview || result.Verified == tc.wantError {
				t.Fatalf("gets=%d reports=%d jobs=%d err=%v output=%s", gets, reports, jobs, err, out)
			}
		})
	}
}

func TestFormPublicationJobNeverRetries(t *testing.T) {
	for _, action := range []string{"publish", "unpublish"} {
		for _, code := range []int{http.StatusOK, http.StatusBadGateway, http.StatusTooManyRequests} {
			t.Run(fmt.Sprintf("%s/%d", action, code), func(t *testing.T) {
				gets, jobs := 0, 0
				_, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodGet:
						gets++
						fmt.Fprint(w, formPublicationFixture("live", formPublicationVersion("live", "Before"), formPublicationVersion("draft", "After")))
					case r.URL.Path == "/api/form-values-reports":
						fmt.Fprint(w, formPublicationReportFixture)
					default:
						jobs++
						w.Header().Set("Retry-After", "0")
						w.WriteHeader(code)
						fmt.Fprint(w, `{"errors":[{"detail":"ambiguous result"}]}`)
					}
				}, action, "form-1", "--apply", "--json")
				if err == nil || gets != 1 || jobs != 1 {
					t.Fatalf("gets=%d jobs=%d error=%v", gets, jobs, err)
				}
			})
		}
	}
}

func TestFormPublicationTransportFailureNeverRetries(t *testing.T) {
	root, flags := rcOfflineSyncRoot(t)
	gets, jobs := 0, 0
	registeredPlatformSource.BindClient = func(c *client.Client, _ *platform.Session) error {
		c.BaseURL, c.Config.KlaviyoApiKey = "https://klaviyo.test", "fixture-key"
		c.Config.Headers = map[string]string{"Idempotency-Key": "fixture-idempotency"}
		c.HTTPClient.Transport = formPublicationTransport(func(r *http.Request) (*http.Response, error) {
			response := httptest.NewRecorder()
			if r.Method == http.MethodGet {
				gets++
				fmt.Fprint(response, formPublicationFixture("draft", formPublicationVersion("draft", "After")))
			} else if r.URL.Path == "/api/form-values-reports" {
				fmt.Fprint(response, formPublicationReportFixture)
			} else {
				jobs++
				return nil, errors.New("connection lost after write")
			}
			return response.Result(), nil
		})
		return c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: "*", Steady: 10000, Interval: time.Second, Burst: 100, MaxAttempts: 4, RetryBudget: time.Second})
	}
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"forms", "publish", "form-1", "--apply", "--json"})
	err := root.Execute()
	finalizePlatformInvocation(flags, &err)
	if err == nil || gets != 1 || jobs != 1 {
		t.Fatalf("gets=%d jobs=%d error=%v", gets, jobs, err)
	}
}

func TestFormPublicationHumanPreviewWithoutLiveVersion(t *testing.T) {
	out, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, formPublicationFixture("draft", formPublicationVersion("draft", "After")))
		} else {
			fmt.Fprint(w, formPublicationReportFixture)
		}
	}, "publish", "form-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"status draft", "no live version yet", `After`, "views 100", "submits 12", "submit rate 12.00%", "preview only"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestFormPublicationReportFailurePreventsApply(t *testing.T) {
	for _, report := range []string{
		`{"data":{"attributes":{}}}`,
		`{"data":{"attributes":{"results":null}}}`,
		strings.Replace(formPublicationReportFixture, `"form-1"`, `"another-form"`, 1),
		strings.Replace(formPublicationReportFixture, `"submits":12`, `"submits":null`, 1),
		strings.Replace(formPublicationReportFixture, `"viewed_form":100`, `"viewed_form":-1`, 1),
	} {
		t.Run(report, func(t *testing.T) {
			jobs := 0
			_, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet:
					fmt.Fprint(w, formPublicationFixture("draft", formPublicationVersion("draft", "After")))
				case r.URL.Path == "/api/form-values-reports":
					fmt.Fprint(w, report)
				default:
					jobs++
					w.WriteHeader(http.StatusAccepted)
				}
			}, "publish", "form-1", "--apply", "--json")
			if err == nil || jobs != 0 {
				t.Fatalf("jobs=%d error=%v", jobs, err)
			}
		})
	}
}

func TestFormPublicationReportFollowsNextLink(t *testing.T) {
	reports := 0
	var firstBody []byte
	out, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, formPublicationFixture("draft", formPublicationVersion("draft", "After")))
			return
		}
		if r.URL.Path != "/api/form-values-reports" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			return
		}
		reports++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if reports == 1 {
			firstBody = body
			fmt.Fprint(w, `{"data":{"attributes":{"results":[]}},"links":{"next":"/api/form-values-reports?page_cursor=next"}}`)
		} else {
			if !bytes.Equal(firstBody, body) || r.URL.Query().Get("page_cursor") != "next" {
				t.Errorf("pagination changed request body or lost cursor")
			}
			fmt.Fprint(w, formPublicationReportFixture)
		}
	}, "publish", "form-1", "--json")
	if err != nil || reports != 2 || !strings.Contains(out, `"views":100`) {
		t.Fatalf("reports=%d error=%v output=%s", reports, err, out)
	}
}

func TestFormPublicationUnpublishNoOpWithoutReport(t *testing.T) {
	jobs := 0
	out, err := executeFormPublicationTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			fmt.Fprint(w, formPublicationFixture("draft", formPublicationVersion("draft", "After")))
		case r.URL.Path == "/api/form-values-reports":
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"errors":[{"detail":"report unavailable"}]}`)
		default:
			jobs++
		}
	}, "unpublish", "form-1", "--apply", "--json")
	if err != nil || jobs != 0 || !strings.Contains(out, "not live") || !strings.Contains(out, "last 30 days unavailable") {
		t.Fatalf("jobs=%d error=%v output=%s", jobs, err, out)
	}
}
