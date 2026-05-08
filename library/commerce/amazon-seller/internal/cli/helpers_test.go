package cli

import (
	"encoding/json"
	"testing"
)

type paginatedGetTestClient struct {
	responses []json.RawMessage
	params    []map[string]string
}

func (c *paginatedGetTestClient) GetWithHeaders(path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	copied := make(map[string]string, len(params))
	for k, v := range params {
		copied[k] = v
	}
	c.params = append(c.params, copied)

	response := c.responses[0]
	c.responses = c.responses[1:]
	return response, nil
}

func TestPaginatedGetFollowsNestedNextTokenAndCollectsResourceArray(t *testing.T) {
	client := &paginatedGetTestClient{
		responses: []json.RawMessage{
			json.RawMessage(`{"inventorySummaries":[{"sellerSku":"sku-1"}],"pagination":{"nextToken":"page-2"}}`),
			json.RawMessage(`{"inventorySummaries":[{"sellerSku":"sku-2"}],"pagination":{}}`),
		},
	}

	got, err := paginatedGet(client, "/fba/inventory/v1/summaries", map[string]string{
		"granularityType": "Marketplace",
	}, nil, true, "nextToken", "pagination.nextToken", "")
	if err != nil {
		t.Fatalf("paginatedGet returned error: %v", err)
	}

	var items []map[string]string
	if err := json.Unmarshal(got, &items); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2; raw = %s", len(items), got)
	}
	if items[0]["sellerSku"] != "sku-1" || items[1]["sellerSku"] != "sku-2" {
		t.Fatalf("items = %#v, want both inventory summaries", items)
	}
	if len(client.params) != 2 {
		t.Fatalf("requests = %d, want 2", len(client.params))
	}
	if client.params[1]["nextToken"] != "page-2" {
		t.Fatalf("second request nextToken = %q, want page-2", client.params[1]["nextToken"])
	}
}
