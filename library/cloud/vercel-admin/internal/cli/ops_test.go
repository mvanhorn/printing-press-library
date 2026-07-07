// Copyright 2026 hiten-shah. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpsRecentDeploymentsSummarizesLiveDeployments(t *testing.T) {
	t.Setenv("VERCEL_ADMIN_TOKEN", "test-token")
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v7/deployments" {
			t.Fatalf("path = %s, want /v7/deployments", r.URL.Path)
		}
		if r.URL.Query().Get("projectId") != "prj_123" {
			t.Fatalf("projectId = %q", r.URL.Query().Get("projectId"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"deployments":[{"uid":"dpl_1","name":"web","state":"READY","target":"production","url":"web.vercel.app","createdAt":1781635000000}]}`))
	}))
	defer srv.Close()
	t.Setenv("VERCEL_ADMIN_BASE_URL", srv.URL)

	cmd := newOpsRecentDeploymentsCmd(&rootFlags{asJSON: true, noCache: true})
	cmd.SetArgs([]string{"--project-id", "prj_123", "--limit", "5"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if seenAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", seenAuth)
	}

	var got []opsDeploymentSummary
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if len(got) != 1 || got[0].ID != "dpl_1" || got[0].State != "READY" {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestOpsFailureBriefJoinsDeploymentEventsChecksAndLogs(t *testing.T) {
	t.Setenv("VERCEL_ADMIN_TOKEN", "test-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v13/deployments/dpl_1":
			w.Write([]byte(`{"uid":"dpl_1","name":"web","state":"ERROR","target":"production","url":"web.vercel.app"}`))
		case "/v3/deployments/dpl_1/events":
			w.Write([]byte(`{"events":[{"type":"build-error","createdAt":1781635000000,"text":"Build failed"}]}`))
		case "/v2/deployments/dpl_1/check-runs":
			w.Write([]byte(`{"checks":[{"name":"lint","status":"completed","conclusion":"failure"}]}`))
		case "/v1/projects/prj_123/deployments/dpl_1/runtime-logs":
			w.Write([]byte(`[{"level":"error","message":"Cannot find module"}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("VERCEL_ADMIN_BASE_URL", srv.URL)

	cmd := newOpsFailureBriefCmd(&rootFlags{asJSON: true, noCache: true})
	cmd.SetArgs([]string{"dpl_1", "--project-id", "prj_123"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var got opsFailureBrief
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if got.Deployment.ID != "dpl_1" || got.Deployment.State != "ERROR" {
		t.Fatalf("deployment summary = %+v", got.Deployment)
	}
	if len(got.Events) != 1 || len(got.Checks) != 1 || len(got.Logs) != 1 {
		t.Fatalf("brief did not include joined signals: %+v", got)
	}
}

func TestOpsFailureBriefKeepsPartialBriefWhenRuntimeLogsAreMissing(t *testing.T) {
	t.Setenv("VERCEL_ADMIN_TOKEN", "test-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v13/deployments/dpl_static":
			w.Write([]byte(`{"uid":"dpl_static","name":"web","state":"ERROR"}`))
		case "/v3/deployments/dpl_static/events":
			w.Write([]byte(`{"events":[{"type":"build-error"}]}`))
		case "/v2/deployments/dpl_static/check-runs":
			w.Write([]byte(`{"checks":[{"name":"lint","conclusion":"failure"}]}`))
		case "/v1/projects/prj_123/deployments/dpl_static/runtime-logs":
			http.NotFound(w, r)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("VERCEL_ADMIN_BASE_URL", srv.URL)

	cmd := newOpsFailureBriefCmd(&rootFlags{asJSON: true, noCache: true})
	cmd.SetArgs([]string{"dpl_static", "--project-id", "prj_123"})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var got opsFailureBrief
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if got.Deployment.ID != "dpl_static" || len(got.Events) != 1 || len(got.Checks) != 1 {
		t.Fatalf("partial brief lost useful data: %+v", got)
	}
	if len(got.Logs) != 0 || len(got.Warnings) != 1 {
		t.Fatalf("runtime-log 404 should be omitted with a warning: %+v", got)
	}
}

func TestObjectListFromEnvelopeFallbackIsDeterministic(t *testing.T) {
	raw := json.RawMessage(`{"z":[{"id":"z"}],"a":[{"id":"a"}]}`)

	for i := 0; i < 20; i++ {
		got := objectListFromEnvelope(raw, "missing")
		if len(got) != 1 || got[0]["id"] != "a" {
			t.Fatalf("fallback returned %+v, want first sorted array key", got)
		}
	}
}
