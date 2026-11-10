package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Automation preview subjects (app main 59ccd6a7b): POST /automations/{id}/dry-run and /test-node take dealId, companyId and
// executionId besides contactId. The public OpenAPI at that commit does not list them; the API catalog's requestBody fields are the
// contract (spec.yaml carries them with the catalog wording). Not-given fields must not be sent.
const (
	previewAuto    = "6f1c0000-0000-4000-8000-0000000000a0"
	previewContact = "6f1c0000-0000-4000-8000-0000000000aa"
	previewDeal    = "6f1c0000-0000-4000-8000-0000000000bb"
	previewCompany = "6f1c0000-0000-4000-8000-0000000000cc"
	previewExec    = "6f1c0000-0000-4000-8000-0000000000dd"
)

func runPreview(t *testing.T, newCmd func(*rootFlags) *cobra.Command, wantPath string, args ...string) (body map[string]any, query string, hits int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != "POST" || r.URL.Path != "/api/v1"+wantPath {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		query = r.URL.RawQuery
		raw, _ := io.ReadAll(r.Body)
		body = map[string]any{}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body is not JSON: %q", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := newCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return
}

func TestDryRunAutomationPreviewSubjectFlags(t *testing.T) {
	path := "/automations/" + previewAuto + "/dry-run"
	for _, c := range []struct {
		flag, key, val string
	}{
		{"--deal-id", "dealId", previewDeal},
		{"--company-id", "companyId", previewCompany},
		{"--execution-id", "executionId", previewExec},
	} {
		body, _, hits := runPreview(t, newAutomationsDryRunAutomationCmd, path, previewAuto, c.flag, c.val)
		if hits != 1 || len(body) != 1 || body[c.key] != c.val {
			t.Errorf("%s: hits=%d body=%v, want only %s=%s", c.flag, hits, body, c.key, c.val)
		}
	}
}

func TestDryRunAutomationAllSubjectsAndQueryRideTogether(t *testing.T) {
	path := "/automations/" + previewAuto + "/dry-run"
	body, query, _ := runPreview(t, newAutomationsDryRunAutomationCmd, path, previewAuto,
		"--contact-id", previewContact, "--deal-id", previewDeal, "--company-id", previewCompany, "--execution-id", previewExec, "--source", "published")
	want := map[string]any{"contactId": previewContact, "dealId": previewDeal, "companyId": previewCompany, "executionId": previewExec}
	if len(body) != len(want) {
		t.Fatalf("body = %v", body)
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
	if query != "source=published" {
		t.Errorf("query = %q, want source=published", query)
	}
}

func TestDryRunAutomationOmittedSubjectsAreNotSent(t *testing.T) {
	path := "/automations/" + previewAuto + "/dry-run"
	body, query, _ := runPreview(t, newAutomationsDryRunAutomationCmd, path, previewAuto, "--contact-id", previewContact)
	if len(body) != 1 || body["contactId"] != previewContact || query != "" {
		t.Errorf("body = %v, query = %q", body, query)
	}
}

func TestTestNodePreviewSubjectFlags(t *testing.T) {
	path := "/automations/" + previewAuto + "/test-node"
	node := "6f1c0000-0000-4000-8000-0000000000ee"
	for _, c := range []struct {
		flag, key, val string
	}{
		{"--deal-id", "dealId", previewDeal},
		{"--company-id", "companyId", previewCompany},
		{"--execution-id", "executionId", previewExec},
	} {
		body, _, hits := runPreview(t, newAutomationsTestNodePostAutomationsIdCmd, path, previewAuto, "--node-id", node, c.flag, c.val)
		if hits != 1 || len(body) != 2 || body[c.key] != c.val || body["nodeId"] != node {
			t.Errorf("%s: hits=%d body=%v, want nodeId and %s only", c.flag, hits, body, c.key)
		}
	}
}

func TestTestNodeRequiresNodeIdBeforeAnyRequest(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	cmd := newAutomationsTestNodePostAutomationsIdCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{previewAuto, "--deal-id", previewDeal})
	if err := cmd.Execute(); err == nil || hits != 0 {
		t.Errorf("err = %v, hits = %d, want a refusal with no request", err, hits)
	}
}

func TestPreviewSubjectFlagHelpCarriesTheRules(t *testing.T) {
	for name, newCmd := range map[string]func(*rootFlags) *cobra.Command{
		"dry-run":   newAutomationsDryRunAutomationCmd,
		"test-node": newAutomationsTestNodePostAutomationsIdCmd,
	} {
		fl := newCmd(&rootFlags{}).Flags()
		for flag, needles := range map[string][]string{
			"deal-id":      {"REQUIRED for a DEAL occurrence"},
			"company-id":   {"REQUIRED for a COMPANY occurrence", "--execution-id does not replace it"},
			"execution-id": {"admin or owner only", "runs carrying a deal are hidden from API keys without the deals scope", "synthetic sample", "unknown/absent"},
		} {
			f := fl.Lookup(flag)
			if f == nil {
				t.Fatalf("%s: --%s missing", name, flag)
			}
			for _, n := range needles {
				if !strings.Contains(f.Usage, n) {
					t.Errorf("%s --%s help lacks %q: %s", name, flag, n, f.Usage)
				}
			}
		}
	}
}

func TestPreviewSubjectsAreInTheManifest(t *testing.T) {
	raw, err := os.ReadFile("../../tools-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tools []struct {
			Name   string `json:"name"`
			Params []struct {
				Name     string `json:"name"`
				Location string `json:"location"`
				Required bool   `json:"required"`
			} `json:"params"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	seen := map[string]map[string]bool{}
	for _, tl := range m.Tools {
		if tl.Name != "automations_dry-run_automation" && tl.Name != "automations_test-node_post-automations-id" {
			continue
		}
		seen[tl.Name] = map[string]bool{}
		for _, p := range tl.Params {
			if p.Location == "body" {
				seen[tl.Name][p.Name] = true
			}
		}
	}
	for _, tool := range []string{"automations_dry-run_automation", "automations_test-node_post-automations-id"} {
		for _, f := range []string{"dealId", "companyId", "executionId", "contactId"} {
			if !seen[tool][f] {
				t.Errorf("%s: manifest lacks body param %s", tool, f)
			}
		}
	}
	if !seen["automations_test-node_post-automations-id"]["nodeId"] {
		t.Error("test-node manifest lacks nodeId")
	}
}
