package sync

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseGraphResponseAcmeSmallFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/salesforce-mock/fixtures/composite_graph/acme_small.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := ParseGraphResponse(data)
	if err != nil {
		t.Fatalf("parse graph response: %v", err)
	}

	assertLen(t, result.Records["Account"], 1, "Account")
	assertLen(t, result.Records["Contact"], 6, "Contact")
	assertLen(t, result.Records["Opportunity"], 3, "Opportunity")
	assertLen(t, result.Records["Case"], 5, "Case")
	assertLen(t, result.Records["Task"], 4, "Task")
	assertLen(t, result.Records["Event"], 2, "Event")
	assertLen(t, result.Records["FeedItem"], 2, "FeedItem")

	var account struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(result.Records["Account"][0], &account); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	if account.ID != "001ACME0001" || account.Name != "Acme Manufacturing" {
		t.Fatalf("account = %+v, want acme account", account)
	}
}

func TestBuildAccountGraphIncludesSinceInEveryQuery(t *testing.T) {
	since := time.Date(2026, 4, 20, 10, 30, 0, 0, time.UTC)
	req, err := BuildAccountGraph("001ACME0001", since)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}
	if len(req.Graphs) != 1 {
		t.Fatalf("graphs len = %d, want 1", len(req.Graphs))
	}
	if got := len(req.Graphs[0].CompositeRequest); got != 8 {
		t.Fatalf("node count = %d, want 8", got)
	}
	for _, node := range req.Graphs[0].CompositeRequest {
		parsed, err := url.Parse(node.URL)
		if err != nil {
			t.Fatalf("parse node url %q: %v", node.URL, err)
		}
		q := parsed.Query().Get("q")
		if !strings.Contains(q, "LastModifiedDate >= 2026-04-20T10:30:00Z") {
			t.Fatalf("%s query missing since filter: %s", node.ReferenceID, q)
		}
	}
}

func assertLen(t *testing.T, records []json.RawMessage, want int, name string) {
	t.Helper()
	if len(records) != want {
		t.Fatalf("%s records len = %d, want %d", name, len(records), want)
	}
}

// TestParseGraphResponseSurfacesNodeErrors locks F-021: an unsuccessful
// graph must surface per-subrequest Salesforce error detail (errorCode +
// message) instead of an opaque "was not successful".
func TestParseGraphResponseSurfacesNodeErrors(t *testing.T) {
	payload := []byte(`{"graphs":[{"graphId":"acme-graph","isSuccessful":false,"graphResponse":{"compositeResponse":[
		{"referenceId":"Account","httpStatusCode":400,"body":[{"message":"No such column 'AnnualRevenue' on entity 'Account'","errorCode":"INVALID_FIELD"}]},
		{"referenceId":"Contacts","httpStatusCode":200,"body":{"totalSize":0,"done":true,"records":[]}}
	]}}]}`)
	_, err := ParseGraphResponse(payload)
	if err == nil {
		t.Fatal("expected error for unsuccessful graph")
	}
	for _, want := range []string{"acme-graph", "Account", "HTTP 400", "INVALID_FIELD", "AnnualRevenue"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}
