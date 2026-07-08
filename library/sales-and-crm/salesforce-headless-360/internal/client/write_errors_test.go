package client

import "testing"

func TestTranslateWriteErrorConflict(t *testing.T) {
	envelope := TranslateWriteError(409, "/services/data/v63.0/sobjects/Account/001ACME0001", []byte(`[
		{"errorCode":"PRECONDITION_FAILED","message":"If-Match precondition failed; stale LastModifiedDate."}
	]`), "dev", "trace-1")

	if envelope.Code != WriteErrorConflictStaleWrite {
		t.Fatalf("code = %q, want %q", envelope.Code, WriteErrorConflictStaleWrite)
	}
	if !IsWriteError(envelope) || !IsConflict(envelope) {
		t.Fatalf("helpers did not identify conflict envelope: %+v", envelope)
	}
	if envelope.Org != "dev" || envelope.TraceID != "trace-1" {
		t.Fatalf("org/trace = %q/%q, want dev/trace-1", envelope.Org, envelope.TraceID)
	}
}

func TestTranslateWriteErrorFLSWriteDenied(t *testing.T) {
	envelope := TranslateWriteError(400, "/services/apexrest/sf360/v1/safeWrite", []byte(`[
		{"errorCode":"INVALID_FIELD_FOR_INSERT_UPDATE","message":"Unable to create/update fields: Salary__c. Please check the security settings of this field.","fields":["Salary__c"]}
	]`), "", "")

	if envelope.Code != WriteErrorFLSWriteDenied {
		t.Fatalf("code = %q, want %q", envelope.Code, WriteErrorFLSWriteDenied)
	}
	if envelope.Data["field"] != "Salary__c" {
		t.Fatalf("field = %#v, want Salary__c", envelope.Data["field"])
	}
}

func TestTranslateWriteErrorValidationRule(t *testing.T) {
	envelope := TranslateWriteError(400, "/services/data/v63.0/sobjects/Task", []byte(`[
		{"errorCode":"VALIDATION_RULE_VIOLATION","message":"SF360_CloseDate_Current: Close date must not be in the past.","fields":["ActivityDate"]}
	]`), "", "")

	if envelope.Code != WriteErrorValidationRuleRejected {
		t.Fatalf("code = %q, want %q", envelope.Code, WriteErrorValidationRuleRejected)
	}
	if envelope.Data["rule_name"] != "SF360_CloseDate_Current" {
		t.Fatalf("rule_name = %#v, want SF360_CloseDate_Current", envelope.Data["rule_name"])
	}
	if envelope.Data["rule_message"] == "" {
		t.Fatalf("rule_message missing: %+v", envelope.Data)
	}
}

func TestTranslateWriteErrorRequiredFieldMissing(t *testing.T) {
	envelope := TranslateWriteError(400, "/services/data/v63.0/sobjects/Task", []byte(`[
		{"errorCode":"REQUIRED_FIELD_MISSING","message":"Required fields are missing: [Subject]","fields":["Subject"]}
	]`), "", "")

	if envelope.Code != WriteErrorRequiredFieldMissing {
		t.Fatalf("code = %q, want %q", envelope.Code, WriteErrorRequiredFieldMissing)
	}
	fields, ok := envelope.Data["fields"].([]string)
	if !ok || len(fields) != 1 || fields[0] != "Subject" {
		t.Fatalf("fields = %#v, want [Subject]", envelope.Data["fields"])
	}
}

func TestTranslateWriteErrorApexCompanionRequired(t *testing.T) {
	envelope := TranslateWriteError(404, "/services/apexrest/sf360/v1/safeWrite", []byte(`[
		{"errorCode":"NOT_FOUND","message":"Could not find Apex REST class."}
	]`), "", "")

	if envelope.Code != WriteErrorApexCompanionRequired {
		t.Fatalf("code = %q, want %q", envelope.Code, WriteErrorApexCompanionRequired)
	}
	if envelope.Hint != "Run: trust install-apex --org <alias>" {
		t.Fatalf("hint = %q", envelope.Hint)
	}
}

func TestTranslateWriteErrorIdempotencyCollision(t *testing.T) {
	envelope := TranslateWriteError(500, "/services/data/v63.0/sobjects/Task/SF360_Idempotency_Key__c/abc123", []byte(`[
		{"errorCode":"DUPLICATE_VALUE","message":"duplicate value found: SF360_Idempotency_Key__c duplicates value on record with id: 00TACME0001 (ExternalId)"}
	]`), "", "")

	if envelope.Code != WriteErrorIdempotencyKeyCollision {
		t.Fatalf("code = %q, want %q", envelope.Code, WriteErrorIdempotencyKeyCollision)
	}
}

func TestTranslateWriteErrorFallback(t *testing.T) {
	envelope := TranslateWriteError(500, "/services/data/v63.0/sobjects/Task", nil, "", "")

	if envelope.Code != WriteErrorSalesforceAPI {
		t.Fatalf("code = %q, want %q", envelope.Code, WriteErrorSalesforceAPI)
	}
	if !IsWriteError(envelope) || IsConflict(envelope) {
		t.Fatalf("helpers misclassified fallback envelope: %+v", envelope)
	}
}
