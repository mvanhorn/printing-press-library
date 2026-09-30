// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func TestHealthCommandsWire(t *testing.T) {
	testenv.Isolate(t)
	for _, path := range [][]string{{"webhooks", "health"}, {"domains", "health"}, {"streams", "health"}} {
		cmd := RootCmd()
		cmd.SetArgs(append(path, "--help"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v --help: %v", path, err)
		}
		if want := strings.Join(path, " ") + " [flags]"; !strings.Contains(out.String(), want) {
			t.Fatalf("%v help missing %q:\n%s", path, want, out.String())
		}
	}
}

func TestWebhookEvaluate(t *testing.T) {
	cases := []struct {
		name   string
		status string
		m      webhookMetrics
		want   int
	}{
		{"healthy", "verified", webhookMetrics{TotalRequests: 1000, SuccessCount: 1000}, 0},
		{"no traffic is fine", "verified", webhookMetrics{}, 0},
		{"unverified", "unverified", webhookMetrics{}, 1},
		{"failures and low success", "verified", webhookMetrics{TotalRequests: 100, SuccessCount: 90, FailureCount: 10}, 2},
		{"one failure, 99.9% success", "verified", webhookMetrics{TotalRequests: 1000, SuccessCount: 999, FailureCount: 1}, 1},
	}
	for _, tc := range cases {
		if got := webhookEvaluate(tc.status, tc.m); len(got) != tc.want {
			t.Errorf("%s: problems = %v, want %d", tc.name, got, tc.want)
		}
	}
	if pct := webhookSuccessPct(webhookMetrics{TotalRequests: 200, SuccessCount: 197}); pct == nil || *pct != 98.5 {
		t.Errorf("success pct = %v", pct)
	}
}

func TestDomainIssues(t *testing.T) {
	healthy := bootstrapDomain{ID: 1, Name: "x.co", DKIMVerified: true, DKIMUpdateStatus: "Verified", ReturnPathDomain: "pm-bounces.x.co", ReturnPathDomainVerified: true}
	if got := domainIssues(healthy); len(got) != 0 {
		t.Fatalf("healthy domain issues = %+v", got)
	}
	broken := bootstrapDomain{ID: 7, Name: "y.co", DKIMPendingHost: "p._domainkey.y.co", DKIMPendingTextValue: "k=rsa; p=Z", ReturnPathDomain: "pm-bounces.y.co", ReturnPathDomainCNAMEValue: "pm.mtasv.net", SafeToRemoveRevokedKey: true, DKIMRevokedHost: "old._domainkey.y.co"}
	issues := domainIssues(broken)
	byCheck := map[string]healthIssue{}
	for _, i := range issues {
		byCheck[i.Check] = i
	}
	dkim := byCheck["dkim"]
	if dkim.Severity != healthSeverityFail || !strings.HasSuffix(dkim.Fix, "postmark-pp-cli domains verify-dkim 7") || dkim.Record == nil || dkim.Record.Host != "p._domainkey.y.co" {
		t.Fatalf("dkim issue = %+v", dkim)
	}
	rp := byCheck["return-path"]
	if rp.Severity != healthSeverityFail || !strings.HasSuffix(rp.Fix, "postmark-pp-cli domains verify-return-path 7") || rp.Record == nil || rp.Record.Value != "pm.mtasv.net" {
		t.Fatalf("return-path issue = %+v", rp)
	}
	if byCheck["revoked-dkim"].Severity != healthSeverityInfo {
		t.Fatalf("revoked key issue = %+v", byCheck["revoked-dkim"])
	}
	noRP := domainIssues(bootstrapDomain{ID: 3, Name: "z.co", DKIMVerified: true})
	if len(noRP) != 1 || noRP[0].Severity != healthSeverityWarn || noRP[0].Fix != "postmark-pp-cli domains update 3 --return-path-domain pm-bounces.z.co" {
		t.Fatalf("missing return-path issue = %+v", noRP)
	}
	if healthHasFail(noRP) {
		t.Fatal("a warning alone must not mark the domain unhealthy")
	}
}

func TestStreamEvaluate(t *testing.T) {
	cases := []struct {
		row  streamHealthRow
		want int
	}{
		{streamHealthRow{Sent: 100, Bounced: 9}, 0},
		{streamHealthRow{Sent: 100, Bounced: 10}, 1},
		{streamHealthRow{Sent: 1000, Spam: 1}, 1},
		{streamHealthRow{Sent: 0, Bounced: 3}, 0},
		{streamHealthRow{Sent: 10, Bounced: 5, Spam: 1}, 2},
	}
	for i, tc := range cases {
		r := tc.row
		streamEvaluate(&r)
		if len(r.Problems) != tc.want || r.Healthy != (tc.want == 0) {
			t.Errorf("case %d: problems = %v", i, r.Problems)
		}
	}
}

func TestWebhooksHealthFlagsUnverifiedAndFailing(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Main App", "tok-main"}, [3]any{2, "Quiet", "tok-quiet"}))
	f.handle("GET /webhooks", func(r *http.Request, _ string) (int, any) {
		if r.Header.Get(postmarkServerTokenHeader) == "tok-quiet" {
			return 200, map[string]any{"Webhooks": []any{}}
		}
		return 200, map[string]any{"Webhooks": []any{
			map[string]any{"ID": 10, "Url": "https://a.example/hook", "MessageStream": "outbound", "Status": "verified", "Triggers": map[string]any{"Bounce": map[string]any{"Enabled": true}, "Open": map[string]any{"Enabled": false}}},
			map[string]any{"ID": 11, "Url": "https://b.example/hook", "MessageStream": "outbound", "Status": "unverified"},
			map[string]any{"ID": 12, "Url": "https://c.example/hook", "MessageStream": "broadcast", "Status": "verified"},
		}}
	})
	f.reply("GET /webhooks/10/statistics", 200, map[string]any{"Metrics": map[string]any{"TotalRequests": 500, "SuccessCount": 500}})
	f.reply("GET /webhooks/11/statistics", 200, map[string]any{"Metrics": map[string]any{"TotalRequests": 0}})
	f.reply("GET /webhooks/12/statistics", 200, map[string]any{"Metrics": map[string]any{"TotalRequests": 100, "SuccessCount": 95, "FailureCount": 5, "SuccessRate": 0.95}})

	stdout, stderr, err := postmarkRun(t, "webhooks", "health", "--all-servers", "--json")
	if err != nil {
		t.Fatalf("webhooks health: %v\n%s", err, stderr)
	}
	var res webhooksHealthResult
	postmarkResults(t, stdout, &res)
	if res.ServersChecked != 2 || len(res.Webhooks) != 3 || res.Unhealthy != 2 {
		t.Fatalf("result = %+v", res)
	}
	if len(res.ServersWithoutWebhooks) != 1 || res.ServersWithoutWebhooks[0] != "Quiet" {
		t.Fatalf("servers without webhooks = %v", res.ServersWithoutWebhooks)
	}
	byID := map[int64]webhookHealthRow{}
	for _, w := range res.Webhooks {
		byID[w.WebhookID] = w
	}
	if !byID[10].Healthy || strings.Join(byID[10].Triggers, ",") != "Bounce" {
		t.Fatalf("webhook 10 = %+v", byID[10])
	}
	if byID[11].Healthy || !strings.Contains(byID[11].Next, "webhooks verify 11") {
		t.Fatalf("webhook 11 = %+v", byID[11])
	}
	if byID[12].Healthy || byID[12].SuccessRatePct == nil || *byID[12].SuccessRatePct != 95 || !strings.Contains(byID[12].Next, "webhooks statistics 12") {
		t.Fatalf("webhook 12 = %+v", byID[12])
	}
	for _, r := range f.log() {
		if strings.HasSuffix(r.Path, "/verify") {
			t.Fatal("webhooks health must never call the verify endpoint")
		}
	}
}

func TestDomainsHealthReportsFixes(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /domains", 200, map[string]any{"TotalCount": 2, "Domains": []any{map[string]any{"ID": 1, "Name": "good.co"}, map[string]any{"ID": 2, "Name": "bad.co"}}})
	f.reply("GET /domains/1", 200, map[string]any{"ID": 1, "Name": "good.co", "DKIMVerified": true, "DKIMUpdateStatus": "Verified", "ReturnPathDomain": "pm-bounces.good.co", "ReturnPathDomainVerified": true})
	f.reply("GET /domains/2", 200, map[string]any{"ID": 2, "Name": "bad.co", "DKIMVerified": false, "DKIMPendingHost": "x._domainkey.bad.co", "DKIMPendingTextValue": "k=rsa; p=Q", "ReturnPathDomain": "pm-bounces.bad.co", "ReturnPathDomainVerified": false})
	f.reply("GET /senders", 200, map[string]any{"TotalCount": 2, "SenderSignatures": []any{
		map[string]any{"ID": 5, "EmailAddress": "ok@good.co", "Domain": "good.co", "Confirmed": true},
		map[string]any{"ID": 6, "EmailAddress": "new@bad.co", "Domain": "bad.co", "Confirmed": false},
	}})
	stdout, stderr, err := postmarkRun(t, "domains", "health", "--json")
	if err != nil {
		t.Fatalf("domains health: %v\n%s", err, stderr)
	}
	var res domainsHealthResult
	postmarkResults(t, stdout, &res)
	if len(res.Domains) != 2 || len(res.Senders) != 2 || res.Failures != 3 {
		t.Fatalf("result = %+v", res)
	}
	for _, d := range res.Domains {
		if d.Domain == "good.co" && !d.Healthy {
			t.Fatalf("good.co = %+v", d)
		}
		if d.Domain == "bad.co" && (d.Healthy || len(d.Issues) != 2) {
			t.Fatalf("bad.co = %+v", d)
		}
	}
	if res.Senders[1].Healthy || res.Senders[1].Issues[0].Fix != "postmark-pp-cli senders resend-confirmation 6" {
		t.Fatalf("sender = %+v", res.Senders[1])
	}
	for _, r := range f.log() {
		if r.AccountTok != "acct-token" || r.ServerToken != "" {
			t.Fatalf("%s must use only the account token: %+v", r.Path, r)
		}
	}
}
