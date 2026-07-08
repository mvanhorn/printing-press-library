package agent

import (
	"errors"
	"testing"
)

func TestCheckBulkAllowsSingleRecordWithoutConfirmation(t *testing.T) {
	if err := CheckBulk(1, 0); err != nil {
		t.Fatalf("CheckBulk returned error: %v", err)
	}
}

func TestCheckBulkDefersUnconfirmedBulk(t *testing.T) {
	err := CheckBulk(5, 0)
	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("CheckBulk error = %T %v, want WriteError", err, err)
	}
	if writeErr.Envelope.Code != "BULK_OPERATIONS_DEFERRED" {
		t.Fatalf("code = %s, want BULK_OPERATIONS_DEFERRED", writeErr.Envelope.Code)
	}
}

func TestCheckBulkRejectsMismatchedConfirmation(t *testing.T) {
	err := CheckBulk(5, 3)
	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("CheckBulk error = %T %v, want WriteError", err, err)
	}
	if writeErr.Envelope.Code != "BULK_CONFIRMATION_MISMATCH" {
		t.Fatalf("code = %s, want BULK_CONFIRMATION_MISMATCH", writeErr.Envelope.Code)
	}
}

func TestCheckBulkAllowsMatchingConfirmation(t *testing.T) {
	if err := CheckBulk(5, 5); err != nil {
		t.Fatalf("CheckBulk returned error: %v", err)
	}
}

func TestCountsAgreeReturnsRequestedIDCount(t *testing.T) {
	got := CountsAgree([]string{"001A", "001B", "001C"})
	if got != 3 {
		t.Fatalf("CountsAgree = %d, want 3", got)
	}
}
