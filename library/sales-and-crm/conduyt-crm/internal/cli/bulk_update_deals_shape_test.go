package cli

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The live API reads {"updates":[{"id":...,fields...}]} for POST /bulk/deals/update (1..100 items), not the old
// ids[] + data{}. The command must expose --updates and no longer expose --ids / --data.
func TestBulkUpdateDealsSendsUpdatesShape(t *testing.T) {
	cmd := newBulkUpdateDealsCmd(&rootFlags{})
	if cmd.Flags().Lookup("updates") == nil {
		t.Fatal("bulk update-deals must take --updates")
	}
	for _, old := range []string{"ids", "data"} {
		if cmd.Flags().Lookup(old) != nil {
			t.Fatalf("bulk update-deals must not take the old --%s flag", old)
		}
	}
	for _, f := range []string{"spec.yaml"} {
		b, err := os.ReadFile(filepath.Join("..", "..", f))
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		i := strings.Index(s, "/bulk/deals/update:")
		j := strings.Index(s[i:], "/imports:")
		op := s[i : i+j]
		if !strings.Contains(op, "- updates") || strings.Contains(op, "- ids") {
			t.Fatalf("%s: /bulk/deals/update must require updates and not ids", f)
		}
	}
}

func runBulkUpdateDeals(t *testing.T, flags *rootFlags, resp string, status int, args ...string) (body string, hits int, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "POST" || r.URL.Path != "/api/v1/bulk/deals/update" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	flags.noCache = true
	cmd := newBulkUpdateDealsCmd(flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return
}

const okResp = `{"data":{"summary":{"total":1,"succeeded":1,"failed":0,"skippedNoOp":0,"written":1},"results":[{"id":"a","success":true}]}}`

// Array flags in this repo (availability set --rules) pass the parsed JSON straight through; the API owns the
// 1..100 and array-type checks (400 / 422), so the client does not duplicate them.
func TestBulkUpdateDealsRequestBody(t *testing.T) {
	arr := `[{"id":"5f0c2b1e-8a47-4c1d-9e3a-2b7d6c4a1f90","priority":"high"}]`
	body, hits, err := runBulkUpdateDeals(t, &rootFlags{}, okResp, 200, "--updates", arr)
	if err != nil || hits != 1 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
	if body != `{"updates":`+arr+`}` {
		t.Fatalf("body = %s", body)
	}
	// gocli1219: under 4.34.2 a non-array --updates is refused locally (the spec types it an array), nothing sent
	if _, hits, err = runBulkUpdateDeals(t, &rootFlags{}, okResp, 200, "--updates", `{"id":"x"}`); err == nil || hits != 0 {
		t.Fatalf("non-array --updates: hits=%d err=%v, want a local refusal", hits, err)
	}
	// 0 items go through untouched; the API answers 400/422
	for _, v := range []string{`[]`} {
		body, hits, _ = runBulkUpdateDeals(t, &rootFlags{}, okResp, 200, "--updates", v)
		if hits != 1 || body != `{"updates":`+v+`}` {
			t.Fatalf("%s: hits=%d body=%s", v, hits, body)
		}
	}
}

func TestBulkUpdateDealsMalformedJSONSendsNothing(t *testing.T) {
	_, hits, err := runBulkUpdateDeals(t, &rootFlags{}, okResp, 200, "--updates", `[{"id":`)
	if err == nil || hits != 0 {
		t.Fatalf("want parse error and no request, err=%v hits=%d", err, hits)
	}
}

func TestBulkUpdateDealsPerItemFailures(t *testing.T) {
	mixed := `{"data":{"summary":{"total":2,"succeeded":1,"failed":1,"skippedNoOp":0,"written":1},"results":[{"id":"a","success":true},{"id":"b","success":false,"error":"Deal not found"}]}}`
	allFailed := `{"data":{"summary":{"total":2,"succeeded":0,"failed":2,"skippedNoOp":0,"written":0},"results":[{"id":"a","success":false,"error":"Missing id"},{"id":"b","success":false,"error":"Deal not found"}]}}`
	for name, resp := range map[string]string{"mixed": mixed, "allFailed": allFailed} {
		_, _, err := runBulkUpdateDeals(t, &rootFlags{}, resp, 200, "--updates", `[{"id":"a"}]`)
		var ce *cliError
		if !errors.As(err, &ce) || ce.code != 6 {
			t.Fatalf("%s: want partial-failure exit 6, got %v", name, err)
		}
		if !strings.Contains(err.Error(), "Deal not found") {
			t.Fatalf("%s: failed item not reported: %v", name, err)
		}
		if _, _, err := runBulkUpdateDeals(t, &rootFlags{allowPartialFailure: true}, resp, 200, "--updates", `[{"id":"a"}]`); err != nil {
			t.Fatalf("%s with --allow-partial-failure: %v", name, err)
		}
	}
	if _, _, err := runBulkUpdateDeals(t, &rootFlags{}, okResp, 200, "--updates", `[{"id":"a"}]`); err != nil {
		t.Fatalf("all-success must exit 0: %v", err)
	}
}
