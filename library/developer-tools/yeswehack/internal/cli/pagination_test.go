package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

type fakePaginatedClient struct {
	requestedPages []string
}

func (f *fakePaginatedClient) GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	f.requestedPages = append(f.requestedPages, params["page"])
	switch params["page"] {
	case "1":
		return json.RawMessage(`{"pagination":{"page":1,"nb_pages":2,"results_per_page":2,"nb_results":3},"items":[{"id":"a"},{"id":"b"}]}`), nil
	case "2":
		return json.RawMessage(`{"pagination":{"page":2,"nb_pages":2,"results_per_page":2,"nb_results":3},"items":[{"id":"c"}]}`), nil
	default:
		return json.RawMessage(`{"pagination":{"page":1,"nb_pages":1},"items":[]}`), nil
	}
}

func TestPaginatedGetPageStyleFetchAll(t *testing.T) {
	client := &fakePaginatedClient{}

	data, err := paginatedGet(client, "/programs", map[string]string{
		"page":           "",
		"resultsPerPage": "2",
	}, nil, true, "", "", "")
	if err != nil {
		t.Fatalf("paginatedGet returned error: %v", err)
	}

	if !reflect.DeepEqual(client.requestedPages, []string{"1", "2"}) {
		t.Fatalf("requested pages = %v, want [1 2]", client.requestedPages)
	}

	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal combined items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("combined item count = %d, want 3", len(items))
	}
}

func TestExtractPaginationFromEnvelopePageStyle(t *testing.T) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"pagination":{"page":2,"nb_pages":4},"items":[{"id":"x"}]}`), &envelope); err != nil {
		t.Fatal(err)
	}

	next, more := extractPaginationFromEnvelope(envelope, "page")
	if next != "3" || !more {
		t.Fatalf("next=%q more=%v, want next=3 more=true", next, more)
	}
}

func TestIsEmptyListEnvelope(t *testing.T) {
	if !isEmptyListEnvelope(json.RawMessage(`{"pagination":{"page":1,"nb_pages":0},"items":[]}`)) {
		t.Fatal("expected empty items envelope to be recognized as a list")
	}
	if isEmptyListEnvelope(json.RawMessage(`{"id":"one","items_count":0}`)) {
		t.Fatal("single object was incorrectly recognized as an empty list")
	}
}
