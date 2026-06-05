// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSnapBuildTransferBody(t *testing.T) {
	b := buildTransferBody("150000.00", "", "bca", "1234567890", "Jane Doe", "ref-1", "mer_src", "jane@example.com", "salary")

	if got := b["beneficiaryAccountNo"]; got != "1234567890" {
		t.Errorf("beneficiaryAccountNo = %v, want 1234567890", got)
	}
	if got := b["beneficiaryBankCode"]; got != "bca" {
		t.Errorf("beneficiaryBankCode = %v, want bca", got)
	}
	if got := b["beneficiaryAccountName"]; got != "Jane Doe" {
		t.Errorf("beneficiaryAccountName = %v, want Jane Doe", got)
	}
	if got := b["partnerReferenceNo"]; got != "ref-1" {
		t.Errorf("partnerReferenceNo = %v, want ref-1", got)
	}
	amount, ok := b["amount"].(map[string]any)
	if !ok {
		t.Fatalf("amount is not an object: %T", b["amount"])
	}
	if amount["value"] != "150000.00" {
		t.Errorf("amount.value = %v, want 150000.00", amount["value"])
	}
	if amount["currency"] != "IDR" {
		t.Errorf("amount.currency = %v, want IDR (default)", amount["currency"])
	}
	if b["beneficiaryEmail"] != "jane@example.com" {
		t.Errorf("beneficiaryEmail = %v", b["beneficiaryEmail"])
	}
	if b["customerReference"] != "salary" {
		t.Errorf("customerReference = %v", b["customerReference"])
	}
}

func TestSnapAutoRefGenerates(t *testing.T) {
	if got := snapAutoRef("explicit"); got != "explicit" {
		t.Errorf("snapAutoRef passthrough = %v, want explicit", got)
	}
	gen := snapAutoRef("")
	if !strings.HasPrefix(gen, "pp-") {
		t.Errorf("snapAutoRef autogen = %q, want pp- prefix", gen)
	}
	if snapAutoRef("") == "" {
		t.Error("snapAutoRef autogen returned empty")
	}
}

func TestSnapResponseOK(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
		want   bool
	}{
		{"success 200 code", 200, "2002400", true},
		{"success empty code", 200, "", true},
		{"success 201", 201, "2002400", true},
		{"accepted 202 http+code", 202, "2022400", true},
		{"accepted 202 code on 200", 200, "2022400", true},
		{"accepted 202 empty code", 202, "", true},
		{"bad responseCode 400 family", 200, "4002401", false},
		{"http error 500", 500, "5002400", false},
		{"http 200 but 5xx code", 200, "5002400", false},
		{"http error empty code", 502, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := snapResponseOK(tt.status, tt.code); got != tt.want {
				t.Errorf("snapResponseOK(%d, %q) = %v, want %v", tt.status, tt.code, got, tt.want)
			}
		})
	}
}

func TestSnapResponseFields(t *testing.T) {
	code, msg := snapResponseFields([]byte(`{"responseCode":"2002400","responseMessage":"Successful"}`))
	if code != "2002400" || msg != "Successful" {
		t.Errorf("snapResponseFields = (%q, %q), want (2002400, Successful)", code, msg)
	}
}

func TestSnapTransferDryRunPrints(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newSnapTransferCmd(flags)
	cmd.SetArgs([]string{"--amount", "150000.00", "--bank", "bca", "--account", "1234567890", "--name", "Jane Doe"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run Execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "would POST /v1.0/transfer-interbank") {
		t.Errorf("dry-run output missing 'would POST', got: %q", out.String())
	}
}

func TestSnapVaUpdateDryRunPrintsPut(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newSnapVaUpdateCmd(flags)
	cmd.SetArgs([]string{"--partner-service-id", "88008800", "--customer-no", "9876543210"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run Execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "would PUT /v1.0/transfer-va/update-va") {
		t.Errorf("dry-run output missing 'would PUT', got: %q", out.String())
	}
}
