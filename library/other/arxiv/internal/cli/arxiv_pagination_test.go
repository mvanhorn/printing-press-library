package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/arxiv/internal/store"
)

type fakeSyncClient struct {
	paths  []string
	params []map[string]string
	pages  []json.RawMessage
}

func (f *fakeSyncClient) Get(path string, params map[string]string) (json.RawMessage, error) {
	f.paths = append(f.paths, path)
	copyParams := map[string]string{}
	for k, v := range params {
		copyParams[k] = v
	}
	f.params = append(f.params, copyParams)
	if len(f.pages) == 0 {
		return json.RawMessage(`[]`), nil
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page, nil
}

func (f *fakeSyncClient) RateLimit() float64 { return 0 }

type fakePageClient struct {
	params []map[string]string
	pages  []json.RawMessage
}

func (f *fakePageClient) GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	copyParams := map[string]string{}
	for k, v := range params {
		copyParams[k] = v
	}
	f.params = append(f.params, copyParams)
	if len(f.pages) == 0 {
		return json.RawMessage(`[]`), nil
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page, nil
}

func atomPage(total, start, perPage int, ids ...string) json.RawMessage {
	raw := `<?xml version='1.0'?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>` + itoa(total) + `</opensearch:totalResults><opensearch:startIndex>` + itoa(start) + `</opensearch:startIndex><opensearch:itemsPerPage>` + itoa(perPage) + `</opensearch:itemsPerPage>`
	for _, id := range ids {
		raw += `<entry><id>` + id + `</id><title>` + id + `</title></entry>`
	}
	raw += `</feed>`
	b, _ := json.Marshal(raw)
	return b
}

func itoa(v int) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestSyncResourceParsesArxivAtomAndUsesOffsetPagination(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	client := &fakeSyncClient{pages: []json.RawMessage{
		atomPage(3, 0, 2, "http://arxiv.org/abs/1", "http://arxiv.org/abs/2"),
		atomPage(3, 2, 1, "http://arxiv.org/abs/3"),
	}}

	res := syncResource(client, db, "query", "", true, 10)
	if res.Err != nil {
		t.Fatalf("syncResource error: %v", res.Err)
	}
	if res.Count != 3 {
		t.Fatalf("synced count = %d, want 3", res.Count)
	}
	if len(client.paths) != 2 || client.paths[0] != "/api/query" || client.paths[1] != "/api/query" {
		t.Fatalf("paths = %#v", client.paths)
	}
	if client.params[0]["max_results"] != "100" || client.params[0]["start"] != "" {
		t.Fatalf("first params = %#v", client.params[0])
	}
	if client.params[1]["start"] != "2" {
		t.Fatalf("second params = %#v, want start=2", client.params[1])
	}
}

func TestPaginatedGetParsesArxivAtomAllPages(t *testing.T) {
	client := &fakePageClient{pages: []json.RawMessage{
		atomPage(2, 0, 1, "http://arxiv.org/abs/1"),
		atomPage(2, 1, 1, "http://arxiv.org/abs/2"),
	}}

	got, err := paginatedGet(client, "/api/query", map[string]string{"max_results": "1"}, nil, true, "start", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(got, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2: %s", len(items), string(got))
	}
	if client.params[1]["start"] != "1" {
		t.Fatalf("second params = %#v, want start=1", client.params[1])
	}
}

func TestWriteThroughCacheStoresArxivEntriesEnvelope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeThroughCache(t.Context(), "query", json.RawMessage(`{"entries":[{"id":"http://arxiv.org/abs/1","title":"One"}]}`))

	db, err := store.Open(defaultDBPath("arxiv-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.List("query", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("cached rows = %d, want 1", len(rows))
	}
}
