// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func TestOverviewHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"overview", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"overview [flags]", "use 'pulse' instead"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestOverviewTotalsOf(t *testing.T) {
	got := overviewTotalsOf([]overviewRow{{Sent: 900, Bounced: 9, SpamComplaints: 1}, {Sent: 100, Bounced: 1}})
	if got.Sent != 1000 || got.Bounced != 10 || got.BounceRate != 1 || got.SpamComplaintsRate != 0.1 {
		t.Fatalf("totals = %+v", got)
	}
	if empty := overviewTotalsOf(nil); empty.BounceRate != 0 {
		t.Fatalf("empty totals = %+v", empty)
	}
}

func TestOverviewFanoutExcludesFailedServer(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("GET /servers", 200, postmarkServersPayload([3]any{1, "Alpha", "tok-a"}, [3]any{2, "Beta", "tok-b"}, [3]any{3, "Gamma", "tok-c"}))
	f.handle("GET /stats/outbound", func(r *http.Request, _ string) (int, any) {
		switch r.Header.Get(postmarkServerTokenHeader) {
		case "tok-a":
			return 200, map[string]any{"Sent": 100, "Bounced": 2, "BounceRate": 2.0, "SpamComplaints": 0, "SpamComplaintsRate": 0}
		case "tok-b":
			return 422, `{"ErrorCode":1500,"Message":"bad"}`
		default:
			return 200, map[string]any{"Sent": 300, "Bounced": 0, "BounceRate": 0, "SpamComplaints": 1, "SpamComplaintsRate": 0.333}
		}
	})
	f.reply("GET /message-streams", 200, map[string]any{"MessageStreams": []any{map[string]any{"ID": "outbound"}, map[string]any{"ID": "inbound", "MessageStreamType": "Inbound"}}})

	stdout, stderr, err := postmarkRun(t, "overview", "--json")
	if err != nil {
		t.Fatalf("overview: %v\n%s", err, stderr)
	}
	var res overviewResult
	postmarkResults(t, stdout, &res)
	if len(res.Servers) != 2 || res.Totals.Sent != 400 || res.Totals.Bounced != 2 || res.Totals.BounceRate != 0.5 {
		t.Fatalf("result = %+v", res)
	}
	if len(res.FetchFailures) != 1 || res.FetchFailures[0].Server != "Beta" {
		t.Fatalf("fetch_failures = %+v", res.FetchFailures)
	}
	if res.Servers[0].DeliveryType != "Live" || res.Servers[0].Streams != 2 {
		t.Fatalf("row = %+v", res.Servers[0])
	}
	if !strings.Contains(stderr, "1 of 3 server fetches failed") {
		t.Fatalf("stderr = %s", stderr)
	}
	for _, r := range f.log() {
		if r.Path == "/stats/outbound" && !strings.Contains(r.Query, "fromdate=") {
			t.Fatalf("stats query missing window: %q", r.Query)
		}
	}
}
