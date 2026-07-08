package client

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	WriteErrorConflictStaleWrite       = "CONFLICT_STALE_WRITE"
	WriteErrorIdempotencyKeyRequired   = "IDEMPOTENCY_KEY_REQUIRED"
	WriteErrorFLSWriteDenied           = "FLS_WRITE_DENIED"
	WriteErrorValidationRuleRejected   = "VALIDATION_RULE_REJECTED"
	WriteErrorApexCompanionRequired    = "APEX_COMPANION_REQUIRED"
	WriteErrorBulkConfirmationMismatch = "BULK_CONFIRMATION_MISMATCH"
	WriteErrorPlanSignatureInvalid     = "PLAN_SIGNATURE_INVALID"
	WriteErrorIntentAuditFailed        = "WRITE_INTENT_AUDIT_FAILED"
	WriteErrorRequiredFieldMissing     = "REQUIRED_FIELD_MISSING"
	WriteErrorIdempotencyKeyCollision  = "IDEMPOTENCY_KEY_COLLISION"
	WriteErrorSalesforceAPI            = "SALESFORCE_API_ERROR"
)

const writeErrorStage = "salesforce_write"

// WriteErrorEnvelope is the D9-shaped write error payload returned by the
// write-specific Salesforce translator.
type WriteErrorEnvelope struct {
	Code       string         `json:"code"`
	HTTPStatus int            `json:"http_status"`
	Stage      string         `json:"stage"`
	Org        string         `json:"org,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	Cause      any            `json:"cause,omitempty"`
	Hint       string         `json:"hint,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

type salesforceError struct {
	ErrorCode string   `json:"errorCode"`
	Message   string   `json:"message"`
	Fields    []string `json:"fields"`
}

// TranslateWriteError maps Salesforce write failures into the extended D9
// envelope used by write callers. The path is used for Apex-companion-specific
// 404s where the Salesforce response itself is otherwise a generic NOT_FOUND.
func TranslateWriteError(status int, path string, body []byte, org string, traceID string) WriteErrorEnvelope {
	errors := parseSalesforceErrors(body)
	bodyText := strings.TrimSpace(string(body))
	envelope := WriteErrorEnvelope{
		Code:       WriteErrorSalesforceAPI,
		HTTPStatus: status,
		Stage:      writeErrorStage,
		Org:        org,
		TraceID:    traceID,
		Cause: map[string]any{
			"raw_body": bodyText,
		},
	}

	if status == http.StatusNotFound && isApexCompanionPath(path) {
		envelope.Code = WriteErrorApexCompanionRequired
		envelope.Hint = "Run: trust install-apex --org <alias>"
		return envelope
	}

	if status == http.StatusConflict && looksLikeIfMatchConflict(bodyText, errors) {
		envelope.Code = WriteErrorConflictStaleWrite
		envelope.Hint = "Fetch the record again and retry with the new LastModifiedDate."
		return envelope
	}

	if status >= 500 && looksLikeIdempotencyCollision(bodyText) {
		envelope.Code = WriteErrorIdempotencyKeyCollision
		envelope.Hint = "Generate a new idempotency key for this write intent."
		return envelope
	}

	for _, sfErr := range errors {
		switch sfErr.ErrorCode {
		case "INVALID_FIELD_FOR_INSERT_UPDATE":
			if looksLikeFLSWriteDenied(sfErr) {
				envelope.Code = WriteErrorFLSWriteDenied
				envelope.Data = map[string]any{"field": firstField(sfErr.Fields)}
				envelope.Hint = "Remove the field or run as a user with write permission for it."
				return envelope
			}
		case "FIELD_CUSTOM_VALIDATION_EXCEPTION", "VALIDATION_RULE_VIOLATION":
			envelope.Code = WriteErrorValidationRuleRejected
			envelope.Data = map[string]any{"rule_message": sfErr.Message}
			if ruleName := validationRuleName(sfErr.Message); ruleName != "" {
				envelope.Data["rule_name"] = ruleName
			}
			return envelope
		case "REQUIRED_FIELD_MISSING":
			envelope.Code = WriteErrorRequiredFieldMissing
			envelope.Data = map[string]any{"fields": sfErr.Fields}
			return envelope
		}
	}

	if len(errors) > 0 {
		envelope.Cause = map[string]any{
			"errors":   errors,
			"raw_body": bodyText,
		}
	}

	return envelope
}

func IsWriteError(envelope WriteErrorEnvelope) bool {
	switch envelope.Code {
	case WriteErrorConflictStaleWrite,
		WriteErrorIdempotencyKeyRequired,
		WriteErrorFLSWriteDenied,
		WriteErrorValidationRuleRejected,
		WriteErrorApexCompanionRequired,
		WriteErrorBulkConfirmationMismatch,
		WriteErrorPlanSignatureInvalid,
		WriteErrorIntentAuditFailed,
		WriteErrorRequiredFieldMissing,
		WriteErrorIdempotencyKeyCollision,
		WriteErrorSalesforceAPI:
		return true
	default:
		return false
	}
}

func IsConflict(envelope WriteErrorEnvelope) bool {
	return envelope.Code == WriteErrorConflictStaleWrite
}

func parseSalesforceErrors(body []byte) []salesforceError {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return nil
	}

	var direct []salesforceError
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil
	}

	if rawErrors, ok := obj["errors"]; ok {
		if err := json.Unmarshal(rawErrors, &direct); err == nil {
			return direct
		}
	}

	var single salesforceError
	if err := json.Unmarshal(body, &single); err == nil && (single.ErrorCode != "" || single.Message != "") {
		return []salesforceError{single}
	}

	message := stringFromRaw(obj["message"])
	if message == "" {
		message = stringFromRaw(obj["error_description"])
	}
	code := stringFromRaw(obj["errorCode"])
	if code == "" {
		code = stringFromRaw(obj["error"])
	}
	if code != "" || message != "" {
		return []salesforceError{{ErrorCode: code, Message: message}}
	}

	return nil
}

func stringFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func isApexCompanionPath(path string) bool {
	return strings.Contains(path, "/services/apexrest/sf360/v1/safeWrite") ||
		strings.Contains(path, "/services/apexrest/sf360/v1/safeUpsert")
}

func looksLikeIfMatchConflict(bodyText string, errors []salesforceError) bool {
	text := strings.ToLower(bodyText)
	for _, sfErr := range errors {
		text += " " + strings.ToLower(sfErr.ErrorCode) + " " + strings.ToLower(sfErr.Message)
	}
	return strings.Contains(text, "if-match") ||
		strings.Contains(text, "precondition") ||
		strings.Contains(text, "entity tag") ||
		strings.Contains(text, "etag") ||
		strings.Contains(text, "stale")
}

func looksLikeIdempotencyCollision(bodyText string) bool {
	text := strings.ToLower(bodyText)
	return strings.Contains(text, "duplicate value") &&
		(strings.Contains(text, "externalid") ||
			strings.Contains(text, "external id") ||
			strings.Contains(text, "sf360_idempotency_key__c"))
}

func looksLikeFLSWriteDenied(sfErr salesforceError) bool {
	text := strings.ToLower(sfErr.Message)
	return len(sfErr.Fields) > 0 ||
		strings.Contains(text, "not updateable") ||
		strings.Contains(text, "not writeable") ||
		strings.Contains(text, "no access") ||
		strings.Contains(text, "insufficient access")
}

func firstField(fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func validationRuleName(message string) string {
	name, _, ok := strings.Cut(message, ":")
	if !ok {
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, " ") {
		return ""
	}
	return name
}
