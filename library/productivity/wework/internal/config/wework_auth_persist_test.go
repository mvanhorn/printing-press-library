package config

import (
	"encoding/base64"
	"testing"
	"time"
)

func makeJWT(exp int64) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + itoa(exp) + `}`))
	return "aaa." + payload + ".bbb"
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestJWTExpiry(t *testing.T) {
	want := int64(1893456000) // 2030-01-01
	got := JWTExpiry(makeJWT(want))
	if got.Unix() != want {
		t.Fatalf("JWTExpiry: got %d, want %d", got.Unix(), want)
	}
}

func TestJWTExpiryInvalid(t *testing.T) {
	cases := []string{"", "not-a-jwt", "a.b", "opaque-token-value"}
	for _, c := range cases {
		if got := JWTExpiry(c); !got.IsZero() {
			t.Errorf("JWTExpiry(%q): expected zero time, got %v", c, got)
		}
	}
}

func TestComposedAuthStatus(t *testing.T) {
	c := &Config{WeworkToken: makeJWT(4102444800), WeworkUuid: "u", WeworkMemberType: "3"}
	hasT, hasU, hasM, exp := c.ComposedAuthStatus()
	if !hasT || !hasU || !hasM {
		t.Fatalf("expected all present, got token=%v uuid=%v member=%v", hasT, hasU, hasM)
	}
	if exp.IsZero() || exp.Before(time.Now()) {
		t.Fatalf("expected future expiry, got %v", exp)
	}
	empty := &Config{}
	if hasT, hasU, hasM, _ := empty.ComposedAuthStatus(); hasT || hasU || hasM {
		t.Fatalf("expected none present on empty config")
	}
}
