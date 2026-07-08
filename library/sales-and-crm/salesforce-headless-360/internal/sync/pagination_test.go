package sync

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

type fakeHeaderGetClient struct {
	responses map[string]json.RawMessage
	headers   http.Header
	calls     []string
}

func (f *fakeHeaderGetClient) GetWithResponseHeaders(path string, _ map[string]string) (json.RawMessage, http.Header, error) {
	f.calls = append(f.calls, path)
	if body, ok := f.responses[path]; ok {
		return body, f.headers, nil
	}
	return nil, f.headers, &notFoundError{path: path}
}

type notFoundError struct {
	path string
}

func (e *notFoundError) Error() string {
	return "not found: " + e.path
}

func TestPaginationFallbackCompletesTruncatedCompositeGraph(t *testing.T) {
	data, err := os.ReadFile("../../testdata/salesforce-mock/fixtures/composite_graph/acme_bulk_threshold.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := ParseGraphResponse(data)
	if err != nil {
		t.Fatalf("parse graph response: %v", err)
	}
	if len(result.PageRefs) != 1 {
		t.Fatalf("page refs len = %d, want 1", len(result.PageRefs))
	}
	if got := len(result.Records["Task"]); got != 1 {
		t.Fatalf("initial task records = %d, want 1 marker", got)
	}

	client := &fakeHeaderGetClient{responses: map[string]json.RawMessage{
		"/services/data/v63.0/query/01gMOCKTASKPAGE-2000": json.RawMessage(`{
			"totalSize": 2001,
			"done": true,
			"records": [
				{"attributes":{"type":"Task"},"Id":"00TBULK0002","WhatId":"001ACME2000","Subject":"Follow-up page"}
			]
		}`),
	}}
	rows, err := FetchRemainingPages(client, result.PageRefs[0], NewGate())
	if err != nil {
		t.Fatalf("fetch remaining pages: %v", err)
	}
	result.Records["Task"] = append(result.Records["Task"], rows...)
	if got := len(result.Records["Task"]); got != 2 {
		t.Fatalf("task records after fallback = %d, want 2", got)
	}
	if got := client.calls[0]; got != "/services/data/v63.0/query/01gMOCKTASKPAGE-2000" {
		t.Fatalf("fallback path = %q", got)
	}
}
