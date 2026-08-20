package cli

import (
	"encoding/json"
	"testing"
)

func TestAddListingPhotoIDsAppendsUploadedIDs(t *testing.T) {
	t.Parallel()

	variables := `{"input":{"data":{"common":{"photo_ids":["existing"]}}}}`
	got, err := addListingPhotoIDs(variables, []string{"uploaded-1", "uploaded-2"})
	if err != nil {
		t.Fatalf("addListingPhotoIDs returned error: %v", err)
	}

	var parsed struct {
		Input struct {
			Data struct {
				Common struct {
					PhotoIDs []string `json:"photo_ids"`
				} `json:"common"`
			} `json:"data"`
		} `json:"input"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not JSON: %v", err)
	}
	want := []string{"existing", "uploaded-1", "uploaded-2"}
	if len(parsed.Input.Data.Common.PhotoIDs) != len(want) {
		t.Fatalf("photo_ids len = %d, want %d: %#v", len(parsed.Input.Data.Common.PhotoIDs), len(want), parsed.Input.Data.Common.PhotoIDs)
	}
	for i := range want {
		if parsed.Input.Data.Common.PhotoIDs[i] != want[i] {
			t.Fatalf("photo_ids[%d] = %q, want %q", i, parsed.Input.Data.Common.PhotoIDs[i], want[i])
		}
	}
}

func TestAddListingPhotoIDsRequiresCreateShape(t *testing.T) {
	t.Parallel()

	if _, err := addListingPhotoIDs(`{"input":{}}`, []string{"photo"}); err == nil {
		t.Fatalf("expected error for missing input.data.common")
	}
}
