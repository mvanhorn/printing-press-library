package hfdata

import (
	"encoding/json"
	"testing"
)

func TestBackendSupportJSON_Embedded(t *testing.T) {
	data, err := BackendSupportJSON()
	if err != nil {
		t.Fatalf("BackendSupportJSON error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("embedded backend-support.json is empty")
	}
	var matrix struct {
		SchemaVersion int `json:"schema_version"`
		Entries       []struct {
			Feature   string `json:"feature"`
			Backend   string `json:"backend"`
			Supported string `json:"supported"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("parsing embedded matrix: %v", err)
	}
	if matrix.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", matrix.SchemaVersion)
	}
	if len(matrix.Entries) < 5 {
		t.Errorf("entries = %d, want >= 5 (matrix is too small)", len(matrix.Entries))
	}
}
