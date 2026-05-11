package auth

import (
	"testing"
	"time"
)

func TestExpiryNearOrPast(t *testing.T) {
	tests := []struct {
		name   string
		expiry time.Time
		slack  time.Duration
		want   bool
	}{
		{"zero time → treated as expired", time.Time{}, 60 * time.Second, true},
		{"expired one minute ago", time.Now().Add(-time.Minute), 0, true},
		{"expires in slack window", time.Now().Add(30 * time.Second), 60 * time.Second, true},
		{"expires beyond slack", time.Now().Add(10 * time.Minute), 60 * time.Second, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpiryNearOrPast(tt.expiry, tt.slack)
			if got != tt.want {
				t.Fatalf("ExpiryNearOrPast(%v, %v) = %v, want %v", tt.expiry, tt.slack, got, tt.want)
			}
		})
	}
}

func TestExtractFacilities(t *testing.T) {
	// Synthetic JWT with crafted payload claims. Header and signature segments
	// are placeholders since ParseJWTClaims only decodes the payload (segment 1).
	tests := []struct {
		name  string
		jwt   string
		want  int
		first string
	}{
		{
			name:  "two facilities from L: and A: groups, deduplicated",
			jwt:   "h." + base64URL(`{"cognito:groups":["L:facility-a:AA","A:facility-a:AA","L:other-spa:AA"]}`) + ".s",
			want:  2,
			first: "facility-a",
		},
		{
			name: "no groups claim returns empty",
			jwt:  "h." + base64URL(`{"sub":"abc"}`) + ".s",
			want: 0,
		},
		{
			name: "USR groups are ignored",
			jwt:  "h." + base64URL(`{"cognito:groups":["USR:foo:SELF","USR:bar:EMAIL"]}`) + ".s",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractFacilities(tt.jwt)
			if err != nil {
				t.Fatalf("ExtractFacilities returned error: %v", err)
			}
			if len(got) != tt.want {
				t.Fatalf("got %v facilities, want %d", got, tt.want)
			}
			if tt.first != "" && got[0] != tt.first {
				t.Fatalf("first facility = %q, want %q", got[0], tt.first)
			}
		})
	}
}

func TestParseJWTClaims_InvalidShape(t *testing.T) {
	if _, err := ParseJWTClaims("only-one-segment"); err == nil {
		t.Fatalf("expected error for single-segment JWT")
	}
	if _, err := ParseJWTClaims("notbase64.{}.x"); err == nil {
		t.Fatalf("expected error for non-base64 header segment surrogate")
	}
}

// base64URL is a tiny helper for test fixtures — avoid pulling in encoding/base64
// helpers in test bodies for readability.
func base64URL(s string) string {
	return base64URLEncode([]byte(s))
}
