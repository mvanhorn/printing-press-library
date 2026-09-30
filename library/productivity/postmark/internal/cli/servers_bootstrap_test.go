// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func TestNovelServersBootstrapHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"servers", "bootstrap", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("servers bootstrap --help error = %v", err)
	}
	for _, want := range []string{"servers bootstrap [name] [flags]", "--delivery-type", "Do NOT use it to change settings on an existing server"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestBootstrapEnvFileContent(t *testing.T) {
	cases := []struct{ existing, want string }{
		{"", "POSTMARK_SERVER_TOKEN=new\n"},
		{"APP=1\n", "APP=1\nPOSTMARK_SERVER_TOKEN=new\n"},
		{"APP=1\nPOSTMARK_SERVER_TOKEN=old\n\nOTHER=2", "APP=1\nPOSTMARK_SERVER_TOKEN=new\n\nOTHER=2\n"},
		{"export POSTMARK_SERVER_TOKEN=old\nPOSTMARK_SERVER_TOKEN=dup\n", "POSTMARK_SERVER_TOKEN=new\n"},
	}
	for _, tc := range cases {
		if got := bootstrapEnvFileContent(tc.existing, "new"); got != tc.want {
			t.Errorf("bootstrapEnvFileContent(%q) = %q, want %q", tc.existing, got, tc.want)
		}
	}
}

func TestBootstrapDNSRecords(t *testing.T) {
	pending := bootstrapDomain{DKIMHost: "old._domainkey.x.co", DKIMTextValue: "old", DKIMPendingHost: "new._domainkey.x.co", DKIMPendingTextValue: "k=rsa; p=NEW"}
	got := bootstrapDNSRecords(pending, "pm-bounces.x.co")
	want := []bootstrapDNSRecord{{Type: "TXT", Host: "new._domainkey.x.co", Value: "k=rsa; p=NEW"}, {Type: "CNAME", Host: "pm-bounces.x.co", Value: "pm.mtasv.net"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pending records = %+v", got)
	}
	verified := bootstrapDomain{DKIMHost: "k._domainkey.x.co", DKIMTextValue: "k=rsa; p=CUR", ReturnPathDomain: "bounce.x.co", ReturnPathDomainCNAMEValue: "pm.mtasv.net"}
	got = bootstrapDNSRecords(verified, "pm-bounces.x.co")
	if got[0].Host != "k._domainkey.x.co" || got[1].Host != "bounce.x.co" {
		t.Fatalf("verified records = %+v", got)
	}
	if got := bootstrapDNSRecords(bootstrapDomain{}, ""); len(got) != 0 {
		t.Fatalf("no domain info should give no records: %+v", got)
	}
}

func TestBootstrapValidate(t *testing.T) {
	o := bootstrapOptions{deliveryType: "Sandbox", domain: "Lumen.Example.com"}
	if err := bootstrapValidate(&o); err != nil || o.deliveryType != "Sandbox" || o.returnPath != "pm-bounces.lumen.example.com" {
		t.Fatalf("validate = %+v, %v", o, err)
	}
	bad := []bootstrapOptions{
		{deliveryType: "prod"},
		{returnPath: "b.x.co"},
		{domain: "x.co", returnPath: "b.other.co"},
		{webhookURL: "ftp://x"},
		{noVerifyWebhook: true},
	}
	for i, b := range bad {
		if err := bootstrapValidate(&b); err == nil {
			t.Errorf("case %d accepted: %+v", i, b)
		}
	}
}

const bootstrapSecret = "tok-lumen-SECRET-0000"

// bootstrapFake models an account with one existing server (Main App)
// and records state so a second run sees what the first one created.
type bootstrapFake struct {
	*postmarkFake
	mu            sync.Mutex
	lumenExists   bool
	broadcast     bool
	webhookURL    string
	webhook       map[string]any
	domainCreated bool
}

func newBootstrapFake(t *testing.T) *bootstrapFake {
	b := &bootstrapFake{postmarkFake: newPostmarkFake(t)}
	b.handle("GET /servers", func(r *http.Request, _ string) (int, any) {
		b.mu.Lock()
		defer b.mu.Unlock()
		servers := [][3]any{{11, "Main App", "tok-main"}}
		if b.lumenExists {
			servers = append(servers, [3]any{99, "Lumen", bootstrapSecret})
		}
		if name := r.URL.Query().Get("name"); name != "" {
			filtered := make([][3]any, 0)
			for _, s := range servers {
				if strings.Contains(strings.ToLower(s[1].(string)), strings.ToLower(name)) {
					filtered = append(filtered, s)
				}
			}
			servers = filtered
		}
		return 200, postmarkServersPayload(servers...)
	})
	b.handle("POST /servers", func(*http.Request, string) (int, any) {
		b.mu.Lock()
		b.lumenExists = true
		b.mu.Unlock()
		return 200, map[string]any{"ID": 99, "Name": "Lumen", "ApiTokens": []string{bootstrapSecret}}
	})
	b.handle("GET /message-streams", func(*http.Request, string) (int, any) {
		b.mu.Lock()
		defer b.mu.Unlock()
		streams := []map[string]any{{"ID": "outbound", "MessageStreamType": "Transactional"}, {"ID": "inbound", "MessageStreamType": "Inbound"}}
		if b.broadcast {
			streams = append(streams, map[string]any{"ID": "broadcast", "MessageStreamType": "Broadcasts"})
		}
		return 200, map[string]any{"MessageStreams": streams, "TotalCount": len(streams)}
	})
	b.handle("POST /message-streams", func(*http.Request, string) (int, any) {
		b.mu.Lock()
		b.broadcast = true
		b.mu.Unlock()
		return 200, map[string]any{"ID": "broadcast", "MessageStreamType": "Broadcasts"}
	})
	b.handle("GET /webhooks", func(*http.Request, string) (int, any) {
		b.mu.Lock()
		defer b.mu.Unlock()
		hooks := []map[string]any{}
		if b.webhook != nil {
			hooks = append(hooks, b.webhook)
		}
		return 200, map[string]any{"Webhooks": hooks}
	})
	b.handle("POST /webhooks", func(_ *http.Request, body string) (int, any) {
		var in map[string]any
		_ = json.Unmarshal([]byte(body), &in)
		b.mu.Lock()
		b.webhookURL, _ = in["Url"].(string)
		b.webhook = map[string]any{"ID": 5, "Url": in["Url"], "MessageStream": in["MessageStream"], "Triggers": in["Triggers"]}
		b.mu.Unlock()
		return 200, map[string]any{"ID": 5, "Url": in["Url"]}
	})
	domain := map[string]any{"ID": 77, "Name": "lumen.example.com", "DKIMPendingHost": "20260930pm._domainkey.lumen.example.com", "DKIMPendingTextValue": "k=rsa; p=ABC", "ReturnPathDomain": "pm-bounces.lumen.example.com", "ReturnPathDomainCNAMEValue": "pm.mtasv.net"}
	b.handle("GET /domains", func(*http.Request, string) (int, any) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if !b.domainCreated {
			return 200, map[string]any{"TotalCount": 0, "Domains": []any{}}
		}
		return 200, map[string]any{"TotalCount": 1, "Domains": []any{map[string]any{"ID": 77, "Name": "lumen.example.com"}}}
	})
	b.reply("GET /domains/77", 200, domain)
	b.handle("POST /domains", func(*http.Request, string) (int, any) {
		b.mu.Lock()
		b.domainCreated = true
		b.mu.Unlock()
		return 200, domain
	})
	b.reply("PUT /templates/push", 200, map[string]any{"TotalCount": 2, "Templates": []any{
		map[string]any{"Action": "Create", "Alias": "welcome", "Name": "Welcome"},
		map[string]any{"Action": "Create", "Alias": "password-reset", "Name": "Password reset"},
	}})
	return b
}

func requestSequence(reqs []postmarkFakeRequest) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, r.Method+" "+r.Path)
	}
	return out
}

func TestServersBootstrapApplyThenIdempotentRerun(t *testing.T) {
	f := newBootstrapFake(t)
	home := postmarkTestEnv(t, f.postmarkFake)
	envFile := filepath.Join(home, "project", ".env")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("APP_NAME=lumen\nPOSTMARK_SERVER_TOKEN=stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"servers", "bootstrap", "Lumen", "--domain", "lumen.example.com", "--broadcast-stream",
		"--webhook-url", "https://lumen.example.com/hooks/postmark", "--copy-templates-from", "Main App",
		"--env-file", envFile, "--apply", "--json"}

	stdout, stderr, err := postmarkRun(t, args...)
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, stderr)
	}
	if strings.Contains(stdout, bootstrapSecret) || strings.Contains(stderr, bootstrapSecret) {
		t.Fatal("server token leaked to output")
	}
	wantSeq := []string{
		"GET /servers",          // exact-name lookup
		"GET /servers",          // resolve --copy-templates-from
		"GET /domains",          // does the domain exist?
		"POST /servers",         // create
		"GET /message-streams",  // broadcast present?
		"POST /message-streams", // create broadcast
		"GET /webhooks",         // webhook present?
		"POST /webhooks",        // create webhook
		"POST /domains",         // add domain
		"PUT /templates/push",   // copy templates
	}
	reqs := f.log()
	if got := requestSequence(reqs); !reflect.DeepEqual(got, wantSeq) {
		t.Fatalf("request sequence:\n got %v\nwant %v", got, wantSeq)
	}
	if !strings.Contains(reqs[0].Query, "name=Lumen") {
		t.Fatalf("server lookup query = %q", reqs[0].Query)
	}
	for _, r := range reqs {
		account := postmarkIsAccountPath(r.Path)
		if account && (r.AccountTok != "acct-token" || r.ServerToken != "") {
			t.Fatalf("%s %s: account path headers wrong (%+v)", r.Method, r.Path, r)
		}
		if !account && (r.ServerToken != bootstrapSecret || r.AccountTok != "") {
			t.Fatalf("%s %s: must use the new server's token only (%+v)", r.Method, r.Path, r)
		}
	}
	var push map[string]any
	var hook map[string]any
	var created map[string]any
	for _, r := range reqs {
		switch r.Method + " " + r.Path {
		case "PUT /templates/push":
			_ = json.Unmarshal([]byte(r.Body), &push)
		case "POST /webhooks":
			_ = json.Unmarshal([]byte(r.Body), &hook)
		case "POST /servers":
			_ = json.Unmarshal([]byte(r.Body), &created)
		}
	}
	if push["SourceServerID"] != float64(11) || push["DestinationServerID"] != float64(99) || push["PerformChanges"] != true {
		t.Fatalf("template push body = %v", push)
	}
	if created["Name"] != "Lumen" || created["DeliveryType"] != "Live" {
		t.Fatalf("server create body = %v", created)
	}
	triggers, _ := hook["Triggers"].(map[string]any)
	for _, trig := range []string{"Bounce", "SpamComplaint", "Delivery"} {
		if tr, _ := triggers[trig].(map[string]any); tr["Enabled"] != true {
			t.Fatalf("webhook trigger %s not enabled: %v", trig, hook)
		}
	}

	var res bootstrapResult
	postmarkResults(t, stdout, &res)
	wantDNS := []bootstrapDNSRecord{
		{Type: "TXT", Host: "20260930pm._domainkey.lumen.example.com", Value: "k=rsa; p=ABC"},
		{Type: "CNAME", Host: "pm-bounces.lumen.example.com", Value: "pm.mtasv.net"},
	}
	if !res.Applied || res.ServerID != 99 || !reflect.DeepEqual(res.DNSRecords, wantDNS) || len(res.Templates) != 2 {
		t.Fatalf("result = %+v", res)
	}
	info, err := os.Stat(envFile)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("env file mode = %v, %v", info.Mode(), err)
	}
	envData, _ := os.ReadFile(envFile)
	if string(envData) != "APP_NAME=lumen\nPOSTMARK_SERVER_TOKEN="+bootstrapSecret+"\n" {
		t.Fatalf("env file = %q", envData)
	}

	// Rerun: the server, stream, webhook, and domain exist, so nothing is created.
	before := len(f.log())
	stdout, stderr, err = postmarkRun(t, args...)
	if err != nil {
		t.Fatalf("rerun: %v\n%s", err, stderr)
	}
	if strings.Contains(stdout, bootstrapSecret) {
		t.Fatal("server token leaked on rerun")
	}
	rerun := requestSequence(f.log()[before:])
	for _, forbidden := range []string{"POST /servers", "POST /message-streams", "POST /webhooks", "POST /domains"} {
		for _, got := range rerun {
			if got == forbidden {
				t.Fatalf("rerun issued %s: %v", forbidden, rerun)
			}
		}
	}
	var again bootstrapResult
	postmarkResults(t, stdout, &again)
	if !again.ServerExists || again.ServerID != 99 {
		t.Fatalf("rerun result = %+v", again)
	}
	for _, s := range again.Steps {
		if s.Step != bootstrapStepTemplates && s.Step != bootstrapStepEnvFile && s.Action != bootstrapActionExists {
			t.Fatalf("rerun step %s action %s, want exists", s.Step, s.Action)
		}
	}
}

func TestServersBootstrapPlanMakesNoWrites(t *testing.T) {
	f := newBootstrapFake(t)
	home := postmarkTestEnv(t, f.postmarkFake)
	envFile := filepath.Join(home, ".env")
	stdout, stderr, err := postmarkRun(t, "servers", "bootstrap", "Lumen", "--domain", "lumen.example.com", "--broadcast-stream", "--copy-templates-from", "Main App", "--env-file", envFile, "--json")
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, stderr)
	}
	for _, r := range f.log() {
		if r.Method != http.MethodGet {
			t.Fatalf("plan mode issued %s %s", r.Method, r.Path)
		}
	}
	if _, err := os.Stat(envFile); !os.IsNotExist(err) {
		t.Fatal("plan mode wrote the env file")
	}
	var res bootstrapResult
	postmarkResults(t, stdout, &res)
	if res.Applied || res.ServerExists || res.Steps[0].Action != bootstrapActionCreate || res.Next == "" {
		t.Fatalf("plan = %+v", res)
	}
}

func TestServersBootstrapRequiresName(t *testing.T) {
	f := newBootstrapFake(t)
	postmarkTestEnv(t, f.postmarkFake)
	_, _, err := postmarkRun(t, "servers", "bootstrap", "--apply", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("missing name should exit 2, got %v", err)
	}
	if len(f.log()) != 0 {
		t.Fatal("usage error made requests")
	}
}

func TestBootstrapWebhookEnablesMissingTriggers(t *testing.T) {
	b := newBootstrapFake(t)
	postmarkTestEnv(t, b.postmarkFake)
	const hook = "https://lumen.example.com/hooks/postmark"
	b.lumenExists = true
	b.webhookURL = hook
	b.webhook = map[string]any{"ID": 5, "Url": hook, "MessageStream": "outbound", "Triggers": map[string]any{
		"Bounce": map[string]any{"Enabled": false, "IncludeContent": true}, "SpamComplaint": map[string]any{"Enabled": true}, "Delivery": map[string]any{"Enabled": false},
	}}
	var putBody string
	b.handle("PUT /webhooks/5", func(_ *http.Request, body string) (int, any) {
		putBody = body
		return 200, map[string]any{"ID": 5, "Url": hook}
	})

	plan, _, err := postmarkRun(t, "servers", "bootstrap", "Lumen", "--webhook-url", hook, "--json")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !strings.Contains(plan, `"update"`) || !strings.Contains(plan, "enable Bounce, Delivery on webhook 5") {
		t.Fatalf("plan should update webhook 5 to enable Bounce and Delivery:\n%s", plan)
	}
	if putBody != "" {
		t.Fatal("plan mode must not edit the webhook")
	}

	if _, _, err := postmarkRun(t, "servers", "bootstrap", "Lumen", "--webhook-url", hook, "--apply", "--json"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var sent struct {
		Triggers map[string]map[string]bool `json:"Triggers"`
	}
	if err := json.Unmarshal([]byte(putBody), &sent); err != nil {
		t.Fatalf("PUT body %q: %v", putBody, err)
	}
	if len(sent.Triggers) != 2 || !sent.Triggers["Delivery"]["Enabled"] || !sent.Triggers["Bounce"]["Enabled"] {
		t.Fatalf("PUT should enable only Bounce and Delivery, got %s", putBody)
	}
	if ic, ok := sent.Triggers["Bounce"]["IncludeContent"]; !ok || !ic {
		t.Fatalf("PUT must keep Bounce IncludeContent=true, got %s", putBody)
	}
	for _, r := range b.log() {
		if r.Method == "POST" && r.Path == "/webhooks" {
			t.Fatal("an existing outbound webhook must be updated, not duplicated")
		}
	}
}

func TestBootstrapWebhookOnOtherStreamIsNotReused(t *testing.T) {
	b := newBootstrapFake(t)
	postmarkTestEnv(t, b.postmarkFake)
	const hook = "https://lumen.example.com/hooks/postmark"
	b.lumenExists = true
	b.webhookURL = hook
	b.webhook = map[string]any{"ID": 5, "Url": hook, "MessageStream": "broadcast", "Triggers": map[string]any{
		"Bounce": map[string]any{"Enabled": true}, "SpamComplaint": map[string]any{"Enabled": true}, "Delivery": map[string]any{"Enabled": true},
	}}
	plan, _, err := postmarkRun(t, "servers", "bootstrap", "Lumen", "--webhook-url", hook, "--json")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !strings.Contains(plan, "create a bounce, spam complaint, and delivery webhook to "+hook) {
		t.Fatalf("a same-URL webhook on another stream should not satisfy the step:\n%s", plan)
	}
}

func TestSameWebhookURL(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://lumen.example.com/hooks/postmark", "https://lumen.example.com/hooks/postmark", true},
		{"https://lumen.example.com/hooks/postmark", "https://lumen.example.com/hooks/postmark/", false},
		{"https://hooks@lumen.example.com/hooks", "https://lumen.example.com/hooks", false},
		{"https://lumen.example.com/hooks", "https://lumen.example.com/hooks?", false},
		{"https://lumen.example.com/hooks#A", "https://lumen.example.com/hooks#%41", false},
		{"https://Lumen.Example.com/hooks/postmark", "HTTPS://lumen.example.com/hooks/postmark", true},
		{"https://lumen.example.com/hooks/Postmark", "https://lumen.example.com/hooks/postmark", false},
		{"https://lumen.example.com/hooks?token=A", "https://lumen.example.com/hooks?token=a", false},
		{"https://lumen.example.com/hooks", "http://lumen.example.com/hooks", false},
	}
	for _, tc := range cases {
		if got := sameWebhookURL(tc.a, tc.b); got != tc.want {
			t.Errorf("sameWebhookURL(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestBootstrapWebhookUpdateHonorsNoVerify(t *testing.T) {
	b := newBootstrapFake(t)
	postmarkTestEnv(t, b.postmarkFake)
	const hook = "https://lumen.example.com/hooks/postmark"
	b.lumenExists = true
	b.webhookURL = hook
	b.webhook = map[string]any{"ID": 5, "Url": hook, "MessageStream": "outbound", "Triggers": map[string]any{
		"Bounce": map[string]any{"Enabled": true}, "SpamComplaint": map[string]any{"Enabled": false, "IncludeContent": false}, "Delivery": map[string]any{"Enabled": true},
	}}
	var putBody, putQuery string
	b.handle("PUT /webhooks/5", func(r *http.Request, body string) (int, any) {
		putBody, putQuery = body, r.URL.RawQuery
		return 200, map[string]any{"ID": 5, "Url": hook}
	})
	if _, _, err := postmarkRun(t, "servers", "bootstrap", "Lumen", "--webhook-url", hook, "--no-verify-webhook", "--apply", "--json"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var sent struct {
		Verify   *bool                     `json:"Verify"`
		Triggers map[string]map[string]any `json:"Triggers"`
	}
	if err := json.Unmarshal([]byte(putBody), &sent); err != nil {
		t.Fatalf("PUT body %q: %v", putBody, err)
	}
	if sent.Verify == nil || *sent.Verify || !strings.Contains(putQuery, "verify=false") {
		t.Fatalf("--no-verify-webhook not honored on update: body=%s query=%s", putBody, putQuery)
	}
	if ic, ok := sent.Triggers["SpamComplaint"]["IncludeContent"]; !ok || ic != false {
		t.Fatalf("IncludeContent=false must be kept: %s", putBody)
	}
}
