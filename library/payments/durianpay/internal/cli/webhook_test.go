// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/snap"
)

// TestWebhookVerifyLegacyEndToEnd drives the assembled webhook command (which
// is not yet wired into root.go) to confirm legacy HMAC verification, exit
// behavior, and the --json envelope.
func TestWebhookVerifyLegacyEndToEnd(t *testing.T) {
	const (
		id     = "dis_e2e"
		amount = "5000.00"
		apiKey = "dp_e2e_key"
	)
	t.Setenv("DURIANPAY_API_KEY", apiKey)
	sig := snap.LegacyCompletionSignature(id, amount, apiKey)

	t.Run("valid exits ok", func(t *testing.T) {
		flags := &rootFlags{asJSON: true}
		cmd := newWebhookCmd(flags)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"verify", "--legacy", "--id", id, "--amount", amount, "--signature", sig})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("valid signature returned error: %v", err)
		}
		if !strings.Contains(out.String(), `"valid": true`) {
			t.Fatalf("output = %q, want valid:true", out.String())
		}
	})

	t.Run("invalid returns error", func(t *testing.T) {
		flags := &rootFlags{}
		cmd := newWebhookCmd(flags)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"verify", "--legacy", "--id", id, "--amount", amount, "--signature", "deadbeef"})
		if err := cmd.Execute(); err == nil {
			t.Fatalf("invalid signature returned nil error, want non-nil")
		}
	})

	t.Run("missing inputs is usage error", func(t *testing.T) {
		flags := &rootFlags{}
		cmd := newWebhookCmd(flags)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"verify", "--legacy", "--id", id})
		err := cmd.Execute()
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("missing amount: err=%v exit=%d, want usage error (exit 2)", err, ExitCode(err))
		}
	})
}

func TestLegacyCompletionSignatureVector(t *testing.T) {
	const (
		id     = "dis_test"
		amount = "10000.00"
		apiKey = "dp_test_secret_key"
	)
	// Independently compute the expected HMAC-SHA256(id|amount) with the key.
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(id + "|" + amount))
	want := hex.EncodeToString(mac.Sum(nil))

	got := snap.LegacyCompletionSignature(id, amount, apiKey)
	if got != want {
		t.Fatalf("LegacyCompletionSignature = %q, want %q", got, want)
	}

	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{name: "exact match", candidate: want, want: true},
		{name: "uppercase match", candidate: upper(want), want: true},
		{name: "tampered", candidate: want[:len(want)-1] + "0", want: false},
		{name: "wrong length", candidate: want[:10], want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if hmacEqual(got, tt.candidate) != tt.want {
				t.Errorf("hmacEqual(%q) = %v, want %v", tt.candidate, !tt.want, tt.want)
			}
		})
	}
}

func upper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}
