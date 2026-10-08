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
	data := cursorListPage(t, 400, map[string]any{"cursor": "upstream-cursor", "has_more": true})
	out := EndpointResponse("POST", data)
	if len(out) > MaxBytes {
		t.Fatalf("result exceeds budget: %d", len(out))
	}
	var got struct {
		Entries        []map[string]any `json:"entries"`
		OmittedEntries []map[string]any `json:"omitted_entries"`
		Cursor         string           `json:"cursor"`
		HasMore        bool             `json:"has_more"`
		Resumable      bool             `json:"resumable"`
		Count          int              `json:"count"`
		ReturnedCount  int              `json:"returned_count"`
		OmittedCount   int              `json:"omitted_count"`
		Preview        string           `json:"preview"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Preview != "" || len(got.Entries) == 0 {
		t.Fatalf("POST list collapsed to a string preview: %.200s", out)
	}
	if !got.Resumable || !got.HasMore || got.Count != 400 || got.ReturnedCount != len(got.Entries) || got.OmittedCount != 400-len(got.Entries) {
		t.Fatalf("envelope = %+v", got)
	}
	// Every entry on the page is either returned in full or summarized, so
	// the upstream cursor resumes with nothing skipped.
	if got.Cursor != "upstream-cursor" || len(got.Entries)+len(got.OmittedEntries) != 400 {
		t.Fatalf("cursor=%q entries=%d omitted=%d", got.Cursor, len(got.Entries), len(got.OmittedEntries))
	}
	all := append(append([]map[string]any{}, got.Entries...), got.OmittedEntries...)
	for i, entry := range all {
		if want := fmt.Sprintf("/Folder/file-%04d.txt", i); entry["path_display"] != want {
			t.Fatalf("entry %d path = %v, want %s", i, entry["path_display"], want)
		}
	}
	if _, padded := got.OmittedEntries[0]["padding"]; padded {
		t.Fatalf("omitted entries should carry identity fields only: %+v", got.OmittedEntries[0])
	}
}

func TestEndpointResponseSummarizesNestedSearchMatches(t *testing.T) {
	matches := make([]map[string]any, 0, 200)
	for i := 0; i < 200; i++ {
		matches = append(matches, map[string]any{"match_type": map[string]any{".tag": "filename"}, "metadata": map[string]any{".tag": "metadata", "metadata": map[string]any{".tag": "file", "id": fmt.Sprintf("id:%d", i), "path_display": fmt.Sprintf("/Folder/m-%04d.txt", i), "padding": strings.Repeat("p", 500)}}})
	}
	data, err := json.Marshal(map[string]any{"matches": matches, "has_more": true, "cursor": "search-cursor"})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Matches        []json.RawMessage   `json:"matches"`
		OmittedEntries []map[string]string `json:"omitted_entries"`
		Cursor         string              `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(EndpointResponse("POST", data)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Cursor != "search-cursor" || len(got.Matches)+len(got.OmittedEntries) != 200 || got.OmittedEntries[0]["id"] == "" {
		t.Fatalf("search page: matches=%d omitted=%d cursor=%q first=%v", len(got.Matches), len(got.OmittedEntries), got.Cursor, got.OmittedEntries)
	}
}

func TestEndpointResponseRenamesCursorWhenEntriesCannotBeSummarized(t *testing.T) {
	items := make([]map[string]any, 0, 200)
	for i := 0; i < 200; i++ {
		items = append(items, map[string]any{"blob": strings.Repeat("b", 500)})
	}
	data, err := json.Marshal(map[string]any{"entries": items, "cursor": "upstream-cursor", "has_more": false})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(EndpointResponse("POST", data)), &got); err != nil {
		t.Fatal(err)
	}
	note, _ := got["note"].(string)
	if _, kept := got["cursor"]; kept || got["cursor_after_page"] != "upstream-cursor" || got["has_more"] != true || got["retry_page_size"] == nil || !strings.Contains(note, "same starting cursor") {
		t.Fatalf("fallback envelope = %v", got)
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
