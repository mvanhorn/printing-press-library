package bound

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func cursorListPage(t *testing.T, entries int, extra map[string]any) json.RawMessage {
	t.Helper()
	items := make([]map[string]any, 0, entries)
	for i := 0; i < entries; i++ {
		items = append(items, map[string]any{".tag": "file", "name": fmt.Sprintf("file-%04d.txt", i), "path_display": fmt.Sprintf("/Folder/file-%04d.txt", i), "padding": strings.Repeat("p", 400)})
	}
	page := map[string]any{"entries": items}
	for k, v := range extra {
		page[k] = v
	}
	data, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= MaxBytes {
		t.Fatalf("fixture must exceed MaxBytes, got %d", len(data))
	}
	return data
}

func TestEndpointResponseBoundsPOSTCursorListAsEntries(t *testing.T) {
	data := cursorListPage(t, 400, map[string]any{"cursor": "upstream-cursor", "has_more": false})
	out := EndpointResponse("POST", data)
	if len(out) > MaxBytes {
		t.Fatalf("result exceeds budget: %d", len(out))
	}
	var got struct {
		Entries         []map[string]any `json:"entries"`
		Cursor          string           `json:"cursor"`
		CursorAfterPage string           `json:"cursor_after_page"`
		HasMore         bool             `json:"has_more"`
		Resumable       bool             `json:"resumable"`
		Count           int              `json:"count"`
		ReturnedCount   int              `json:"returned_count"`
		OmittedCount    int              `json:"omitted_count"`
		RetryPageSize   int              `json:"retry_page_size"`
		Preview         string           `json:"preview"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Preview != "" || len(got.Entries) == 0 {
		t.Fatalf("POST list collapsed to a string preview: %.200s", out)
	}
	if !got.Resumable || !got.HasMore || got.Count != 400 || got.ReturnedCount != len(got.Entries) || got.OmittedCount != 400-len(got.Entries) || got.RetryPageSize != len(got.Entries) {
		t.Fatalf("envelope = %+v", got)
	}
	if got.Cursor != "" || got.CursorAfterPage != "upstream-cursor" {
		t.Fatalf("a cut page must not hand out a cursor that skips entries: cursor=%q after=%q", got.Cursor, got.CursorAfterPage)
	}
}

func TestEndpointResponseBoundsPOSTContinuationWithMoreFlag(t *testing.T) {
	data := cursorListPage(t, 300, map[string]any{"more": true})
	out := EndpointResponse("POST", data)
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := got["entries"]; !ok || got["more"] != true || got["resumable"] != true {
		t.Fatalf("continuation page = %.300s", out)
	}
}

func TestEndpointResponseKeepsPreviewForPOSTWithoutListSignals(t *testing.T) {
	data := cursorListPage(t, 300, nil)
	out := EndpointResponse("POST", data)
	if !strings.Contains(out, `"preview"`) {
		t.Fatalf("POST object without cursor or more flag should keep the preview contract: %.200s", out)
	}
}
