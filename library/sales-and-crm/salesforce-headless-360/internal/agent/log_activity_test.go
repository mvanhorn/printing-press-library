package agent

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func TestLogActivityCallCreatesTaskThroughWritePipeline(t *testing.T) {
	opts, err := NewLogActivityWriteOptions(LogActivityOptions{
		Type:           "call",
		WhatID:         "001ACME0001",
		WhoID:          "003ACME0001",
		Subject:        "Q2 check-in",
		Description:    "Talked through expansion plan",
		IdempotencyKey: "call-1",
		Now:            fixedWriteNow,
	})
	if err != nil {
		t.Fatalf("NewLogActivityWriteOptions error: %v", err)
	}
	fake := &highLevelWriteClient{postResponse: json.RawMessage(`{"id":"00TWRITE0001","success":true}`), postStatus: http.StatusCreated}
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
	if result.SObject != "Task" || result.RecordID != "00TWRITE0001" {
		t.Fatalf("result target = %s/%s", result.SObject, result.RecordID)
	}
	if fake.postPath != "/services/data/v63.0/sobjects/Task" {
		t.Fatalf("post path = %s", fake.postPath)
	}
	fields := fake.postBody.(map[string]any)
	want := map[string]any{
		"Subject":                  "Q2 check-in",
		"Description":              "Talked through expansion plan",
		"ActivityDate":             "2026-04-22",
		"WhatId":                   "001ACME0001",
		"WhoId":                    "003ACME0001",
		"Status":                   "Completed",
		"Priority":                 "Normal",
		"TaskSubtype":              "call",
		"SF360_Idempotency_Key__c": "call-1",
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("post body mismatch:\ngot  %#v\nwant %#v", fields, want)
	}
	if audit.pending != 1 || audit.executed != 1 {
		t.Fatalf("audit pending/executed = %d/%d", audit.pending, audit.executed)
	}
	if audit.intent.IdempotencyKey != "call-1" || audit.intent.SObject != "Task" {
		t.Fatalf("audit intent = %#v", audit.intent)
	}
}

func TestLogActivityMeetingCreatesEventThroughWritePipeline(t *testing.T) {
	start := time.Date(2026, 5, 1, 17, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	opts, err := NewLogActivityWriteOptions(LogActivityOptions{
		Type:           "meeting",
		WhatID:         "001ACME0001",
		Subject:        "Demo",
		Start:          start,
		End:            end,
		IdempotencyKey: "meet-1",
	})
	if err != nil {
		t.Fatalf("NewLogActivityWriteOptions error: %v", err)
	}
	fake := &highLevelWriteClient{postResponse: json.RawMessage(`{"id":"00UWRITE0001","success":true}`), postStatus: http.StatusCreated}
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
	if result.SObject != "Event" || result.RecordID != "00UWRITE0001" {
		t.Fatalf("result target = %s/%s", result.SObject, result.RecordID)
	}
	fields := fake.postBody.(map[string]any)
	if fields["StartDateTime"] != "2026-05-01T17:00:00Z" || fields["EndDateTime"] != "2026-05-01T17:30:00Z" {
		t.Fatalf("meeting times = %#v", fields)
	}
	if audit.intent.SObject != "Event" || audit.intent.IdempotencyKey != "meet-1" {
		t.Fatalf("audit intent = %#v", audit.intent)
	}
}

func TestLogActivityMeetingRequiresStartAndEnd(t *testing.T) {
	_, err := NewLogActivityWriteOptions(LogActivityOptions{
		Type:           "meeting",
		WhatID:         "001ACME0001",
		Subject:        "Demo",
		IdempotencyKey: "meet-1",
	})
	assertErrContains(t, err, "MISSING_REQUIRED_FLAG")
}

func TestLogActivityRejectsInvalidType(t *testing.T) {
	_, err := NewLogActivityWriteOptions(LogActivityOptions{
		Type:           "sms",
		WhatID:         "001ACME0001",
		Subject:        "Q2 check-in",
		IdempotencyKey: "call-1",
	})
	assertErrContains(t, err, "INVALID_TYPE")
}

func TestLogActivityRequiresRelatedRecord(t *testing.T) {
	_, err := NewLogActivityWriteOptions(LogActivityOptions{
		Type:           "call",
		Subject:        "Q2 check-in",
		IdempotencyKey: "call-1",
	})
	assertErrContains(t, err, "MISSING_RELATED_RECORD")
}

type highLevelWriteClient struct {
	getBody      json.RawMessage
	getBodies    []json.RawMessage
	getPath      string
	getParams    map[string]string
	postPath     string
	postBody     any
	postResponse json.RawMessage
	postStatus   int
	patchPath    string
	patchBody    any
	patchStatus  int
}

func (c *highLevelWriteClient) GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error) {
	c.getPath = path
	c.getParams = params
	if len(c.getBodies) > 0 {
		body := c.getBodies[0]
		c.getBodies = c.getBodies[1:]
		return body, nil, nil
	}
	return c.getBody, nil, nil
}

func (c *highLevelWriteClient) Post(path string, body any) (json.RawMessage, int, error) {
	c.postPath = path
	c.postBody = body
	status := c.postStatus
	if status == 0 {
		status = http.StatusCreated
	}
	response := c.postResponse
	if len(response) == 0 {
		response = json.RawMessage(`{"id":"a00WRITE0001","success":true}`)
	}
	return response, status, nil
}

func (c *highLevelWriteClient) PatchWithResponseHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, http.Header, error) {
	c.patchPath = path
	c.patchBody = body
	status := c.patchStatus
	if status == 0 {
		status = http.StatusOK
	}
	return json.RawMessage(`{"id":"` + path[strings.LastIndex(path, "/")+1:] + `","success":true}`), status, nil, nil
}

type capturingWriteAudit struct {
	pending  int
	executed int
	rejected int
	intent   trust.WriteIntentClaims
}

func (a *capturingWriteAudit) WritePending(intent trust.WriteIntentClaims, _ []byte, _ map[string]any) error {
	a.pending++
	a.intent = intent
	return nil
}

func (a *capturingWriteAudit) UpdateExecuted(string, map[string]any) error {
	a.executed++
	return nil
}

func (a *capturingWriteAudit) UpdateRejected(string, string, string) error {
	a.rejected++
	return nil
}

func (a *capturingWriteAudit) UpdateConflict(string, time.Time, time.Time) error {
	return nil
}

func assertErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want containing %q", err, want)
	}
}
