package cli

import (
	"encoding/json"
	"testing"
)

func TestParseArxivAtomJSON(t *testing.T) {
	raw := `<?xml version='1.0'?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>1</opensearch:totalResults><opensearch:startIndex>0</opensearch:startIndex><opensearch:itemsPerPage>1</opensearch:itemsPerPage><entry><id>http://arxiv.org/abs/1706.03762v7</id><title>Attention Is All You Need</title><summary>Transformer paper.</summary><author><name>Ashish Vaswani</name></author><category term="cs.CL"/><link href="https://arxiv.org/pdf/1706.03762v7" rel="related" type="application/pdf" title="pdf"/></entry></feed>`
	b, _ := json.Marshal(raw)
	got, err := parseArxivAtomJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		TotalResults int `json:"total_results"`
		Entries      []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.TotalResults != 1 || len(parsed.Entries) != 1 {
		t.Fatalf("unexpected parsed feed: %+v", parsed)
	}
	if parsed.Entries[0].ID != "http://arxiv.org/abs/1706.03762v7" {
		t.Fatalf("bad id: %s", parsed.Entries[0].ID)
	}
	if parsed.Entries[0].Authors[0].Name != "Ashish Vaswani" {
		t.Fatalf("bad author: %+v", parsed.Entries[0].Authors)
	}
}

func TestParseArxivAtomWrappedResults(t *testing.T) {
	raw := `<?xml version='1.0'?><feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"><opensearch:totalResults>1</opensearch:totalResults><entry><id>http://arxiv.org/abs/1</id><title>One</title></entry></feed>`
	b, _ := json.Marshal(map[string]string{"results": raw})
	got, err := parseArxivAtomJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Entries []struct {
			ID string `json:"id"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Entries) != 1 || parsed.Entries[0].ID != "http://arxiv.org/abs/1" {
		t.Fatalf("unexpected parsed feed: %s", string(got))
	}
}
