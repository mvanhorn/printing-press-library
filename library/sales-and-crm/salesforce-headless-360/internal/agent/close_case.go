package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
)

type CloseCaseOptions struct {
	CaseID     string
	Resolution string
	Status     string
}

func NewCloseCaseWriteOptions(opts CloseCaseOptions) (WriteOptions, error) {
	opts.CaseID = strings.TrimSpace(opts.CaseID)
	opts.Resolution = strings.TrimSpace(opts.Resolution)
	opts.Status = strings.TrimSpace(opts.Status)
	if opts.CaseID == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --case is required")
	}
	if opts.Resolution == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --resolution is required")
	}
	if opts.Status == "" {
		opts.Status = "Closed"
	}
	return NewUpdateWriteOptions(opts.CaseID, map[string]any{
		"Status":        opts.Status,
		"Resolution__c": opts.Resolution,
	}), nil
}

func ExecuteCloseCase(ctx context.Context, opts WriteOptions) (*WriteResult, error) {
	opts = normalizeWriteOptions(opts)
	if opts.Client == nil {
		return nil, fmt.Errorf("write client is required")
	}
	if err := CheckBulk(opts.RecordCount, opts.ConfirmBulk); err != nil {
		return nil, err
	}
	before, _, err := fetchCaseStatus(ctx, opts)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(stringField(before, "Status"), "Closed") {
		if opts.LogWarn != nil {
			opts.LogWarn("ALREADY_CLOSED: case %s is already Closed; no DML was sent", opts.RecordID)
		}
		return &WriteResult{
			Operation:  WriteOperationUpdate,
			SObject:    "Case",
			RecordID:   opts.RecordID,
			AfterState: before,
			DryRun:     opts.DryRun,
			NoChange:   true,
			HTTPStatus: http.StatusOK,
		}, nil
	}
	return ExecuteWrite(ctx, opts)
}

func fetchCaseStatus(ctx context.Context, opts WriteOptions) (map[string]any, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	raw, _, err := opts.Client.GetWithResponseHeaders(
		"/services/data/"+security.APIVersion+"/ui-api/records/"+opts.RecordID,
		map[string]string{"fields": "Case.Status,Case.LastModifiedDate"},
	)
	if err != nil {
		return nil, "", err
	}
	before, lmd, err := parseUIRecord(raw)
	if err != nil {
		return nil, "", err
	}
	return before, formatOptionalTime(lmd), nil
}
