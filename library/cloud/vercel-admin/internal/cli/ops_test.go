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
