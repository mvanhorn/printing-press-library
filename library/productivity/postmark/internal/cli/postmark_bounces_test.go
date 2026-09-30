// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func TestResolveBounceType(t *testing.T) {
	for in, want := range map[string]string{"": "", "any": "", "ALL": "", "hardbounce": "HardBounce", "SpamComplaint": "SpamComplaint"} {
		if got, err := resolveBounceType(in); err != nil || got != want {
			t.Errorf("resolveBounceType(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := resolveBounceType("Bogus"); err == nil {
		t.Errorf("unknown bounce type should error")
	}
}

func TestPostmarkEasternTimestamp(t *testing.T) {
	ts := time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC)
	if got := postmarkEasternTimestamp(ts); got != "2026-09-30T14:00:00" {
		t.Errorf("EDT conversion = %s", got)
	}
	winter := time.Date(2026, 1, 15, 18, 0, 0, 0, time.UTC)
	if got := postmarkEasternTimestamp(winter); got != "2026-01-15T13:00:00" {
		t.Errorf("EST conversion = %s", got)
	}
	if got, err := sinceToEastern("7d", ts); err != nil || got != "2026-09-23T14:00:00" {
		t.Errorf("sinceToEastern(7d) = %s, %v", got, err)
	}
	if _, err := sinceToEastern("soon", ts); err == nil {
		t.Errorf("invalid --since should error")
	}
}

func TestMatchesBounceFiltersAndBlocker(t *testing.T) {
	b := bounceRecord{Email: "Jane@Example.com", Type: "HardBounce", Inactive: true, CanActivate: true}
	cases := []struct {
		domain, email string
		want          bool
	}{
		{"", "", true},
		{"example.com", "", true},
		{"@EXAMPLE.com", "", true},
		{"mail.example.com", "", false},
		{"", "jane@example.com", true},
		{"", "jane@example.co", false},
	}
	for _, tc := range cases {
		if got := matchesBounceFilters(b, tc.domain, tc.email); got != tc.want {
			t.Errorf("matches(%q,%q) = %v, want %v", tc.domain, tc.email, got, tc.want)
		}
	}
	blockers := map[string]bounceRecord{
		"":                  b,
		"spam complaints":   {Type: "SpamComplaint", Inactive: true, CanActivate: true},
		"already active":    {Type: "HardBounce", Inactive: false, CanActivate: true},
		"CanActivate=false": {Type: "HardBounce", Inactive: true, CanActivate: false},
	}
	for want, rec := range blockers {
		got := reactivationBlocker(rec)
		if want == "" && got != "" || want != "" && !strings.Contains(got, want) {
			t.Errorf("blocker(%+v) = %q, want %q", rec, got, want)
		}
	}
}

func TestPlanBounceReactivation(t *testing.T) {
	bounces := []bounceRecord{
		{ID: 1, Email: "a@example.com", Type: "HardBounce", Inactive: true, CanActivate: true},
		{ID: 2, Email: "b@other.com", Type: "HardBounce", Inactive: true, CanActivate: true},
		{ID: 3, Email: "c@example.com", Type: "SpamComplaint", Inactive: true, CanActivate: false},
		{ID: 4, Email: "d@example.com", Type: "HardBounce", Inactive: true, CanActivate: true},
		{ID: 5, Email: "e@example.com", Type: "HardBounce", Inactive: true, CanActivate: false},
	}
	items, skipped, over := planBounceReactivation(bounces, "example.com", "", 1)
	if len(items) != 1 || items[0].ID != 1 || over != 1 {
		t.Fatalf("items=%+v over=%d", items, over)
	}
	if len(skipped) != 2 || skipped[0].ID != 3 || skipped[1].ID != 5 {
		t.Fatalf("skipped = %+v", skipped)
	}
	items, _, _ = planBounceReactivation(bounces, "nomatch.org", "", 50)
	if len(items) != 0 {
		t.Fatalf("mismatching domain must plan nothing, got %+v", items)
	}
}

func bounceFixtureRoutes(bounces []bounceRecord) map[string]fakeRoute {
	routes := map[string]fakeRoute{
		"GET /bounces": jsonRoute(map[string]any{"TotalCount": len(bounces), "Bounces": bounces}),
	}
	for _, b := range bounces {
		id := b.ID
		routes["PUT /bounces/"+bounceIDString(id)+"/activate"] = func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, http.StatusOK, map[string]any{"Message": "OK", "Bounce": map[string]any{"ID": id, "Inactive": false}})
		}
	}
	return routes
}

func bounceIDString(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestBouncesReactivatePlanIssuesNoWrites(t *testing.T) {
	bounces := []bounceRecord{
		{ID: 11, Email: "a@example.com", Type: "HardBounce", Inactive: true, CanActivate: true, BouncedAt: "2026-09-20T10:00:00Z"},
		{ID: 12, Email: "b@example.com", Type: "SpamComplaint", Inactive: true, CanActivate: false},
		{ID: 13, Email: "c@other.com", Type: "HardBounce", Inactive: true, CanActivate: true},
	}
	srv, requests := newFakePostmarkServer(t, bounceFixtureRoutes(bounces))
	out, stderr, err := runWithServerToken(t, srv.URL, "bounces", "reactivate", "--domain", "example.com", "--since", "90d", "--type", "any", "--json")
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, stderr)
	}
	var view bounceReactivateView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.Mode != "plan" || view.Planned != 1 || view.Writes != 0 || len(view.Skipped) != 1 || view.Items[0].ID != 11 {
		t.Fatalf("view = %+v", view)
	}
	if got := mutatingRequests(requests()); len(got) != 0 {
		t.Fatalf("plan mode issued writes: %v", got)
	}
	q, _ := url.ParseQuery(requests()[0].Query)
	if q.Get("inactive") != "true" || q.Get("count") != "500" || q.Get("offset") != "0" || q.Get("fromdate") == "" || q.Has("type") {
		t.Fatalf("bounce query = %v", q)
	}

	// Default type is HardBounce and is sent to the API.
	if _, _, err := runWithServerToken(t, srv.URL, "bounces", "reactivate", "--since", "90d", "--json"); err != nil {
		t.Fatal(err)
	}
	reqs := requests()
	q, _ = url.ParseQuery(reqs[len(reqs)-1].Query)
	if q.Get("type") != "HardBounce" {
		t.Fatalf("default type filter = %q", q.Get("type"))
	}
}

func TestBouncesReactivateYesActivatesPlannedOnly(t *testing.T) {
	bounces := []bounceRecord{
		{ID: 21, Email: "a@example.com", Type: "HardBounce", Inactive: true, CanActivate: true},
		{ID: 22, Email: "b@example.com", Type: "HardBounce", Inactive: true, CanActivate: true},
		{ID: 23, Email: "c@example.com", Type: "SpamComplaint", Inactive: true, CanActivate: false},
		{ID: 24, Email: "d@other.com", Type: "HardBounce", Inactive: true, CanActivate: true},
	}
	srv, requests := newFakePostmarkServer(t, bounceFixtureRoutes(bounces))
	out, stderr, err := runWithServerToken(t, srv.URL, "bounces", "reactivate", "--domain", "example.com", "--type", "any", "--yes", "--json")
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, stderr)
	}
	want := []string{"PUT /bounces/21/activate", "PUT /bounces/22/activate"}
	if got := mutatingRequests(requests()); !reflect.DeepEqual(got, want) {
		t.Fatalf("writes = %v, want %v", got, want)
	}
	var view bounceReactivateView
	_ = json.Unmarshal([]byte(out), &view)
	if view.Mode != "apply" || view.Activated != 2 || view.Writes != 2 || view.Items[0].Status != statusActivated {
		t.Fatalf("view = %+v", view)
	}

	out, _, err = runWithServerToken(t, srv.URL, "bounces", "reactivate", "--domain", "example.com", "--type", "any", "--csv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 || !strings.Contains(lines[0], "decision") {
		t.Fatalf("csv should have a header and 3 rows:\n%s", out)
	}
}

func resendFixture() ([]bounceRecord, map[string]fakeRoute) {
	bounces := []bounceRecord{
		{ID: 31, Email: "jane@example.com", Type: "HardBounce", Inactive: true, CanActivate: true, MessageID: "m1", Subject: "Reset", BouncedAt: "2026-09-29T10:00:00Z"},
		{ID: 32, Email: "old@example.com", Type: "HardBounce", Inactive: true, CanActivate: true, MessageID: "m2"},
		{ID: 33, Email: "files@example.com", Type: "HardBounce", Inactive: true, CanActivate: true, MessageID: "m3"},
		{ID: 34, Email: "jane@example.com", Type: "HardBounce", Inactive: true, CanActivate: true, MessageID: "m4", Subject: "Receipt"},
	}
	trackOpens := true
	details := func(id, subject string, attachments []any) outboundMessageDetails {
		raw, _ := json.Marshal(attachments)
		var atts []json.RawMessage
		_ = json.Unmarshal(raw, &atts)
		return outboundMessageDetails{MessageID: id, From: "App <app@example.com>", Subject: subject, HtmlBody: "<p>" + subject + "</p>", TextBody: subject,
			Tag: "auth", MessageStream: "outbound", Metadata: map[string]any{"user_id": "u1"}, TrackOpens: &trackOpens, TrackLinks: "None", Attachments: atts}
	}
	routes := map[string]fakeRoute{
		"GET /bounces":                      jsonRoute(map[string]any{"TotalCount": len(bounces), "Bounces": bounces}),
		"GET /messages/outbound/m1/details": jsonRoute(details("m1", "Reset", []any{})),
		"GET /messages/outbound/m4/details": jsonRoute(details("m4", "Receipt", nil)),
		"GET /messages/outbound/m3/details": jsonRoute(details("m3", "Invoice", []any{"invoice.pdf"})),
		"GET /messages/outbound/m2/details": func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ErrorCode": 701, "Message": "This message was not found."})
		},
		"PUT /bounces/31/activate": jsonRoute(map[string]any{"Message": "OK"}),
		"POST /email": func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			writeJSON(w, http.StatusOK, map[string]any{"ErrorCode": 0, "Message": "OK", "MessageID": "new-id"})
		},
	}
	return bounces, routes
}

func TestBouncesResendBlockedPlanAndSend(t *testing.T) {
	_, routes := resendFixture()
	srv, requests := newFakePostmarkServer(t, routes)
	out, stderr, err := runWithServerToken(t, srv.URL, "bounces", "resend-blocked", "--since", "7d", "--json")
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, stderr)
	}
	var view resendBlockedView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.Resendable != 2 || view.NotResendable != 2 || view.Sent != 0 {
		t.Fatalf("plan view = %+v", view)
	}
	byID := map[int64]resendBlockedItem{}
	for _, it := range view.Items {
		byID[it.BounceID] = it
	}
	if !strings.Contains(byID[32].Reason, "retention") || !strings.Contains(byID[33].Reason, "attachment") {
		t.Fatalf("not-resendable reasons = %q / %q", byID[32].Reason, byID[33].Reason)
	}
	if byID[31].ActivationStatus != statusPlanned || byID[34].ActivationStatus != resendActivationShared {
		t.Fatalf("activation should happen once per address: %+v / %+v", byID[31], byID[34])
	}
	if got := mutatingRequests(requests()); len(got) != 0 {
		t.Fatalf("plan mode issued writes: %v", got)
	}
	q, _ := url.ParseQuery(requests()[0].Query)
	// Only still-inactive bounces are resend candidates, so a second --send run
	// after a successful reactivation finds nothing to resend.
	if q.Get("type") != "HardBounce" || q.Get("inactive") != "true" {
		t.Fatalf("resend bounce query = %v", q)
	}

	out, stderr, err = runWithServerToken(t, srv.URL, "bounces", "resend-blocked", "--since", "7d", "--send", "--json")
	if err != nil {
		t.Fatalf("send: %v\n%s\n%s", err, out, stderr)
	}
	want := []string{"PUT /bounces/31/activate", "POST /email", "POST /email"}
	if got := mutatingRequests(requests()); !reflect.DeepEqual(got, want) {
		t.Fatalf("writes = %v, want %v", got, want)
	}
	subjects := []string{}
	for _, r := range requests() {
		if r.key() != "POST /email" {
			continue
		}
		var p resendPayload
		if err := json.Unmarshal([]byte(r.Body), &p); err != nil {
			t.Fatal(err)
		}
		if p.To != "jane@example.com" || p.From != "App <app@example.com>" || p.MessageStream != "outbound" || p.Tag != "auth" || p.Metadata["user_id"] != "u1" {
			t.Fatalf("resend payload = %+v", p)
		}
		subjects = append(subjects, p.Subject)
	}
	if !reflect.DeepEqual(subjects, []string{"Reset", "Receipt"}) {
		t.Fatalf("resent subjects = %v", subjects)
	}
	view = resendBlockedView{}
	_ = json.Unmarshal([]byte(out), &view)
	if view.Mode != "send" || view.Sent != 2 || view.Failed != 0 {
		t.Fatalf("send view = %+v", view)
	}
}

func TestBouncesResendBlockedRefusesSendUnderHarness(t *testing.T) {
	_, routes := resendFixture()
	srv, requests := newFakePostmarkServer(t, routes)
	// runWithServerToken clears harness env, so drive the command directly.
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	t.Setenv("POSTMARK_BASE_URL", srv.URL)
	t.Setenv("POSTMARK_SERVER_TOKEN", "server-test-token")
	cmd := RootCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"bounces", "resend-blocked", "--send", "--json", "--no-learn"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"refused":true`) || len(requests()) != 0 {
		t.Fatalf("harness must refuse before any request: %s, %d requests", out.String(), len(requests()))
	}
}
