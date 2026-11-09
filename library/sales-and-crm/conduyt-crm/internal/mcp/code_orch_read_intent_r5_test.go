package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
)

// gocli1219 r5 (Codex gocli1219-r4 P2): a code-orchestration POST/PUT/PATCH that is a read by intent must reach the
// client through the *Query* methods (readOnlyIntent=true), as the direct CLI does; a mutation keeps the mutation
// methods. See .printing-press-patches.json entry gocli1219-r5-read-intent-orchestration.

// recordingWriteSender records the client method codeOrchSendWrite chose.
type recordingWriteSender struct{ called []string }

func (r *recordingWriteSender) rec(name string) (json.RawMessage, int, error) {
	r.called = append(r.called, name)
	return json.RawMessage(`{}`), 200, nil
}

func (r *recordingWriteSender) Post(context.Context, string, any) (json.RawMessage, int, error) {
	return r.rec("Post")
}
func (r *recordingWriteSender) PostWithHeaders(context.Context, string, any, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PostWithHeaders")
}
func (r *recordingWriteSender) PostQueryWithParams(context.Context, string, map[string]string, any) (json.RawMessage, int, error) {
	return r.rec("PostQueryWithParams")
}
func (r *recordingWriteSender) PostQueryWithParamsAndHeaders(context.Context, string, map[string]string, any, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PostQueryWithParamsAndHeaders")
}
func (r *recordingWriteSender) PostMultipartWithHeaders(context.Context, string, map[string]string, map[string]string, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PostMultipartWithHeaders")
}
func (r *recordingWriteSender) PostQueryMultipartWithParamsAndHeaders(context.Context, string, map[string]string, map[string]string, map[string]string, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PostQueryMultipartWithParamsAndHeaders")
}
func (r *recordingWriteSender) Put(context.Context, string, any) (json.RawMessage, int, error) {
	return r.rec("Put")
}
func (r *recordingWriteSender) PutWithHeaders(context.Context, string, any, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PutWithHeaders")
}
func (r *recordingWriteSender) PutQueryWithParams(context.Context, string, map[string]string, any) (json.RawMessage, int, error) {
	return r.rec("PutQueryWithParams")
}
func (r *recordingWriteSender) PutQueryWithParamsAndHeaders(context.Context, string, map[string]string, any, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PutQueryWithParamsAndHeaders")
}
func (r *recordingWriteSender) PutMultipartWithHeaders(context.Context, string, map[string]string, map[string]string, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PutMultipartWithHeaders")
}
func (r *recordingWriteSender) PutQueryMultipartWithParamsAndHeaders(context.Context, string, map[string]string, map[string]string, map[string]string, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PutQueryMultipartWithParamsAndHeaders")
}
func (r *recordingWriteSender) Patch(context.Context, string, any) (json.RawMessage, int, error) {
	return r.rec("Patch")
}
func (r *recordingWriteSender) PatchWithHeaders(context.Context, string, any, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PatchWithHeaders")
}
func (r *recordingWriteSender) PatchQueryWithParams(context.Context, string, map[string]string, any) (json.RawMessage, int, error) {
	return r.rec("PatchQueryWithParams")
}
func (r *recordingWriteSender) PatchQueryWithParamsAndHeaders(context.Context, string, map[string]string, any, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PatchQueryWithParamsAndHeaders")
}
func (r *recordingWriteSender) PatchMultipartWithHeaders(context.Context, string, map[string]string, map[string]string, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PatchMultipartWithHeaders")
}
func (r *recordingWriteSender) PatchQueryMultipartWithParamsAndHeaders(context.Context, string, map[string]string, map[string]string, map[string]string, map[string]string) (json.RawMessage, int, error) {
	return r.rec("PatchQueryMultipartWithParamsAndHeaders")
}

// cliReadOnlyByRoute maps "METHOD path" to the mcp:read-only annotation of every direct CLI command bound to it: the
// generator sets that annotation from the same read-command classifier that picks PostQueryWithParams.
func cliReadOnlyByRoute() map[string][]bool {
	out := map[string][]bool{}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if m, p := c.Annotations["pp:method"], c.Annotations["pp:path"]; m != "" && p != "" {
			key := strings.ToUpper(m) + " " + p
			out[key] = append(out[key], c.Annotations["mcp:read-only"] == "true")
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(cli.RootCmd())
	return out
}

// TestCodeOrchWriteMethodFollowsIntentForEveryEndpoint walks every generated code-orchestration endpoint: a POST/PUT/
// PATCH that is not gated (reachable through conduyt-crm_read) must dispatch to a *Query* client method, a gated one to
// the mutation method, with and without headers, and the intent must agree with the direct CLI's mcp:read-only.
func TestCodeOrchWriteMethodFollowsIntentForEveryEndpoint(t *testing.T) {
	routes := cliReadOnlyByRoute()
	total, writes, reads, cliChecked := 0, 0, 0, 0
	var readIDs []string
	for i := range codeOrchEndpoints {
		ep := &codeOrchEndpoints[i]
		total++
		method := codeOrchNormalizeMethod(ep.Method)
		if method != "POST" && method != "PUT" && method != "PATCH" {
			continue
		}
		writes++
		readIntent := !codeOrchResolvedNeedsConfirm(ep, method, ep.Path, map[string]string{})
		if readIntent != (!ep.NeedsConfirm && !ep.Mutating) {
			t.Errorf("%s: resolved intent %v disagrees with NeedsConfirm=%v Mutating=%v", ep.ID, readIntent, ep.NeedsConfirm, ep.Mutating)
		}
		if cliRO, ok := routes[method+" "+ep.Path]; ok {
			cliChecked++
			for _, ro := range cliRO {
				if ro != readIntent {
					t.Errorf("%s (%s %s): code-orchestration read intent %v, direct CLI mcp:read-only %v", ep.ID, method, ep.Path, readIntent, ro)
				}
			}
		}
		if readIntent {
			reads++
			readIDs = append(readIDs, ep.ID)
		}
		var multipart *codeOrchMultipartBody
		if ep.Multipart {
			multipart = &codeOrchMultipartBody{fields: map[string]string{}, files: map[string]string{}}
		}
		for _, hdrs := range []map[string]string{nil, {"X-Test": "1"}} {
			rec := &recordingWriteSender{}
			if _, err := codeOrchSendWrite(context.Background(), rec, method, readIntent, ep.Path, map[string]any{}, multipart, hdrs); err != nil {
				t.Fatalf("%s: %v", ep.ID, err)
			}
			if len(rec.called) != 1 {
				t.Fatalf("%s: %d client calls, want 1", ep.ID, len(rec.called))
			}
			got := rec.called[0]
			wantPrefix := map[string]string{"POST": "Post", "PUT": "Put", "PATCH": "Patch"}[method]
			if !strings.HasPrefix(got, wantPrefix) {
				t.Errorf("%s: %s dispatched to %s", ep.ID, method, got)
			}
			if strings.Contains(got, "Query") != readIntent {
				t.Errorf("%s (read intent %v, headers %v, multipart %v): dispatched to %s", ep.ID, readIntent, hdrs != nil, ep.Multipart, got)
			}
			if strings.Contains(got, "Multipart") != ep.Multipart {
				t.Errorf("%s: multipart %v dispatched to %s", ep.ID, ep.Multipart, got)
			}
		}
	}
	for _, id := range []string{"dashboards.run.post-dashboards-id", "dialer.get-token"} {
		found := false
		for _, r := range readIDs {
			found = found || r == id
		}
		if !found {
			t.Errorf("%s must be a read by intent", id)
		}
	}
	t.Logf("endpoints=%d write-method=%d read-intent(Query)=%d %v cli-cross-checked=%d", total, writes, reads, readIDs, cliChecked)
}

type readIntentServer struct {
	hits    atomic.Int32
	failFor int32 // the first failFor requests answer 503 Retry-After: 0
}

func newReadIntentServer(t *testing.T, failFor int32) *readIntentServer {
	t.Helper()
	s := &readIntentServer{failFor: failFor}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := s.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n <= s.failFor {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ok":true},"meta":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_CACHE_DIR", t.TempDir())
	t.Setenv(cliutil.VerifyEnvVar, "")
	t.Setenv(cliutil.VerifyLiveHTTPEnvVar, "")
	t.Setenv(cliutil.DogfoodEnvVar, "")
	return s
}

// seedCacheMarker writes a file in the response-cache resources tree the mutation path evicts.
func seedCacheMarker(t *testing.T) string {
	t.Helper()
	dir, err := cliutil.CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "http", "resources", "seed", "marker.json")
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return marker
}

func readIntentCall(t *testing.T, execute bool, id string, params map[string]any) *mcplib.CallToolResult {
	t.Helper()
	fresh := make(map[string]any, len(params)) // codeOrchInvoke consumes positional params from the map
	for k, v := range params {
		fresh[k] = v
	}
	args := map[string]any{"endpoint_id": id, "params": fresh}
	if execute {
		args["confirm"] = true
	}
	req := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: args}}
	var res *mcplib.CallToolResult
	var err error
	if execute {
		res, err = handleCodeOrchExecute(context.Background(), req)
	} else {
		res, err = handleCodeOrchRead(context.Background(), req)
	}
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	return res
}

func resultText(res *mcplib.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcplib.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

var readIntentPOSTs = []struct {
	id     string
	params map[string]any
}{
	{"dashboards.run.post-dashboards-id", map[string]any{"id": "d1"}},
	{"dialer.get-token", map[string]any{}},
}

// Through conduyt-crm_read, verify mode performs the read (readOnlyIntent=true skips the short-circuit).
func TestCodeOrchReadIntentPOSTRunsInVerifyMode(t *testing.T) {
	for _, tc := range readIntentPOSTs {
		t.Run(tc.id, func(t *testing.T) {
			s := newReadIntentServer(t, 0)
			t.Setenv(cliutil.VerifyEnvVar, "1")
			res := readIntentCall(t, false, tc.id, tc.params)
			if res.IsError || s.hits.Load() != 1 || !strings.Contains(resultText(res), `"ok"`) {
				t.Fatalf("hits=%d isErr=%v text=%s; a read-intent POST must reach the server in verify mode", s.hits.Load(), res.IsError, resultText(res))
			}
		})
	}
}

// Through conduyt-crm_read, a transient 503 is retried (a read is replay-safe).
func TestCodeOrchReadIntentPOSTRetriesATransientFailure(t *testing.T) {
	for _, tc := range readIntentPOSTs {
		t.Run(tc.id, func(t *testing.T) {
			s := newReadIntentServer(t, 1)
			res := readIntentCall(t, false, tc.id, tc.params)
			if res.IsError || s.hits.Load() != 2 {
				t.Fatalf("hits=%d isErr=%v text=%s; want one retry after the 503", s.hits.Load(), res.IsError, resultText(res))
			}
		})
	}
}

// Through conduyt-crm_read, a successful read leaves the response cache alone.
func TestCodeOrchReadIntentPOSTDoesNotInvalidateTheCache(t *testing.T) {
	for _, tc := range readIntentPOSTs {
		t.Run(tc.id, func(t *testing.T) {
			s := newReadIntentServer(t, 0)
			marker := seedCacheMarker(t)
			res := readIntentCall(t, false, tc.id, tc.params)
			if res.IsError || s.hits.Load() != 1 {
				t.Fatalf("hits=%d isErr=%v text=%s", s.hits.Load(), res.IsError, resultText(res))
			}
			if _, err := os.Stat(marker); err != nil {
				t.Fatalf("a read-intent POST evicted the response cache: %v", err)
			}
		})
	}
}

// A mutation through conduyt-crm_execute is unchanged: verify mode short-circuits it, a 503 is not retried, and a
// success evicts the response cache.
func TestCodeOrchMutationPOSTKeepsMutationIntent(t *testing.T) {
	const id = "contacts.create"
	params := func() map[string]any { return map[string]any{"firstName": "A"} }
	t.Run("verify mode short-circuits", func(t *testing.T) {
		s := newReadIntentServer(t, 0)
		t.Setenv(cliutil.VerifyEnvVar, "1")
		res := readIntentCall(t, true, id, params())
		if res.IsError || s.hits.Load() != 0 {
			t.Fatalf("hits=%d isErr=%v text=%s; a mutation must not reach the server in verify mode", s.hits.Load(), res.IsError, resultText(res))
		}
	})
	t.Run("no retry", func(t *testing.T) {
		s := newReadIntentServer(t, 1)
		res := readIntentCall(t, true, id, params())
		if !res.IsError || s.hits.Load() != 1 {
			t.Fatalf("hits=%d isErr=%v text=%s; a mutation 503 must not be retried", s.hits.Load(), res.IsError, resultText(res))
		}
	})
	t.Run("cache invalidated", func(t *testing.T) {
		s := newReadIntentServer(t, 0)
		marker := seedCacheMarker(t)
		res := readIntentCall(t, true, id, params())
		if res.IsError || s.hits.Load() != 1 {
			t.Fatalf("hits=%d isErr=%v text=%s", s.hits.Load(), res.IsError, resultText(res))
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("a mutation left the response cache in place")
		}
	})
	t.Run("not callable through read", func(t *testing.T) {
		s := newReadIntentServer(t, 0)
		res := readIntentCall(t, false, id, params())
		if !res.IsError || s.hits.Load() != 0 {
			t.Fatalf("hits=%d isErr=%v; a gated endpoint must be refused by conduyt-crm_read", s.hits.Load(), res.IsError)
		}
	})
}
