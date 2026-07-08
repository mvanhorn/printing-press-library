package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

const (
	WriteOperationUpdate = "update"
	WriteOperationUpsert = "upsert"
	WriteOperationCreate = "create"

	AuthMethodJWT = "jwt"
)

const apexSafeWritePath = "/services/apexrest/sf360/v1/safeWrite"

type WriteOptions struct {
	Operation      string
	SObject        string
	RecordID       string
	Fields         map[string]any
	IdempotencyKey string
	IfLastModified time.Time
	ForceStale     bool
	DryRun         bool
	RunAsUser      string
	ConfirmBulk    int
	RecordCount    int

	Client                 WriteClient
	Filter                 security.WriteFilter
	Signer                 WriteIntentSigner
	AuditWriter            WriteAuditWriter
	AuthMethod             string
	ApexCompanionInstalled bool
	OrgAlias               string
	OrgID                  string
	ActingUser             string
	Now                    func() time.Time
	LogWarn                func(format string, args ...any)
	PlanExpiresIn          time.Duration
	JTI                    string
	AuditMetadata          map[string]any
}

type WriteResult struct {
	JTI             string                    `json:"jti"`
	Operation       string                    `json:"operation"`
	SObject         string                    `json:"sobject"`
	RecordID        string                    `json:"record_id,omitempty"`
	Diff            map[string]map[string]any `json:"diff"`
	AfterState      map[string]any            `json:"after_state,omitempty"`
	DryRun          bool                      `json:"dry_run"`
	ApexUsed        bool                      `json:"apex_used"`
	FilterDropped   []string                  `json:"filter_dropped,omitempty"`
	NoChange        bool                      `json:"no_change,omitempty"`
	HTTPStatus      int                       `json:"http_status,omitempty"`
	IdempotencyKey  string                    `json:"idempotency_key,omitempty"`
	WritePath       string                    `json:"write_path,omitempty"`
	Intent          trust.WriteIntentClaims   `json:"intent,omitempty"`
	StaleAccepted   bool                      `json:"stale_write_accepted,omitempty"`
	LastModifiedUTC string                    `json:"last_modified,omitempty"`
}

type WriteClient interface {
	GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error)
	Post(path string, body any) (json.RawMessage, int, error)
	PatchWithResponseHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, http.Header, error)
}

type WriteIntentSigner interface {
	SignWriteIntent(claims trust.WriteIntentClaims) ([]byte, error)
}

type WriteAuditWriter interface {
	WritePending(intent trust.WriteIntentClaims, jws []byte, fieldDiff map[string]any) error
	UpdateExecuted(jti string, afterState map[string]any) error
	UpdateRejected(jti string, errCode string, errMsg string) error
	UpdateConflict(jti string, expectedLMD, actualLMD time.Time) error
}

type WriteError struct {
	Envelope client.WriteErrorEnvelope
	Message  string
}

func (e *WriteError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Envelope.Code
}

func ExecuteWrite(ctx context.Context, opts WriteOptions) (*WriteResult, error) {
	opts = normalizeWriteOptions(opts)
	if err := validateWriteOptions(opts); err != nil {
		return nil, err
	}
	if opts.AuthMethod == AuthMethodJWT {
		if opts.RunAsUser == "" {
			return nil, newWriteError("JWT_REQUIRES_RUN_AS_USER", http.StatusBadRequest, opts, "JWT auth mode requires --run-as-user <Id> to enforce FLS", "")
		}
		if !opts.ApexCompanionInstalled {
			return nil, newWriteError(client.WriteErrorApexCompanionRequired, http.StatusPreconditionRequired, opts, "Apex companion is required for JWT writes", "Run: trust install-apex")
		}
	}
	if err := CheckBulk(opts.RecordCount, opts.ConfirmBulk); err != nil {
		return nil, err
	}

	before := map[string]any{}
	var beforeLMD time.Time
	var err error
	if opts.Operation == WriteOperationUpdate {
		before, beforeLMD, err = fetchBeforeState(ctx, opts)
		if err != nil {
			return nil, err
		}
		if opts.IfLastModified.IsZero() {
			opts.IfLastModified = beforeLMD
		}
	}

	filtered, dropped := filterWritableFields(opts)
	if len(filtered) == 0 {
		return nil, newWriteError(client.WriteErrorFLSWriteDenied, http.StatusForbidden, opts, "no requested fields are writeable for this user", "")
	}
	if opts.Operation == WriteOperationCreate && opts.IdempotencyKey != "" {
		filtered["SF360_Idempotency_Key__c"] = opts.IdempotencyKey
	}

	diffBefore := before
	if opts.Operation != WriteOperationUpdate {
		diffBefore = map[string]any{}
	}
	diff := ComputeFieldDiff(diffBefore, filtered)
	jti := opts.JTI
	if jti == "" {
		jti = trust.GenerateTraceID()
	}
	intent, jws, err := signWriteIntent(opts, jti, diff)
	if err != nil {
		return nil, err
	}
	apexUsed := opts.AuthMethod == AuthMethodJWT
	writePath := selectedWritePath(opts, apexUsed)
	result := &WriteResult{
		JTI:             jti,
		Operation:       opts.Operation,
		SObject:         opts.SObject,
		RecordID:        opts.RecordID,
		Diff:            diff,
		DryRun:          opts.DryRun,
		ApexUsed:        apexUsed,
		FilterDropped:   dropped,
		HTTPStatus:      http.StatusOK,
		IdempotencyKey:  opts.IdempotencyKey,
		WritePath:       writePath,
		Intent:          intent,
		StaleAccepted:   opts.ForceStale,
		LastModifiedUTC: formatOptionalTime(opts.IfLastModified),
	}
	if opts.DryRun {
		result.AfterState = mergeAfterState(before, filtered, nil)
		return result, nil
	}

	if opts.AuditWriter != nil {
		if err := opts.AuditWriter.WritePending(intent, jws, withAuditMetadata(diffAsAny(diff), opts.AuditMetadata)); err != nil {
			return nil, err
		}
	}

	after, status, headers, err := executeDML(opts, filtered, apexUsed)
	if err != nil {
		envelope := translateWriteExecutionError(err, status, writePath, opts, jti)
		if opts.AuditWriter != nil {
			if client.IsConflict(envelope) {
				_ = opts.AuditWriter.UpdateConflict(jti, opts.IfLastModified, time.Time{})
			} else {
				_ = opts.AuditWriter.UpdateRejected(jti, envelope.Code, envelopeMessage(envelope))
			}
		}
		return nil, &WriteError{Envelope: envelope}
	}

	result.AfterState = mergeAfterState(before, filtered, after)
	if id := stringField(after, "id", "Id"); id != "" {
		result.RecordID = id
	}
	if result.RecordID == "" {
		result.RecordID = opts.RecordID
	}
	if status == http.StatusNoContent {
		result.NoChange = true
		result.AfterState = map[string]any{}
	}
	result.HTTPStatus = status
	if lmd := lastModifiedFrom(headers, after); !lmd.IsZero() {
		result.LastModifiedUTC = lmd.UTC().Format(time.RFC3339)
	}
	if opts.AuditWriter != nil {
		if err := opts.AuditWriter.UpdateExecuted(jti, result.AfterState); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func ComputeFieldDiff(before, requested map[string]any) map[string]map[string]any {
	diff := map[string]map[string]any{}
	for _, key := range sortedKeys(requested) {
		after := requested[key]
		prior := before[key]
		if !reflect.DeepEqual(prior, after) {
			diff[key] = map[string]any{"before": prior, "after": after}
		}
	}
	return diff
}

func ParseFieldAssignments(assignments []string) (map[string]any, error) {
	fields := map[string]any{}
	for _, assignment := range assignments {
		name, raw, ok := strings.Cut(assignment, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("field must be NAME=VALUE: %q", assignment)
		}
		fields[name] = parseFieldValue(raw)
	}
	return fields, nil
}

func normalizeWriteOptions(opts WriteOptions) WriteOptions {
	opts.Operation = strings.ToLower(strings.TrimSpace(opts.Operation))
	opts.SObject = strings.TrimSpace(opts.SObject)
	opts.RecordID = strings.TrimSpace(opts.RecordID)
	opts.IdempotencyKey = strings.TrimSpace(opts.IdempotencyKey)
	opts.RunAsUser = strings.TrimSpace(opts.RunAsUser)
	opts.AuthMethod = strings.ToLower(strings.TrimSpace(opts.AuthMethod))
	opts.RecordCount = normalizedRecordCount(opts.RecordCount)
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.ActingUser == "" {
		opts.ActingUser = opts.RunAsUser
	}
	if opts.SObject == "" && opts.RecordID != "" {
		opts.SObject = InferSObjectFromID(opts.RecordID)
	}
	return opts
}

func validateWriteOptions(opts WriteOptions) error {
	if opts.Operation != WriteOperationUpdate && opts.Operation != WriteOperationUpsert && opts.Operation != WriteOperationCreate {
		return fmt.Errorf("unsupported write operation %q", opts.Operation)
	}
	if opts.Client == nil {
		return fmt.Errorf("write client is required")
	}
	if opts.Signer == nil {
		return fmt.Errorf("write intent signer is required")
	}
	if opts.SObject == "" {
		return fmt.Errorf("--sobject is required")
	}
	if opts.Operation == WriteOperationUpdate && opts.RecordID == "" {
		return fmt.Errorf("record id is required for update")
	}
	if (opts.Operation == WriteOperationCreate || opts.Operation == WriteOperationUpsert) && opts.IdempotencyKey == "" {
		return newWriteError(client.WriteErrorIdempotencyKeyRequired, http.StatusBadRequest, opts, "--idempotency-key is required for create/upsert", "")
	}
	if len(opts.Fields) == 0 {
		return fmt.Errorf("at least one --field NAME=VALUE is required")
	}
	return nil
}

func fetchBeforeState(ctx context.Context, opts WriteOptions) (map[string]any, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	params := map[string]string{"fields": qualifiedFieldList(opts.SObject, opts.Fields)}
	body, _, err := opts.Client.GetWithResponseHeaders("/services/data/"+security.APIVersion+"/ui-api/records/"+opts.RecordID, params)
	if err != nil {
		return nil, time.Time{}, err
	}
	return parseUIRecord(body)
}

func parseUIRecord(body json.RawMessage) (map[string]any, time.Time, error) {
	body = unwrapWriteEnvelope(body)
	var payload struct {
		Fields map[string]struct {
			Value any `json:"value"`
		} `json:"fields"`
		LastModifiedDate string `json:"lastModifiedDate"`
		ID               string `json:"id"`
		APIName          string `json:"apiName"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, time.Time{}, err
	}
	out := map[string]any{}
	for key, field := range payload.Fields {
		out[key] = field.Value
	}
	if payload.ID != "" {
		out["Id"] = payload.ID
	}
	lmd, _ := time.Parse(time.RFC3339, payload.LastModifiedDate)
	return out, lmd, nil
}

func filterWritableFields(opts WriteOptions) (map[string]any, []string) {
	filtered := map[string]any{}
	var dropped []string
	for _, key := range sortedKeys(opts.Fields) {
		if opts.Filter != nil && !opts.Filter.AllowFieldWrite(opts.RunAsUser, opts.SObject, key) {
			dropped = append(dropped, key)
			if opts.LogWarn != nil {
				opts.LogWarn("dropping field %s.%s: write permission denied", opts.SObject, key)
			}
			continue
		}
		filtered[key] = opts.Fields[key]
	}
	return filtered, dropped
}

func signWriteIntent(opts WriteOptions, jti string, diff map[string]map[string]any) (trust.WriteIntentClaims, []byte, error) {
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		return trust.WriteIntentClaims{}, nil, fmt.Errorf("marshal field diff: %w", err)
	}
	sum := sha256.Sum256(diffJSON)
	now := opts.Now().UTC()
	intent := trust.WriteIntentClaims{
		Iss:            firstNonEmptyString(opts.OrgAlias, opts.OrgID, "salesforce-headless-360"),
		Sub:            firstNonEmptyString(opts.ActingUser, opts.RunAsUser, "unknown-user"),
		Aud:            trust.WriteIntentAudience,
		Iat:            now.Unix(),
		Exp:            now.Add(10 * time.Minute).Unix(),
		Jti:            jti,
		SObject:        opts.SObject,
		RecordID:       opts.RecordID,
		Operation:      opts.Operation,
		DiffSha256:     hex.EncodeToString(sum[:]),
		IdempotencyKey: opts.IdempotencyKey,
		IfLastModified: formatOptionalTime(opts.IfLastModified),
	}
	jws, err := opts.Signer.SignWriteIntent(intent)
	if err != nil {
		return trust.WriteIntentClaims{}, nil, err
	}
	return intent, jws, nil
}

func executeDML(opts WriteOptions, fields map[string]any, apexUsed bool) (map[string]any, int, http.Header, error) {
	if apexUsed {
		body := map[string]any{
			"operation":       opts.Operation,
			"sobject":         opts.SObject,
			"record_id":       opts.RecordID,
			"fields":          fields,
			"idempotency_key": opts.IdempotencyKey,
			"run_as_user":     opts.RunAsUser,
			"force_stale":     opts.ForceStale,
		}
		if !opts.IfLastModified.IsZero() {
			body["if_last_modified"] = opts.IfLastModified.UTC().Format(time.RFC3339)
		}
		raw, status, err := opts.Client.Post(apexSafeWritePath, body)
		return parseWriteResponse(raw), status, nil, err
	}

	switch opts.Operation {
	case WriteOperationUpdate:
		headers := map[string]string{}
		if !opts.ForceStale && !opts.IfLastModified.IsZero() {
			headers["If-Match"] = opts.IfLastModified.UTC().Format(time.RFC3339)
		}
		raw, status, responseHeaders, err := opts.Client.PatchWithResponseHeaders(
			"/services/data/"+security.APIVersion+"/ui-api/records/"+opts.RecordID,
			map[string]any{"fields": fields},
			headers,
		)
		return parseWriteResponse(raw), status, responseHeaders, err
	case WriteOperationUpsert:
		raw, status, responseHeaders, err := opts.Client.PatchWithResponseHeaders(
			"/services/data/"+security.APIVersion+"/sobjects/"+url.PathEscape(opts.SObject)+"/SF360_Idempotency_Key__c/"+url.PathEscape(opts.IdempotencyKey),
			fields,
			nil,
		)
		return parseWriteResponse(raw), status, responseHeaders, err
	case WriteOperationCreate:
		raw, status, err := opts.Client.Post(
			"/services/data/"+security.APIVersion+"/sobjects/"+url.PathEscape(opts.SObject),
			fields,
		)
		return parseWriteResponse(raw), status, nil, err
	default:
		return nil, 0, nil, fmt.Errorf("unsupported operation %q", opts.Operation)
	}
}

func parseWriteResponse(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	raw = unwrapWriteEnvelope(raw)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func translateWriteExecutionError(err error, status int, path string, opts WriteOptions, traceID string) client.WriteErrorEnvelope {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return client.TranslateWriteError(apiErr.StatusCode, apiErr.Path, []byte(apiErr.Body), opts.OrgID, traceID)
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}
	return client.WriteErrorEnvelope{
		Code:       client.WriteErrorSalesforceAPI,
		HTTPStatus: status,
		Stage:      "salesforce_write",
		Org:        opts.OrgID,
		TraceID:    traceID,
		Cause:      map[string]any{"error": err.Error(), "path": path},
	}
}

func newWriteError(code string, status int, opts WriteOptions, message string, hint string) error {
	return &WriteError{
		Message: message,
		Envelope: client.WriteErrorEnvelope{
			Code:       code,
			HTTPStatus: status,
			Stage:      "salesforce_write",
			Org:        opts.OrgID,
			Hint:       hint,
		},
	}
}

func selectedWritePath(opts WriteOptions, apexUsed bool) string {
	if apexUsed {
		return apexSafeWritePath
	}
	switch opts.Operation {
	case WriteOperationUpdate:
		return "/services/data/" + security.APIVersion + "/ui-api/records/" + opts.RecordID
	case WriteOperationUpsert:
		return "/services/data/" + security.APIVersion + "/sobjects/" + opts.SObject + "/SF360_Idempotency_Key__c/" + opts.IdempotencyKey
	case WriteOperationCreate:
		return "/services/data/" + security.APIVersion + "/sobjects/" + opts.SObject
	default:
		return ""
	}
}

func qualifiedFieldList(sobject string, fields map[string]any) string {
	values := make([]string, 0, len(fields)+1)
	for key := range fields {
		values = append(values, sobject+"."+key)
	}
	values = append(values, sobject+".LastModifiedDate")
	sort.Strings(values)
	return strings.Join(values, ",")
}

func InferSObjectFromID(id string) string {
	switch {
	case strings.HasPrefix(id, "001"):
		return "Account"
	case strings.HasPrefix(id, "003"):
		return "Contact"
	case strings.HasPrefix(id, "006"):
		return "Opportunity"
	case strings.HasPrefix(id, "500"):
		return "Case"
	case strings.HasPrefix(id, "00T"):
		return "Task"
	case strings.HasPrefix(id, "00U"):
		return "Event"
	default:
		return ""
	}
}

var jsonNumberRE = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func parseFieldValue(raw string) any {
	value := strings.TrimSpace(raw)
	switch value {
	case "null":
		return nil
	case "true":
		return true
	case "false":
		return false
	}
	if jsonNumberRE.MatchString(value) {
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err == nil {
			return decoded
		}
	}
	return raw
}

func unwrapWriteEnvelope(body json.RawMessage) json.RawMessage {
	var wrapper struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if json.Unmarshal(body, &wrapper) == nil && len(wrapper.Envelope) > 0 {
		return wrapper.Envelope
	}
	return body
}

func diffAsAny(diff map[string]map[string]any) map[string]any {
	out := make(map[string]any, len(diff))
	for key, value := range diff {
		out[key] = value
	}
	return out
}

func withAuditMetadata(diff map[string]any, metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return diff
	}
	out := make(map[string]any, len(diff)+len(metadata))
	for key, value := range diff {
		out[key] = value
	}
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func mergeAfterState(before, fields, response map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range before {
		out[key] = value
	}
	for key, value := range fields {
		out[key] = value
	}
	for key, value := range response {
		out[key] = value
	}
	return out
}

func lastModifiedFrom(headers http.Header, after map[string]any) time.Time {
	if headers != nil {
		for _, key := range []string{"LastModifiedDate", "Lastmodifieddate"} {
			if raw := headers.Get(key); raw != "" {
				if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
					return parsed
				}
			}
		}
	}
	if raw := stringField(after, "lastModifiedDate", "LastModifiedDate"); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func stringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key].(string); ok {
			return value
		}
	}
	return ""
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func envelopeMessage(envelope client.WriteErrorEnvelope) string {
	if message, ok := envelope.Data["rule_message"].(string); ok {
		return message
	}
	if cause, ok := envelope.Cause.(map[string]any); ok {
		if raw, ok := cause["raw_body"].(string); ok {
			return raw
		}
	}
	return envelope.Code
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
