package agent

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNotePostsChatterFeedItemAndAudits(t *testing.T) {
	opts := NewNoteWriteOptions("001ACME0001", "Met with CFO")
	fake := &highLevelWriteClient{
		postResponse: json.RawMessage(`{"id":"0D5ACME0001","feedElementType":"FeedItem","body":{"messageSegments":[{"type":"Text","text":"Met with CFO"}]}}`),
		postStatus:   http.StatusCreated,
	}
	audit := &capturingWriteAudit{}
	opts.Client = fake
	opts.Signer = noopWriteSigner{}
	opts.AuditWriter = audit
	opts.OrgAlias = "prod"
	opts.ActingUser = "005USER"
	opts.Now = fixedWriteNow

	result, err := ExecuteNote(contextWithTestDeadline(t), opts)
	if err != nil {
		t.Fatalf("ExecuteNote error: %v", err)
	}
	if result.RecordID != "0D5ACME0001" || result.SObject != "FeedItem" || result.Operation != WriteOperationCreate {
		t.Fatalf("result = %#v", result)
	}
	if fake.postPath != "/services/data/v63.0/chatter/feeds/record/001ACME0001/feed-elements" {
		t.Fatalf("post path = %s", fake.postPath)
	}
	body := fake.postBody.(map[string]any)
	if body["feedElementType"] != "FeedItem" || body["subjectId"] != "001ACME0001" {
		t.Fatalf("post body = %#v", body)
	}
	message := body["body"].(map[string]any)["messageSegments"].([]map[string]any)[0]
	if message["type"] != "Text" || message["text"] != "Met with CFO" {
		t.Fatalf("message = %#v", message)
	}
	if audit.pending != 1 || audit.executed != 1 || audit.intent.SObject != "FeedItem" || audit.intent.Operation != WriteOperationCreate {
		t.Fatalf("audit = %#v", audit)
	}
}

func TestNoteRejectsEmptyTextBeforeSalesforceCall(t *testing.T) {
	opts := NewNoteWriteOptions("001ACME0001", "  ")
	fake := &highLevelWriteClient{}
	opts.Client = fake
	opts.Signer = noopWriteSigner{}

	_, err := ExecuteNote(contextWithTestDeadline(t), opts)
	assertErrContains(t, err, "EMPTY_NOTE")
	if fake.postPath != "" {
		t.Fatalf("unexpected post path: %s", fake.postPath)
	}
}
