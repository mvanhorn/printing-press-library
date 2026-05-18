package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/beehiiv/internal/store"
)

type recordingDependentSyncClient struct {
	paths     []string
	responses map[string]json.RawMessage
}

func (c *recordingDependentSyncClient) Get(path string, params map[string]string) (json.RawMessage, error) {
	c.paths = append(c.paths, path)
	if response, ok := c.responses[path]; ok {
		return response, nil
	}
	return json.RawMessage(`[]`), nil
}

func (c *recordingDependentSyncClient) RateLimit() float64 {
	return 0
}

func TestSyncDependentResourceResolvesNestedPathParams(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	if err := db.Upsert("publications", "pub_1", json.RawMessage(`{"id":"pub_1"}`)); err != nil {
		t.Fatalf("upsert publication: %v", err)
	}
	if err := db.Upsert("automations", "aut_1", json.RawMessage(`{"id":"aut_1","parent_id":"pub_1"}`)); err != nil {
		t.Fatalf("upsert automation: %v", err)
	}

	wantPath := "/publications/pub_1/automations/aut_1/emails"
	client := &recordingDependentSyncClient{
		responses: map[string]json.RawMessage{
			wantPath: json.RawMessage(`{"data":[{"id":"email_1"}]}`),
		},
	}

	result := syncDependentResource(client, db, dependentResourceDef{
		Name:          "emails",
		ParentTable:   "automations",
		ParentIDParam: "automationId",
		ScopeTable:    "publications",
		ScopeIDParam:  "publicationId",
		PathTemplate:  "/publications/{publicationId}/automations/{automationId}/emails",
	}, "", true, 1)
	if result.Err != nil {
		t.Fatalf("sync error: %v", result.Err)
	}
	if result.Warn != nil {
		t.Fatalf("sync warning: %v", result.Warn)
	}
	if result.Count != 1 {
		t.Fatalf("synced count = %d, want 1", result.Count)
	}
	if len(client.paths) != 1 || client.paths[0] != wantPath {
		t.Fatalf("paths = %v, want [%s]", client.paths, wantPath)
	}
	if strings.Contains(client.paths[0], "{") || strings.Contains(client.paths[0], "}") {
		t.Fatalf("path contains unresolved placeholder: %s", client.paths[0])
	}

	raw, err := db.Get("emails", "email_1")
	if err != nil {
		t.Fatalf("get synced email: %v", err)
	}
	var synced map[string]any
	if err := json.Unmarshal(raw, &synced); err != nil {
		t.Fatalf("decode synced email: %v", err)
	}
	if synced["parent_id"] != "aut_1" {
		t.Fatalf("email parent_id = %v, want aut_1", synced["parent_id"])
	}
}

func TestSyncPathWithParamsRejectsUnresolvedPlaceholders(t *testing.T) {
	_, err := syncPathWithParams(
		"/publications/{publicationId}/automations/{automationId}/emails",
		map[string]string{"publicationId": "pub_1"},
	)
	if err == nil {
		t.Fatal("expected unresolved placeholder error")
	}
}
