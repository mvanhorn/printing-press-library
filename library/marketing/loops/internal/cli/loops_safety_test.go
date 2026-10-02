package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func runLoopsCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("LOOPS_NO_LEARN", "true")
	cmd := RootCmd()
	var output, errors bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&errors)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func TestLoopsMutationPreviewIsJSONAndRedacted(t *testing.T) {
	out, err := runLoopsCommand(t, "events", "--body-json", `{"eventName":"synthetic","email":"person@example.test"}`, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "person@example.test") {
		t.Fatalf("preview exposed recipient: %s", out)
	}
	var result struct {
		Executed bool   `json:"executed"`
		DryRun   bool   `json:"dry_run"`
		Action   string `json:"action"`
		Preview  struct {
			Effect   string   `json:"effect"`
			Requires string   `json:"requires"`
			Flags    []string `json:"providedFlags"`
		} `json:"preview"`
	}
	if json.Unmarshal([]byte(out), &result) != nil || result.Executed || !result.DryRun || result.Action == "" || result.Preview.Effect != "sends_event_may_send_email" || result.Preview.Requires != "--confirm-send" {
		t.Fatalf("unexpected preview: %s", out)
	}
}

func TestLoopsNoLearnCommandsReturnJSON(t *testing.T) {
	for _, args := range [][]string{
		{"teach", "--query", "synthetic", "--resource", "GROUP-synthetic", "--resource-type", "items", "--json"},
		{"teach-playbook", "--query", "synthetic", "--notes", "synthetic", "--json"},
		{"playbook", "amend", "--query", "synthetic", "--add-note", "synthetic", "--json"},
	} {
		out, err := runLoopsCommand(t, args...)
		var result map[string]any
		if err != nil || json.Unmarshal([]byte(out), &result) != nil || result["skipped"] != "no-learn" {
			t.Fatalf("command %v returned %q (%v)", args, out, err)
		}
	}
}

func TestLoopsMutationConfirmationBeforeNetwork(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(200) }))
	defer server.Close()
	t.Setenv("LOOPS_BASE_URL", server.URL)
	t.Setenv("LOOPS_API_KEY", "synthetic")
	args := []string{"events", "--body-json", `{"eventName":"synthetic","email":"person@example.test"}`, "--execute", "--team", "Synthetic Team", "--json"}
	if _, err := runLoopsCommand(t, args...); err == nil || !strings.Contains(err.Error(), "--confirm-send") {
		t.Fatalf("missing confirmation error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("network calls = %d, want zero", calls)
	}
	args = append(args, "--confirm-send")
	if _, err := runLoopsCommand(t, args...); err == nil || !strings.Contains(err.Error(), "--idempotency-key") {
		t.Fatalf("missing key error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("network calls = %d, want zero", calls)
	}
}

func TestLoopsAllGeneratedMutationsHavePreview(t *testing.T) {
	root := RootCmd()
	count := 0
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			walk(child)
		}
		method := cmd.Annotations["pp:method"]
		if method != "" && method != "GET" {
			count++
			if !strings.Contains(cmd.Long, "previews by default") {
				t.Errorf("unwrapped mutation: %s", cmd.CommandPath())
			}
		}
	}
	walk(root)
	if count != 36 {
		t.Fatalf("mutation count = %d, want 36", count)
	}
}

func TestLoopsAgentContactLookupRedactsIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/contacts/find" || r.Header.Get("Authorization") != "Bearer synthetic" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"email":"person@example.test","firstName":"Synthetic","userId":"synthetic-user","subscribed":true,"optInStatus":"optedIn"}]`))
	}))
	defer server.Close()
	t.Setenv("LOOPS_BASE_URL", server.URL)
	t.Setenv("LOOPS_API_KEY", "synthetic")
	out, err := runLoopsCommand(t, "contacts", "find", "--email", "person@example.test", "--agent", "--data-source", "live", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "person@example.test") || strings.Contains(out, "Synthetic") || strings.Contains(out, "synthetic-user") {
		t.Fatalf("agent lookup exposed identity: %s", out)
	}
	if !strings.Contains(out, "optedIn") {
		t.Fatalf("agent lookup lost status: %s", out)
	}
}

func TestLoopsLifecycleAuditPaginatesWithoutContactData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api-key":
			_, _ = w.Write([]byte(`{"success":true,"teamName":"Synthetic Team"}`))
		case "/v1/lists":
			_, _ = w.Write([]byte(`[{"id":"synthetic-list"}]`))
		case "/v1/audience-segments":
			if r.URL.Query().Get("cursor") == "next" {
				_, _ = w.Write([]byte(`{"data":[{"id":"segment-2"}],"pagination":{"nextCursor":null}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":[{"id":"segment-1"}],"pagination":{"nextCursor":"next"}}`))
			}
		case "/v1/campaigns":
			_, _ = w.Write([]byte(`{"data":[{"status":"Draft"}],"pagination":{"nextCursor":null}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[],"pagination":{"nextCursor":null}}`))
		}
	}))
	defer server.Close()
	t.Setenv("LOOPS_BASE_URL", server.URL)
	t.Setenv("LOOPS_API_KEY", "synthetic")
	out, err := runLoopsCommand(t, "audit", "lifecycle", "--team", "Synthetic Team", "--agent", "--rate-limit", "20")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Counts map[string]int `json:"counts"`
	}
	if json.Unmarshal([]byte(out), &result) != nil || result.Counts["segments"] != 2 || result.Counts["campaignDrafts"] != 1 || result.Counts["lists"] != 1 {
		t.Fatalf("unexpected audit: %s", out)
	}
	if strings.Contains(out, "synthetic-list") || strings.Contains(out, "segment-1") {
		t.Fatalf("audit exposed account details: %s", out)
	}
}

func TestLoopsPreflightReturnsCountsAndNoIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api-key":
			_, _ = w.Write([]byte(`{"success":true,"teamName":"Synthetic Team"}`))
		case "/v1/campaigns/synthetic-campaign":
			_, _ = w.Write([]byte(`{"status":"Draft","mailingListId":"synthetic-list","emailMessageId":"synthetic-message"}`))
		case "/v1/email-messages/synthetic-message/guardian":
			_, _ = w.Write([]byte(`{"errors":[{"title":"Synthetic error"}],"warnings":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("LOOPS_BASE_URL", server.URL)
	t.Setenv("LOOPS_API_KEY", "synthetic")
	out, err := runLoopsCommand(t, "campaigns", "preflight", "--id", "synthetic-campaign", "--team", "Synthetic Team", "--agent", "--rate-limit", "20")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"guardianErrors":1`) || strings.Contains(out, "synthetic-campaign") || strings.Contains(out, "Synthetic error") {
		t.Fatalf("preflight output is unsafe or incomplete: %s", out)
	}
}

func TestLoopsReadChecksUseExpectedTeamAndNewestDraft(t *testing.T) {
	var selected string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api-key":
			_, _ = w.Write([]byte(`{"success":true,"teamName":"Synthetic Team"}`))
		case "/v1/campaigns":
			_, _ = w.Write([]byte(`{"data":[{"id":"older","status":"Draft","createdAt":"2026-01-01T00:00:00Z"},{"id":"newer","status":"Draft","createdAt":"2026-02-01T00:00:00Z"}],"pagination":{"nextCursor":null}}`))
		case "/v1/campaigns/newer":
			selected = "newer"
			_, _ = w.Write([]byte(`{"status":"Draft","mailingListId":"synthetic-list","emailMessageId":"synthetic-message"}`))
		case "/v1/email-messages/synthetic-message/guardian":
			_, _ = w.Write([]byte(`{"errors":[],"warnings":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("LOOPS_BASE_URL", server.URL)
	t.Setenv("LOOPS_API_KEY", "synthetic")
	t.Setenv("LOOPS_EXPECTED_TEAM", "Synthetic Team")
	out, err := runLoopsCommand(t, "campaigns", "preflight", "--latest-draft", "--agent", "--rate-limit", "20")
	if err != nil || selected != "newer" || !strings.Contains(out, `"apiChecksPassed":true`) || strings.Contains(out, "newer") {
		t.Fatalf("latest draft preflight: %s (%v); selected=%s", out, err, selected)
	}
	if _, err := runLoopsCommand(t, "team", "verify", "--agent", "--rate-limit", "20"); err != nil {
		t.Fatalf("team environment was not accepted: %v", err)
	}
	if _, err := runLoopsCommand(t, "campaigns", "preflight", "--id", "synthetic", "--latest-draft", "--agent"); err == nil {
		t.Fatal("ambiguous campaign selection should fail")
	}
}

func TestLoopsBulkImportPreviewsAndRejectsUnkeyedSends(t *testing.T) {
	out, err := runLoopsCommand(t, "import", "contacts", "--input", "-", "--agent")
	if err != nil || !strings.Contains(out, `"executed":false`) || !strings.Contains(out, `"effect":"changes_data"`) {
		t.Fatalf("import preview: %s (%v)", out, err)
	}
	if _, err := runLoopsCommand(t, "import", "events", "--input", "-", "--agent"); err == nil || !strings.Contains(err.Error(), "bulk sends are unavailable") {
		t.Fatalf("unkeyed bulk send was allowed: %v", err)
	}
}
