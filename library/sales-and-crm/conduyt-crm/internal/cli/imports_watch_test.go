// Sweep findings: launch-monitor failures and requested verification that is missing, false, malformed, or unfetched are partial and API-class non-zero.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestObjectItemsRejectsUnexpectedValidObjects(t *testing.T) {
	for _, payload := range []string{`{"error":"upstream unavailable"}`, `{"message":"try later"}`, `{"error":{"code":"down"},"data":[]}`, `{"data":{"error":"upstream unavailable","rows":[]}}`} {
		t.Run(payload, func(t *testing.T) {
			if items, err := objectItems(json.RawMessage(payload), false); err == nil || items != nil {
				t.Fatalf("items=%v err=%v", items, err)
			}
			if items, err := objectItems(json.RawMessage(payload), true); err == nil || items != nil {
				t.Fatalf("single items=%v err=%v", items, err)
			}
		})
	}
}

type watchGetterFunc func(context.Context, string, map[string]string) (json.RawMessage, error)

func (f watchGetterFunc) GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	return f(ctx, path, params)
}

func TestNovelImportsWatchHelpWires(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := runNovel(t, []string{"imports", "watch", "--help"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyImportJobCountsDecodeFailures(t *testing.T) {
	for _, badPath := range []string{"/imports/job", "/imports/job/side-effects"} {
		t.Run(badPath, func(t *testing.T) {
			getter := watchGetterFunc(func(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
				if path == badPath {
					return json.RawMessage(`{`), nil
				}
				if strings.HasSuffix(path, "/side-effects") {
					return json.RawMessage(`{"data":{"queued":0,"retryableFailed":0,"exhaustedFailed":0,"total":0,"completed":0,"failed":0,"held":0,"actionRequired":false,"isComplete":true}}`), nil
				}
				return json.RawMessage(`{"data":{"status":"completed","totalRows":1}}`), nil
			})
			verification, failures, fetchErr := verifyImportJob(context.Background(), getter, "job")
			if fetchErr != nil {
				t.Fatal(fetchErr)
			}
			if failures != 1 || verification.Verified || !strings.Contains(verification.Reason, "parseable") {
				t.Fatalf("failures=%d verification=%+v", failures, verification)
			}
		})
	}
}

func TestVerifyImportJobPreferredAndLegacySideEffectCounters(t *testing.T) {
	tests := []struct {
		name       string
		counters   string
		verified   bool
		wantReason string
	}{
		{"legacy only", `"retryable":1,"exhausted":2`, true, ""},
		{"preferred only", `"retryableFailed":1,"exhaustedFailed":2`, true, ""},
		{"invalid preferred with valid legacy", `"retryableFailed":"bad","retryable":1,"exhaustedFailed":2`, false, "invalid retryableFailed"},
		{"conflicting dual keys", `"retryableFailed":1,"retryable":2,"exhaustedFailed":2`, false, "conflicting retryableFailed=1 and retryable=2"},
		{"null preferred", `"retryableFailed":null,"retryable":1,"exhaustedFailed":2`, false, "invalid retryableFailed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getter := watchGetterFunc(func(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
				if strings.HasSuffix(path, "/side-effects") {
					return json.RawMessage(`{"data":{"queued":0,` + tc.counters + `,"total":0,"completed":0,"failed":0,"held":0,"actionRequired":false,"isComplete":true}}`), nil
				}
				return json.RawMessage(`{"data":{"status":"completed","totalRows":1}}`), nil
			})
			verification, failures, fetchErr := verifyImportJob(context.Background(), getter, "job")
			if fetchErr != nil || failures != 0 || verification.Verified != tc.verified || tc.wantReason != "" && !strings.Contains(verification.Reason, tc.wantReason) {
				t.Fatalf("fetchErr=%v failures=%d verification=%+v", fetchErr, failures, verification)
			}
		})
	}
}

func TestImportsWatchVerifyStrictRowCountAliases(t *testing.T) {
	for _, key := range []string{"totalRows", "createdRows", "updatedRows", "skippedRows", "errorRows", "processedRows"} {
		legacy := strings.TrimSuffix(key, "Rows") + "_rows"
		baseFields := map[string]string{
			"createdRows": `"updatedRows":0,"skippedRows":0,"errorRows":0`,
			"updatedRows": `"createdRows":0,"skippedRows":0,"errorRows":0`,
			"skippedRows": `"createdRows":0,"updatedRows":0,"errorRows":0`,
			"errorRows":   `"createdRows":0,"updatedRows":0,"skippedRows":0`,
		}
		base := baseFields[key]
		if base == "" {
			base = `"createdRows":0,"updatedRows":0,"skippedRows":0,"errorRows":0`
		}
		for _, tc := range []struct {
			name       string
			fields     string
			verified   bool
			wantReason string
		}{
			{"negative preferred", fmt.Sprintf(`%q:-1`, key), false, "invalid " + key},
			{"malformed preferred with valid legacy", fmt.Sprintf(`%q:"bad",%q:4`, key, legacy), false, "invalid " + key},
			{"conflicting dual keys", fmt.Sprintf(`%q:3,%q:4`, key, legacy), false, "conflicting " + key + "=3 and " + legacy + "=4"},
			{"legacy only valid", fmt.Sprintf(`%q:4`, legacy), true, ""},
			{"preferred only valid", fmt.Sprintf(`%q:4`, key), true, ""},
		} {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				testenv.Isolate(t)
				out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "5s", "--verify", "--json"}, func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/imports/job":
						_, _ = w.Write([]byte(`{"data":{"status":"completed",` + base + `,` + tc.fields + `}}`))
					case "/imports/job/launch-monitor":
						_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
					case "/imports/job/side-effects":
						_, _ = w.Write([]byte(`{"data":{"queued":0,"retryableFailed":0,"exhaustedFailed":0,"total":0,"completed":0,"failed":0,"held":0,"isComplete":true}}`))
					}
				})
				compact := compactTestJSON(out)
				if tc.verified {
					if err != nil || !strings.Contains(compact, `"verified":true`) {
						t.Fatalf("err=%v out=%s", err, out)
					}
				} else if ExitCode(err) != 5 || !strings.Contains(compact, `"verified":false`) || !strings.Contains(out, tc.wantReason) {
					t.Fatalf("err=%v out=%s", err, out)
				}
			})
		}
	}
}

func TestImportsWatchVerifyStrictActionRequiredAliases(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fields     string
		verified   bool
		wantReason string
		wantAction string
	}{
		{"invalid preferred with valid legacy", `"actionRequired":"bad","action_required":true`, false, "invalid actionRequired", ""},
		{"conflicting dual keys", `"actionRequired":false,"action_required":true`, false, "conflicting actionRequired=false and action_required=true", ""},
		{"legacy only true", `"action_required":true`, true, "", `"action_required":true`},
		{"legacy only false", `"action_required":false`, true, "", `"action_required":false`},
		{"absent is optional and remains unknown", ``, true, "", `"action_required":null`},
		{"preferred true", `"actionRequired":true`, true, "", `"action_required":true`},
		{"preferred false", `"actionRequired":false`, true, "", `"action_required":false`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "5s", "--verify", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/imports/job":
					_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":0,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
				case "/imports/job/launch-monitor":
					_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
				case "/imports/job/side-effects":
					comma := ""
					if tc.fields != "" {
						comma = ","
					}
					_, _ = w.Write([]byte(`{"data":{"queued":0,"retryableFailed":0,"exhaustedFailed":0,"total":0,"completed":0,"failed":0,"held":0,"isComplete":true` + comma + tc.fields + `}}`))
				}
			})
			compact := compactTestJSON(out)
			if tc.verified {
				if err != nil || !strings.Contains(compact, `"verified":true`) || !strings.Contains(compact, tc.wantAction) {
					t.Fatalf("err=%v out=%s", err, out)
				}
			} else if ExitCode(err) != 5 || !strings.Contains(out, tc.wantReason) {
				t.Fatalf("err=%v out=%s", err, out)
			}
			if tc.fields == "" {
				testenv.Isolate(t)
				table, _, tableErr := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "5s", "--verify", "--human-friendly"}, func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/imports/job":
						_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":0,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
					case "/imports/job/launch-monitor":
						_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
					case "/imports/job/side-effects":
						_, _ = w.Write([]byte(`{"data":{"queued":0,"retryableFailed":0,"exhaustedFailed":0,"total":0,"completed":0,"failed":0,"held":0,"isComplete":true}}`))
					}
				})
				if tableErr != nil || !strings.Contains(table, "ACTION REQUIRED: unknown") {
					t.Fatalf("err=%v table=%s", tableErr, table)
				}
			}
		})
	}
}

func TestImportsWatchVerifyIsCompleteExactOutput(t *testing.T) {
	for _, tc := range []struct {
		name           string
		field          string
		wantCode       int
		wantVerified   bool
		wantIsComplete bool
		wantOutput     string
	}{
		{"true", `"isComplete":true`, 0, true, true, `{"job_id":"job","status":"completed","stage":"done","created":0,"updated":0,"skipped":0,"errors":0,"verification":{"verified":true,"status":"completed","created":0,"updated":0,"skipped":0,"errors":0,"side_effects":{"queued":0,"retryable":0,"exhausted":0,"total":0,"completed":0,"failed":0,"held":0,"action_required":null,"is_complete":true},"action_required":null},"fetch_failures":0,"partial":false}`},
		{"false", `"isComplete":false`, 5, false, false, `{"job_id":"job","status":"completed","stage":"done","created":0,"updated":0,"skipped":0,"errors":0,"verification":{"verified":false,"reason":"missing side effects are not complete","status":"completed","created":0,"updated":0,"skipped":0,"errors":0,"side_effects":{"queued":0,"retryable":0,"exhausted":0,"total":0,"completed":0,"failed":0,"held":0,"action_required":null,"is_complete":false},"action_required":null},"fetch_failures":0,"partial":true,"failures":["missing side effects are not complete"]}`},
		{"absent", ``, 5, false, false, `{"job_id":"job","status":"completed","stage":"done","created":0,"updated":0,"skipped":0,"errors":0,"verification":{"verified":false,"reason":"missing or invalid isComplete","status":"completed","created":0,"updated":0,"skipped":0,"errors":0,"side_effects":{"queued":0,"retryable":0,"exhausted":0,"total":0,"completed":0,"failed":0,"held":0,"action_required":null,"is_complete":false},"action_required":null},"fetch_failures":0,"partial":true,"failures":["missing or invalid isComplete"]}`},
		{"malformed", `"isComplete":"false"`, 5, false, false, `{"job_id":"job","status":"completed","stage":"done","created":0,"updated":0,"skipped":0,"errors":0,"verification":{"verified":false,"reason":"missing or invalid isComplete","status":"completed","created":0,"updated":0,"skipped":0,"errors":0,"side_effects":{"queued":0,"retryable":0,"exhausted":0,"total":0,"completed":0,"failed":0,"held":0,"action_required":null,"is_complete":false},"action_required":null},"fetch_failures":0,"partial":true,"failures":["missing or invalid isComplete"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "5s", "--verify", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/imports/job":
					_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":0,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
				case "/imports/job/launch-monitor":
					_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
				case "/imports/job/side-effects":
					comma := ""
					if tc.field != "" {
						comma = ","
					}
					_, _ = w.Write([]byte(`{"data":{"queued":0,"retryableFailed":0,"exhaustedFailed":0,"total":0,"completed":0,"failed":0,"held":0` + comma + tc.field + `}}`))
				}
			})
			if tc.wantCode == 0 && err != nil || tc.wantCode != 0 && ExitCode(err) != tc.wantCode {
				t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
			}
			if got := compactTestJSON(out); got != tc.wantOutput {
				t.Fatalf("output mismatch\nwant: %s\n got: %s", tc.wantOutput, got)
			}
			if !tc.wantVerified && tc.name == "false" && !strings.Contains(out, "side effects are not complete") {
				t.Fatalf("typed failure does not name incomplete side effects: %s", out)
			}
			if !tc.wantVerified && tc.name != "false" && !strings.Contains(out, "missing or invalid isComplete") {
				t.Fatalf("typed failure does not name isComplete: %s", out)
			}
		})
	}
}

func TestImportsWatchVerifyRejectsNegativeSideEffectCounters(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value int
	}{{"queued", -1}, {"total", -2}, {"completed", -3}, {"failed", -4}, {"held", -5}, {"valid", 0}} {
		t.Run(tc.key, func(t *testing.T) {
			testenv.Isolate(t)
			counters := map[string]int{"queued": 0, "total": 0, "completed": 0, "failed": 0, "held": 0}
			if tc.key != "valid" {
				counters[tc.key] = tc.value
			}
			out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "5s", "--verify", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/imports/job":
					_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":0,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
				case "/imports/job/launch-monitor":
					_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
				case "/imports/job/side-effects":
					_, _ = fmt.Fprintf(w, `{"data":{"queued":%d,"retryableFailed":0,"exhaustedFailed":0,"total":%d,"completed":%d,"failed":%d,"held":%d,"actionRequired":false,"isComplete":true}}`, counters["queued"], counters["total"], counters["completed"], counters["failed"], counters["held"])
				}
			})
			compact := compactTestJSON(out)
			if tc.key == "valid" {
				if err != nil || !strings.Contains(compact, `"verified":true`) || strings.Contains(compact, `"partial":true`) {
					t.Fatalf("err=%v out=%s", err, out)
				}
				return
			}
			if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(out, "invalid "+tc.key) {
				t.Fatalf("err=%v out=%s", err, out)
			}
		})
	}
}

func TestImportsWatchVerifyRequiresEveryRenderedSideEffectCounter(t *testing.T) {
	for _, missing := range []string{"total", "completed", "failed", "held"} {
		t.Run(missing, func(t *testing.T) {
			testenv.Isolate(t)
			counters := map[string]any{
				"queued": 0, "retryableFailed": 0, "exhaustedFailed": 0,
				"total": 0, "completed": 0, "failed": 0, "held": 0,
				"actionRequired": false, "isComplete": true,
			}
			delete(counters, missing)
			sideEffects, marshalErr := json.Marshal(map[string]any{"data": counters})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "5s", "--verify", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/imports/job":
					_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":0,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
				case "/imports/job/launch-monitor":
					_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
				case "/imports/job/side-effects":
					_, _ = w.Write(sideEffects)
				}
			})
			compact := compactTestJSON(out)
			if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, `"verified":false`) || !strings.Contains(out, "missing "+missing) || !strings.Contains(compact, `"`+missing+`":null`) {
				t.Fatalf("err=%v out=%s", err, out)
			}
		})
	}
}

func TestImportsWatchPollsToCompleted(t *testing.T) {
	testenv.Isolate(t)
	var polls atomic.Int32
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job":
			if polls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"data":{"status":"processing"}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":4,"updatedRows":2,"skippedRows":1,"errorRows":0}}`))
			}
		case "/imports/job/launch-monitor":
			_, _ = w.Write([]byte(`{"data":{"stage":"delivery"}}`))
		case "/reports/sms-delivery":
			t.Fatal("job verification must not use the tenant-wide SMS delivery report")
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{"queued":2,"retryableFailed":1,"exhaustedFailed":3,"total":6,"completed":4,"failed":2,"held":1,"actionRequired":true,"isComplete":true}}`))
		default:
			http.NotFound(w, r)
		}
	}
	out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "5s", "--verify", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"status":"completed"`, `"stage":"delivery"`, `"created":4`, `"updated":2`, `"skipped":1`, `"errors":0`, `"verification":{"verified":true`, `"queued":2`, `"retryable":1`, `"exhausted":3`, `"action_required":true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}
func TestImportsWatchFailedReturnsAPIErrAfterRecap(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "watch", "bad", "--interval", "1ms", "--timeout", "1s", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/launch-monitor") {
			_, _ = w.Write([]byte(`{"data":{"stage":"rows","failureType":"validation"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"status":"failed","errorRows":3}}`))
	})
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("err=%v code=%d", err, ExitCode(err))
	}
	out = compactTestJSON(out)
	if !strings.Contains(out, `"failure_type":"validation"`) || !strings.Contains(out, `"errors":3`) {
		t.Fatalf("output=%s", out)
	}
}

func TestImportsWatchRejectsErrorEnvelopesBeforeTerminalJobData(t *testing.T) {
	for _, field := range []string{"error", "message"} {
		for _, value := range []string{`"unavailable"`, `{"code":"unavailable"}`} {
			for _, nested := range []bool{false, true} {
				name := field + "/string"
				if strings.HasPrefix(value, "{") {
					name = field + "/object"
				}
				if nested {
					name += "/nested"
				} else {
					name += "/top-level"
				}
				t.Run(name, func(t *testing.T) {
					testenv.Isolate(t)
					data := `"status":"completed","createdRows":1,"updatedRows":0,"skippedRows":0,"errorRows":0`
					payload := `{"` + field + `":` + value + `,"data":{` + data + `}}`
					if nested {
						payload = `{"data":{"` + field + `":` + value + `,` + data + `}}`
					}
					out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "1s", "--json"}, func(w http.ResponseWriter, r *http.Request) {
						_, _ = w.Write([]byte(payload))
					})
					if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), `"partial":true`) {
						t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
					}
				})
			}
		}
	}
}

func TestImportsWatchRejectsErrorEnvelopesBeforeLaunchMonitorData(t *testing.T) {
	for _, field := range []string{"error", "message"} {
		for _, value := range []string{`"unavailable"`, `{"code":"unavailable"}`} {
			for _, nested := range []bool{false, true} {
				name := field + "/string"
				if strings.HasPrefix(value, "{") {
					name = field + "/object"
				}
				if nested {
					name += "/nested"
				} else {
					name += "/top-level"
				}
				t.Run(name, func(t *testing.T) {
					testenv.Isolate(t)
					payload := `{"` + field + `":` + value + `,"data":{"stage":"done"}}`
					if nested {
						payload = `{"data":{"` + field + `":` + value + `,"stage":"done"}}`
					}
					out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "1s", "--json"}, func(w http.ResponseWriter, r *http.Request) {
						if strings.HasSuffix(r.URL.Path, "/launch-monitor") {
							_, _ = w.Write([]byte(payload))
							return
						}
						_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":1,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
					})
					if ExitCode(err) != 5 || !strings.Contains(compactTestJSON(out), `"partial":true`) {
						t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
					}
				})
			}
		}
	}
}

func TestImportsWatchUnverifiedPrintsPartialAndReturnsAPIErr(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "1s", "--verify", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job":
			_, _ = w.Write([]byte(`{"data":{"status":"completed"}}`))
		case "/imports/job/launch-monitor":
			_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
		case "/imports/job/side-effects":
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	})
	if ExitCode(err) != 5 {
		t.Fatalf("err=%v", err)
	}
	compact := compactTestJSON(out)
	if !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, `"verified":false`) {
		t.Fatalf("out=%s", out)
	}
}

func TestImportsWatchMissingTerminalCountersIsPartialWithoutVerify(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "1s", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/imports/job":
			_, _ = w.Write([]byte(`{"data":{"status":"completed"}}`))
		case "/imports/job/launch-monitor":
			_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
		}
	})
	compact := compactTestJSON(out)
	if ExitCode(err) != 5 || !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, "missing or invalid counters: created, errors, skipped, updated") {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestImportsWatchPartialFetchFailuresAreTyped(t *testing.T) {
	for _, endpoint := range []string{"launch-monitor", "verification-side-effects"} {
		for _, tc := range []struct{ status, code int }{{401, 4}, {403, 4}, {404, 3}, {500, 5}} {
			t.Run(endpoint+"/"+http.StatusText(tc.status), func(t *testing.T) {
				testenv.Isolate(t)
				args := []string{"imports", "watch", "job", "--interval", "1ms", "--timeout", "10s", "--json"}
				if endpoint == "verification-side-effects" {
					args = append(args, "--verify")
				}
				out, _, err := runNovel(t, args, func(w http.ResponseWriter, r *http.Request) {
					failed := endpoint == "launch-monitor" && strings.HasSuffix(r.URL.Path, "/launch-monitor") || endpoint == "verification-side-effects" && strings.HasSuffix(r.URL.Path, "/side-effects")
					if failed {
						w.WriteHeader(tc.status)
						_, _ = w.Write([]byte(`{"message":"failure"}`))
						return
					}
					if strings.HasSuffix(r.URL.Path, "/launch-monitor") {
						_, _ = w.Write([]byte(`{"data":{"stage":"done"}}`))
						return
					}
					if strings.HasSuffix(r.URL.Path, "/side-effects") {
						_, _ = w.Write([]byte(`{"data":{"queued":0,"retryableFailed":0,"exhaustedFailed":0,"total":0,"completed":0,"failed":0,"held":0,"actionRequired":false,"isComplete":true}}`))
						return
					}
					_, _ = w.Write([]byte(`{"data":{"status":"completed","createdRows":1,"updatedRows":0,"skippedRows":0,"errorRows":0}}`))
				})
				if ExitCode(err) != tc.code || !strings.Contains(compactTestJSON(out), `"partial":true`) {
					t.Fatalf("err=%v code=%d out=%s", err, ExitCode(err), out)
				}
			})
		}
	}
}
