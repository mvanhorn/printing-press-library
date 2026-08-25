package mcp

import (
	"encoding/json"
	"testing"
)

func TestFilterListedAshbyResponseExcludesUnlistedJobs(t *testing.T) {
	raw := json.RawMessage(`{"apiVersion":"1","jobs":[{"id":"public","isListed":true},{"id":"hidden","isListed":false}]}`)
	filtered, err := filterListedAshbyResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Jobs []struct {
			ID string `json:"id"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(filtered, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Jobs) != 1 || response.Jobs[0].ID != "public" {
		t.Fatalf("jobs=%v, want only public", response.Jobs)
	}
}

func TestFilterListedAshbyResponseRejectsMissingJobs(t *testing.T) {
	if _, err := filterListedAshbyResponse(json.RawMessage(`{"apiVersion":"1"}`)); err == nil {
		t.Fatal("expected missing jobs error")
	}
}
