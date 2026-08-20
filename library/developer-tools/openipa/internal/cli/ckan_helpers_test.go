// Hand-written addition: tests for shared CKAN datastore_search parsing — preserve on regeneration.

package cli

import (
	"strings"
	"testing"
)

func TestParseCKANDatastore(t *testing.T) {
	t.Run("success with records", func(t *testing.T) {
		raw := []byte(`{"success":true,"result":{"records":[{"a":"1"},{"a":"2"}],"total":5}}`)
		records, total, err := parseCKANDatastore(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 2 {
			t.Errorf("records = %d, want 2", len(records))
		}
		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}
	})

	// CKAN returns HTTP 200 with success:false on API-level errors (e.g. a
	// stale resource_id). This must surface as an error, not an empty result.
	t.Run("success false with error message", func(t *testing.T) {
		raw := []byte(`{"success":false,"error":{"__type":"Not Found Error","message":"Resource \"x\" was not found."}}`)
		records, total, err := parseCKANDatastore(raw)
		if err == nil {
			t.Fatal("expected error for success:false, got nil")
		}
		if records != nil || total != 0 {
			t.Errorf("records/total = %v/%d, want nil/0", records, total)
		}
		if !strings.Contains(err.Error(), "was not found") {
			t.Errorf("error %q does not carry the CKAN message", err.Error())
		}
	})

	t.Run("success false falls back to __type", func(t *testing.T) {
		raw := []byte(`{"success":false,"error":{"__type":"Validation Error"}}`)
		_, _, err := parseCKANDatastore(raw)
		if err == nil || !strings.Contains(err.Error(), "Validation Error") {
			t.Errorf("error = %v, want it to carry __type", err)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		if _, _, err := parseCKANDatastore([]byte(`{not json`)); err == nil {
			t.Fatal("expected parse error, got nil")
		}
	})
}
