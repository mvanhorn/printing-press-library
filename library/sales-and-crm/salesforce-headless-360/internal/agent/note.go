package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func NewNoteWriteOptions(entityID, text string) WriteOptions {
	return WriteOptions{
		Operation: WriteOperationCreate,
		SObject:   "FeedItem",
		RecordID:  strings.TrimSpace(entityID),
		Fields:    map[string]any{"Body": strings.TrimSpace(text)},
	}
}

func ExecuteNote(ctx context.Context, opts WriteOptions) (*WriteResult, error) {
	opts = normalizeWriteOptions(opts)
	if err := validateNoteWriteOptions(opts); err != nil {
		return nil, err
	}
	if err := CheckBulk(opts.RecordCount, opts.ConfirmBulk); err != nil {
		return nil, err
	}
	diff := ComputeFieldDiff(map[string]any{}, opts.Fields)
	jti := opts.JTI
	if jti == "" {
		jti = trust.GenerateTraceID()
	}
	intent, jws, err := signWriteIntent(opts, jti, diff)
	if err != nil {
		return nil, err
	}
	result := &WriteResult{
		JTI:        jti,
		Operation:  WriteOperationCreate,
		SObject:    "FeedItem",
		RecordID:   opts.RecordID,
		Diff:       diff,
		DryRun:     opts.DryRun,
		ApexUsed:   false,
		HTTPStatus: http.StatusOK,
		WritePath:  noteWritePath(opts.RecordID),
		Intent:     intent,
	}
	if opts.DryRun {
		result.AfterState = map[string]any{
			"subjectId": opts.RecordID,
			"body":      opts.Fields["Body"],
		}
		return result, nil
	}
	if opts.AuditWriter != nil {
		if err := opts.AuditWriter.WritePending(intent, jws, withAuditMetadata(diffAsAny(diff), opts.AuditMetadata)); err != nil {
			return nil, err
		}
	}
	after, status, err := postChatterNote(opts)
	if err != nil {
		envelope := translateWriteExecutionError(err, status, result.WritePath, opts, jti)
		if opts.AuditWriter != nil {
			_ = opts.AuditWriter.UpdateRejected(jti, envelope.Code, envelopeMessage(envelope))
		}
		return nil, &WriteError{Envelope: envelope}
	}
	result.AfterState = after
	result.HTTPStatus = status
	if id := stringField(after, "id", "Id"); id != "" {
		result.RecordID = id
	}
	if opts.AuditWriter != nil {
		if err := opts.AuditWriter.UpdateExecuted(jti, result.AfterState); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func validateNoteWriteOptions(opts WriteOptions) error {
	if opts.Client == nil {
		return fmt.Errorf("write client is required")
	}
	if opts.Signer == nil {
		return fmt.Errorf("write intent signer is required")
	}
	if opts.RecordID == "" {
		return fmt.Errorf("MISSING_REQUIRED_FLAG: --entity is required")
	}
	text, _ := opts.Fields["Body"].(string)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("EMPTY_NOTE: --text must not be empty")
	}
	return nil
}

func postChatterNote(opts WriteOptions) (map[string]any, int, error) {
	text, _ := opts.Fields["Body"].(string)
	body := map[string]any{
		"body": map[string]any{
			"messageSegments": []map[string]any{{
				"type": "Text",
				"text": text,
			}},
		},
		"feedElementType": "FeedItem",
		"subjectId":       opts.RecordID,
	}
	raw, status, err := opts.Client.Post(noteWritePath(opts.RecordID), body)
	return parseWriteResponse(raw), status, err
}

func noteWritePath(entityID string) string {
	return "/services/data/" + security.APIVersion + "/chatter/feeds/record/" + url.PathEscape(entityID) + "/feed-elements"
}
