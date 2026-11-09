package cli

// PATCH(dialer-initiate-call-contract-pin-930): contract pin for POST /dialer/call (operationId initiateCall, board #930).
// The body keys the generated command sends must be exactly the request schema's properties, and `to` must stay required in the
// command's behaviour. The expected list is HARD-CODED below and also compared to spec.yaml (the print input) and
// upstream/openapi.yaml (the recorded live snapshot), so a property renamed in code, manifest and snapshot together still fails.
// The MCP runtime path is pinned in internal/mcp/dialer_initiate_call_contract_test.go; tools-manifest.json is checked here only
// as a secondary mirror (the MCP binary does not load it).
// Not covered by design: initiateCall declares no dry_run query parameter, so there is no server-side preview to pin; the global
// --dry-run is covered by the subtest below.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var initiateCallProps = []string{"autoDialer", "callerIdStrategy", "contactId", "dncOverride", "to", "vendorBridgeNonce", "vendorBridgeable"}

func initiateCallSchema(t *testing.T, file string) (props []string, required []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", file))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `yaml:"operationId"`
			RequestBody struct {
				Content map[string]struct {
					Schema struct {
						Required   []string       `yaml:"required"`
						Properties map[string]any `yaml:"properties"`
					} `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	op, ok := doc.Paths["/dialer/call"]["post"]
	if !ok || op.OperationID != "initiateCall" {
		t.Fatalf(file+" has no initiateCall at POST /dialer/call (got %+v)", op.OperationID)
	}
	s := op.RequestBody.Content["application/json"].Schema
	for k := range s.Properties {
		props = append(props, k)
	}
	sort.Strings(props)
	return props, s.Required
}

func TestInitiateCallSchemaSnapshotsMatchPinnedList(t *testing.T) {
	for _, f := range []string{"spec.yaml", "upstream/openapi.yaml"} {
		props, required := initiateCallSchema(t, f)
		if !reflect.DeepEqual(props, initiateCallProps) || !reflect.DeepEqual(required, []string{"to"}) {
			t.Errorf("%s initiateCall properties %v required %v, pinned %v / [to]; if the API changed on purpose, reprint and update this pin", f, props, required, initiateCallProps)
		}
	}
}

func initiateCallServer(t *testing.T, hits *int, got *map[string]any) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		if r.URL.RawQuery != "" {
			t.Errorf("unexpected query string %q", r.URL.RawQuery)
		}
		if r.Method != "POST" || r.URL.Path != "/api/v1/dialer/call" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		*got = map[string]any{}
		if err := json.Unmarshal(raw, got); err != nil {
			t.Errorf("body is not JSON: %q", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
}

func TestInitiateCallRequiresTo(t *testing.T) {
	var hits int
	var got map[string]any
	initiateCallServer(t, &hits, &got)
	cmd := newDialerInitiateCallCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--contact-id", "00000000-0000-0000-0000-000000000000"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `required flag "to" not set`) {
		t.Fatalf("err = %v, want required flag \"to\" not set", err)
	}
	if hits != 0 {
		t.Errorf("%d requests sent without --to", hits)
	}
}

func TestInitiateCallGlobalDryRunSendsNothing(t *testing.T) {
	var hits int
	var got map[string]any
	initiateCallServer(t, &hits, &got)
	cmd := newDialerInitiateCallCmd(&rootFlags{noCache: true, dryRun: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--to", "+15555550100"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Errorf("%d requests sent under --dry-run", hits)
	}
}

func TestInitiateCallBodyKeysMatchUpstreamSchema(t *testing.T) {
	props, _ := initiateCallSchema(t, "upstream/openapi.yaml")
	if !reflect.DeepEqual(props, initiateCallProps) {
		t.Fatalf("upstream properties %v != pinned %v", props, initiateCallProps)
	}

	var hits int
	var got map[string]any
	initiateCallServer(t, &hits, &got)

	// Every body flag set, so every key the command can emit appears in the body.
	cmd := newDialerInitiateCallCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"--to", "+15555550100",
		"--contact-id", "11111111-1111-4111-8111-111111111111",
		"--dnc-override", "--auto-dialer=false",
		"--caller-id-strategy", "iphone",
		"--vendor-bridge-nonce", "nonce-sentinel",
		"--vendor-bridgeable",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if hits != 1 || !reflect.DeepEqual(keys, initiateCallProps) {
		t.Errorf("hits %d, command body keys %v != pinned initiateCall properties %v", hits, keys, initiateCallProps)
	}
	// Each key carries ITS value (a reprint that swaps contactId into `to` fails here).
	want := map[string]any{"to": "+15555550100", "contactId": "11111111-1111-4111-8111-111111111111", "dncOverride": true, "autoDialer": false,
		"callerIdStrategy": "iphone", "vendorBridgeNonce": "nonce-sentinel", "vendorBridgeable": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

// Secondary mirror only: tools-manifest.json is documentation the MCP binary does not load (the runtime path is pinned in
// internal/mcp). It must still list the same body params.
func TestInitiateCallToolsManifestMatchesUpstreamSchema(t *testing.T) {
	props, required := initiateCallSchema(t, "upstream/openapi.yaml")
	raw, err := readRepoFile("tools-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tools []struct {
			Name   string `json:"name"`
			Method string `json:"method"`
			Path   string `json:"path"`
			Params []struct {
				Name     string `json:"name"`
				Location string `json:"location"`
				Required bool   `json:"required"`
			} `json:"params"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tl := range m.Tools {
		if tl.Name != "dialer_initiate-call" {
			continue
		}
		found = true
		if tl.Method != "POST" || tl.Path != "/dialer/call" {
			t.Errorf("manifest route = %s %s", tl.Method, tl.Path)
		}
		var keys, req []string
		for _, p := range tl.Params {
			if p.Location != "body" {
				t.Errorf("param %s location %q, want body", p.Name, p.Location)
			}
			keys = append(keys, p.Name)
			if p.Required {
				req = append(req, p.Name)
			}
		}
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, props) {
			t.Errorf("tools-manifest dialer_initiate-call params %v != upstream properties %v", keys, props)
		}
		if !reflect.DeepEqual(req, required) {
			t.Errorf("tools-manifest required %v != upstream required %v", req, required)
		}
	}
	if !found {
		t.Fatal("tools-manifest.json has no dialer_initiate-call")
	}
}

// The pin must survive a reprint: its standalone per-patch record (the source of truth the library normalizes to) must stay
// present. The legacy .printing-press-patches.json index is deliberately NOT asserted: the library deletes it after merge.
func TestInitiateCallPinHasPatchRecord(t *testing.T) {
	if _, err := readRepoFile(".printing-press-patches/dialer-initiate-call-contract-pin-930.json"); err != nil {
		t.Errorf("patch record missing: %v", err)
	}
	for _, f := range []string{"internal/cli/dialer_initiate_call_contract_test.go", "internal/mcp/dialer_initiate_call_contract_test.go"} {
		src, err := readRepoFile(f)
		if err != nil || !strings.Contains(src, "PATCH(dialer-initiate-call-contract-pin-930)") {
			t.Errorf("%s lacks the PATCH marker (err=%v)", f, err)
		}
	}
}
