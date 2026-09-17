package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
)

func TestReportsGetFunnelResolvesOnlyPipeline(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"reports", "get-funnel", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pipelines":
			_, _ = w.Write([]byte(`{"data":[{"id":"11111111-1111-4111-8111-111111111111","name":"Sales"}]}`))
		case "/reports/funnel":
			if r.URL.Query().Get("pipelineId") != "11111111-1111-4111-8111-111111111111" {
				t.Fatalf("query=%s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		}
	})
	if err != nil || !strings.Contains(compactTestJSON(out), `"pipeline_id_source":"only_pipeline"`) {
		t.Fatalf("err=%v out=%s", err, out)
	}
}

func TestReportsGetFunnelPipelineResolutionErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status, code     int
	}{
		{"none", `{"data":[]}`, "no pipelines found", 200, 3},
		{"several", `{"data":[{"id":"11111111-1111-4111-8111-111111111111","name":"Sales"},{"id":"22222222-2222-4222-8222-222222222222","name":"Renewals"}]}`, "Sales", 200, 2},
		{"forbidden", `{"error":"forbidden"}`, "pass --pipeline-id or grant the pipelines scope", 403, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			_, _, err := runNovel(t, []string{"reports", "get-funnel", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			if ExitCode(err) != tc.code || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestReportsGetFunnelPipelineResolutionRequiresCompleteEnumeration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		secondBody string
	}{
		{"inconsistent total", `{"data":[{"id":"11111111-1111-4111-8111-111111111111","name":"Sales"}],"meta":{"total":2}}`, `{"data":[],"meta":{"total":2}}`},
		{"next page signal", `{"data":[{"id":"11111111-1111-4111-8111-111111111111","name":"Sales"}],"meta":{"next_page":2}}`, ""},
		{"truncation signal", `{"data":[{"id":"11111111-1111-4111-8111-111111111111","name":"Sales"}],"meta":{"has_more":true}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			calls := 0
			_, _, err := runNovel(t, []string{"reports", "get-funnel", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/pipelines" {
					t.Fatalf("unexpected request %s", r.URL.Path)
				}
				calls++
				if calls == 1 {
					_, _ = w.Write([]byte(tc.body))
					return
				}
				if tc.secondBody != "" {
					_, _ = w.Write([]byte(tc.secondBody))
					return
				}
				http.Error(w, "later page unavailable", http.StatusBadGateway)
			})
			if ExitCode(err) != 5 || err == nil {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestReportsGetFunnelRefusesDriftingPipelineTotal(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	_, _, err := runNovel(t, []string{"reports", "get-funnel", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pipelines" {
			t.Fatalf("auto-selection reached %s", r.URL.Path)
		}
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"data":[{"id":"11111111-1111-4111-8111-111111111111","name":"Sales"}],"meta":{"total":2,"next_page":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[],"meta":{"total":1}}`))
	})
	if ExitCode(err) != 5 || !strings.Contains(err.Error(), "total changed from 2 to 1") || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
