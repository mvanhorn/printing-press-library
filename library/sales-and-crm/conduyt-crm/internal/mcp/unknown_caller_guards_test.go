package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// Hand-written (parity round 3, see .printing-press-patches/unknown-caller-link-and-counter.md): the code-orchestration executor
// must apply the same unknown-caller guards as the CLI, before any request is made.
func execGuard(t *testing.T, endpoint string, params map[string]any) (hits int, body, rawQuery string, res *mcplib.CallToolResult) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		rawQuery = r.URL.RawQuery
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		body = string(b[:n])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{},"meta":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	res, err := codeOrchTestInvoke(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
		Arguments: map[string]any{"endpoint_id": endpoint, "params": params},
	}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return hits, body, rawQuery, res
}

func TestCodeOrchContactsCreateRefusesCallIdWithSourceOrOwner(t *testing.T) {
	cases := map[string]map[string]any{
		"callId+source":     {"callId": "c1", "source": "Web", "firstName": "A"},
		"callId+assignedTo": {"callId": "c1", "assignedTo": "wrong-rep", "firstName": "A"},
		"callId+both":       {"callId": "c1", "source": "Web", "assignedTo": "wrong-rep"},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			hits, _, _, res := execGuard(t, "contacts.create", params)
			if hits != 0 {
				t.Fatalf("request was sent (%d hits); the guard must refuse first", hits)
			}
			if !res.IsError {
				t.Fatalf("expected a tool error")
			}
		})
	}
}

func TestCodeOrchContactsCreateCallIdAlonePosts(t *testing.T) {
	hits, body, _, res := execGuard(t, "contacts.create", map[string]any{"callId": "c1", "firstName": "A"})
	if res.IsError || hits != 1 || !strings.Contains(body, `"callId":"c1"`) {
		t.Fatalf("hits=%d isErr=%v body=%s", hits, res.IsError, body)
	}
}

func TestCodeOrchCountUnknownCallersSinceIsStrict(t *testing.T) {
	for _, since := range []string{"2026-10-15T07:00:00", "2026-02-31T00:00:00Z", " 2026-10-15T07:00:00Z", "yesterday"} {
		hits, _, _, res := execGuard(t, "calls.count-unknown-callers", map[string]any{"line": "l1", "since": since})
		if hits != 0 || !res.IsError {
			t.Fatalf("since %q: hits=%d isErr=%v, want refused with no request", since, hits, res.IsError)
		}
	}
	hits, _, q, res := execGuard(t, "calls.count-unknown-callers", map[string]any{"line": "l1", "since": "2026-10-15T07:00:00Z"})
	if res.IsError || hits != 1 || !strings.Contains(q, "since=2026-10-15T07%3A00%3A00Z") {
		t.Fatalf("valid since: hits=%d isErr=%v query=%s", hits, res.IsError, q)
	}
}

// Identity rules v2 (board #69): the executor applies the same duplicate-proposal checks as the CLI, before any request.
const (
	dpContactID  = "3f8a0c4e-1111-4222-8333-444455556666"
	dpProposalID = "9b2d7c1a-5e4f-4a3b-9c8d-7e6f5a4b3c2d"
)

func TestCodeOrchDuplicateProposalDecisionIsStrict(t *testing.T) {
	const post = "contacts.duplicate-proposals.decide-contact"
	bad := map[string]map[string]any{
		"no decision":     {"id": dpContactID, "proposalId": dpProposalID},
		"merged":          {"id": dpContactID, "proposalId": dpProposalID, "decision": "merged"},
		"not a string":    {"id": dpContactID, "proposalId": dpProposalID, "decision": true},
		"extra key":       {"id": dpContactID, "proposalId": dpProposalID, "decision": "different", "dryRun": true},
		"bad contact id":  {"id": "../x", "proposalId": dpProposalID, "decision": "different"},
		"bad proposal id": {"id": dpContactID, "proposalId": "nope", "decision": "different"},
		"missing id":      {"proposalId": dpProposalID, "decision": "different"},
	}
	for name, params := range bad {
		t.Run(name, func(t *testing.T) {
			hits, _, _, res := execGuard(t, post, params)
			if hits != 0 || !res.IsError {
				t.Fatalf("hits=%d isError=%v; the guard must refuse before any request", hits, res.IsError)
			}
		})
	}
}

func TestCodeOrchDuplicateProposalsValidCallsAreSent(t *testing.T) {
	for _, d := range []string{"not_sure", "different", "merge"} {
		hits, body, _, res := execGuard(t, "contacts.duplicate-proposals.decide-contact", map[string]any{"id": dpContactID, "proposalId": dpProposalID, "decision": d})
		if hits != 1 || res.IsError || !strings.Contains(body, `"decision":"`+d+`"`) {
			t.Fatalf("%s: hits=%d isError=%v body=%s", d, hits, res.IsError, body)
		}
	}
	hits, _, _, res := execGuard(t, "contacts.duplicate-proposals.list-contact", map[string]any{"id": dpContactID})
	if hits != 1 || res.IsError {
		t.Fatalf("list: hits=%d isError=%v", hits, res.IsError)
	}
	hits, _, _, res = execGuard(t, "contacts.duplicate-proposals.list-contact", map[string]any{"id": "nope"})
	if hits != 0 || !res.IsError {
		t.Fatalf("list with a bad id: hits=%d isError=%v", hits, res.IsError)
	}
}
