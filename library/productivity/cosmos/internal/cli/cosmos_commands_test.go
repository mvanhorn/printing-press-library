// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/cosmos/internal/platform"
	"github.com/spf13/cobra"
)

func newCosmosMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Operation string         `json:"operationName"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		var data any
		switch request.Operation {
		case "GetMe":
			data = map[string]any{"me": map[string]any{"id": 1, "username": "example"}}
		case "GetUserClusters":
			data = map[string]any{"userClusters": map[string]any{"items": []any{
				map[string]any{"id": 10, "name": "Alpha", "numberOfElements": 2},
				map[string]any{"id": 20, "name": "Beta", "numberOfElements": 2},
			}}}
		case "GetClusterElements":
			id := idString(request.Variables["clusterId"])
			items := []any{}
			if id == "10" {
				items = []any{mockConnection(1, "https://media.example/one.jpg", ""), mockConnection(2, "https://media.example/shared.jpg", "https://source.example/two")}
			} else {
				items = []any{mockConnection(2, "https://media.example/shared.jpg", "https://source.example/two"), mockConnection(3, "https://media.example/three.jpg", "https://other.example/three")}
			}
			data = map[string]any{"clusterConnections": map[string]any{"items": items, "meta": map[string]any{"count": len(items)}}}
		case "SearchGlobalElements":
			data = map[string]any{"searchElements": map[string]any{"items": []any{
				mockElement(2, "https://media.example/shared.jpg", "https://source.example/two"),
				mockElement(4, "https://media.example/four.jpg", "https://new.example/four"),
			}, "meta": map[string]any{"count": 2}}}
		case "GetAllElements":
			data = map[string]any{"allElementsV2": map[string]any{"items": []any{
				mockElement(1, "https://media.example/one.jpg", ""),
				mockElement(5, "https://media.example/one.jpg", ""),
			}}}
		case "GetSimilarElements":
			data = map[string]any{"similarElementsV2": map[string]any{"items": []any{mockElement(6, "https://media.example/six.jpg", "https://trail.example/six")}, "meta": map[string]any{"count": 1}}}
		default:
			t.Fatalf("unexpected operation %q", request.Operation)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
}

func mockConnection(id int, media, source string) map[string]any {
	return map[string]any{"element": mockElement(id, media, source)}
}

func mockElement(id int, media, source string) map[string]any {
	return map[string]any{
		"id": id, "__typename": "MediaElementTile", "createdAt": "2099-01-01T00:00:00Z",
		"shareUrl": "https://cosmos.so/e/example", "source": map[string]any{"url": source},
		"media":       map[string]any{"url": media, "aiGenerated": id == 5},
		"userContext": map[string]any{"connections": map[string]any{"meta": map[string]any{"count": 0}}},
	}
}

func runCosmosJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	cmd := RootCmd()
	args = append(args, "--json", "--no-learn", "--rate-limit", "0")
	cmd.SetArgs(args)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v (output=%s)", args, err, output.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output for %v: %v (output=%s)", args, err, output.String())
	}
	return decoded
}

func TestCosmosNovelAnalysisCommands(t *testing.T) {
	server := newCosmosMockServer(t)
	defer server.Close()
	t.Setenv("COSMOS_BASE_URL", server.URL)
	t.Setenv("COSMOS_TOKEN", "test-token")
	home := t.TempDir()
	t.Setenv("COSMOS_HOME", home)

	overlap := runCosmosJSON(t, "collection", "overlap", "10", "20")
	if got := int(overlap["jaccard_similarity"].(float64) * 100); got != 33 {
		t.Fatalf("overlap similarity = %d%%, want 33%%", got)
	}
	coverage := runCosmosJSON(t, "collection", "coverage", "--collection", "10", "--query", "example", "--limit", "2")
	if got := len(coverage["promising_unsaved"].([]any)); got != 1 {
		t.Fatalf("coverage promising count = %d, want 1", got)
	}
	provenance := runCosmosJSON(t, "provenance", "audit", "--collection", "10")
	if got := len(provenance["missing_source_url"].([]any)); got != 1 {
		t.Fatalf("missing source count = %d, want 1", got)
	}
	review := runCosmosJSON(t, "review", "--since", "7d")
	if got := len(review["duplicate_media"].([]any)); got != 1 {
		t.Fatalf("duplicate media groups = %d, want 1", got)
	}
	trail := runCosmosJSON(t, "element", "trail", "--id", "1", "--depth", "1", "--limit", "2")
	if got := int(trail["node_count"].(float64)); got != 2 {
		t.Fatalf("trail node count = %d, want 2", got)
	}
}

func TestCosmosMutationsDryRunBeforeValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("COSMOS_HOME", home)
	for _, args := range [][]string{
		{"collection", "create", "--dry-run"},
		{"collection", "create-sub", "--dry-run"},
		{"collection", "connect", "--dry-run"},
		{"collection", "disconnect", "--dry-run"},
		{"element", "save-url", "--dry-run"},
	} {
		result := runCosmosJSON(t, args...)
		if result["dry_run"] != true {
			t.Fatalf("%v did not return dry_run=true: %#v", args, result)
		}
	}
}

func TestCosmosGraphQLQueryUsesReadTransportDuringVerification(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"me": map[string]any{"id": 1}}})
	}))
	defer server.Close()
	t.Setenv("COSMOS_BASE_URL", server.URL)
	t.Setenv("COSMOS_TOKEN", "test-token")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	flags := &rootFlags{timeout: time.Second, rateLimit: 0, noCache: true}
	cmd := &cobra.Command{Use: "fixture"}
	cmd.SetContext(context.Background())
	if _, err := cosmosGraphQL(flags, cmd, "GetMe", cosmosGetMeQuery, map[string]any{}, nil); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if requests != 1 {
		t.Fatalf("query requests = %d, want 1", requests)
	}
}

func TestCosmosCollectionElementsFollowsPageCursor(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		cursor, _ := request.Variables["pageCursor"].(string)
		items := []any{mockConnection(1, "https://media.example/one.jpg", "")}
		next := any("page-2")
		if cursor == "page-2" {
			items = []any{mockConnection(2, "https://media.example/two.jpg", "")}
			next = nil
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"clusterConnections": map[string]any{"items": items, "meta": map[string]any{"nextPageCursor": next}},
		}})
	}))
	defer server.Close()
	t.Setenv("COSMOS_BASE_URL", server.URL)
	t.Setenv("COSMOS_TOKEN", "test-token")

	flags := &rootFlags{timeout: time.Second, rateLimit: 0, noCache: true}
	cmd := &cobra.Command{Use: "fixture"}
	cmd.SetContext(context.Background())
	items, err := cosmosCollectionElements(flags, cmd, int64(10), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || requests != 2 {
		t.Fatalf("items=%d requests=%d, want 2 and 2", len(items), requests)
	}
}

func TestCosmosMyCollectionsFollowsPageCursor(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		cursor, _ := request.Variables["pageCursor"].(string)
		items := []any{map[string]any{"id": 1, "name": "First"}}
		next := any("page-2")
		if cursor == "page-2" {
			items = []any{map[string]any{"id": 2, "name": "Second"}}
			next = nil
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"userClusters": map[string]any{"items": items, "meta": map[string]any{"nextPageCursor": next}},
		}})
	}))
	defer server.Close()
	t.Setenv("COSMOS_BASE_URL", server.URL)
	t.Setenv("COSMOS_TOKEN", "test-token")

	flags := &rootFlags{timeout: time.Second, rateLimit: 0, noCache: true}
	cmd := &cobra.Command{Use: "fixture"}
	cmd.SetContext(context.Background())
	items, err := cosmosMyCollectionsForUser(flags, cmd, int64(10), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || requests != 2 {
		t.Fatalf("items=%d requests=%d, want 2 and 2", len(items), requests)
	}
}

func TestCosmosExportRequiresExplicitOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(path, []byte("replacement"), false); err == nil {
		t.Fatal("existing output was accepted without --overwrite")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "original" {
		t.Fatalf("existing output changed without --overwrite: %q", body)
	}
	if err := writePrivateFile(path, []byte("replacement"), true); err != nil {
		t.Fatalf("explicit overwrite rejected: %v", err)
	}

	flags := &rootFlags{}
	if export := newCosmosExportCmd(flags); export.Annotations["mcp:hidden"] != "true" {
		t.Fatal("export command with caller-selected output paths is exposed to MCP")
	}
	for _, command := range []*cobra.Command{newCosmosExportCollectionCmd(flags), newCosmosExportGalleryCmd(flags)} {
		if command.Annotations["mcp:local-write"] == "true" {
			t.Fatalf("%s is incorrectly advertised as a non-destructive local write", command.Name())
		}
	}
}

func TestAuthSetTokenRejectsProcessArguments(t *testing.T) {
	cmd := newAuthSetTokenCmd(&rootFlags{})
	cmd.SetArgs([]string{"secret-from-argv"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("set-token accepted a secret in process arguments")
	}
}

func TestNormalizeElementRemovesCosmosHighlightMarkup(t *testing.T) {
	item := normalizeElement(map[string]any{
		"generatedCaption": map[string]any{"text": "A &amp; B with <n>Brutalist</n> type"},
		"websiteTitle":     "<n>Example</n> &amp; reference",
		"source":           map[string]any{},
		"media":            map[string]any{},
	})
	if got := item["caption"]; got != "A & B with Brutalist type" {
		t.Fatalf("caption = %q", got)
	}
	if got := item["title"]; got != "Example & reference" {
		t.Fatalf("title = %q", got)
	}
}

func TestCosmosSnapshotDiffReportsAddedAndMoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("COSMOS_HOME", home)
	flags := &rootFlags{asJSON: true, platformSession: &platform.Session{
		Paths:            platform.Paths{DataFile: filepath.Join(home, "clients", "fixture", "cosmos", "data.db")},
		ObservedIdentity: map[string]string{"account_id": "fixture-account"},
	}}
	from := cosmosSnapshot{
		CapturedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		AccountID:  "fixture-account",
		Elements: map[string][]map[string]any{
			"10": {{"id": 1}, {"id": 2}},
			"20": {{"id": 3}},
		},
	}
	to := cosmosSnapshot{
		CapturedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		AccountID:  "fixture-account",
		Elements: map[string][]map[string]any{
			"10": {{"id": 2}, {"id": 4}},
			"20": {{"id": 1}, {"id": 3}},
		},
	}
	if _, err := saveCosmosSnapshot(flags, from); err != nil {
		t.Fatal(err)
	}
	if _, err := saveCosmosSnapshot(flags, to); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "fixture"}
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := runCosmosSnapshotDiff(cmd, flags, "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	data := result["data"].(map[string]any)
	counts := data["counts"].(map[string]any)
	if counts["added"].(float64) != 1 || counts["moved"].(float64) != 1 || counts["removed"].(float64) != 0 {
		t.Fatalf("unexpected snapshot diff counts: %#v", counts)
	}
}

func TestCosmosSnapshotRejectsDifferentVerifiedAccount(t *testing.T) {
	home := t.TempDir()
	flags := &rootFlags{platformSession: &platform.Session{
		Paths:            platform.Paths{DataFile: filepath.Join(home, "clients", "fixture", "cosmos", "data.db")},
		ObservedIdentity: map[string]string{"account_id": "verified-account"},
	}}
	_, err := saveCosmosSnapshot(flags, cosmosSnapshot{CapturedAt: time.Now().UTC(), AccountID: "other-account"})
	if err == nil {
		t.Fatal("cross-account snapshot was accepted")
	}
}

func TestCosmosSnapshotHistoryIsIsolatedByClientProfile(t *testing.T) {
	home := t.TempDir()
	flagsFor := func(profile string) *rootFlags {
		return &rootFlags{platformSession: &platform.Session{
			Paths:            platform.Paths{DataFile: filepath.Join(home, "clients", profile, "cosmos", "data.db")},
			ObservedIdentity: map[string]string{"account_id": "fixture-account"},
		}}
	}
	first, err := snapshotDir(flagsFor("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := snapshotDir(flagsFor("second"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("snapshot directories overlap across profiles: %s", first)
	}
	if _, err := saveCosmosSnapshot(flagsFor("first"), cosmosSnapshot{
		CapturedAt: time.Now().UTC(),
		AccountID:  "fixture-account",
	}); err != nil {
		t.Fatal(err)
	}
	secondSnapshots, err := loadCosmosSnapshots(flagsFor("second"))
	if err != nil {
		t.Fatal(err)
	}
	if len(secondSnapshots) != 0 {
		t.Fatalf("second profile loaded %d snapshots from first profile", len(secondSnapshots))
	}
}

func TestLoadCosmosSnapshotsRejectsWrongAccountAndOversizedFiles(t *testing.T) {
	newFlags := func(home string) *rootFlags {
		return &rootFlags{platformSession: &platform.Session{
			Paths:            platform.Paths{DataFile: filepath.Join(home, "clients", "fixture", "cosmos", "data.db")},
			ObservedIdentity: map[string]string{"account_id": "verified-account"},
		}}
	}

	t.Run("wrong account", func(t *testing.T) {
		flags := newFlags(t.TempDir())
		dir, err := snapshotDir(flags)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		capturedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		body, err := json.Marshal(cosmosSnapshot{CapturedAt: capturedAt, AccountID: "other-account"})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, capturedAt.Format("20060102T150405.000000000Z")+".json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadCosmosSnapshots(flags); err == nil || !strings.Contains(err.Error(), "different Cosmos account") {
			t.Fatalf("wrong-account load error = %v", err)
		}
	})

	t.Run("oversized", func(t *testing.T) {
		flags := newFlags(t.TempDir())
		dir, err := snapshotDir(flags)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		capturedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		path := filepath.Join(dir, capturedAt.Format("20060102T150405.000000000Z")+".json")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate((16 << 20) + 1); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := loadCosmosSnapshots(flags); err == nil || !strings.Contains(err.Error(), "16 MiB safety limit") {
			t.Fatalf("oversized load error = %v", err)
		}
	})
}

func TestCosmosAuthExcludesStoredCredentialLoginFlows(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"login", "refresh"} {
		if command, _, err := root.Find([]string{"auth", name}); err == nil && command.Name() == name {
			t.Fatalf("auth %s remained public despite env-only client profiles", name)
		}
	}
}

func TestCosmosRawGraphQLCommandsAreNotExecutable(t *testing.T) {
	for _, args := range [][]string{{"elements", "get"}, {"api", "elements"}} {
		root := RootCmd()
		root.SetArgs(args)
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&output)
		if err := root.Execute(); err == nil {
			t.Fatalf("raw GraphQL path %v remained executable: %s", args, output.String())
		}
	}
}
