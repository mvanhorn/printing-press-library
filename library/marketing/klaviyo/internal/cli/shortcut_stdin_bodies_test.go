// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

func TestReadShortcutBodyOfType(t *testing.T) {
	for _, tt := range []struct {
		name    string
		input   string
		wantErr string
	}{
		{"preserves numbers", `{"data":{"attributes":{"amount":1.2300e+10,"id":9007199254740993},"type":"profile"}}`, ""},
		{"missing data", `{"meta":{}}`, `stdin data.type must be "profile"`},
		{"missing type", `{"data":{}}`, `stdin data.type must be "profile"`},
		{"wrong type", `{"data":{"type":"campaign"}}`, `stdin data.type must be "profile"`},
		{"array type", `{"data":{"type":[]}}`, `stdin data.type must be "profile"`},
		{"malformed JSON", `{"data":`, "parsing stdin JSON object: unexpected EOF"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetIn(strings.NewReader(tt.input))
			body, err := readShortcutBodyOfType(cmd, "profile")
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			attrs := body["data"].(map[string]any)["attributes"].(map[string]any)
			if attrs["id"] != json.Number("9007199254740993") || attrs["amount"] != json.Number("1.2300e+10") {
				t.Fatalf("numbers lost precision or type: %#v", attrs)
			}
			got, err := json.Marshal(body)
			if err != nil || string(got) != tt.input {
				t.Fatalf("body = %s, error = %v, want %s", got, err, tt.input)
			}
		})
	}
}

func TestWriteShortcutsStdinBodies(t *testing.T) {
	endpoints := []struct {
		name     string
		bodyType string
		newCmd   func(*rootFlags) *cobra.Command
		query    url.Values
		args     []string
	}{
		{"back-in-stock-subscriptions", "back-in-stock-subscription", newBackInStockSubscriptionsPromotedCmd, nil, nil},
		{"campaign-clone", "campaign", newCampaignClonePromotedCmd, url.Values{"fields[campaign]": {"id,name"}}, []string{"--fields-campaign", "id,name"}},
		{"conversation-messages", "conversation-message", newConversationMessagesPromotedCmd, nil, nil},
		{"data-privacy-deletion-jobs", "data-privacy-deletion-job", newDataPrivacyDeletionJobsPromotedCmd, nil, nil},
		{"data-source-record-bulk-create-jobs", "data-source-record-bulk-create-job", newDataSourceRecordBulkCreateJobsPromotedCmd, nil, nil},
		{"data-source-record-create-jobs", "data-source-record-create-job", newDataSourceRecordCreateJobsPromotedCmd, nil, nil},
		{"event-bulk-create-jobs", "event-bulk-create-job", newEventBulkCreateJobsPromotedCmd, nil, nil},
		{"metric-aggregates", "metric-aggregate", newMetricAggregatesPromotedCmd, url.Values{"fields[metric-aggregate]": {"id,data"}}, []string{"--fields-metric-aggregate", "id,data"}},
		{"profile-import", "profile", newProfileImportPromotedCmd, url.Values{"fields[profile]": {"id,email"}, "additional-fields[profile]": {"subscriptions"}}, []string{"--fields-profile", "id,email", "--additional-fields-profile", "subscriptions"}},
		{"profile-merge", "profile-merge", newProfileMergePromotedCmd, url.Values{"fields[profile]": {"id,email"}}, []string{"--fields-profile", "id,email"}},
		{"profile-subscription-bulk-create-jobs", "profile-subscription-bulk-create-job", newProfileSubscriptionBulkCreateJobsPromotedCmd, nil, nil},
		{"profile-subscription-bulk-delete-jobs", "profile-subscription-bulk-delete-job", newProfileSubscriptionBulkDeleteJobsPromotedCmd, nil, nil},
	}

	previousRegistration := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previousRegistration })
	registeredPlatformSource = nil // Test the shortcut wire contract, independent of tenant setup.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KLAVIYO_API_KEY", "test-key")

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			// Canonical key order lets us compare wire bytes, including number spelling.
			body := fmt.Sprintf(`{"data":{"attributes":{"large":9007199254740993,"nested":[false,null,{"amount":1.2300e+10}]},"relationships":{"profile":{"data":{"id":"profile-1","type":"profile"}}},"type":%q},"meta":{"source":"contract-test"}}`, endpoint.bodyType)
			var formatted bytes.Buffer
			if err := json.Indent(&formatted, []byte(body), "", "  "); err != nil {
				t.Fatal(err)
			}
			typeError := fmt.Sprintf("stdin data.type must be %q", endpoint.bodyType)
			tests := []struct {
				name      string
				input     string
				stdin     bool
				dryRun    bool
				withQuery bool
				wantErr   string
			}{
				{name: "forward body", input: body, stdin: true},
				{name: "forward formatted body and query", input: formatted.String(), stdin: true, withQuery: true},
				{name: "wrong type", input: `{"data":{"type":"wrong"}}`, stdin: true, wantErr: typeError},
				{name: "missing type", input: `{"data":{}}`, stdin: true, wantErr: typeError},
				{name: "missing data", input: `{"meta":{}}`, stdin: true, wantErr: typeError},
				{name: "null data", input: `{"data":null}`, stdin: true, wantErr: typeError},
				{name: "array data", input: `{"data":[]}`, stdin: true, wantErr: typeError},
				{name: "null type", input: `{"data":{"type":null}}`, stdin: true, wantErr: typeError},
				{name: "numeric type", input: `{"data":{"type":1}}`, stdin: true, wantErr: typeError},
				{name: "object type", input: `{"data":{"type":{}}}`, stdin: true, wantErr: typeError},
				{name: "empty stdin", stdin: true, wantErr: "parsing stdin JSON object: EOF"},
				{name: "null body", input: `null`, stdin: true, wantErr: "stdin must contain a non-empty JSON request object"},
				{name: "empty object", input: `{}`, stdin: true, wantErr: "stdin must contain a non-empty JSON request object"},
				{name: "malformed body", input: `{"data":`, stdin: true, wantErr: "parsing stdin JSON object: unexpected EOF"},
				{name: "multiple bodies", input: body + ` {}`, stdin: true, wantErr: "stdin must contain a single JSON request object"},
				{name: "trailing junk", input: body + ` trailing`, stdin: true, wantErr: "stdin must contain a single JSON request object"},
				{name: "no stdin flag", input: body, wantErr: endpoint.name + " needs a request body: pipe the JSON:API body to --stdin"},
				{name: "dry run body", input: body, stdin: true, dryRun: true, withQuery: true},
				{name: "dry run without stdin", dryRun: true},
				{name: "dry run wrong type", input: `{"data":{"type":"wrong"}}`, stdin: true, dryRun: true, wantErr: typeError},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var calls atomic.Int32
					wantURI := "/api/" + endpoint.name
					if tt.withQuery && len(endpoint.query) > 0 {
						wantURI += "?" + endpoint.query.Encode()
					}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls.Add(1)
						if r.Method != http.MethodPost || r.URL.RequestURI() != wantURI {
							t.Errorf("request = %s %s, want POST %s", r.Method, r.URL.RequestURI(), wantURI)
						}
						for header, want := range map[string]string{"revision": "2026-04-15", "Content-Type": "application/json", "User-Agent": "klaviyo-pp-cli/2026-04-15", "Authorization": "Klaviyo-API-Key test-key"} {
							if got := r.Header.Get(header); got != want {
								t.Errorf("%s = %q, want %q", header, got, want)
							}
						}
						got, err := io.ReadAll(r.Body)
						if err != nil {
							t.Errorf("read request: %v", err)
						} else if string(got) != body {
							t.Errorf("request body = %s, want exact JSON bytes %s", got, body)
						}
						w.Header().Set("Content-Type", "application/vnd.api+json")
						_, _ = io.WriteString(w, `{"data":{"id":"ok","type":"test-result"}}`)
					}))
					defer server.Close()
					t.Setenv("KLAVIYO_BASE_URL", server.URL)

					flags := &rootFlags{asJSON: true}
					cmd := endpoint.newCmd(flags)
					cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Show request without sending")
					var output bytes.Buffer
					cmd.SetOut(&output)
					cmd.SetErr(io.Discard)
					cmd.SetIn(strings.NewReader(tt.input))
					var args []string
					if tt.stdin {
						args = append(args, "--stdin")
					}
					if tt.dryRun {
						args = append(args, "--dry-run")
					}
					if tt.withQuery {
						args = append(args, endpoint.args...)
					}
					cmd.SetArgs(args)

					// The shared HTTP client prints its preview to process stderr.
					var preview *os.File
					if tt.dryRun && tt.wantErr == "" {
						var err error
						preview, err = os.CreateTemp(t.TempDir(), "preview")
						if err != nil {
							t.Fatal(err)
						}
						previousStderr := os.Stderr
						os.Stderr = preview
						defer func() { os.Stderr = previousStderr; preview.Close() }()
					}
					err := cmd.Execute()
					if tt.wantErr != "" {
						if err == nil || err.Error() != tt.wantErr {
							t.Errorf("error = %v, want %q", err, tt.wantErr)
						}
					} else if err != nil {
						t.Errorf("execute: %v", err)
					}
					wantCalls := int32(1)
					if tt.dryRun || tt.wantErr != "" {
						wantCalls = 0
					}
					if got := calls.Load(); got != wantCalls {
						t.Errorf("HTTP calls = %d, want %d", got, wantCalls)
					}
					if preview != nil {
						if _, err := preview.Seek(0, io.SeekStart); err != nil {
							t.Fatal(err)
						}
						got, err := io.ReadAll(preview)
						if err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(string(got), "POST "+server.URL+wantURI+"\n") || !strings.Contains(string(got), "(dry run - no request sent)") {
							t.Errorf("missing dry-run request preview: %s", got)
						}
						_, bodyPreview, ok := strings.Cut(string(got), "  Body:\n")
						var request json.RawMessage
						if !ok || json.NewDecoder(strings.NewReader(bodyPreview)).Decode(&request) != nil {
							t.Fatalf("missing dry-run body: %s", got)
						}
						var compact bytes.Buffer
						if err := json.Compact(&compact, request); err != nil {
							t.Fatal(err)
						}
						wantBody := body
						if !tt.stdin {
							wantBody = "{}"
						}
						if compact.String() != wantBody {
							t.Errorf("preview body = %s, want %s", compact.String(), wantBody)
						}
					}
				})
			}
		})
	}
}
