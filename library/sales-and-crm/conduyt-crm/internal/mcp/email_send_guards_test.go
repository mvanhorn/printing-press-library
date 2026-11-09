package mcp

import (
	"strings"
	"testing"
)

// Go CLI #324: the executor passes the live key names for email.send and email.send-bulk and refuses any other key locally,
// with the accepted list, before a request is made. The spec-declared query parameters (dry_run, sandbox) ride the query.
func TestCodeOrchEmailSendKeys(t *testing.T) {
	good := map[string]any{"contactId": "6f1c0000-0000-4000-8000-0000000000a1", "to": "a@b.co", "subject": "S", "html": "<p>x</p>", "sandbox": true}
	hits, body, rawQuery, res := execGuard(t, "email.send", good)
	if hits != 1 || res.IsError {
		t.Fatalf("valid send: hits=%d isError=%v", hits, res.IsError)
	}
	if rawQuery != "sandbox=true" || strings.Contains(body, "sandbox") || !strings.Contains(body, `"to":"a@b.co"`) {
		t.Errorf("query=%q body=%s", rawQuery, body)
	}
	for _, p := range []map[string]any{
		{"contactId": "x", "to": "a@b.co", "subject": "S", "body": "old"},
		{"contactId": "x", "to": "a@b.co", "subject": "S", "html": "x", "scheduledAt": "2026-10-23T00:00:00Z"},
	} {
		hits, _, _, res := execGuard(t, "email.send", p)
		if hits != 0 || !res.IsError {
			t.Errorf("%v: hits=%d isError=%v, want a local refusal", p, hits, res.IsError)
		}
	}
	hits, _, _, res = execGuard(t, "email.send-bulk", map[string]any{"contactIds": []any{"x"}, "subject": "S", "bodyHtml": "old"})
	if hits != 0 || !res.IsError {
		t.Fatalf("bulk old key: hits=%d isError=%v", hits, res.IsError)
	}
	hits, _, _, res = execGuard(t, "email.send-bulk", map[string]any{"contactIds": []any{"x"}, "templateId": "t", "subjectEdited": true, "dry_run": true})
	if hits != 1 || res.IsError {
		t.Errorf("bulk valid: hits=%d isError=%v", hits, res.IsError)
	}
}
