// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"testing"
)

func TestFindStringForTransaction(t *testing.T) {
	// transactionID may be a string or number across Expensify payloads.
	data := json.RawMessage(`{
		"jsonCode": 200,
		"transactionList": [
			{"transactionID": "111", "reportID": "999"},
			{"transactionID": 222, "reportID": 888}
		]
	}`)
	if got := findStringForTransaction(data, "111", "reportID"); got != "999" {
		t.Errorf("string id: got %q, want 999", got)
	}
	if got := findStringForTransaction(data, "222", "reportID"); got != "888" {
		t.Errorf("numeric id: got %q, want 888", got)
	}
	if got := findStringForTransaction(data, "404", "reportID"); got != "" {
		t.Errorf("missing id: got %q, want empty", got)
	}
}

func TestFindReportActionIDForTransaction(t *testing.T) {
	// Nested report -> reportActions -> action with originalMessage.IOUTransactionID.
	data := json.RawMessage(`{
		"reportActions": {
			"8314": {
				"a1": {"reportActionID": "a1", "originalMessage": {"type": "comment"}},
				"a2": {"reportActionID": "555", "originalMessage": {"IOUTransactionID": "111"}}
			}
		}
	}`)
	if got := findReportActionIDForTransaction(data, "111"); got != "555" {
		t.Errorf("got %q, want 555", got)
	}
	if got := findReportActionIDForTransaction(data, "999"); got != "" {
		t.Errorf("missing tx: got %q, want empty", got)
	}
}

// fakePoster returns canned responses keyed by path, recording the bodies sent.
type fakePoster struct {
	responses map[string]json.RawMessage
	calls     []string
}

func (f *fakePoster) Post(_ context.Context, path string, body any) (json.RawMessage, int, error) {
	f.calls = append(f.calls, path)
	return f.responses[path], 200, nil
}

func TestResolveExpenseDeleteRefs(t *testing.T) {
	f := &fakePoster{responses: map[string]json.RawMessage{
		"/Get":        json.RawMessage(`{"transactionList":[{"transactionID":"111","reportID":"999"}]}`),
		"/OpenReport": json.RawMessage(`{"reportActions":{"999":{"a2":{"reportActionID":"555","originalMessage":{"IOUTransactionID":"111"}}}}}`),
	}}
	rid, aid, err := resolveExpenseDeleteRefs(context.Background(), f, "111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rid != "999" || aid != "555" {
		t.Errorf("got reportID=%q reportActionID=%q, want 999/555", rid, aid)
	}
	if len(f.calls) != 2 || f.calls[0] != "/Get" || f.calls[1] != "/OpenReport" {
		t.Errorf("unexpected call sequence: %v", f.calls)
	}
}

func TestResolveExpenseDeleteRefs_NotFound(t *testing.T) {
	f := &fakePoster{responses: map[string]json.RawMessage{
		"/Get": json.RawMessage(`{"transactionList":[]}`),
	}}
	if _, _, err := resolveExpenseDeleteRefs(context.Background(), f, "111"); err == nil {
		t.Error("expected error when reportID cannot be resolved, got nil")
	}
}
