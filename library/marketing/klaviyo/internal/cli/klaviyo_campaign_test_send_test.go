// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
)

func executeCampaignTestSend(t *testing.T, flags *rootFlags, args ...string) (campaignTestSendResult, string, error) {
	t.Helper()
	cmd := newCampaignsTestSendCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var result campaignTestSendResult
	if flags.asJSON && out.Len() > 0 {
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatalf("invalid JSON output: %v: %s", decodeErr, &out)
		}
	}
	return result, out.String(), err
}

func TestCampaignTestSendSharesQAAndPostsEveryEmailVariation(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("QA_fail=%t", fail), func(t *testing.T) {
			f := &campaignModelFixture{t: t, multi: true, bodies: map[string]map[string]any{}}
			if fail {
				f.html = map[string]string{"template-2": "<p>Missing links and unsubscribe</p>"}
			}
			var bodies []map[string]any
			var sentAt []time.Time
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/template-preview-send-jobs" {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					bodies = append(bodies, body)
					sentAt = append(sentAt, time.Now())
					w.WriteHeader(http.StatusAccepted)
					return
				}
				f.ServeHTTP(w, r)
			}))
			qaCmd := newPlanQAGateCmd(&rootFlags{asJSON: true, noCache: true, rateLimit: 10000})
			var qaOut bytes.Buffer
			qaCmd.SetOut(&qaOut)
			qaCmd.SetErr(io.Discard)
			qaCmd.SetArgs([]string{"--campaign-id", modelContainerID})
			qaErr := qaCmd.Execute()
			var qa struct {
				Verdict  string           `json:"verdict"`
				Findings []map[string]any `json:"findings"`
			}
			if err := json.Unmarshal(qaOut.Bytes(), &qa); err != nil {
				t.Fatal(err)
			}
			result, _, err := executeCampaignTestSend(t, &rootFlags{asJSON: true, rateLimit: 10000}, "--campaign-id", modelContainerID, "--to", "one@example.com,two@example.com")
			if result.CampaignID != modelContainerID || result.QAVerdict != qa.Verdict || !reflect.DeepEqual(result.Findings, qa.Findings) {
				t.Fatalf("test-send QA differs from qa-gate: %+v / %+v", result, qa)
			}
			if fail {
				if err == nil || qaErr == nil || ExitCode(err) != ExitCode(qaErr) || len(bodies) != 0 {
					t.Fatalf("failed QA: test-send=%v qa-gate=%v posts=%d", err, qaErr, len(bodies))
				}
				return
			}
			if err != nil || qaErr != nil || len(bodies) != 2 || len(result.Variations) != 2 || result.QAVerdict != "warn" {
				t.Fatalf("test-send=%v qa-gate=%v posts=%d result=%+v", err, qaErr, len(bodies), result)
			}
			if sentAt[1].Sub(sentAt[0]) < time.Minute/15 {
				t.Fatal("preview jobs exceeded the steady rate")
			}
			for i, templateID := range []string{"TPL0001", "template-2"} {
				var expected map[string]any
				json.Unmarshal([]byte(fmt.Sprintf(`{"data":{"type":"template-preview-send-job","attributes":{"recipients":["one@example.com","two@example.com"]},"relationships":{"template":{"data":{"type":"template","id":%q}}}}}`, templateID)), &expected)
				if !reflect.DeepEqual(bodies[i], expected) {
					t.Fatalf("body=%v want=%v", bodies[i], expected)
				}
				v := result.Variations[i]
				if v.Result != "sent" || v.TemplateID != templateID || v.Subject != "A useful subject" || len(v.Recipients) != 2 {
					t.Fatalf("variation=%+v", v)
				}
			}
		})
	}
}

func TestCampaignTestSendRecipientInputs(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		to, want  []string
		bad       bool
	}{
		{name: "repeat comma trim dedupe", to: []string{"one@example.com,two@example.com", " one@example.com ", "ONE@example.com"}, want: []string{"one@example.com", "two@example.com"}},
		{name: "environment fallback", env: " one@example.com, two@example.com,one@example.com ", want: []string{"one@example.com", "two@example.com"}},
		{name: "explicit overrides environment", env: "invalid", to: []string{"one@example.com"}, want: []string{"one@example.com"}},
		{name: "five allowed", to: []string{"a@example.com,b@example.com,c@example.com,d@example.com,e@example.com"}, want: []string{"a@example.com", "b@example.com", "c@example.com", "d@example.com", "e@example.com"}},
		{name: "missing", bad: true},
		{name: "bad shape", to: []string{"not-an-email"}, bad: true},
		{name: "display name", to: []string{"Person <one@example.com>"}, bad: true},
		{name: "newline", to: []string{"one@example.com\nBcc:two@example.com"}, bad: true},
		{name: "empty explicit does not fall back", env: "one@example.com", to: []string{""}, bad: true},
		{name: "too many flags", to: []string{"a@example.com,b@example.com,c@example.com,d@example.com,e@example.com,f@example.com"}, bad: true},
		{name: "too many environment", env: "a@example.com,b@example.com,c@example.com,d@example.com,e@example.com,f@example.com", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KLAVIYO_TEST_SEND_TO", tc.env)
			f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
			posts := 0
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					w.WriteHeader(http.StatusAccepted)
					return
				}
				f.ServeHTTP(w, r)
			}))
			args := []string{"--campaign-id", modelMessageID}
			for _, value := range tc.to {
				args = append(args, "--to", value)
			}
			result, _, err := executeCampaignTestSend(t, &rootFlags{asJSON: true, rateLimit: 10000}, args...)
			if tc.bad {
				if err == nil || ExitCode(err) != 2 || posts != 0 || len(f.requests) != 0 {
					t.Fatalf("invalid input: error=%v posts=%d reads=%v", err, posts, f.requests)
				}
			} else if err != nil || posts != 1 || !reflect.DeepEqual(result.Variations[0].Recipients, tc.want) {
				t.Fatalf("error=%v posts=%d result=%+v", err, posts, result)
			}
		})
	}
}

func TestCampaignTestSendVariationScopeAndDryRun(t *testing.T) {
	for _, tc := range []struct {
		name, id, variation string
		dry, bad, qaFail    bool
		want                int
	}{
		{name: "all dry run", id: modelContainerID, dry: true, want: 2},
		{name: "selected variation", id: modelContainerID, variation: "variation-2", want: 1},
		{name: "message scope", id: modelMessageID, want: 1},
		{name: "non-email variation", id: modelContainerID, variation: "sms-variation", bad: true},
		{name: "unrelated variation", id: modelContainerID, variation: "not-in-campaign", bad: true},
		{name: "selected variation still gates whole campaign", id: modelContainerID, variation: modelVariationID, qaFail: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &campaignModelFixture{t: t, multi: true, bodies: map[string]map[string]any{}}
			if tc.qaFail {
				f.html = map[string]string{"template-2": "<p>No unsubscribe</p>"}
			}
			posts := 0
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					w.WriteHeader(http.StatusAccepted)
					return
				}
				f.ServeHTTP(w, r)
			}))
			args := []string{"--campaign-id", tc.id, "--to", "one@example.com"}
			if tc.variation != "" {
				args = append(args, "--variation-id", tc.variation)
			}
			result, _, err := executeCampaignTestSend(t, &rootFlags{asJSON: true, dryRun: tc.dry, rateLimit: 10000}, args...)
			if tc.bad {
				if err == nil || ExitCode(err) != 3 || posts != 0 {
					t.Fatalf("error=%v posts=%d", err, posts)
				}
				return
			}
			if tc.qaFail {
				if err == nil || result.QAVerdict != "fail" || posts != 0 {
					t.Fatalf("error=%v posts=%d QA=%s", err, posts, result.QAVerdict)
				}
				return
			}
			if err != nil || len(result.Variations) != tc.want || result.CampaignID != modelContainerID {
				t.Fatalf("error=%v result=%+v", err, result)
			}
			if tc.dry {
				if posts != 0 || result.Variations[0].Result != "dry-run" || !strings.Contains(strings.Join(f.requests, " "), "/api/templates/") {
					t.Fatalf("dry run posts=%d result=%+v reads=%v", posts, result, f.requests)
				}
			} else if posts != tc.want {
				t.Fatalf("posts=%d want=%d", posts, tc.want)
			}
			if tc.id == modelMessageID && strings.Contains(strings.Join(f.requests, " "), "message-2") {
				t.Fatal("test-send expanded historical message scope")
			}
		})
	}
}

func TestCampaignTestSendStopsWithoutRetry(t *testing.T) {
	for _, code := range []int{http.StatusTooManyRequests, http.StatusBadGateway, http.StatusForbidden} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			f := &campaignModelFixture{t: t, multi: true, bodies: map[string]map[string]any{}}
			posts := 0
			campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/template-preview-send-jobs" {
					posts++
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(code)
					fmt.Fprint(w, `{"errors":[{"detail":"rejected"}]}`)
					return
				}
				f.ServeHTTP(w, r)
			}))
			registerClientHook(func(c *client.Client) error {
				c.Config.Headers = map[string]string{"Idempotency-Key": "test-preview-job"}
				return nil
			})
			result, out, err := executeCampaignTestSend(t, &rootFlags{asJSON: true, rateLimit: 10000}, "--campaign-id", modelContainerID, "--to", "one@example.com")
			if err == nil || posts != 1 || len(result.Variations) != 2 || result.Variations[0].Result != "failed" || result.Variations[1].Result != "failed" || !strings.HasPrefix(result.Variations[1].Reason, "not sent:") {
				t.Fatalf("error=%v posts=%d output=%s", err, posts, out)
			}
			if code == http.StatusTooManyRequests && (!strings.Contains(out, "Klaviyo allows 100 test sends a day") || ExitCode(err) != 7) {
				t.Fatalf("rate limit error=%v output=%s", err, out)
			}
			if code >= 500 && !strings.Contains(out, "unknown, check the inbox") {
				t.Fatalf("ambiguous error=%v output=%s", err, out)
			}
		})
	}
}

func TestCampaignTestSendRegisteredAndHumanSummary(t *testing.T) {
	f := &campaignModelFixture{t: t, bodies: map[string]map[string]any{}}
	campaignFixtureClient(t, f)
	flags := &rootFlags{}
	root := newRootCmd(flags)
	cmd, _, err := root.Find([]string{"campaigns", "test-send"})
	if err != nil || cmd.Name() != "test-send" {
		t.Fatalf("registration: cmd=%v error=%v", cmd, err)
	}
	// Exercise inherited --dry-run and --json through the command users invoke.
	root.PersistentPreRunE = nil
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"campaigns", "test-send", "--campaign-id", modelMessageID, "--to", "one@example.com", "--dry-run", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result campaignTestSendResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Variations[0].Result != "dry-run" {
		t.Fatalf("root output=%s error=%v", &out, err)
	}
	_, human, err := executeCampaignTestSend(t, &rootFlags{dryRun: true, rateLimit: 10000}, "--campaign-id", modelMessageID, "--to", "one@example.com")
	for _, want := range []string{"QA: warn", "warn:", "dry-run", "TPL0001", "A useful subject", "one@example.com"} {
		if err != nil || !strings.Contains(human, want) {
			t.Fatalf("human output missing %q: %s error=%v", want, human, err)
		}
	}
}

func TestCampaignTestSendCancelledPacingStopsNextJob(t *testing.T) {
	f := &campaignModelFixture{t: t, multi: true, bodies: map[string]map[string]any{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	posts := 0
	campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			w.WriteHeader(http.StatusAccepted)
			cancel()
			return
		}
		f.ServeHTTP(w, r)
	}))
	cmd := newCampaignsTestSendCmd(&rootFlags{asJSON: true, rateLimit: 10000})
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--campaign-id", modelContainerID, "--to", "one@example.com"})
	if err := cmd.Execute(); err == nil || posts != 1 {
		t.Fatalf("error=%v posts=%d output=%s", err, posts, &out)
	}
}
