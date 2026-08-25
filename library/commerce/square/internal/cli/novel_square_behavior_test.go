// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/store"
	"github.com/spf13/cobra"
)

func TestNovelLocalCommandsRejectLiveDataSource(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(*rootFlags) *cobra.Command
		args []string
	}{
		{"reconcile close", newNovelReconcileCloseCmd, nil},
		{"inventory drift", newNovelInventoryDriftCmd, nil},
		{"customer timeline", newNovelCustomerTimelineCmd, []string{"CUSTOMER_1"}},
		{"webhook health", newNovelWebhookHealthCmd, nil},
		{"service review", newNovelServiceReviewCmd, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd(&rootFlags{dataSource: "live", maxAge: 30 * time.Minute})
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "no live equivalent") {
				t.Fatalf("error = %v, want local-only rejection", err)
			}
		})
	}
}

func TestNovelLocalCommandsRejectLiveDataSourceDuringDryRun(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(*rootFlags) *cobra.Command
		args []string
	}{
		{"inventory drift", newNovelInventoryDriftCmd, nil},
		{"customer timeline", newNovelCustomerTimelineCmd, []string{"CUSTOMER_1"}},
		{"webhook health", newNovelWebhookHealthCmd, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd(&rootFlags{dataSource: "live", dryRun: true, maxAge: 30 * time.Minute})
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "no live equivalent") {
				t.Fatalf("dry-run error = %v, want local-only rejection", err)
			}
		})
	}
}

func TestLoadLocalSquareRecordsAndCustomerMatch(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw := json.RawMessage(`{"id":"PAY_1","customer_id":"CUSTOMER_1","status":"COMPLETED","created_at":"2026-08-04T12:00:00Z"}`)
	if err := db.Upsert("payments", "PAY_1", raw); err != nil {
		t.Fatal(err)
	}
	records, err := loadLocalSquareRecords(context.Background(), db, []string{"payments"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != "PAY_1" {
		t.Fatalf("records = %#v", records)
	}
	if !referencesCustomer(records[0].Data, "CUSTOMER_1") {
		t.Fatal("customer reference was not detected")
	}
	if got := recordTime(records[0]); !got.Equal(time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("record time = %v", got)
	}
}

func TestRequestCheckReportsSafetyWithoutSending(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "payment.json")
	if err := os.WriteFile(bodyPath, []byte(`{"idempotency_key":"ORDER_123","amount_money":{"amount":500,"currency":"USD"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newNovelRequestCheckCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--method", "POST", "--path", "/v2/payments", "--body", bodyPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	var result struct {
		Valid       bool   `json:"valid"`
		Safe        bool   `json:"safe_to_send"`
		Mutation    bool   `json:"mutation"`
		Idempotency bool   `json:"idempotency_present"`
		Note        string `json:"note"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if result.Valid || result.Safe || !result.Mutation || !result.Idempotency {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.Note, "did not send") {
		t.Fatalf("missing no-send assurance: %+v", result)
	}
}

func TestRequestCheckAgentOutputReportsComputedSource(t *testing.T) {
	flags := &rootFlags{agent: true, asJSON: true, configPath: filepath.Join(t.TempDir(), "missing.toml")}
	cmd := newNovelRequestCheckCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--method", "GET", "--path", "/v2/locations"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if envelope.Meta["source"] != "computed" {
		t.Fatalf("meta.source = %v, want computed", envelope.Meta["source"])
	}
}

func TestRequestCheckRejectsAbsoluteURL(t *testing.T) {
	cmd := newNovelRequestCheckCmd(&rootFlags{})
	cmd.SetArgs([]string{"--method", "GET", "--path", "https://connect.squareup.com/v2/locations"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "relative API path") {
		t.Fatalf("error = %v", err)
	}
}

func TestContainsKeyNested(t *testing.T) {
	value := map[string]any{"request": map[string]any{"Idempotency_Key": "abc"}}
	if !containsKey(value, "idempotency_key") {
		t.Fatal("nested key was not detected case-insensitively")
	}
}
