package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
)

const WriteAuditPath = "/services/data/" + APIVersion + "/sobjects/SF360_Write_Audit__c/"

type WriteAuditRow = store.WriteAuditRow
type WriteAuditFilter = store.WriteAuditFilter

type WriteAuditMirror interface {
	InsertWriteAudit(row store.WriteAuditRow) error
	UpdateWriteAuditStatus(jti, status, remoteError string) error
	UpdateWriteAuditExecution(jti, executionStatus, executionError, executedAt string) error
	GetWriteAudit(jti string) (store.WriteAuditRow, error)
	ListWriteAudit(filter store.WriteAuditFilter) ([]store.WriteAuditRow, error)
}

type WriteAuditOptions struct {
	Client     SFClient
	Store      WriteAuditMirror
	DBPath     string
	HIPAAMode  bool
	LogWarn    func(format string, args ...any)
	Now        func() time.Time
	ClientHost string
}

type WriteAuditWriter struct {
	common     AuditWriter
	store      WriteAuditMirror
	dbPath     string
	hipaa      bool
	logWarn    func(format string, args ...any)
	now        func() time.Time
	clientHost string
}

func NewWriteAuditWriter(opts WriteAuditOptions) *WriteAuditWriter {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	common := AuditWriter(&AsyncWriter{
		Client:     opts.Client,
		LogWarn:    opts.LogWarn,
		WarnFormat: "write audit write failed: %v",
	})
	if opts.HIPAAMode {
		common = &SyncWriter{Client: opts.Client}
	}
	return &WriteAuditWriter{
		common:     common,
		store:      opts.Store,
		dbPath:     opts.DBPath,
		hipaa:      opts.HIPAAMode,
		logWarn:    opts.LogWarn,
		now:        now,
		clientHost: opts.ClientHost,
	}
}

// Wait blocks until any pending async audit writes have finished.
// Safe to call on a HIPAA-mode (sync) writer; it is a no-op there.
// Tests and CLI shutdown paths should call this to avoid leaking
// goroutines or leaving SQLite mirror writes in flight.
func (w *WriteAuditWriter) Wait() {
	if a, ok := w.common.(*AsyncWriter); ok {
		a.Wait()
	}
}

func (w *WriteAuditWriter) WritePending(intent WriteIntentClaims, jws []byte, fieldDiff map[string]any) error {
	row, err := w.pendingRow(intent, jws, fieldDiff)
	if err != nil {
		return err
	}
	if err := w.withMirror(func(m WriteAuditMirror) error { return m.InsertWriteAudit(row) }); err != nil {
		if w.hipaa {
			return fmt.Errorf("WRITE_AUDIT_LOCAL_WRITE_FAILED: %w", err)
		}
		warnAudit(w.logWarn, "write audit local mirror write failed: %v", err)
	}
	return w.common.Write(context.Background(), writeAuditRemoteRow{writer: w, row: row})
}

func (w *WriteAuditWriter) UpdateExecuted(jti string, _ map[string]any) error {
	return w.withMirror(func(m WriteAuditMirror) error {
		return m.UpdateWriteAuditExecution(jti, "executed", "", w.now().UTC().Format(time.RFC3339))
	})
}

func (w *WriteAuditWriter) UpdateRejected(jti string, errCode string, errMsg string) error {
	executionError := errCode
	if errMsg != "" {
		executionError = errCode + ": " + errMsg
	}
	return w.withMirror(func(m WriteAuditMirror) error {
		return m.UpdateWriteAuditExecution(jti, "rejected", executionError, w.now().UTC().Format(time.RFC3339))
	})
}

func (w *WriteAuditWriter) UpdateConflict(jti string, expectedLMD, actualLMD time.Time) error {
	msg := fmt.Sprintf("CONFLICT_STALE_WRITE: expected LastModifiedDate %s, got %s",
		expectedLMD.UTC().Format(time.RFC3339),
		actualLMD.UTC().Format(time.RFC3339),
	)
	return w.withMirror(func(m WriteAuditMirror) error {
		return m.UpdateWriteAuditExecution(jti, "conflict", msg, w.now().UTC().Format(time.RFC3339))
	})
}

func (w *WriteAuditWriter) pendingRow(intent WriteIntentClaims, jws []byte, fieldDiff map[string]any) (store.WriteAuditRow, error) {
	diffJSON, err := json.Marshal(fieldDiff)
	if err != nil {
		return store.WriteAuditRow{}, fmt.Errorf("marshal field diff: %w", err)
	}
	clientHost := w.clientHost
	if clientHost == "" {
		clientHost, _ = os.Hostname()
	}
	traceID := intent.Jti
	if traceID == "" {
		traceID = GenerateTraceID()
	}
	return store.WriteAuditRow{
		JTI:             traceID,
		ActingUser:      intent.Sub,
		ActingKID:       mustExtractKID(jws),
		TargetSObject:   intent.SObject,
		TargetRecordID:  intent.RecordID,
		Operation:       intent.Operation,
		IntentJWS:       string(jws),
		IdempotencyKey:  intent.IdempotencyKey,
		FieldDiff:       string(diffJSON),
		ExecutionStatus: "pending",
		ClientHost:      clientHost,
		TraceID:         traceID,
		HIPAAMode:       w.hipaa,
		GeneratedAt:     w.now().UTC().Format(time.RFC3339),
		WriteStatus:     "pending",
	}, nil
}

func (w *WriteAuditWriter) withMirror(fn func(WriteAuditMirror) error) error {
	if w.store != nil {
		return fn(w.store)
	}
	if w.dbPath == "" {
		return nil
	}
	s, err := store.Open(w.dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

type writeAuditRemoteRow struct {
	writer *WriteAuditWriter
	row    store.WriteAuditRow
}

func (r writeAuditRemoteRow) PostPath() string {
	return WriteAuditPath
}

func (r writeAuditRemoteRow) PostBody() (any, error) {
	body := map[string]any{
		"ActingUser__c":      r.row.ActingUser,
		"ActingKid__c":       r.row.ActingKID,
		"TargetSObject__c":   r.row.TargetSObject,
		"TargetRecordId__c":  r.row.TargetRecordID,
		"Operation__c":       r.row.Operation,
		"IntentJws__c":       r.row.IntentJWS,
		"IdempotencyKey__c":  r.row.IdempotencyKey,
		"FieldDiff__c":       r.row.FieldDiff,
		"ExecutionStatus__c": r.row.ExecutionStatus,
		"ExecutionError__c":  r.row.ExecutionError,
		"ClientHost__c":      r.row.ClientHost,
		"TraceId__c":         r.row.TraceID,
		"HipaaMode__c":       r.row.HIPAAMode,
		"GeneratedAt__c":     r.row.GeneratedAt,
	}
	if r.row.ExecutedAt != "" {
		body["ExecutedAt__c"] = r.row.ExecutedAt
	}
	return body, nil
}

func (r writeAuditRemoteRow) Mirror(status, remoteError string) error {
	return r.writer.withMirror(func(m WriteAuditMirror) error {
		return m.UpdateWriteAuditStatus(r.row.JTI, status, remoteError)
	})
}

func (r writeAuditRemoteRow) FailureCode() string {
	return "WRITE_INTENT_AUDIT_FAILED"
}

func (r writeAuditRemoteRow) LocalFailureCode() string {
	return "WRITE_AUDIT_LOCAL_WRITE_FAILED"
}

func mustExtractKID(jws []byte) string {
	kid, err := ExtractKIDUnsafe(string(jws))
	if err != nil {
		return ""
	}
	return kid
}
