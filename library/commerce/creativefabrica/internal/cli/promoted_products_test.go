package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
)

func TestParseAlgoliaRequestsDefault(t *testing.T) {
	reqs, err := ParseAlgoliaRequests("")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].IndexName != algolia.IndexRelevance {
		t.Fatalf("default request = %+v", reqs)
	}
}

func TestParseAlgoliaRequestsArray(t *testing.T) {
	raw := `[{"indexName":"prod_Productsv2","query":"svg","hitsPerPage":5}]`
	reqs, err := ParseAlgoliaRequests(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Query != "svg" || reqs[0].HitsPerPage != 5 {
		t.Fatalf("parsed = %+v", reqs)
	}
}

func TestParseAlgoliaRequestsWrapped(t *testing.T) {
	raw := `{"requests":[{"query":"fonts"}]}`
	reqs, err := ParseAlgoliaRequests(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].Query != "fonts" {
		t.Fatalf("parsed = %+v", reqs)
	}
}

func TestParseAlgoliaRequestsRejectsGarbage(t *testing.T) {
	if _, err := ParseAlgoliaRequests("{"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseAlgoliaRequestsJSONRoundTrip(t *testing.T) {
	raw := `[{"indexName":"prod_Productsv2","query":"a"}]`
	reqs, err := ParseAlgoliaRequests(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(reqs)
	if err != nil || len(b) == 0 {
		t.Fatalf("marshal: %v %s", err, b)
	}
}
