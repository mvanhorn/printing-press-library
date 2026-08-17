// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/store"
)

func TestManifestCommandSurfaceResolves(t *testing.T) {
	paths := []string{
		"graphql request", "graphql parse", "graphql atlas", "regions endpoint", "org current", "apps list", "spaces list",
		"app-instances list", "app-instances create", "app-installs list", "app-versions list",
		"tokens management create", "tokens viewer create", "app-runtime inspect", "app-runtime validate",
		"playgrounds templates list", "playgrounds files get", "playgrounds files put", "playgrounds data get", "playgrounds data put",
		"playgrounds preview", "playgrounds viewer get", "sync", "search", "doctor", "auth inspect", "auth capabilities",
		"playgrounds impact", "playgrounds readiness", "playgrounds config-drift", "playgrounds create-reconcile",
		"playgrounds contract-check", "playgrounds preview-drift",
	}
	root := RootCmd()
	for _, path := range paths {
		command, remainder, err := root.Find(strings.Fields(path))
		if err != nil || command == root || len(remainder) != 0 {
			t.Errorf("command %q did not resolve: command=%q remainder=%v err=%v", path, command.CommandPath(), remainder, err)
		}
	}
}

func TestMutationDryRunsDoNotReadPayloads(t *testing.T) {
	cases := [][]string{
		{"app-instances", "create", "--input", "/definitely/missing/input.json", "--dry-run", "--json", "--no-learn"},
		{"playgrounds", "files", "put", "app-1", "--space-id", "space-1", "--dir", "/definitely/missing/dir", "--expected-last-modified", "123", "--dry-run", "--json", "--no-learn"},
		{"playgrounds", "data", "put", "app-1", "--space-id", "space-1", "--input", "/definitely/missing/data.json", "--dry-run", "--json", "--no-learn"},
	}
	for _, args := range cases {
		stdout, stderr, err := runRootArgs(t, args...)
		if err != nil {
			t.Fatalf("dry-run %v failed: %v stderr=%s", args, err, stderr)
		}
		if !strings.Contains(stdout, `"sent": false`) {
			t.Errorf("dry-run %v did not emit an unsent plan: %s", args, stdout)
		}
	}
}

func TestLocalPlaygroundsAnalysisFeatures(t *testing.T) {
	home := t.TempDir()
	dataDir := filepath.Join(home, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	seed := func(resourceType, id string, value map[string]any) {
		raw, _ := json.Marshal(value)
		if err := s.Upsert(resourceType, id, raw); err != nil {
			t.Fatal(err)
		}
	}
	seed("app_instances", "instance-a", map[string]any{"id": "instance-a", "appId": "studio-app", "appUuid": "app-1", "spaceId": "space-1", "appInstallId": "install-1", "status": "ACTIVE", "config": map[string]any{"theme": "dark"}})
	seed("app_instances", "instance-b", map[string]any{"id": "instance-b", "appId": "studio-app", "appUuid": "app-1", "spaceId": "space-2", "appInstallId": "install-2", "status": "INACTIVE", "config": map[string]any{"theme": map[string]any{"name": "light"}}})
	seed("app_installs", "install-1", map[string]any{"id": "install-1", "appId": "studio-app", "spaceId": "space-1"})
	seed("app_installs", "install-2", map[string]any{"id": "install-2", "appId": "studio-app", "spaceId": "space-2"})
	seed("spaces", "space-1", map[string]any{"id": "space-1"})
	seed("spaces", "space-2", map[string]any{"id": "space-2"})
	seed("channels", "channel-a", map[string]any{"id": "channel-a", "name": "Lobby", "content": map[string]any{"appUuid": "app-1"}})
	seed("app_versions", "version-a", map[string]any{"id": "version-a", "appId": "", "version": "1", "isLatest": true})
	seed("playgrounds_metadata", "app-1", map[string]any{"id": "app-1", "app_uuid": "app-1", "refreshed_at": time.Now().UTC().Format(time.RFC3339), "production_available": true, "production_last_modified": time.Now().Add(-48 * time.Hour).UnixMilli(), "preview_last_modified": time.Now().Add(-10 * 24 * time.Hour).UnixMilli()})
	if err := s.SaveSyncState("channels", "complete", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_versions", "complete", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_instances", "complete", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_installs", "complete", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("spaces", "complete", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	working := filepath.Join(home, "working")
	if err := os.MkdirAll(working, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(working, "index.html"), []byte("<main>reviewed</main>"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runRootArgs(t, "--home", home, "--no-learn", "--json", "playgrounds", "impact", "app-1", "--dir", working)
	if err != nil || !strings.Contains(stdout, "channel-a") {
		t.Fatalf("impact failed: err=%v stdout=%s", err, stdout)
	}
	stdout, _, err = runRootArgs(t, "--home", home, "--no-learn", "--json", "playgrounds", "readiness", "--app-uuid", "app-1")
	if err != nil || !strings.Contains(stdout, "inactive") {
		t.Fatalf("readiness failed: err=%v stdout=%s", err, stdout)
	}
	stdout, _, err = runRootArgs(t, "--home", home, "--no-learn", "--json", "playgrounds", "config-drift", "--app-uuid", "app-1")
	if err != nil || !strings.Contains(stdout, `"drift": true`) {
		t.Fatalf("config-drift failed: err=%v stdout=%s", err, stdout)
	}
	stdout, _, err = runRootArgs(t, "--home", home, "--no-learn", "--json", "playgrounds", "preview-drift", "--older-than", "7d")
	if err != nil || !strings.Contains(stdout, "production_ahead") {
		t.Fatalf("preview-drift failed: err=%v stdout=%s", err, stdout)
	}
}

func TestCreateReconcileBuildsIdempotentResumePlan(t *testing.T) {
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"stage":"studio_instance_created","app_uuid":"app-1","space_id":"space-1","secret":"must-not-echo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "--no-learn", "--json", "playgrounds", "create-reconcile", "--receipt", receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "upload_files") || !strings.Contains(stdout, "upload_data") {
		t.Fatalf("missing resume actions: %s", stdout)
	}
	if strings.Contains(stdout, "must-not-echo") {
		t.Fatalf("receipt secret leaked: %s", stdout)
	}
}

func TestUnsynchronizedMirrorNeverClaimsConclusiveHealth(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	checks := [][]string{
		{"playgrounds", "readiness"},
		{"playgrounds", "config-drift", "--app-uuid", "app-1"},
		{"playgrounds", "preview-drift"},
	}
	for _, args := range checks {
		full := append([]string{"--home", home, "--no-learn", "--json"}, args...)
		stdout, _, err := runRootArgs(t, full...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(stdout, `"complete": false`) || !strings.Contains(strings.ToLower(stdout), "hint") {
			t.Fatalf("%v claimed conclusive health without sync evidence: %s", args, stdout)
		}
	}
}

func TestLiveReadOnlyContractAndCapabilityFeaturesAgainstMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/graphql":
			var body struct {
				Query string `json:"query"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			switch {
			case strings.Contains(body.Query, "CLICurrentToken"):
				fmt.Fprint(w, `{"data":{"currentToken":{"permissions":["app_instance:update","token:create"]}},"meta":{"graphqlQueryCost":1}}`)
			case strings.Contains(body.Query, "CLICurrentUser"):
				fmt.Fprint(w, `{"data":{"currentUser":{"permissions":[],"status":"ACTIVE"}},"meta":{"graphqlQueryCost":1}}`)
			case strings.Contains(body.Query, "CLIPermissionsCatalog"):
				fmt.Fprint(w, `{"data":{"permissionsList":{"app_instance":["read","update"],"token":["create"]}},"meta":{"graphqlQueryCost":1}}`)
			case strings.Contains(body.Query, "MintManagementToken"):
				fmt.Fprint(w, `{"data":{"createSignedAppManagementJwt":{"signedAppManagementToken":"management-jwt","tokenType":"Bearer","expiresAt":999999}}}`)
			case strings.Contains(body.Query, "MintViewerToken"):
				fmt.Fprint(w, `{"data":{"createSignedAppViewerJwt":{"signedAppViewerToken":"viewer-jwt","tokenType":"Bearer","expiresAt":999999}}}`)
			default:
				http.Error(w, `{"errors":[{"message":"unexpected query"}]}`, http.StatusBadRequest)
			}
		case r.URL.Path == "/files/app-1":
			if r.Header.Get("Authorization") == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"files":{"html":"x","css":"y","js":"z"},"lastModified":"1","location":"mock"}`)
		case r.URL.Path == "/data/app-1":
			if r.Header.Get("Authorization") == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"data":{},"lastModified":"1","location":"mock"}`)
		case r.URL.Path == "/apps/app-1":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><html><body>ok</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_PLAYGROUNDS_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "organization-api-key")

	stdout, stderr, err := runRootArgs(t, "--no-learn", "--json", "--yes", "playgrounds", "contract-check", "--app-uuid", "app-1", "--space-id", "space-1")
	if err != nil {
		t.Fatalf("contract-check failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, `"passed": true`) || strings.Contains(stdout, "management-jwt") {
		t.Fatalf("unsafe or failed contract output: %s", stdout)
	}

	stdout, stderr, err = runRootArgs(t, "--no-learn", "--json", "auth", "capabilities", "--for", "playgrounds files put")
	if err != nil {
		t.Fatalf("capabilities failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "appears_available") || strings.Contains(stdout, "app_instance:update") && strings.Contains(stdout, "raw_grants") == false {
		t.Fatalf("unexpected capability output: %s", stdout)
	}
	if strings.Contains(stdout, "organization-api-key") {
		t.Fatalf("credential leaked: %s", stdout)
	}
}

func TestAgentModeNeverApprovesMutations(t *testing.T) {
	_, _, err := runRootArgs(t, "--no-learn", "--agent", "graphql", "--query", "mutation { unsafe }")
	if err == nil || !strings.Contains(err.Error(), "without --yes") {
		t.Fatalf("--agent unexpectedly approved GraphQL mutation: %v", err)
	}
	_, _, err = runRootArgs(t, "--no-learn", "--agent", "playgrounds", "contract-check", "--app-uuid", "app-1", "--space-id", "space-1")
	if err == nil || !strings.Contains(err.Error(), "rerun with --yes") {
		t.Fatalf("--agent unexpectedly approved scoped JWT mint: %v", err)
	}
}

func TestCapabilitiesFailClosedOnPartialVisibility(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case strings.Contains(body.Query, "CLICurrentToken"):
			fmt.Fprint(w, `{"errors":[{"message":"forbidden"}]}`)
		case strings.Contains(body.Query, "CLICurrentUser"):
			fmt.Fprint(w, `{"data":{"currentUser":{"permissions":["app_instance:update","token:create"]}}}`)
		default:
			fmt.Fprint(w, `{"data":{"permissionsList":{"app_instance":["update"],"token":["create"]}}}`)
		}
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, stderr, err := runRootArgs(t, "--no-learn", "--json", "auth", "capabilities", "--for", "playgrounds files put")
	if err != nil {
		t.Fatalf("partial capability diagnostic failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"decision": "unknown"`) || !strings.Contains(stdout, `"partial_visibility": true`) {
		t.Fatalf("partial visibility did not fail closed: %s", stdout)
	}
}

func TestOrganizationExpectationMismatchFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"currentOrgId":"actual-org"}}`)
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, _, err := runRootArgs(t, "--no-learn", "--json", "org", "current", "--expected-org-id", "expected-org")
	if err == nil || !strings.Contains(err.Error(), "different organization") {
		t.Fatalf("organization mismatch did not fail closed: err=%v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, `"organization_match": false`) {
		t.Fatalf("organization mismatch result missing: %s", stdout)
	}
}

func TestSyncSanitizerPreservesTopologyButDropsPrivateValues(t *testing.T) {
	nodes := []json.RawMessage{json.RawMessage(`{"id":"instance-1","config":{"appUuid":"app-1","secret":"must-not-store","nested":{"token":"also-secret"}}}`)}
	result, err := sanitizeSyncNodes("app_instances", nodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected one sanitized node")
	}
	text := string(result[0])
	if !strings.Contains(text, `"playgroundsAppUuid":"app-1"`) || strings.Contains(text, "must-not-store") || strings.Contains(text, "also-secret") {
		t.Fatalf("unsafe or incomplete sanitized node: %s", text)
	}
}

func TestSyncSanitizerDerivesStableCompositeEdgeIDs(t *testing.T) {
	nodes := []json.RawMessage{
		json.RawMessage(`{"channelId":"channel-1","screenId":"screen-1"}`),
		json.RawMessage(`{"channelId":"channel-1","screenId":"screen-2"}`),
	}
	first, err := sanitizeSyncNodes("associations", nodes)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sanitizeSyncNodes("associations", nodes)
	if err != nil {
		t.Fatal(err)
	}
	var firstEdge, repeatedEdge, distinctEdge map[string]any
	if err := json.Unmarshal(first[0], &firstEdge); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second[0], &repeatedEdge); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(first[1], &distinctEdge); err != nil {
		t.Fatal(err)
	}
	if firstEdge["id"] == "" || firstEdge["id"] != repeatedEdge["id"] {
		t.Fatalf("composite edge ID is missing or unstable: first=%v repeated=%v", firstEdge["id"], repeatedEdge["id"])
	}
	if firstEdge["id"] == distinctEdge["id"] {
		t.Fatalf("distinct composite edges received the same ID: %v", firstEdge["id"])
	}
}

func TestGraphQLMutationDetectionSkipsCommentsAndStrings(t *testing.T) {
	cases := []struct {
		document string
		want     bool
	}{
		{"# harmless comment\n mutation Update { updateThing }", true},
		{`query Read { field(arg: "mutation is text") }`, false},
		{"query Read { field(arg: \"\"\"mutation in a block string\"\"\") }", false},
		{"fragment F on Thing { id }\nmutation Update { updateThing }", true},
	}
	for _, tc := range cases {
		if got := graphqlDocumentHasMutation(tc.document); got != tc.want {
			t.Errorf("graphqlDocumentHasMutation(%q)=%v want %v", tc.document, got, tc.want)
		}
	}
}

func TestSyncMarksMaxPageBoundaryIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"allApps":{"totalCount":2,"nodes":[{"id":"app-1","name":"One","slug":"one","isInstalled":true}]}}}`)
	}))
	defer server.Close()
	home := t.TempDir()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, stderr, err := runRootArgs(t, "--home", home, "--no-learn", "--json", "sync", "--resources", "apps", "--first", "1", "--max-pages", "1")
	if err != nil {
		t.Fatalf("sync failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"complete": false`) || !strings.Contains(stdout, `"apps": "truncated"`) {
		t.Fatalf("truncated sync claimed completeness: %s", stdout)
	}
	s, err := store.OpenReadOnly(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cursor, _, _, err := s.GetSyncState("apps")
	if err != nil || cursor != "truncated" {
		t.Fatalf("expected truncated sync state, cursor=%q err=%v", cursor, err)
	}
}

func TestConfigDriftNoMatchIsIncomplete(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"id":"instance-1","playgroundsAppUuid":"other","config":{"theme":""}}`)
	if err := s.Upsert("app_instances", "instance-1", raw); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_instances", "complete", 1); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	stdout, _, err := runRootArgs(t, "--home", home, "--no-learn", "--json", "playgrounds", "config-drift", "--app-uuid", "missing")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"complete": false`) || !strings.Contains(stdout, "No synchronized app instance matched") {
		t.Fatalf("zero-match drift was conclusive: %s", stdout)
	}
}

func TestCreateReconcileRejectsAmbiguousFailureStages(t *testing.T) {
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"stage":"files_failed","app_uuid":"app-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := runRootArgs(t, "--no-learn", "--json", "playgrounds", "create-reconcile", "--receipt", receipt)
	if err == nil || !strings.Contains(err.Error(), "requires explicit") {
		t.Fatalf("ambiguous failure receipt was accepted: %v", err)
	}
}

func TestReadinessWithoutExpectedPlaygroundsAppIsIncomplete(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_instances", "complete", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_versions", "complete", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_installs", "complete", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("spaces", "complete", 0); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	stdout, _, err := runRootArgs(t, "--home", home, "--no-learn", "--json", "playgrounds", "readiness")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"complete": false`) || !strings.Contains(stdout, "No Playgrounds app instances") {
		t.Fatalf("empty fleet was reported conclusive: %s", stdout)
	}
}

func TestContractCheckRequiresExplicitUnauthenticatedDenial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/graphql":
			var body struct {
				Query string `json:"query"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if strings.Contains(body.Query, "MintManagementToken") {
				fmt.Fprint(w, `{"data":{"createSignedAppManagementJwt":{"signedAppManagementToken":"management-jwt"}}}`)
			} else {
				fmt.Fprint(w, `{"data":{"createSignedAppViewerJwt":{"signedAppViewerToken":"viewer-jwt"}}}`)
			}
		case r.URL.Path == "/files/app-1" && r.Header.Get("Authorization") == "":
			http.Error(w, `{"error":"temporary outage"}`, http.StatusInternalServerError)
		case r.URL.Path == "/files/app-1":
			fmt.Fprint(w, `{"files":{},"lastModified":"1"}`)
		case r.URL.Path == "/data/app-1":
			fmt.Fprint(w, `{"data":{},"lastModified":"1"}`)
		case r.URL.Path == "/apps/app-1":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><html></html>`)
		}
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_PLAYGROUNDS_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")
	stdout, _, err := runRootArgs(t, "--no-learn", "--json", "--yes", "playgrounds", "contract-check", "--app-uuid", "app-1", "--space-id", "space-1")
	if err == nil || !strings.Contains(err.Error(), "contract check failed") {
		t.Fatalf("server failure was accepted as an auth denial: err=%v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, `"unauthenticated_files_read_rejected"`) || !strings.Contains(stdout, `"unauthenticated_data_read_rejected"`) || !strings.Contains(stdout, `"passed": false`) {
		t.Fatalf("failed auth-boundary check was not reported: %s", stdout)
	}
}

func TestCapabilityMatchingIsExactAndCommandPathsAreUnambiguous(t *testing.T) {
	if capabilityMatches([]string{"app_instance_extra:update"}, "app_instance:update") {
		t.Fatal("a neighboring permission domain matched")
	}
	if capabilityMatches([]string{"app_instance:update_extra"}, "app_instance:update") {
		t.Fatal("a neighboring permission action matched")
	}
	if got := requiredCapabilities("sync everything"); got != nil {
		t.Fatalf("command prefix was treated as an exact command: %v", got)
	}
	if got := requiredCapabilities("playgrounds files put"); len(got) != 2 {
		t.Fatalf("exact command mapping missing: %v", got)
	}
}

func TestCompleteSyncPrunesRowsMissingFromRemoteTraversal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"allApps":{"totalCount":1,"nodes":[{"id":"current","name":"Current","slug":"current","isInstalled":true}]}}}`)
	}))
	defer server.Close()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert("apps", "stale", json.RawMessage(`{"id":"stale"}`)); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, stderr, err := runRootArgs(t, "--home", home, "--no-learn", "--json", "sync", "--resources", "apps", "--first", "10", "--max-pages", "1")
	if err != nil {
		t.Fatalf("sync failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"pruned"`) || !strings.Contains(stdout, `"apps": 1`) {
		t.Fatalf("prune result missing: %s", stdout)
	}
	s, err = store.OpenReadOnly(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Get("apps", "stale"); err == nil {
		t.Fatal("stale row survived a complete traversal")
	}
	if _, err := s.Get("apps", "current"); err != nil {
		t.Fatalf("current row missing after sync: %v", err)
	}
}

func TestDataPutUsesObservedDataOnlyContract(t *testing.T) {
	input := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(input, []byte(`{"message":"reviewed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/graphql" {
			fmt.Fprint(w, `{"data":{"createSignedAppManagementJwt":{"signedAppManagementToken":"management-jwt"}}}`)
			return
		}
		if r.URL.Path != "/data/app-1" || r.Method != http.MethodPut {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode PUT: %v", err)
		}
		if _, ok := body["lastModified"]; ok || len(body) != 1 || body["data"] == nil {
			t.Errorf("unexpected data PUT body: %#v", body)
		}
		fmt.Fprint(w, `{"data":{},"lastModified":"2"}`)
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_PLAYGROUNDS_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, stderr, err := runRootArgs(t, "--no-learn", "--json", "--yes", "playgrounds", "data", "put", "app-1", "--space-id", "space-1", "--input", input)
	if err != nil {
		t.Fatalf("data put failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"completion_confirmed": true`) {
		t.Fatalf("write receipt was not confirmed: %s", stdout)
	}
	if !strings.Contains(stdout, `"stage": "standalone_data_uploaded"`) || !strings.Contains(stdout, `"reconcile_compatible": false`) {
		t.Fatalf("standalone write receipt implied create-workflow completion: %s", stdout)
	}
}

func TestAppInstanceCreateReceiptFeedsReconciler(t *testing.T) {
	input := filepath.Join(t.TempDir(), "instance.json")
	if err := os.WriteFile(input, []byte(`{"spaceId":"space-1","config":{"appUuid":"app-1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"createAppInstance":{"appInstance":{"id":"instance-1"},"clientMutationId":"client-1"}}}`)
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, stderr, err := runRootArgs(t, "--no-learn", "--json", "--yes", "app-instances", "create", "--input", input)
	if err != nil {
		t.Fatalf("create failed: %v stderr=%s", err, stderr)
	}
	for _, expected := range []string{`"stage": "studio_instance_created"`, `"app_instance_id": "instance-1"`, `"app_uuid": "app-1"`, `"space_id": "space-1"`} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("receipt missing %s: %s", expected, stdout)
		}
	}
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receipt, []byte(stdout), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, _, err := runRootArgs(t, "--no-learn", "--json", "playgrounds", "create-reconcile", "--receipt", receipt)
	if err != nil || !strings.Contains(plan, "upload_files") {
		t.Fatalf("generated receipt was not reconcilable: err=%v plan=%s", err, plan)
	}
}

func TestPreviewMetadataNeedsUsableTimestamp(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert("app_instances", "instance-1", json.RawMessage(`{"id":"instance-1","playgroundsAppUuid":"app-1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert("playgrounds_metadata", "app-1", json.RawMessage(`{"id":"app-1","app_uuid":"app-1","production_last_modified":"not-a-time"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("app_instances", "complete", 1); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	stdout, _, err := runRootArgs(t, "--home", home, "--no-learn", "--json", "playgrounds", "preview-drift")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"complete": false`) || !strings.Contains(stdout, `"app-1"`) {
		t.Fatalf("malformed timestamp counted as observed metadata: %s", stdout)
	}
}

func TestSyncFailsClosedWhenConnectionEvidenceIsMissing(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(home, "data", "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert("apps", "must-survive", json.RawMessage(`{"id":"must-survive"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSyncState("apps", "complete", 1); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	stateAtRequest := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirror, openErr := store.OpenReadOnly(dbPath)
		if openErr == nil {
			cursor, _, _, stateErr := mirror.GetSyncState("apps")
			_ = mirror.Close()
			if stateErr == nil {
				stateAtRequest <- cursor
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	_, _, err = runRootArgs(t, "--home", home, "--no-learn", "--json", "sync", "--resources", "apps", "--max-pages", "1")
	if err == nil || !strings.Contains(err.Error(), "metadata sync completed") {
		t.Fatalf("missing connection was not rejected: %v", err)
	}
	select {
	case state := <-stateAtRequest:
		if state != "in_progress" {
			t.Fatalf("sync did not invalidate the old complete state before I/O: %q", state)
		}
	default:
		t.Fatal("server could not observe the sync state before responding")
	}
	s, err = store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Get("apps", "must-survive"); err != nil {
		t.Fatalf("invalid response pruned the previous mirror: %v", err)
	}
	cursor, _, _, err := s.GetSyncState("apps")
	if err != nil || cursor != "failed" {
		t.Fatalf("old complete state remained visible: cursor=%q err=%v", cursor, err)
	}
}

func TestSyncRejectsShortPageThatContradictsTotalCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"allApps":{"totalCount":2,"nodes":[{"id":"only-one","name":"One","slug":"one","isInstalled":true}]}}}`)
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, _, err := runRootArgs(t, "--home", t.TempDir(), "--no-learn", "--json", "sync", "--resources", "apps", "--first", "10", "--max-pages", "1")
	if err == nil || !strings.Contains(err.Error(), "metadata sync completed") {
		t.Fatalf("contradictory short page was accepted: err=%v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "ended early") || !strings.Contains(stdout, `"apps": "failed"`) {
		t.Fatalf("short-page failure was not explicit: %s", stdout)
	}
}

func TestSyncRejectsDuplicateIDsBeforeReconciliation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"allApps":{"totalCount":2,"nodes":[{"id":"duplicate","name":"One","slug":"one","isInstalled":true},{"id":"duplicate","name":"One again","slug":"one","isInstalled":true}]}}}`)
	}))
	defer server.Close()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, _, err := runRootArgs(t, "--home", t.TempDir(), "--no-learn", "--json", "sync", "--resources", "apps", "--first", "10", "--max-pages", "1")
	if err == nil || !strings.Contains(stdout, "repeated id") || !strings.Contains(stdout, `"apps": "failed"`) {
		t.Fatalf("duplicate IDs were accepted: err=%v stdout=%s", err, stdout)
	}
}

func TestArrayShapeAnalysisUsesAllDistinctElementShapes(t *testing.T) {
	value := []any{
		map[string]any{"title": "one"},
		map[string]any{"count": float64(2)},
		map[string]any{"title": "duplicate"},
	}
	sanitized := sanitizeStructure(value)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"title"`) || !strings.Contains(text, `"count"`) {
		t.Fatalf("sanitizer dropped a heterogeneous array shape: %s", text)
	}
	_, paths := structuralFingerprint(value)
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "$[].title:string") || !strings.Contains(joined, "$[].count:number") {
		t.Fatalf("fingerprint ignored a heterogeneous array shape: %v", paths)
	}
}

func TestPrivatePlaygroundsWritesHardenModesAndRejectSymlinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDirectory(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("private directory mode=%o want=700", got)
	}
	output := filepath.Join(dir, "data.json")
	if err := os.WriteFile(output, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(output, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("private file mode=%o want=600", got)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(link, []byte("must-not-follow")); err == nil {
		t.Fatal("private write followed a symlink")
	}
}

func TestPreviewRefreshIncludesDataOnlyPreviewWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/graphql":
			fmt.Fprint(w, `{"data":{"createSignedAppManagementJwt":{"signedAppManagementToken":"management-jwt"}}}`)
		case "/files/app-1":
			fmt.Fprint(w, `{"files":{},"lastModified":"2026-07-31T12:00:00Z"}`)
		case "/data/app-1":
			fmt.Fprint(w, `{"data":{},"lastModified":"2026-07-31T12:30:00Z"}`)
		case "/files/app-1-preview":
			http.NotFound(w, r)
		case "/data/app-1-preview":
			fmt.Fprint(w, `{"data":{"draft":true},"lastModified":"2026-08-01T12:00:00Z"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	home := t.TempDir()
	t.Setenv("SCREENCLOUD_BASE_URL", server.URL)
	t.Setenv("SCREENCLOUD_PLAYGROUNDS_URL", server.URL)
	t.Setenv("SCREENCLOUD_API_KEY", "key")
	stdout, stderr, err := runRootArgs(t, "--home", home, "--no-learn", "--json", "--yes", "playgrounds", "preview-drift", "--refresh", "--app-uuid", "app-1", "--space-id", "space-1")
	if err != nil {
		t.Fatalf("preview refresh failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, `"complete": true`) || strings.Contains(stdout, `"missing_metadata_app_uuids": [\n    "app-1"`) {
		t.Fatalf("data-only preview was treated as unavailable: %s", stdout)
	}
}

func TestAnalysisEvidenceFreshnessRejectsStaleOrFutureTimestamps(t *testing.T) {
	if !freshEvidenceTimestamp(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)) {
		t.Fatal("fresh timestamp was rejected")
	}
	if freshEvidenceTimestamp(time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)) {
		t.Fatal("stale timestamp was accepted")
	}
	if freshEvidenceTimestamp(time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)) {
		t.Fatal("implausible future timestamp was accepted")
	}
}

func TestPreviewDriftComparesFilesAndDataIndependently(t *testing.T) {
	findings := []map[string]any{}
	appendPreviewDriftFindings(&findings, "app-1", "files", time.Now(), time.Now().Add(-time.Hour), 7*24*time.Hour)
	appendPreviewDriftFindings(&findings, "app-1", "data", time.Now().Add(-time.Hour), time.Now(), 7*24*time.Hour)
	encoded, _ := json.Marshal(findings)
	text := string(encoded)
	if !strings.Contains(text, `"resource":"files"`) || !strings.Contains(text, `"type":"production_ahead"`) || !strings.Contains(text, `"resource":"data"`) || !strings.Contains(text, `"type":"preview_ahead"`) {
		t.Fatalf("crossed resource drift collapsed into one workspace timestamp: %s", text)
	}
}

func TestCompleteReceiptCannotProduceUnverifiedNoop(t *testing.T) {
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"stage":"complete"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "--no-learn", "--json", "playgrounds", "create-reconcile", "--receipt", receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, `"state": "noop"`) || !strings.Contains(stdout, `"state": "verification_required"`) {
		t.Fatalf("unverified receipt produced a no-op: %s", stdout)
	}
}
