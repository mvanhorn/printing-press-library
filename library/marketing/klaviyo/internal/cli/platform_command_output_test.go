// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

func TestPlatformCommandWrapperEnvelopesBareJSONAndMarksCaps(t *testing.T) {
	flags := &rootFlags{asJSON: true, platformSession: platformCommandTestSession(t)}
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	receiptWriter, err := platform.NewReceiptWriter(flags.platformSession, platform.ReceiptOptions{CLI: platform.ReceiptCLI{Name: "fixture-pp-cli", Version: "test"}, ReceiptFile: receiptPath})
	if err != nil {
		t.Fatal(err)
	}
	flags.receiptWriter = receiptWriter
	root := &cobra.Command{Use: "fixture-pp-cli"}
	var limit int
	child := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), `[{"id":"one"}]`)
		return err
	}}
	child.Flags().IntVar(&limit, "limit", 1, "fixture cap")
	root.AddCommand(child)
	wrapPlatformCommandOutputs(root, flags)
	var output bytes.Buffer
	root.SetOut(receiptWriter.CaptureWriter("stdout", &output))
	root.SetArgs([]string{"list", "--limit", "1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data []map[string]any        `json:"data"`
		Meta platform.OutputMetadata `json:"meta"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not an envelope: %v: %s", err, output.String())
	}
	if len(envelope.Data) != 1 || !envelope.Meta.Truncated || len(envelope.Meta.TruncationReasons) == 0 || envelope.Meta.TruncationReasons[0].Kind != "limit" {
		t.Fatalf("envelope = %+v", envelope)
	}
	if err := receiptWriter.Finalize(context.Background(), "success", 0, nil, flags.platformSession.OutputMetadata(), flags.platformSession.RateLimitMetadata()); err != nil {
		t.Fatal(err)
	}
	receiptData, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt platform.Receipt
	if err := json.Unmarshal(receiptData, &receipt); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(output.Bytes()))
	if receipt.Rows["output"].Emitted != 1 || len(receipt.Outputs) != 1 || receipt.Outputs[0].SHA256 != digest {
		t.Fatalf("receipt rows/outputs = %+v / %+v", receipt.Rows, receipt.Outputs)
	}
}

func TestPlatformCommandWindowResolvesOnceFromFlags(t *testing.T) {
	flags := &rootFlags{platformSession: platformCommandTestSession(t)}
	cmd := &cobra.Command{Use: "report"}
	var since string
	cmd.Flags().StringVar(&since, "since", "30d", "fixture window")
	if err := cmd.Flags().Parse([]string{"--since", "7d"}); err != nil {
		t.Fatal(err)
	}
	if err := adoptPlatformCommandWindow(cmd, flags); err != nil {
		t.Fatal(err)
	}
	window := flags.platformSession.OutputMetadata().ResolvedWindow
	if window == nil || window.Requested.Days != 7 || window.Requested.Start != nil || window.Requested.End != nil || window.End.Sub(window.Start) != 7*24*time.Hour || !window.EndExclusive {
		t.Fatalf("resolved window = %+v", window)
	}
}

func TestPlatformCommandWindowPreservesCalendarInputs(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.March, 9, 12, 0, 0, 0, location)

	relative := &cobra.Command{Use: "relative"}
	var since string
	relative.Flags().StringVar(&since, "since", "", "fixture relative window")
	if err := relative.Flags().Parse([]string{"--since", "1d"}); err != nil {
		t.Fatal(err)
	}
	request, endExclusive, ok := platformWindowRequestFromFlags(relative, now, location, location.String())
	if !ok || !endExclusive || request.Days != 1 || request.Start != nil || request.End != nil {
		t.Fatalf("relative calendar request = %+v exclusive=%v ok=%v", request, endExclusive, ok)
	}

	absolute := &cobra.Command{Use: "absolute"}
	var startDate, endDate string
	absolute.Flags().StringVar(&startDate, "start-date", "", "fixture start")
	absolute.Flags().StringVar(&endDate, "end-date", "", "fixture end")
	if err := absolute.Flags().Parse([]string{"--start-date", "2026-03-07", "--end-date", "2026-03-09"}); err != nil {
		t.Fatal(err)
	}
	request, endExclusive, ok = platformWindowRequestFromFlags(absolute, now, location, location.String())
	if !ok || !endExclusive || !request.CompletePeriods || request.Start == nil || request.End == nil || request.End.Sub(*request.Start) != 71*time.Hour {
		t.Fatalf("absolute calendar request = %+v exclusive=%v ok=%v", request, endExclusive, ok)
	}

	monthCommand := &cobra.Command{Use: "monthly"}
	var month string
	monthCommand.Flags().StringVar(&month, "month", "", "fixture month")
	if err := monthCommand.Flags().Parse([]string{"--month", "2026-03"}); err != nil {
		t.Fatal(err)
	}
	request, endExclusive, ok = platformWindowRequestFromFlags(monthCommand, now, location, location.String())
	window, err := platform.ResolveWindow(request, platform.WindowPolicy{EndExclusive: endExclusive})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || request.Month != "2026-03" || request.Start != nil || request.End != nil || !window.CompletePeriods || !window.EndExclusive {
		t.Fatalf("calendar month request=%+v resolved=%+v ok=%v", request, window, ok)
	}

	mcpRelative, _, ok := platformWindowRequestFromValues(map[string]string{"since": "1d"}, now, location, location.String())
	if !ok || mcpRelative.Days != 1 || mcpRelative.Start != nil || mcpRelative.End != nil {
		t.Fatalf("MCP relative calendar request = %+v ok=%v", mcpRelative, ok)
	}
}

func TestPlatformMCPWindowPreservesCalendarMonthInput(t *testing.T) {
	session := platformCommandTestSession(t)
	if err := AdoptMCPOutputSemantics(session, map[string]any{"month": "2026-03", "timezone": "America/Los_Angeles"}); err != nil {
		t.Fatal(err)
	}
	window := session.OutputMetadata().ResolvedWindow
	if window == nil || window.Requested.Month != "2026-03" || !window.CompletePeriods || !window.EndExclusive || window.Start.Format("2006-01-02T15:04:05Z07:00") != "2026-03-01T00:00:00-08:00" || window.End.Format("2006-01-02T15:04:05Z07:00") != "2026-04-01T00:00:00-07:00" {
		t.Fatalf("MCP calendar month window = %+v", window)
	}
}

func TestPlatformCommandWindowDisclosesProviderClamp(t *testing.T) {
	flags := &rootFlags{platformSession: platformCommandTestSession(t)}
	previous := registeredPlatformSource
	registration := *previous
	registration.WindowMaximumDays = 364
	registration.WindowMaximumName = "fixture_364_day_maximum"
	registeredPlatformSource = &registration
	t.Cleanup(func() { registeredPlatformSource = previous })
	cmd := &cobra.Command{Use: "report"}
	var last string
	cmd.Flags().StringVar(&last, "last", "365d", "fixture provider window")
	if err := cmd.Flags().Parse([]string{"--last", "365d"}); err != nil {
		t.Fatal(err)
	}
	if err := adoptPlatformCommandWindow(cmd, flags); err != nil {
		t.Fatal(err)
	}
	window := flags.platformSession.OutputMetadata().ResolvedWindow
	if window == nil || !window.Clamped || window.ProviderMaximum != "fixture_364_day_maximum" || window.End.Sub(window.Start) != 364*24*time.Hour {
		t.Fatalf("clamped window = %+v", window)
	}
}

func TestPlatformCommandWrapperRejectsSilentAnalyticsAcrossFormats(t *testing.T) {
	tests := []struct {
		name   string
		flags  func(*platform.Session) *rootFlags
		output string
	}{
		{name: "json", flags: func(session *platform.Session) *rootFlags { return &rootFlags{asJSON: true, platformSession: session} }, output: `{"input_rows":1,"matched_rows":1,"total_revenue":0}`},
		{name: "csv", flags: func(session *platform.Session) *rootFlags { return &rootFlags{csv: true, platformSession: session} }, output: "total_revenue\n0"},
		{name: "human", flags: func(session *platform.Session) *rootFlags { return &rootFlags{platformSession: session} }, output: "Total revenue: 0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := test.flags(platformCommandTestSession(t))
			root := &cobra.Command{Use: "fixture-pp-cli"}
			var output, diagnostics bytes.Buffer
			zero := 0.0
			child := &cobra.Command{Use: "revenue", RunE: func(cmd *cobra.Command, _ []string) error {
				declarePlatformAnalytics(flags, platform.AnalyticsDeclaration{
					DataSource:       "fixture",
					RequiredInputs:   []string{"orders"},
					JoinKeys:         []string{},
					InputRows:        map[string]int64{"orders": 1},
					ValidJoinKeys:    map[string]int64{},
					MissingJoinKeys:  map[string]int64{},
					MatchedRows:      1,
					NewestRowAt:      map[string]time.Time{},
					FreshnessStatus:  map[string]string{"orders": "unknown"},
					BusinessMetrics:  map[string]*float64{"total_revenue": &zero},
					MinimumInputRows: map[string]int64{"orders": 1},
					Remediation:      "verify fixture order input and revenue calculation",
				})
				_, err := fmt.Fprintln(cmd.OutOrStdout(), test.output)
				return err
			}}
			root.AddCommand(child)
			wrapPlatformCommandOutputs(root, flags)
			root.SetOut(&output)
			root.SetErr(&diagnostics)
			root.SetArgs([]string{"revenue"})
			err := root.Execute()
			var failure *platform.AnalyticsFailure
			if !errors.As(err, &failure) || failure.Reason != "all_business_metrics_zero_or_null" {
				t.Fatalf("error = %v, want typed all-zero analytics failure", err)
			}
			diagnosticOutput := diagnostics.String()
			if flags.asJSON || flags.agent {
				diagnosticOutput = output.String()
			}
			if !strings.Contains(diagnosticOutput, `"code":"analytics_result_rejected"`) || strings.Contains(diagnosticOutput, "acct-a") {
				t.Fatalf("safe diagnostic = %q", diagnosticOutput)
			}
		})
	}
}

func TestPlatformAnalyticsCommandIncludesKlaviyoIntelligence(t *testing.T) {
	for _, name := range []string{
		"flow-cannibalization",
		"optimal-send-time",
		"revenue-per-email",
		"send-fatigue",
		"subject-line-analysis",
	} {
		t.Run(name, func(t *testing.T) {
			if !platformAnalyticsCommand(&cobra.Command{Use: name}) {
				t.Fatalf("%s was not classified as analytics", name)
			}
		})
	}
}

func TestPlatformAnalyticsDoesNotInventBusinessMetrics(t *testing.T) {
	flags := &rootFlags{asJSON: true, platformSession: platformCommandTestSession(t)}
	cmd := &cobra.Command{Use: "subject-line-analysis"}
	autoDeclarePlatformAnalytics(cmd, []byte(`{"campaigns":[{"id":"one"}]}`), flags)
	if flags.platformAnalytics == nil {
		t.Fatal("analytics declaration was not recorded")
	}
	for name, value := range flags.platformAnalytics.BusinessMetrics {
		if value != nil {
			t.Fatalf("invented business metric %s = %v", name, *value)
		}
	}
	var failure *platform.AnalyticsFailure
	if err := validatePlatformAnalytics(flags); !errors.As(err, &failure) || failure.Reason != "all_business_metrics_zero_or_null" {
		t.Fatalf("validation error = %#v, want typed all_business_metrics_zero_or_null", err)
	}
}

func TestPlatformAnalyticsSkipsDryRunPlans(t *testing.T) {
	flags := &rootFlags{asJSON: true, platformSession: platformCommandTestSession(t)}
	cmd := &cobra.Command{Use: "subject-line-analysis"}
	autoDeclarePlatformAnalytics(cmd, []byte(`{"dry_run":true,"planned_steps":["fetch_subjects"]}`), flags)
	if flags.platformAnalytics != nil {
		t.Fatalf("dry-run plan was treated as computed analytics: %#v", flags.platformAnalytics)
	}
}

func platformCommandTestSession(t *testing.T) *platform.Session {
	t.Helper()
	profile := &platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "tenant-a", Sources: map[string]platform.SourceProfile{"test-source": {CredentialRef: "op://Test Vault/Test Item/token", ExpectedAccountID: "acct-a"}}}
	session, err := platform.PrepareProfileSourceFromConfig(profile, "fixture-pp-cli", "test-source")
	if err != nil {
		t.Fatal(err)
	}
	session.GateOutcome = platform.GateVerified
	session.CredentialFingerprint = "fixture-fingerprint"
	session.ObservedIdentity = map[string]string{"account_id": "acct-a"}
	return session
}

func platformTestOutputPayload(t *testing.T, raw []byte) []byte {
	t.Helper()
	current := append([]byte(nil), raw...)
	for range 4 {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(current, &envelope); err != nil || envelope["meta"] == nil {
			return current
		}
		if envelope["results"] != nil {
			current = envelope["results"]
			continue
		}
		if envelope["data"] != nil {
			current = envelope["data"]
			continue
		}
		return current
	}
	t.Fatalf("output contains more than four nested platform envelopes: %s", raw)
	return nil
}
