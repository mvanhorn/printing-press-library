package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/client"
)

func TestIsSyncAccessWarningGoogleHealthAccountNotLinked(t *testing.T) {
	err := &client.APIError{
		Method:     "GET",
		Path:       "/v4/users/me/dataTypes/steps/dataPoints",
		StatusCode: 400,
		Body:       `{"error":{"status":"FAILED_PRECONDITION","details":[{"reason":"ACCOUNT_NOT_LINKED"}]}}`,
	}

	warn, ok := isSyncAccessWarning(err)
	if !ok {
		t.Fatalf("expected warning classification, got ok=false")
	}
	if warn == nil {
		t.Fatalf("expected non-nil warning")
	}
	if warn.Reason != "account_not_linked" {
		t.Fatalf("expected reason account_not_linked, got %q", warn.Reason)
	}
}

func TestIsSyncAccessWarningGoogleHealthUnsupportedDataTypeAction(t *testing.T) {
	err := &client.APIError{
		Method:     "GET",
		Path:       "/v4/users/me/dataTypes/floors/dataPoints",
		StatusCode: 400,
		Body:       `{"error":{"status":"INVALID_ARGUMENT","details":[{"reason":"UNSUPPORTED_DATA_TYPE_ACTION"}]}}`,
	}

	warn, ok := isSyncAccessWarning(err)
	if !ok {
		t.Fatalf("expected warning classification, got ok=false")
	}
	if warn == nil {
		t.Fatalf("expected non-nil warning")
	}
	if warn.Reason != "unsupported_data_type_action" {
		t.Fatalf("expected reason unsupported_data_type_action, got %q", warn.Reason)
	}
}
