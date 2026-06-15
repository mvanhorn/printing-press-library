// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    refParts
		wantErr bool
	}{
		{
			name: "field",
			raw:  "op://Production/GitHub/token",
			want: refParts{Raw: "op://Production/GitHub/token", Vault: "Production", Item: "GitHub", Field: "token"},
		},
		{
			name: "section field",
			raw:  "op://Production/GitHub/API/token?attribute=value",
			want: refParts{Raw: "op://Production/GitHub/API/token?attribute=value", Vault: "Production", Item: "GitHub", Section: "API", Field: "token", Query: "attribute=value"},
		},
		{
			name:    "not reference",
			raw:     "Production/GitHub/token",
			wantErr: true,
		},
		{
			name:    "missing field",
			raw:     "op://Production/GitHub",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRef(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseRef(%q) returned nil error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRef(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("parseRef(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDenyRefBlocksCardValues(t *testing.T) {
	tests := []string{
		"op://Vault/Card/number",
		"op://Vault/Payments/cvv",
		"op://Vault/Credit Card/expiry",
		"op://Vault/Payments/pan",
		"op://Vault/Bank/security_code",
	}
	for _, raw := range tests {
		ref, err := parseRef(raw)
		if err != nil {
			t.Fatalf("parseRef(%q) error = %v", raw, err)
		}
		if denied, _ := denyRef(ref); !denied {
			t.Fatalf("denyRef(%q) = false, want true", raw)
		}
	}
}

func TestDenyRefBlocksProductionValues(t *testing.T) {
	tests := []string{
		"op://Production/API/token",
		"op://ProductionVault/deploy-key/token",
		"op://Vault/prodapi/token",
		"op://Vault/App/prodserver/token",
	}
	for _, raw := range tests {
		ref, err := parseRef(raw)
		if err != nil {
			t.Fatalf("parseRef(%q) error = %v", raw, err)
		}
		if denied, _ := denyRef(ref); !denied {
			t.Fatalf("denyRef(%q) = false, want true", ref.Raw)
		}
	}
}

func TestDenyRefAllowsNonCardAccountReferences(t *testing.T) {
	tests := []string{
		"op://Work/service-account/key",
		"op://Corp/aws-account-id/value",
		"op://Personal/google-account/password",
	}
	for _, raw := range tests {
		ref, err := parseRef(raw)
		if err != nil {
			t.Fatalf("parseRef(%q) error = %v", raw, err)
		}
		if denied, reason := denyRef(ref); denied {
			t.Fatalf("denyRef(%q) = true (%s), want false", ref.Raw, reason)
		}
	}
}

func TestDenyRefAllowsNonProductionProdSubstrings(t *testing.T) {
	tests := []string{
		"op://Work/product-catalog/api-key",
		"op://Work/productivity-app/token",
		"op://Work/producer-api/token",
	}
	for _, raw := range tests {
		ref, err := parseRef(raw)
		if err != nil {
			t.Fatalf("parseRef(%q) error = %v", raw, err)
		}
		if denied, reason := denyRef(ref); denied {
			t.Fatalf("denyRef(%q) = true (%s), want false", ref.Raw, reason)
		}
	}
}

func TestDecodeVerifyJSONMatchesCommandTokens(t *testing.T) {
	var detail opItemDetail
	if err := decodeVerifyJSON([]string{"item", "get", "abc", "--vault", "vault list backups"}, &detail); err != nil {
		t.Fatalf("decodeVerifyJSON item get error = %v", err)
	}
	if detail.ID == "" {
		t.Fatalf("decodeVerifyJSON item get returned empty detail")
	}

	var vaults []opVaultSummary
	if err := decodeVerifyJSON([]string{"vault", "list", "--format", "json"}, &vaults); err != nil {
		t.Fatalf("decodeVerifyJSON vault list error = %v", err)
	}
	if len(vaults) != 1 {
		t.Fatalf("decodeVerifyJSON vault list returned %d vaults, want 1", len(vaults))
	}
}

func TestRefsInTextDeduplicatesReferences(t *testing.T) {
	got := refsInText("use op://Vault/App/token and op://Vault/App/token, plus op://Vault/App/user")
	want := []string{"op://Vault/App/token", "op://Vault/App/user"}
	if len(got) != len(want) {
		t.Fatalf("refsInText returned %d refs, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("refsInText[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
