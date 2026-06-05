// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSplitSnapCode(t *testing.T) {
	tests := []struct {
		name                            string
		code                            string
		wantHTTP, wantService, wantCase string
		wantErr                         bool
	}{
		{name: "bank transfer error", code: "4001801", wantHTTP: "400", wantService: "18", wantCase: "01"},
		{name: "success", code: "2001800", wantHTTP: "200", wantService: "18", wantCase: "00"},
		{name: "balance conflict", code: "4091100", wantHTTP: "409", wantService: "11", wantCase: "00"},
		{name: "too short", code: "40018", wantErr: true},
		{name: "non-digit", code: "40018xx", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s, c, err := splitSnapCode(tt.code)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.code)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if h != tt.wantHTTP || s != tt.wantService || c != tt.wantCase {
				t.Fatalf("got %s/%s/%s want %s/%s/%s", h, s, c, tt.wantHTTP, tt.wantService, tt.wantCase)
			}
		})
	}
}

func TestBuildExplainResult(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		wantErr     bool
		wantMessage string
		wantService string
		wantOutcome string
		wantFixSub  string
	}{
		{
			name:        "bank transfer invalid field",
			code:        "4001801",
			wantMessage: "Invalid Field Format {field name}",
			wantService: "Bank Transfer",
			wantOutcome: "error",
			wantFixSub:  "field's format",
		},
		{
			name:        "success bank transfer",
			code:        "2001800",
			wantMessage: "Successful",
			wantService: "Bank Transfer",
			wantOutcome: "success",
		},
		{
			name:        "balance inquiry conflict",
			code:        "4091100",
			wantMessage: "Conflict",
			wantService: "Balance Inquiry",
			wantOutcome: "error",
			wantFixSub:  "X-EXTERNAL-ID",
		},
		{
			name:        "token expired",
			code:        "4017301",
			wantMessage: "Invalid Token (B2B)",
			wantService: "Access Token (B2B)",
			wantOutcome: "error",
			wantFixSub:  "snap token --mint",
		},
		{
			name:        "signature rejected",
			code:        "4011800",
			wantMessage: "Unauthorized [reason]",
			wantOutcome: "error",
			wantFixSub:  "snap sign --debug",
		},
		{
			name:    "unknown case code",
			code:    "4009999",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := buildExplainResult(tt.code)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.code)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Message != tt.wantMessage {
				t.Errorf("message = %q want %q", res.Message, tt.wantMessage)
			}
			if tt.wantService != "" && res.Service != tt.wantService {
				t.Errorf("service = %q want %q", res.Service, tt.wantService)
			}
			if res.Outcome != tt.wantOutcome {
				t.Errorf("outcome = %q want %q", res.Outcome, tt.wantOutcome)
			}
			if tt.wantFixSub != "" && !contains(res.Fix, tt.wantFixSub) {
				t.Errorf("fix = %q want substring %q", res.Fix, tt.wantFixSub)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
