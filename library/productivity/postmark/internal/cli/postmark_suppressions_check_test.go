// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestShellQuoteWord(t *testing.T) {
	cases := map[string]string{
		"outbound":    "outbound",
		"Main App":    "'Main App'",
		"it's":        `'it'\''s'`,
		`[{"a":"b"}]`: `'[{"a":"b"}]'`,
		"":            "''",
		"jane@x.co":   "jane@x.co",
	}
	for in, want := range cases {
		if got := shellQuoteWord(in); got != want {
			t.Errorf("shellQuoteWord(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestSuppressionNextCommand(t *testing.T) {
	cmd, note := suppressionNextCommand("outbound", "jane@example.com", "HardBounce", "Main App")
	want := `postmark-pp-cli suppressions delete outbound --suppressions '[{"EmailAddress":"jane@example.com"}]' --server 'Main App'`
	if cmd != want || !strings.Contains(note, "reactivates") {
		t.Errorf("HardBounce next = %q (%q)", cmd, note)
	}
	cmd, note = suppressionNextCommand("broadcast", "jane@example.com", "SpamComplaint", "")
	if cmd != "" || !strings.Contains(note, "cannot be removed") {
		t.Errorf("SpamComplaint next = %q (%q)", cmd, note)
	}
	cmd, _ = suppressionNextCommand("outbound", "jane@example.com", "ManualSuppression", "")
	if strings.Contains(cmd, "--server") {
		t.Errorf("no --server when none was selected: %q", cmd)
	}
}

func TestSuppressionRowForExactMatch(t *testing.T) {
	stream := postmarkStream{ID: "outbound", Name: "Default", MessageStreamType: "Transactional"}
	entries := []suppressionEntry{
		{EmailAddress: "jane@example.com.au", SuppressionReason: "HardBounce"},
		{EmailAddress: "Jane@Example.com", SuppressionReason: "ManualSuppression", Origin: "Customer", CreatedAt: "2026-04-01T16:38:16-04:00"},
	}
	row := suppressionRowFor(stream, entries, "jane@example.com", "")
	if !row.Suppressed || row.Reason != "ManualSuppression" || row.Origin != "Customer" || row.NextCommand == "" {
		t.Fatalf("row = %+v", row)
	}
	row = suppressionRowFor(stream, entries[:1], "jane@example.com", "")
	if row.Suppressed {
		t.Fatalf("partial address match must not count: %+v", row)
	}
}

func TestSuppressionsCheckAcrossStreams(t *testing.T) {
	routes := map[string]fakeRoute{
		"GET /message-streams": jsonRoute(map[string]any{"TotalCount": 3, "MessageStreams": []map[string]any{
			{"ID": "outbound", "Name": "Default Transactional Stream", "MessageStreamType": "Transactional"},
			{"ID": "inbound", "Name": "Default Inbound Stream", "MessageStreamType": "Inbound"},
			{"ID": "broadcast", "Name": "Default Broadcast Stream", "MessageStreamType": "Broadcasts"},
		}}),
		"GET /message-streams/outbound/suppressions/dump": jsonRoute(map[string]any{"Suppressions": []map[string]any{
			{"EmailAddress": "jane@example.com", "SuppressionReason": "HardBounce", "Origin": "Recipient", "CreatedAt": "2026-04-01T16:38:16-04:00"},
		}}),
		"GET /message-streams/broadcast/suppressions/dump": jsonRoute(map[string]any{"Suppressions": []map[string]any{
			{"EmailAddress": "jane@example.com", "SuppressionReason": "SpamComplaint", "Origin": "Recipient", "CreatedAt": "2026-05-01T10:00:00-04:00"},
		}}),
	}
	srv, requests := newFakePostmarkServer(t, routes)
	out, stderr, err := runWithServerToken(t, srv.URL, "suppressions", "check", "jane@example.com", "--json")
	if err != nil {
		t.Fatalf("check: %v\n%s", err, stderr)
	}
	var view suppressionCheckView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if !view.SuppressedAnywhere || len(view.Streams) != 2 || len(view.SkippedStreams) != 1 || view.SkippedStreams[0] != "inbound" {
		t.Fatalf("view = %+v", view)
	}
	out0, out1 := view.Streams[0], view.Streams[1]
	if out0.Stream != "outbound" || !out0.Suppressed || out0.Reason != "HardBounce" || !strings.Contains(out0.NextCommand, "suppressions delete outbound") {
		t.Fatalf("outbound row = %+v", out0)
	}
	if out1.Stream != "broadcast" || !out1.Suppressed || out1.NextCommand != "" || !strings.Contains(out1.Note, "cannot be removed") {
		t.Fatalf("broadcast row = %+v", out1)
	}
	for _, r := range requests() {
		if strings.Contains(r.Path, "/inbound/") {
			t.Fatalf("inbound stream must not be queried: %s", r.Path)
		}
		if strings.HasSuffix(r.Path, "/dump") {
			q, _ := url.ParseQuery(r.Query)
			if q.Get("EmailAddress") != "jane@example.com" {
				t.Fatalf("dump query = %v", q)
			}
		}
		if r.Path == "/message-streams" {
			q, _ := url.ParseQuery(r.Query)
			if q.Get("IncludeArchivedStreams") != "false" {
				t.Fatalf("streams query = %v", q)
			}
		}
	}

	// --stream checks one stream without listing, and a clean address is false.
	routes["GET /message-streams/outbound/suppressions/dump"] = jsonRoute(map[string]any{"Suppressions": []any{}})
	srv2, requests2 := newFakePostmarkServer(t, routes)
	out, _, err = runWithServerToken(t, srv2.URL, "suppressions", "check", "someone@example.org", "--stream", "outbound", "--json")
	if err != nil {
		t.Fatal(err)
	}
	view = suppressionCheckView{}
	_ = json.Unmarshal([]byte(out), &view)
	if view.SuppressedAnywhere || len(view.Streams) != 1 || view.Streams[0].Suppressed {
		t.Fatalf("clean address view = %+v", view)
	}
	if reqs := requests2(); len(reqs) != 1 || reqs[0].Path != "/message-streams/outbound/suppressions/dump" {
		t.Fatalf("--stream requests = %+v", reqs)
	}
}

func TestSuppressionsCheckReportsFetchFailures(t *testing.T) {
	routes := map[string]fakeRoute{
		"GET /message-streams": jsonRoute(map[string]any{"MessageStreams": []map[string]any{
			{"ID": "outbound", "MessageStreamType": "Transactional"},
			{"ID": "broken", "MessageStreamType": "Broadcasts"},
		}}),
		"GET /message-streams/outbound/suppressions/dump": jsonRoute(map[string]any{"Suppressions": []any{}}),
		"GET /message-streams/broken/suppressions/dump": func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ErrorCode": 1226, "Message": "The message stream for the provided 'ID' was not found."})
		},
	}
	srv, _ := newFakePostmarkServer(t, routes)
	out, stderr, err := runWithServerToken(t, srv.URL, "suppressions", "check", "jane@example.com", "--json")
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("partial failure should exit 5, got %v", err)
	}
	var view suppressionCheckView
	if jerr := json.Unmarshal([]byte(out), &view); jerr != nil {
		t.Fatalf("stdout must stay JSON: %v\n%s", jerr, out)
	}
	if len(view.FetchFailures) != 1 || view.FetchFailures[0].ID != "broken" || len(view.Streams) != 1 {
		t.Fatalf("view = %+v", view)
	}
	if !strings.Contains(stderr, "1 of 2 streams could not be checked") {
		t.Fatalf("stderr warning missing: %s", stderr)
	}
}
