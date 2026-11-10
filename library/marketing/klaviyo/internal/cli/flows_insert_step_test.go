// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
)

const flowInsertStepTestInput = `{"type":"time-delay","data":{"unit":"hours","value":2}}`

type flowInsertStepTestTransport struct{ handler http.HandlerFunc }

func (transport flowInsertStepTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, req)
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	response := recorder.Result()
	response.Request = req
	return response, nil
}

func flowInsertStepTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	t.Setenv("KLAVIYO_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("KLAVIYO_BASE_URL", "https://a.klaviyo.com")
	t.Setenv("KLAVIYO_API_KEY", "synthetic")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	previousSource, previousHooks := registeredPlatformSource, clientHooks
	registeredPlatformSource, clientHooks = nil, nil
	t.Cleanup(func() { registeredPlatformSource, clientHooks = previousSource, previousHooks })
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient.Transport = flowInsertStepTestTransport{handler: handler}
		return nil
	})
}

func flowInsertStepTestDefinition(kind, target string, inserted bool) map[string]any {
	anchor := map[string]any{"id": "anchor", "type": kind, "data": map[string]any{"message": map[string]any{"subject_line": "Welcome"}}}
	switch kind {
	case "conditional-split":
		anchor["links"] = map[string]any{"next_if_true": nil, "next_if_false": target}
	case "multi-branch-split":
		anchor["data"] = map[string]any{"name": "Audience", "branches": []any{
			map[string]any{"branch_id": "vip", "name": "VIP", "links": map[string]any{"next": target}},
			map[string]any{"branch_id": "else", "is_else": true, "links": map[string]any{"next": nil}},
		}}
	default:
		anchor["links"] = map[string]any{"next": target}
	}
	actions := []any{anchor, map[string]any{"id": "end", "type": "send-email", "links": map[string]any{"next": nil}, "data": map[string]any{"message": map[string]any{"name": "Follow up"}}}}
	if inserted {
		actions = append(actions, map[string]any{"id": "created", "type": "time-delay", "links": map[string]any{"next": "end"}, "data": map[string]any{"unit": "hours", "value": 3}})
	}
	return map[string]any{"entry_action_id": "anchor", "triggers": []any{}, "profile_filter": nil, "actions": actions}
}

func flowInsertStepTestWriteFlow(w http.ResponseWriter, status string, definition map[string]any) {
	w.Header().Set("Content-Type", "application/vnd.api+json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "flow", "type": "flow", "attributes": map[string]any{
		"name": "Welcome flow", "status": status, "definition": definition,
	}}})
}

func flowInsertStepTestExecute(t *testing.T, flags *rootFlags, input string, args ...string) (flowInsertStepResult, string, error) {
	t.Helper()
	var output bytes.Buffer
	cmd := newFlowsInsertStepCmd(flags)
	cmd.SilenceUsage = true
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var result flowInsertStepResult
	if output.Len() > 0 {
		if decodeErr := json.Unmarshal(output.Bytes(), &result); decodeErr != nil {
			t.Fatalf("invalid JSON output: %v: %s", decodeErr, output.String())
		}
	}
	return result, output.String(), err
}

func TestFlowsInsertStepPreviewMakesNoPost(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprint("dry-run=", dryRun), func(t *testing.T) {
			gets, posts := 0, 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					posts++
					http.Error(w, "unexpected write", 500)
					return
				}
				gets++
				if r.URL.Query().Get("additional-fields[flow]") != "definition" {
					t.Error("flow definition was not requested")
				}
				flowInsertStepTestWriteFlow(w, "draft", flowInsertStepTestDefinition("send-email", "end", false))
			})
			args := []string{"flow", "--after", "anchor", "--stdin"}
			if dryRun {
				args = append(args, "--apply")
			}
			result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true, dryRun: dryRun}, flowInsertStepTestInput, args...)
			if err != nil {
				t.Fatal(err)
			}
			if gets != 1 || posts != 0 || !result.Preview || result.DryRun != dryRun || result.Applied || result.Verified {
				t.Fatalf("gets=%d posts=%d result=%+v", gets, posts, result)
			}
			if len(result.Before.Actions) != 2 || len(result.After.Actions) != 3 || !result.After.Actions[1].New || result.After.Actions[1].Label != "2 hours" {
				t.Fatalf("before/after preview: %+v / %+v", result.Before, result.After)
			}
			if result.Before.Actions[0].Label != "Welcome" || result.AfterSource != "preview" {
				t.Fatalf("preview labels/source: %+v", result)
			}
		})
	}
}

func TestFlowsInsertStepApplyBodyAndUncachedVerification(t *testing.T) {
	for _, tt := range []struct{ kind, link string }{{"send-email", "next"}, {"conditional-split", "next_if_false"}, {"multi-branch-split", "vip"}} {
		t.Run(tt.kind, func(t *testing.T) {
			var calls []string
			var posted map[string]any
			gets := 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				if r.Header.Get("revision") != "2026-10-15" {
					t.Errorf("revision = %q", r.Header.Get("revision"))
				}
				switch r.Method + " " + r.URL.Path {
				case "GET /api/flows/flow":
					gets++
					target := "end"
					if gets == 2 {
						target = "created"
					}
					flowInsertStepTestWriteFlow(w, "draft", flowInsertStepTestDefinition(tt.kind, target, gets == 2))
				case "POST /api/flow-actions":
					if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
						t.Error(err)
					}
					w.Header().Set("Content-Type", "application/vnd.api+json")
					w.WriteHeader(http.StatusCreated)
					fmt.Fprint(w, `{"data":{"id":"created","type":"flow-action"}}`)
				default:
					http.NotFound(w, r)
				}
			})
			result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, flowInsertStepTestInput,
				"flow", "--after", "anchor", "--link", tt.link, "--stdin", "--apply")
			if err != nil {
				t.Fatal(err)
			}
			if !result.Applied || !result.Verified || result.Preview || result.ActionID != "created" || result.AfterSource != "re-read" {
				t.Fatalf("result = %+v", result)
			}
			wantCalls := []string{"GET /api/flows/flow", "POST /api/flow-actions", "GET /api/flows/flow"}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Fatalf("requests = %v, want %v", calls, wantCalls)
			}
			anchor := map[string]any{"type": tt.kind, "links": map[string]any{tt.link: result.TemporaryID}}
			if tt.kind == "multi-branch-split" {
				delete(anchor, "links")
				anchor["data"] = map[string]any{"branches": []any{map[string]any{"branch_id": tt.link, "links": map[string]any{"next": result.TemporaryID}}}}
			}
			want := map[string]any{"data": map[string]any{"type": "flow-action", "attributes": map[string]any{
				"definition":        map[string]any{"type": "time-delay", "temporary_id": result.TemporaryID, "data": map[string]any{"unit": "hours", "value": float64(2)}},
				"allow_live_action": false,
				"anchor-action":     map[string]any{"data": map[string]any{"type": "flow-action", "id": "anchor", "attributes": map[string]any{"definition": anchor}}},
			}}}
			if !reflect.DeepEqual(posted, want) || !strings.HasPrefix(result.TemporaryID, "insert-") {
				t.Fatalf("posted = %#v, want %#v", posted, want)
			}
			encoded, _ := json.Marshal(result.After)
			if !bytes.Contains(encoded, []byte(`"label":"3 hours"`)) || bytes.Contains(encoded, []byte(result.TemporaryID)) {
				t.Fatalf("after view was not built from the persisted definition: %s", encoded)
			}
		})
	}
}

func TestFlowsInsertStepRejectsUnsupportedTypesBeforePost(t *testing.T) {
	for _, kind := range []string{"conditional-split", "trigger-split", "action-output-split", "ab-test", "content-experiment", "send-whatsapp-with-ac", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls++; http.Error(w, "unexpected request", 500) })
			_, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, fmt.Sprintf(`{"type":%q}`, kind),
				"flow", "--after", "anchor", "--stdin", "--apply")
			if err == nil || !strings.Contains(err.Error(), "unsupported step type") || calls != 0 {
				t.Fatalf("error=%v requests=%d", err, calls)
			}
		})
	}
}

func TestFlowsInsertStepRejectsInvalidInputBeforePost(t *testing.T) {
	for _, input := range []string{
		`{"type":"time-delay","links":null,"data":{"value":1}}`,
		`{"type":"time-delay","links":{"next":"end"},"data":{"value":1}}`,
		`{"type":"send-email","id":"existing"}`,
		`{"type":"send-email","data":[]}`,
		`{"type":"send-email"} {}`,
		`{"type":"send-email"} trailing`,
		`{"type":"multi-branch-split","data":{"branches":[{"branch_id":"a","links":{"next":"end"}}]}}`,
		`null`,
	} {
		t.Run(input, func(t *testing.T) {
			calls := 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
			_, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, input, "flow", "--after", "anchor", "--stdin", "--apply")
			if err == nil || calls != 0 {
				t.Fatalf("error=%v requests=%d", err, calls)
			}
		})
	}
}

func TestFlowsInsertStepLiveGuard(t *testing.T) {
	for _, kind := range []string{"time-delay", "countdown-delay", "target-date", "update-profile", "send-webhook", "list-update", "multi-branch-split", "back-in-stock-delay", "code"} {
		t.Run(kind, func(t *testing.T) {
			posts := 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				flowInsertStepTestWriteFlow(w, "live", flowInsertStepTestDefinition("send-email", "end", false))
			})
			result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, fmt.Sprintf(`{"type":%q,"data":{}}`, kind),
				"flow", "--after", "anchor", "--stdin", "--apply")
			if err == nil || !strings.Contains(err.Error(), "--allow-live-action") || posts != 0 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "profiles already in the flow") {
				t.Fatalf("error=%v posts=%d result=%+v", err, posts, result)
			}
		})
	}
}

func TestFlowsInsertStepLiveOptInAndDraftCommunication(t *testing.T) {
	for _, tt := range []struct {
		kind  string
		allow bool
	}{{"time-delay", true}, {"send-email", false}, {"send-whatsapp", false}} {
		t.Run(tt.kind, func(t *testing.T) {
			posts := 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if anyPath(body, "data.attributes.allow_live_action") != tt.allow {
						t.Error("live-action flag missing from request")
					}
					w.WriteHeader(http.StatusCreated)
					fmt.Fprint(w, `{"data":{"id":"created"}}`)
					return
				}
				target := "end"
				if posts > 0 {
					target = "created"
				}
				def := flowInsertStepTestDefinition("send-email", target, posts > 0)
				if posts > 0 {
					def["actions"].([]any)[2].(map[string]any)["type"] = tt.kind
				}
				flowInsertStepTestWriteFlow(w, "live", def)
			})
			args := []string{"flow", "--after", "anchor", "--stdin", "--apply"}
			if tt.allow {
				args = append(args, "--allow-live-action")
			}
			result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, fmt.Sprintf(`{"type":%q,"data":{"value":2}}`, tt.kind), args...)
			if err != nil || posts != 1 || !result.Verified {
				t.Fatalf("error=%v posts=%d result=%+v", err, posts, result)
			}
			if !tt.allow && (len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "Draft")) {
				t.Fatalf("missing Draft notice: %+v", result)
			}
		})
	}
}

func TestFlowsInsertStepValidatesAnchorAndLinkBeforePost(t *testing.T) {
	for _, tt := range []struct{ kind, anchor, link string }{
		{"send-email", "missing", "next"}, {"send-email", "anchor", "next_if_true"},
		{"conditional-split", "anchor", "next"}, {"multi-branch-split", "anchor", "missing"},
	} {
		t.Run(tt.kind+"/"+tt.anchor+"/"+tt.link, func(t *testing.T) {
			posts := 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				flowInsertStepTestWriteFlow(w, "draft", flowInsertStepTestDefinition(tt.kind, "end", false))
			})
			_, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, flowInsertStepTestInput,
				"flow", "--after", tt.anchor, "--link", tt.link, "--stdin", "--apply")
			if err == nil || posts != 0 {
				t.Fatalf("error=%v posts=%d", err, posts)
			}
		})
	}
}

func TestFlowsInsertStepVerificationFailureReturnsError(t *testing.T) {
	for _, failure := range []string{"unchanged-link", "missing-action", "wrong-type", "missing-response-id", "read-error"} {
		t.Run(failure, func(t *testing.T) {
			gets, posts := 0, 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					w.WriteHeader(http.StatusCreated)
					if failure == "missing-response-id" {
						fmt.Fprint(w, `{"data":{}}`)
					} else {
						fmt.Fprint(w, `{"data":{"id":"created"}}`)
					}
					return
				}
				gets++
				if gets == 2 && failure == "read-error" {
					http.Error(w, "read failed", 404)
					return
				}
				target := "end"
				inserted := gets == 2 && failure != "missing-action"
				if gets == 2 && failure != "unchanged-link" {
					target = "created"
				}
				def := flowInsertStepTestDefinition("send-email", target, inserted)
				if gets == 2 && failure == "wrong-type" {
					def["actions"].([]any)[2].(map[string]any)["type"] = "send-email"
				}
				flowInsertStepTestWriteFlow(w, "draft", def)
			})
			result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, flowInsertStepTestInput,
				"flow", "--after", "anchor", "--stdin", "--apply")
			if err == nil || !strings.Contains(err.Error(), "verification failed") || gets != 2 || posts != 1 || !result.Applied || result.Verified {
				t.Fatalf("error=%v gets=%d posts=%d result=%+v", err, gets, posts, result)
			}
		})
	}
}

func TestFlowsInsertStepVerifiesDownstreamLink(t *testing.T) {
	for _, tc := range []struct {
		name, downstream string
		links            map[string]any
		verified         bool
	}{
		{"preserved", "end", map[string]any{"next": "end"}, true},
		{"missing-next", "end", map[string]any{}, false},
		{"missing-links", "end", nil, false},
		{"null-next", "end", map[string]any{"next": nil}, false},
		{"wrong-next", "end", map[string]any{"next": "anchor"}, false},
		{"terminal-null", "", map[string]any{"next": nil}, true},
		{"terminal-wrong-next", "", map[string]any{"next": "end"}, false},
		{"terminal-empty-next", "", map[string]any{"next": ""}, false},
		{"terminal-missing-next", "", map[string]any{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gets, posts := 0, 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					w.WriteHeader(http.StatusCreated)
					fmt.Fprint(w, `{"data":{"id":"created"}}`)
					return
				}
				gets++
				target := tc.downstream
				if gets == 2 {
					target = "created"
				}
				def := flowInsertStepTestDefinition("send-email", target, gets == 2)
				if gets == 1 && tc.downstream == "" {
					def["actions"].([]any)[0].(map[string]any)["links"] = map[string]any{"next": nil}
				}
				if gets == 2 {
					def["actions"].([]any)[2].(map[string]any)["links"] = tc.links
				}
				flowInsertStepTestWriteFlow(w, "draft", def)
			})
			result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, flowInsertStepTestInput,
				"flow", "--after", "anchor", "--stdin", "--apply")
			if gets != 2 || posts != 1 || !result.Applied || result.Verified != tc.verified {
				t.Fatalf("error=%v gets=%d posts=%d result=%+v", err, gets, posts, result)
			}
			if tc.verified {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "verification failed") || !strings.Contains(result.Error, "downstream") {
				t.Fatalf("expected downstream verification failure: error=%v result=%+v", err, result)
			}
		})
	}
}

func TestFlowsInsertStepUnknownResultNeverRetries(t *testing.T) {
	for _, failure := range []string{"500", "timeout", "redirect"} {
		t.Run(failure, func(t *testing.T) {
			gets, posts := 0, 0
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					gets++
					flowInsertStepTestWriteFlow(w, "draft", flowInsertStepTestDefinition("send-email", "end", false))
					return
				}
				posts++
				if r.Header.Get("Idempotency-Key") != "" || r.Header.Get("X-Idempotency-Key") != "" {
					t.Error("insert carried an idempotency header")
				}
				switch failure {
				case "500":
					http.Error(w, "server error", 500)
				case "timeout":
					<-r.Context().Done()
				case "redirect":
					http.Redirect(w, r, "/api/flow-actions", http.StatusTemporaryRedirect)
				}
			})
			registerClientHook(func(c *client.Client) error {
				c.Config.Headers = map[string]string{"Idempotency-Key": "synthetic-replay-key", "X-Idempotency-Key": "synthetic-replay-key"}
				return nil
			})
			result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true, timeout: 100 * time.Millisecond}, flowInsertStepTestInput,
				"flow", "--after", "anchor", "--stdin", "--apply")
			if err == nil || !strings.Contains(err.Error(), "unknown, re-read the flow before trying again") || gets != 1 || posts != 1 || result.Verified {
				t.Fatalf("error=%v gets=%d posts=%d result=%+v", err, gets, posts, result)
			}
		})
	}
}

type flowInsertStepTestTerminal struct{ bytes.Buffer }

func (*flowInsertStepTestTerminal) IsTerminal() bool { return true }

func TestFlowsInsertStepRendersBranches(t *testing.T) {
	for _, tt := range []struct{ kind, link, label string }{
		{"conditional-split", "next_if_false", "next_if_false"}, {"multi-branch-split", "vip", "vip VIP"},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				flowInsertStepTestWriteFlow(w, "draft", flowInsertStepTestDefinition(tt.kind, "end", false))
			})
			var output flowInsertStepTestTerminal
			cmd := newFlowsInsertStepCmd(&rootFlags{})
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)
			cmd.SetIn(strings.NewReader(flowInsertStepTestInput))
			cmd.SetArgs([]string{"flow", "--after", "anchor", "--link", tt.link, "--stdin"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{"Before:", "After (preview):", "  - " + tt.label, "    - insert-", "(time-delay) 2 hours [NEW]", "    - end (send-email) Follow up"} {
				if !strings.Contains(output.String(), text) {
					t.Fatalf("missing %q in:\n%s", text, output.String())
				}
			}
		})
	}
}

func TestFlowsInsertStepFileAndRegistration(t *testing.T) {
	flowInsertStepTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		flowInsertStepTestWriteFlow(w, "draft", flowInsertStepTestDefinition("send-email", "end", false))
	})
	file := filepath.Join(t.TempDir(), "step.json")
	if err := os.WriteFile(file, []byte(flowInsertStepTestInput), 0600); err != nil {
		t.Fatal(err)
	}
	result, _, err := flowInsertStepTestExecute(t, &rootFlags{asJSON: true}, "", "flow", "--after", "anchor", "--step-file", file)
	if err != nil || !result.Preview {
		t.Fatalf("error=%v result=%+v", err, result)
	}
	root := newRootCmd(&rootFlags{})
	cmd, _, err := root.Find([]string{"flows", "insert-step"})
	if err != nil || cmd.Name() != "insert-step" {
		t.Fatalf("command not registered: %v", err)
	}
}
