package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	crmsync "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/sync"
)

type fakeLiveClient struct {
	body json.RawMessage
}

func (f fakeLiveClient) PostWithResponseHeaders(string, any) (json.RawMessage, int, http.Header, error) {
	return f.body, http.StatusOK, http.Header{}, nil
}

func (f fakeLiveClient) GetWithResponseHeaders(string, map[string]string) (json.RawMessage, http.Header, error) {
	return nil, http.Header{}, nil
}

type countingSecurityFilter struct {
	count int
}

func (f *countingSecurityFilter) Apply(_ context.Context, record *security.Record) *security.Record {
	f.count++
	return record
}

func TestAssembleLiveManifestFromCompositeGraphAppliesFilter(t *testing.T) {
	data, err := os.ReadFile("../../testdata/salesforce-mock/fixtures/composite_graph/acme_small.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	graph, err := crmsync.ParseGraphResponse(data)
	if err != nil {
		t.Fatalf("ParseGraphResponse: %v", err)
	}
	expected := 0
	for _, records := range graph.Records {
		expected += len(records)
	}
	filter := &countingSecurityFilter{}
	manifest, _, err := AssembleLiveManifest(context.Background(), fakeLiveClient{body: data}, LiveAssemblyOptions{
		AccountID: "001ACME0001",
		Filter:    filter,
	})
	if err != nil {
		t.Fatalf("AssembleLiveManifest: %v", err)
	}
	if manifest.Account == nil || manifest.Account.ID != "001ACME0001" {
		t.Fatalf("account not mapped: %+v", manifest.Account)
	}
	if len(manifest.Contacts) != 6 || len(manifest.Opportunities) != 3 {
		t.Fatalf("unexpected manifest counts: contacts=%d opps=%d", len(manifest.Contacts), len(manifest.Opportunities))
	}
	if filter.count != expected {
		t.Fatalf("filter count=%d expected=%d", filter.count, expected)
	}
}
