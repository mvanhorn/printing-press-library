package agent

import (
	"fmt"
	"net/http"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
)

const bulkOperationsDoc = "docs/plans/2026-04-22-004-feat-salesforce-360-writes-plan.md#unit-8"

func CheckBulk(recordCount int, confirmBulk int) error {
	if recordCount <= 0 {
		recordCount = 1
	}
	if recordCount == 1 {
		return nil
	}
	if confirmBulk == 0 {
		return bulkGateError(
			"BULK_OPERATIONS_DEFERRED",
			http.StatusBadRequest,
			fmt.Sprintf("bulk write of %d records is deferred to v1.2", recordCount),
			fmt.Sprintf("v1.1 only permits one record per command. See %s for the v1.2 bulk-write path.", bulkOperationsDoc),
			recordCount,
			confirmBulk,
		)
	}
	if confirmBulk != recordCount {
		return bulkGateError(
			"BULK_CONFIRMATION_MISMATCH",
			http.StatusBadRequest,
			fmt.Sprintf("--confirm-bulk=%d does not match actual record count %d", confirmBulk, recordCount),
			fmt.Sprintf("Re-run with --confirm-bulk %d only after confirming the bulk write is intentional.", recordCount),
			recordCount,
			confirmBulk,
		)
	}
	return nil
}

func CountsAgree(requestedIDs []string) int {
	return len(requestedIDs)
}

func normalizedRecordCount(recordCount int) int {
	if recordCount <= 0 {
		return 1
	}
	return recordCount
}

func bulkGateError(code string, status int, message string, hint string, recordCount int, confirmBulk int) error {
	return &WriteError{
		Message: message,
		Envelope: client.WriteErrorEnvelope{
			Code:       code,
			HTTPStatus: status,
			Stage:      "bulk_gate",
			Hint:       hint,
			Data: map[string]any{
				"record_count": recordCount,
				"confirm_bulk": confirmBulk,
				"doc":          bulkOperationsDoc,
			},
		},
	}
}
