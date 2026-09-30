// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func TestDiagnoseHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"diagnose", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"diagnose [email] [flags]", "Do NOT use it to only check suppression status; use 'suppressions check' instead."} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestDiagnoseDecide(t *testing.T) {
	msg := func(id, status string, events ...string) diagnoseMessage {
		m := diagnoseMessage{MessageID: id, Status: status, Stream: "outbound", Subject: "Reset your password", Source: postmarkSourceLive}
		for _, e := range events {
			m.Events = append(m.Events, diagnoseEvent{Type: e, At: "2026-09-30T10:00:00-04:00"})
		}
		return m
	}
	cases := []struct {
		name        string
		d           diagnoseServer
		wantVerdict string
		wantNext    string
		wantMsgID   string
	}{
		{
			name:        "delivered",
			d:           diagnoseServer{Messages: []diagnoseMessage{msg("m1", "Sent", "Delivered")}},
			wantVerdict: verdictDelivered, wantMsgID: "m1",
		},
		{
			name:        "not found",
			d:           diagnoseServer{},
			wantVerdict: verdictNotFound,
		},
		{
			name: "hard bounce on latest, reactivatable",
			d: diagnoseServer{
				Messages: []diagnoseMessage{msg("m2", "Sent", "Bounced")},
				Bounces:  []diagnoseBounce{{ID: 42, Type: "HardBounce", MessageID: "m2", Inactive: true, CanActivate: true}},
			},
			wantVerdict: verdictBounced, wantNext: "postmark-pp-cli bounces activate 42 --server Staging", wantMsgID: "m2",
		},
		{
			name: "older bounce does not override a delivered latest message",
			d: diagnoseServer{
				Messages: []diagnoseMessage{msg("m3", "Sent", "Delivered"), msg("m0", "Sent")},
				Bounces:  []diagnoseBounce{{ID: 7, Type: "SoftBounce", MessageID: "m0"}},
			},
			wantVerdict: verdictDelivered, wantMsgID: "m3",
		},
		{
			name: "spam complaint beats everything",
			d: diagnoseServer{
				Messages:     []diagnoseMessage{msg("m4", "Sent", "Delivered")},
				Suppressions: []diagnoseSuppression{{Stream: "outbound", Reason: "SpamComplaint"}},
			},
			wantVerdict: verdictSpamComplaint,
		},
		{
			name: "manual suppression gives a delete command",
			d: diagnoseServer{
				Suppressions: []diagnoseSuppression{{Stream: "broadcast", Reason: "ManualSuppression"}},
			},
			wantVerdict: verdictSuppressed, wantNext: `postmark-pp-cli suppressions delete broadcast --suppressions '[{"EmailAddress":"jane@example.com"}]' --server Staging`,
		},
		{
			name: "hard-bounce suppression prefers bounce activation",
			d: diagnoseServer{
				Suppressions: []diagnoseSuppression{{Stream: "outbound", Reason: "HardBounce"}},
				Bounces:      []diagnoseBounce{{ID: 9, Type: "HardBounce", MessageID: "old", Inactive: true, CanActivate: true}},
				Messages:     []diagnoseMessage{msg("m5", "Sent", "Delivered")},
			},
			wantVerdict: verdictSuppressed, wantNext: "postmark-pp-cli bounces activate 9 --server Staging", wantMsgID: "m5",
		},
		{
			name:        "queued while retrying",
			d:           diagnoseServer{Messages: []diagnoseMessage{msg("m6", "Sent", "Transient", "Transient")}},
			wantVerdict: verdictQueued, wantMsgID: "m6",
		},
		{
			name:        "status queued",
			d:           diagnoseServer{Messages: []diagnoseMessage{msg("m7", "Queued")}},
			wantVerdict: verdictQueued, wantMsgID: "m7",
		},
		{
			name: "transient bounce record on latest message",
			d: diagnoseServer{
				Messages: []diagnoseMessage{msg("m8", "Sent", "Transient")},
				Bounces:  []diagnoseBounce{{ID: 11, Type: "Transient", MessageID: "m8", CanActivate: true}},
			},
			wantVerdict: verdictBounced, wantNext: "postmark-pp-cli bounces get 11 --server Staging --json", wantMsgID: "m8",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.d
			d.Server = "Staging"
			diagnoseDecide("jane@example.com", &d)
			if d.Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %s (%s), want %s", d.Verdict, d.Reason, tc.wantVerdict)
			}
			if tc.wantNext != "" && d.Next != tc.wantNext {
				t.Fatalf("next = %q, want %q", d.Next, tc.wantNext)
			}
			if tc.wantVerdict == verdictSpamComplaint && d.Next != "" {
				t.Fatalf("spam complaints must not suggest a resend: %q", d.Next)
			}
			if d.MessageID != tc.wantMsgID {
				t.Fatalf("message_id = %q, want %q", d.MessageID, tc.wantMsgID)
			}
		})
	}
}

func TestDiagnosePrimaryPrefersNewestMessage(t *testing.T) {
	now := time.Now()
	servers := []diagnoseServer{
		{Server: "A", Verdict: verdictNotFound},
		{Server: "B", Verdict: verdictDelivered, Messages: []diagnoseMessage{{at: now.Add(-2 * time.Hour)}}},
		{Server: "C", Verdict: verdictBounced, Messages: []diagnoseMessage{{at: now.Add(-time.Hour)}}},
	}
	if got := diagnosePrimary(servers); got != 2 {
		t.Fatalf("primary = %d, want 2 (newest message)", got)
	}
	quiet := []diagnoseServer{{Verdict: verdictNotFound}, {Verdict: verdictSuppressed}}
	if got := diagnosePrimary(quiet); got != 1 {
		t.Fatalf("primary without messages = %d, want the suppressed server", got)
	}
}

func TestDiagnoseLiveDeliveredAndNotFound(t *testing.T) {
	f := newPostmarkFake(t)
	home := postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Main App", "tok-main"}))
	f.reply("GET /message-streams", 200, map[string]any{"MessageStreams": []any{
		map[string]any{"ID": "outbound", "MessageStreamType": "Transactional"},
		map[string]any{"ID": "inbound", "MessageStreamType": "Inbound"},
	}})
	f.handle("GET /messages/outbound", func(r *http.Request, _ string) (int, any) {
		if r.URL.Query().Get("recipient") != "jane@example.com" {
			return 200, map[string]any{"TotalCount": 0, "Messages": []any{}}
		}
		return 200, map[string]any{"TotalCount": 1, "Messages": []any{map[string]any{
			"MessageID": "m-1", "Subject": "Reset", "Status": "Sent", "MessageStream": "outbound",
			"ReceivedAt": "2026-09-30T10:00:00-04:00", "Recipients": []string{"jane@example.com"},
		}}}
	})
	f.reply("GET /messages/outbound/m-1/details", 200, map[string]any{"Status": "Sent", "MessageEvents": []any{
		map[string]any{"Recipient": "jane@example.com", "Type": "Delivered", "ReceivedAt": "2026-09-30T10:00:02-04:00", "Details": map[string]any{"DeliveryMessage": "250 OK"}},
	}})
	f.reply("GET /bounces", 200, map[string]any{"TotalCount": 0, "Bounces": []any{}})
	f.reply("GET /message-streams/outbound/suppressions/dump", 200, map[string]any{"Suppressions": []any{}})

	db := home + "/none.db"
	stdout, stderr, err := postmarkRun(t, "diagnose", "jane@example.com", "--server", "Main App", "--db", db, "--json")
	if err != nil {
		t.Fatalf("diagnose: %v\n%s", err, stderr)
	}
	var res diagnoseResult
	postmarkResults(t, stdout, &res)
	if res.Verdict != verdictDelivered || res.MessageID != "m-1" || res.Server != "Main App" {
		t.Fatalf("result = %+v", res)
	}
	for _, r := range f.log() {
		if r.Path == "/message-streams/inbound/suppressions/dump" {
			t.Fatal("inbound stream should not be checked")
		}
		if r.Path == "/bounces" && !strings.Contains(r.Query, "emailFilter=jane%40example.com") {
			t.Fatalf("bounce query = %q", r.Query)
		}
	}

	stdout, _, err = postmarkRun(t, "diagnose", "never@example.com", "--server", "Main App", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var none diagnoseResult
	postmarkResults(t, stdout, &none)
	if none.Verdict != verdictNotFound || none.MessageID != "" {
		t.Fatalf("never-mailed result = %+v", none)
	}
}

func TestDiagnoseRequiresEmail(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := postmarkRun(t, "diagnose", "not-an-address", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("want usage error, got %v", err)
	}
}
