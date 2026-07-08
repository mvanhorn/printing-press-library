package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
)

const BundleAuditPath = "/services/data/" + APIVersion + "/sobjects/SF360_Bundle_Audit__c/"

type BundleAuditClient interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

type BundleAuditRequest struct {
	KID             string
	GeneratedBy     string
	GeneratedAt     time.Time
	AccountID       string
	BundleJTI       string
	SourcesUsed     []string
	RedactionCounts map[string]int
	ClientHost      string
	TraceID         string
	HIPAAMode       bool
}

type BundleAuditOptions struct {
	Client  BundleAuditClient
	DBPath  string
	Sync    bool
	LogWarn func(format string, args ...any)
}

func RecordBundleAudit(ctx context.Context, req BundleAuditRequest, opts BundleAuditOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	req = normalizeBundleAuditRequest(req)
	if err := writeBundleAuditLocal(opts.DBPath, req, "pending", ""); err != nil {
		if opts.Sync || req.HIPAAMode {
			return fmt.Errorf("BUNDLE_AUDIT_LOCAL_WRITE_FAILED: %w", err)
		}
		warnBundleAudit(opts, "bundle audit local mirror write failed: %v", err)
	}

	if opts.Sync || req.HIPAAMode {
		return (&SyncWriter{Client: opts.Client}).Write(ctx, bundleAuditRow{req: req, dbPath: opts.DBPath})
	}

	return (&AsyncWriter{
		Client:     opts.Client,
		LogWarn:    opts.LogWarn,
		WarnFormat: "bundle audit write failed: %v",
	}).Write(ctx, bundleAuditRow{req: req, dbPath: opts.DBPath})
}

func normalizeBundleAuditRequest(req BundleAuditRequest) BundleAuditRequest {
	if req.GeneratedAt.IsZero() {
		req.GeneratedAt = time.Now().UTC()
	} else {
		req.GeneratedAt = req.GeneratedAt.UTC()
	}
	if req.AccountID == "" {
		req.AccountID = "unknown"
	}
	if req.BundleJTI == "" {
		req.BundleJTI = req.TraceID
	}
	if req.TraceID == "" {
		req.TraceID = req.BundleJTI
	}
	if req.RedactionCounts == nil {
		req.RedactionCounts = map[string]int{}
	}
	if req.SourcesUsed == nil {
		req.SourcesUsed = []string{}
	}
	if req.ClientHost == "" {
		req.ClientHost, _ = os.Hostname()
	}
	return req
}

func bundleAuditSObject(req BundleAuditRequest) (map[string]any, error) {
	sources, err := json.Marshal(req.SourcesUsed)
	if err != nil {
		return nil, err
	}
	redactions, err := json.Marshal(req.RedactionCounts)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"GeneratedBy__c":     req.GeneratedBy,
		"GeneratedAt__c":     req.GeneratedAt.Format(time.RFC3339),
		"AccountId__c":       req.AccountID,
		"BundleJti__c":       req.BundleJTI,
		"SourcesUsed__c":     string(sources),
		"RedactionCounts__c": string(redactions),
		"ClientHost__c":      req.ClientHost,
		"TraceId__c":         req.TraceID,
		"HipaaMode__c":       req.HIPAAMode,
	}, nil
}

type bundleAuditRow struct {
	req    BundleAuditRequest
	dbPath string
}

func (r bundleAuditRow) PostPath() string {
	return BundleAuditPath
}

func (r bundleAuditRow) PostBody() (any, error) {
	return bundleAuditSObject(r.req)
}

func (r bundleAuditRow) Mirror(status, remoteError string) error {
	return writeBundleAuditLocal(r.dbPath, r.req, status, remoteError)
}

func (r bundleAuditRow) FailureCode() string {
	return "BUNDLE_AUDIT_WRITE_FAILED"
}

func (r bundleAuditRow) LocalFailureCode() string {
	return "BUNDLE_AUDIT_LOCAL_WRITE_FAILED"
}

func writeBundleAuditLocal(dbPath string, req BundleAuditRequest, status, remoteError string) error {
	if dbPath == "" {
		return nil
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.RecordBundleAuditLocal(req.KID, req.BundleJTI, req.AccountID, req.GeneratedAt, status, remoteError)
}

func warnBundleAudit(opts BundleAuditOptions, format string, args ...any) {
	warnAudit(opts.LogWarn, format, args...)
}
