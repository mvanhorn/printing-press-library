package sync

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type fakeBulkClient struct {
	postBody json.RawMessage
	gets     map[string]json.RawMessage
}

func (f *fakeBulkClient) PostWithResponseHeaders(_ string, _ any) (json.RawMessage, int, http.Header, error) {
	return f.postBody, http.StatusOK, nil, nil
}

func (f *fakeBulkClient) GetWithResponseHeaders(path string, _ map[string]string) (json.RawMessage, http.Header, error) {
	return f.gets[path], nil, nil
}

func TestBulkFallbackRequiresExplicitUnsafeFlag(t *testing.T) {
	_, err := RunBulkFallback(&fakeBulkClient{}, "Task", "SELECT Id FROM Task", false, nil)
	if err == nil {
		t.Fatalf("expected unsafe bulk fallback to be rejected")
	}
	if !strings.Contains(err.Error(), "--allow-bulk-fls-unsafe") {
		t.Fatalf("error = %v, want guidance flag", err)
	}
}

func TestBulkFallbackWarnsAndRecordsUnsafeProvenance(t *testing.T) {
	fake := &fakeBulkClient{
		postBody: json.RawMessage(`{"id":"750MOCK","state":"JobComplete"}`),
		gets: map[string]json.RawMessage{
			"/services/data/v63.0/jobs/query/750MOCK/results": json.RawMessage(`[{"Id":"00TBULK0001"}]`),
		},
	}
	var log bytes.Buffer
	result, err := RunBulkFallback(fake, "Task", "SELECT Id FROM Task", true, &log)
	if err != nil {
		t.Fatalf("run bulk fallback: %v", err)
	}
	if !strings.Contains(log.String(), BulkUnsafeWarning) {
		t.Fatalf("log = %q, want warning", log.String())
	}
	if result.Provenance["fls_unsafe"] != true || result.Provenance["warning"] != BulkUnsafeWarning {
		t.Fatalf("provenance = %#v, want unsafe warning", result.Provenance)
	}
	if len(result.Records) != 1 {
		t.Fatalf("records len = %d, want 1", len(result.Records))
	}
}
