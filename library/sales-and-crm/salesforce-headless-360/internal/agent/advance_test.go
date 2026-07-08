package agent

import (
	"encoding/json"
	"testing"
)

func TestAdvanceUpdatesStageAndCloseDate(t *testing.T) {
	opts, err := NewAdvanceWriteOptions(contextWithTestDeadline(t), AdvanceOptions{
		OpportunityID: "006ACME0001",
		StageName:     "Closed Won",
		CloseDate:     "2026-05-01",
	})
	if err != nil {
		t.Fatalf("NewAdvanceWriteOptions error: %v", err)
	}
	fake := &highLevelWriteClient{
		getBody: json.RawMessage(`{"fields":{"StageName":{"value":"Negotiation/Review"},"CloseDate":{"value":"2026-06-01"}},"id":"006ACME0001","lastModifiedDate":"2026-04-18T14:00:00Z"}`),
	}
	audit := &capturingWriteAudit{}
	opts.Client = fake
	opts.Filter = allowAllWriteFilter{}
	opts.Signer = noopWriteSigner{}
	opts.AuditWriter = audit
	opts.Now = fixedWriteNow

	result, err := ExecuteWrite(contextWithTestDeadline(t), opts)
	if err != nil {
		t.Fatalf("ExecuteWrite error: %v", err)
	}
	if result.SObject != "Opportunity" || result.RecordID != "006ACME0001" {
		t.Fatalf("result target = %s/%s", result.SObject, result.RecordID)
	}
	if fake.patchPath != "/services/data/v63.0/ui-api/records/006ACME0001" {
		t.Fatalf("patch path = %s", fake.patchPath)
	}
	body := fake.patchBody.(map[string]any)
	fields := body["fields"].(map[string]any)
	if fields["StageName"] != "Closed Won" || fields["CloseDate"] != "2026-05-01" {
		t.Fatalf("patch fields = %#v", fields)
	}
	if audit.intent.SObject != "Opportunity" || audit.intent.Operation != WriteOperationUpdate {
		t.Fatalf("audit intent = %#v", audit.intent)
	}
}

func TestAdvanceRejectsInvalidPicklistValue(t *testing.T) {
	_, err := NewAdvanceWriteOptions(contextWithTestDeadline(t), AdvanceOptions{
		OpportunityID: "006ACME0001",
		StageName:     "Not A Real Stage",
	})
	assertErrContains(t, err, "INVALID_PICKLIST_VALUE")
	assertErrContains(t, err, "Closed Won")
}

func TestAdvanceUsesDescribePicklistWhenAvailable(t *testing.T) {
	fake := &highLevelWriteClient{
		getBody: json.RawMessage(`{"fields":[{"name":"StageName","picklistValues":[{"value":"Custom Stage","active":true},{"value":"Inactive Stage","active":false}]}]}`),
	}
	_, err := NewAdvanceWriteOptions(contextWithTestDeadline(t), AdvanceOptions{
		OpportunityID: "006ACME0001",
		StageName:     "Closed Won",
		Client:        fake,
	})
	assertErrContains(t, err, "INVALID_PICKLIST_VALUE")
	if fake.getPath != "/services/data/v63.0/sobjects/Opportunity/describe" {
		t.Fatalf("describe path = %s", fake.getPath)
	}
}

func TestAdvanceRequiresOppAndStage(t *testing.T) {
	_, err := NewAdvanceWriteOptions(contextWithTestDeadline(t), AdvanceOptions{StageName: "Closed Won"})
	assertErrContains(t, err, "MISSING_REQUIRED_FLAG")

	_, err = NewAdvanceWriteOptions(contextWithTestDeadline(t), AdvanceOptions{OpportunityID: "006ACME0001"})
	assertErrContains(t, err, "MISSING_REQUIRED_FLAG")
}
